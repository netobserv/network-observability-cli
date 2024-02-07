package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"net/textproto"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/eiannone/keyboard"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/rodaine/table"
	"github.com/spf13/cobra"

	"github.com/fatih/color"
)

var flowCmd = &cobra.Command{
	Use:   "get-flows",
	Short: "",
	Long:  "",
	Run:   runFlowCapture,
}

var (
	regexes     = []string{}
	flowsToShow = 40
	raw         = "Raw"
	standard    = "Standard"
	pktDrop     = "PktDrop"
	dns         = "DNS"
	rtt         = "RTT"
	display     = []string{pktDrop, dns, rtt}
	lastFlows   = []config.GenericMap{}
)

func runFlowCapture(cmd *cobra.Command, args []string) {
	go scanner()

	wg := sync.WaitGroup{}
	wg.Add(len(addresses))
	for i := range addresses {
		go func(idx int) {
			defer wg.Done()
			runFlowCaptureOnAddr(addresses[idx], nodes[idx])
		}(i)
	}
	wg.Wait()
}

func runFlowCaptureOnAddr(addr string, filename string) {
	log.Infof("Starting Flow Capture for %s...", filename)

	tcpServer, err := net.ResolveTCPAddr("tcp", addr)

	if err != nil {
		log.Error("ResolveTCPAddr failed:", err.Error())
		log.Fatal(err)
	}
	conn, err := net.DialTCP("tcp", nil, tcpServer)
	if err != nil {
		log.Error("Dial failed:", err.Error())
		log.Fatal(err)
	}
	reader := bufio.NewReader(conn)
	tp := textproto.NewReader(reader)
	var f *os.File
	err = os.MkdirAll("./output/flow/", 0700)
	if err != nil {
		log.Errorf("Create directory failed: %v", err.Error())
		log.Fatal(err)
	}
	f, err = os.Create("./output/flow/" + filename)
	if err != nil {
		log.Errorf("Create file %s failed: %v", filename, err.Error())
		log.Fatal(err)
	}
	defer CleanupCapture(conn, f)
	for {
		// read one line (ended with \n or \r\n)
		line, err := tp.ReadLineBytes()
		if err != nil {
			log.Error("Read line failed:", err.Error())
		} else {
			// append new line between each record to read file easilly
			_, err = f.Write(append(line, []byte("\n")...))
			if err != nil {
				log.Fatal(err)
			}
			go manageFlowsDisplay(line)
		}
	}
}

func manageFlowsDisplay(line []byte) {
	genericMap := config.GenericMap{}
	err := json.Unmarshal(line, &genericMap)
	if err != nil {
		log.Error("Error while parsing json", err)
		return
	}

	// lock since we are updating lastFlows concurrently
	mutex.Lock()

	lastFlows = append(lastFlows, genericMap)
	sort.Slice(lastFlows, func(i, j int) bool {
		return lastFlows[i]["TimeFlowEndMs"].(float64) < lastFlows[j]["TimeFlowEndMs"].(float64)
	})
	if len(regexes) > 0 {
		filtered := []config.GenericMap{}
		for _, flow := range lastFlows {
			match := true
			for i := range regexes {
				ok, _ := regexp.MatchString(regexes[i], fmt.Sprintf("%v", flow))
				match = match && ok
				if !match {
					break
				}
			}
			if match {
				filtered = append(filtered, flow)
			}
		}
		lastFlows = filtered
	}
	if len(lastFlows) > flowsToShow {
		lastFlows = lastFlows[len(lastFlows)-flowsToShow:]
	}
	updateTable()

	// unlock
	mutex.Unlock()
}

func updateTable() {
	// don't refresh terminal too often to avoid blinking
	now := time.Now()
	if int(now.Sub(lastRefresh)) > int(maxRefreshRate) {
		lastRefresh = now

		// clear terminal to render table properly
		c := exec.Command("clear")
		c.Stdout = os.Stdout
		err := c.Run()
		if err != nil {
			log.Fatal(err)
		}

		fmt.Print("Running network-observability-cli as Flow Capture\n")
		fmt.Printf("Log level: %s\n", logLevel)
		fmt.Printf("Collection filters: %s\n", filter)
		fmt.Printf("Showing last: %d Use Up / Down keyboard arrows to increase / decrease limit\n", flowsToShow)
		fmt.Printf("Display: %s	Use Left / Right keyboard arrows to cycle views\n", strings.Join(display, ","))

		// recreate table from scratch
		headerFmt := color.New(color.BgHiBlue, color.Bold).SprintfFunc()
		columnFmt := color.New(color.FgHiYellow).SprintfFunc()
		cols := []interface{}{
			"Time",
			"SrcAddr",
			"SrcPort",
			"DstAddr",
			"DstPort",
			"Dir",
			"Interface",
			"Proto",
			"Dscp",
			"Bytes",
			"Packets",
		}

		if slices.Contains(display, pktDrop) {
			cols = append(cols, []interface{}{
				"DropBytes",
				"DropPackets",
				"DropState",
				"DropCause",
			}...)
		}

		if slices.Contains(display, dns) {
			cols = append(cols, []interface{}{
				"DnsId",
				"DnsLatencyMs",
				"DnsRCode",
				"DnsErrno",
			}...)
		}

		if slices.Contains(display, rtt) {
			cols = append(cols, []interface{}{
				"RTT",
			}...)
		}

		if slices.Contains(display, raw) {
			fmt.Print("Raw flow logs:\n")
			for _, flow := range lastFlows {
				fmt.Printf("%v\n", flow)
			}
			fmt.Printf("%s\n", strings.Repeat("-", 500))
		} else {
			tbl := table.New(cols...)
			tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)

			// append most recent rows
			for _, flow := range lastFlows {
				row := []interface{}{
					time.UnixMilli(int64(flow["TimeFlowEndMs"].(float64))).Format("15:04:05.000000"),
					ToText(flow, "SrcAddr"),
					ToText(flow, "SrcPort"),
					ToText(flow, "DstAddr"),
					ToText(flow, "DstPort"),
					ToDirection(flow, "FlowDirection"),
					ToText(flow, "Interface"),
					ToProto(flow, "Proto"),
					ToDSCP(flow, "Dscp"),
					ToPacketCount(flow, "Bytes"),
					ToText(flow, "Packets"),
				}

				if slices.Contains(display, pktDrop) {
					row = append(row, []interface{}{
						ToPacketCount(flow, "PktDropBytes"),
						ToText(flow, "PktDropPackets"),
						ToText(flow, "PktDropLatestState"),
						ToText(flow, "PktDropLatestDropCause"),
					}...)
				}

				if slices.Contains(display, dns) {
					row = append(row, []interface{}{
						ToText(flow, "DnsId"),
						ToDuration(flow, "DnsLatencyMs", time.Millisecond),
						ToText(flow, "DnsRCode"),
						ToText(flow, "DnsErrno"),
					}...)
				}

				if slices.Contains(display, rtt) {
					row = append(row, []interface{}{
						ToDuration(flow, "TimeFlowRttNs", time.Nanosecond),
					}...)
				}

				tbl.AddRow(row...)
			}

			// inserting empty row ensure minimum column sizes
			emptyRow := []interface{}{
				strings.Repeat("-", 16), // TimeFlowEndMs
				strings.Repeat("-", 16), // SrcAddr
				strings.Repeat("-", 6),  // SrcPort
				strings.Repeat("-", 16), // DstAddr
				strings.Repeat("-", 6),  // DstPort
				strings.Repeat("-", 10), // FlowDirection
				strings.Repeat("-", 16), // Interface
				strings.Repeat("-", 6),  // Proto
				strings.Repeat("-", 8),  // Dscp
				strings.Repeat("-", 6),  // Bytes
				strings.Repeat("-", 6),  // Packets
			}

			if slices.Contains(display, pktDrop) {
				emptyRow = append(emptyRow, []interface{}{
					strings.Repeat("-", 12), // PktDropBytes
					strings.Repeat("-", 12), // PktDropPackets
					strings.Repeat("-", 20), // PktDropLatestState
					strings.Repeat("-", 40), // PktDropLatestDropCause
				}...)
			}

			if slices.Contains(display, dns) {
				emptyRow = append(emptyRow, []interface{}{
					strings.Repeat("-", 6), // DnsId
					strings.Repeat("-", 6), // DnsLatencyMs
					strings.Repeat("-", 6), // DnsRCode
					strings.Repeat("-", 6), // DnsErrno
				}...)
			}

			if slices.Contains(display, rtt) {
				emptyRow = append(emptyRow, []interface{}{
					strings.Repeat("-", 6), // TimeFlowRttNs
				}...)
			}

			tbl.AddRow(emptyRow...)

			// print table
			tbl.Print()
		}

		if len(regexes) > 0 {
			fmt.Printf("Live table filter: %s Press enter to match multiple regexes at once\n", regexes)
		} else {
			fmt.Printf("Type anything to filter incoming flows in view\n")
		}
	}
}

func scanner() {
	if err := keyboard.Open(); err != nil {
		panic(err)
	}
	defer func() {
		_ = keyboard.Close()
	}()

	for {
		char, key, err := keyboard.GetKey()
		if err != nil {
			panic(err)
		}
		if key == keyboard.KeyCtrlC {
			log.Info("Ctrl-C pressed, exiting program.")

			// exit program
			os.Exit(0)
		} else if key == keyboard.KeyArrowUp {
			flowsToShow = flowsToShow + 1
		} else if key == keyboard.KeyArrowDown {
			if flowsToShow > 10 {
				flowsToShow = flowsToShow - 1
			}
		} else if key == keyboard.KeyArrowRight {
			if slices.Contains(display, pktDrop) && slices.Contains(display, dns) && slices.Contains(display, rtt) {
				display = []string{raw}
			} else if slices.Contains(display, raw) {
				display = []string{standard}
			} else if slices.Contains(display, standard) {
				display = []string{pktDrop}
			} else if slices.Contains(display, pktDrop) {
				display = []string{dns}
			} else if slices.Contains(display, dns) {
				display = []string{rtt}
			} else {
				display = []string{pktDrop, dns, rtt}
			}
		} else if key == keyboard.KeyArrowLeft {
			if slices.Contains(display, pktDrop) && slices.Contains(display, dns) && slices.Contains(display, rtt) {
				display = []string{rtt}
			} else if slices.Contains(display, rtt) {
				display = []string{dns}
			} else if slices.Contains(display, dns) {
				display = []string{pktDrop}
			} else if slices.Contains(display, pktDrop) {
				display = []string{standard}
			} else if slices.Contains(display, standard) {
				display = []string{raw}
			} else {
				display = []string{pktDrop, dns, rtt}
			}
		} else if key == keyboard.KeyBackspace || key == keyboard.KeyBackspace2 {
			if len(regexes) > 0 {
				lastIndex := len(regexes) - 1
				if len(regexes[lastIndex]) > 0 {
					regexes[lastIndex] = regexes[lastIndex][:len(regexes[lastIndex])-1]
				} else {
					regexes = regexes[:lastIndex]
				}
			}
		} else if key == keyboard.KeyEnter {
			regexes = append(regexes, "")
		} else {
			if len(regexes) == 0 {
				regexes = []string{string(char)}
			} else {
				lastIndex := len(regexes) - 1
				regexes[lastIndex] = regexes[lastIndex] + string(char)
			}
		}
		lastRefresh = startupTime
		updateTable()
	}
}
