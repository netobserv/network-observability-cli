package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/eiannone/keyboard"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/write/grpc"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/write/grpc/genericmap"
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
	flowsToShow = 35
	regexes     = []string{}
	lastFlows   = []config.GenericMap{}

	rawDisplay      = "Raw"
	standardDisplay = "Standard"
	pktDropDisplay  = "PktDrop"
	dnsDisplay      = "DNS"
	rttDisplay      = "RTT"
	display         = []string{pktDropDisplay, dnsDisplay, rttDisplay}

	noEnrichment       = "None"
	zoneEnrichment     = "Zone"
	hostEnrichment     = "Host"
	ownerEnrichment    = "Owner"
	resourceEnrichment = "Resource"
	enrichement        = []string{resourceEnrichment}
)

func runFlowCapture(cmd *cobra.Command, args []string) {
	go scanner()

	wg := sync.WaitGroup{}
	wg.Add(len(ports))
	for i := range ports {
		go func(idx int) {
			defer wg.Done()
			runFlowCaptureOnAddr(ports[idx], nodes[idx])
		}(i)
	}
	wg.Wait()
}

func runFlowCaptureOnAddr(port int, filename string) {
	if len(filename) > 0 {
		log.Infof("Starting Flow Capture for %s...", filename)
	} else {
		log.Infof("Starting Flow Capture...")
		filename = strings.Replace(
			time.Now().UTC().Format(time.RFC3339),
			":", "", -1) // get rid of offensive colons
	}

	var f *os.File
	err := os.MkdirAll("./output/flow/", 0700)
	if err != nil {
		log.Errorf("Create directory failed: %v", err.Error())
		log.Fatal(err)
	}
	f, err = os.Create("./output/flow/" + filename + ".json")
	if err != nil {
		log.Errorf("Create file %s failed: %v", filename, err.Error())
		log.Fatal(err)
	}
	defer f.Close()

	flowPackets := make(chan *genericmap.Flow, 100)
	collector, err := grpc.StartCollector(port, flowPackets)
	if err != nil {
		log.Error("StartCollector failed:", err.Error())
		log.Fatal(err)
	}
	go func() {
		<-utils.ExitChannel()
		close(flowPackets)
		collector.Close()
	}()
	for fp := range flowPackets {
		go manageFlowsDisplay(fp.GenericMap.Value)
		// append new line between each record to read file easilly
		_, err = f.Write(append(fp.GenericMap.Value, []byte(",\n")...))
		if err != nil {
			log.Fatal(err)
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
		// regexes may change during the render so we make a copy first
		rCopy := make([]string, len(regexes))
		copy(rCopy[:], regexes)
		filtered := []config.GenericMap{}
		for _, flow := range lastFlows {
			match := true
			for i := range rCopy {
				ok, _ := regexp.MatchString(rCopy[i], fmt.Sprintf("%v", flow))
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

func toSize(fieldName string) int {
	switch fieldName {
	case "SrcName", "DstName", "SrcOwnerName", "DstOwnerName", "SrcHostName", "DstHostName":
		return 45
	case "DropCause", "SrcAddr", "DstAddr":
		return 40
	case "DropState":
		return 20
	case "Time", "Interface", "SrcZone", "DstZone":
		return 16
	case "DropBytes", "DropPackets", "SrcOwnerType", "DstOwnerType":
		return 12
	case "Dir":
		return 10
	case "Dscp", "SrcType", "DstType":
		return 8
	default:
		return 6
	}
}

func updateTable() {
	// don't refresh terminal too often to avoid blinking
	now := time.Now()
	if int(now.Sub(lastRefresh)) > int(maxRefreshRate) {
		lastRefresh = now

		// clear terminal to render table properly
		fmt.Print("\x1bc")
		// no wrap
		fmt.Print("\033[?7l")

		fmt.Print("Running network-observability-cli as Flow Capture\n")
		fmt.Printf("Log level: %s\n", logLevel)
		fmt.Printf("Collection filters: %s\n", filter)
		fmt.Printf("Showing last: %d Use Up / Down keyboard arrows to increase / decrease limit\n", flowsToShow)
		fmt.Printf("Display: %s	Use Left / Right keyboard arrows to cycle views\n", strings.Join(display, ","))
		fmt.Printf("Enrichment: %s	Use Page Up / Page Down keyboard keys to cycle enrichment scopes\n", strings.Join(enrichement, ","))

		if slices.Contains(display, rawDisplay) {
			fmt.Print("Raw flow logs:\n")
			for _, flow := range lastFlows {
				fmt.Printf("%v\n", flow)
			}
			fmt.Printf("%s\n", strings.Repeat("-", 500))
		} else {
			// recreate table from scratch
			headerFmt := color.New(color.BgHiBlue, color.Bold).SprintfFunc()
			columnFmt := color.New(color.FgHiYellow).SprintfFunc()

			// main field, always show the end time
			cols := []string{
				"Time",
			}

			// enrichment fields
			if !slices.Contains(enrichement, noEnrichment) {
				if slices.Contains(enrichement, zoneEnrichment) {
					cols = append(cols,
						"SrcZone",
						"DstZone",
					)
				}

				if slices.Contains(enrichement, hostEnrichment) {
					cols = append(cols,
						"SrcHostName",
						"DstHostName",
					)
				}

				if slices.Contains(enrichement, ownerEnrichment) {
					cols = append(cols,
						"SrcOwnerName",
						"SrcOwnerType",
						"DstOwnerName",
						"DstOwnerType",
					)
				}

				if slices.Contains(enrichement, resourceEnrichment) {
					cols = append(cols,
						"SrcName",
						"SrcType",
						"DstName",
						"DstType",
					)
				}
			} else {
				cols = append(cols,
					"SrcAddr",
					"SrcPort",
					"DstAddr",
					"DstPort",
				)
			}

			// standard / feature fields
			if !slices.Contains(display, standardDisplay) {
				if slices.Contains(display, pktDropDisplay) {
					cols = append(cols,
						"DropBytes",
						"DropPackets",
						"DropState",
						"DropCause",
					)
				}

				if slices.Contains(display, dnsDisplay) {
					cols = append(cols,
						"DnsId",
						"DnsLatency",
						"DnsRCode",
						"DnsErrno",
					)
				}

				if slices.Contains(display, rttDisplay) {
					cols = append(cols,
						"RTT",
					)
				}
			} else {
				cols = append(cols,
					"Dir",
					"Interface",
					"Proto",
					"Dscp",
					"Bytes",
					"Packets",
				)
			}

			colInterfaces := make([]interface{}, len(cols))
			for i, c := range cols {
				colInterfaces[i] = c
			}
			tbl := table.New(colInterfaces...)
			tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)

			// append most recent rows
			for _, flow := range lastFlows {
				tbl.AddRow(ToTableRow(flow, cols)...)
			}

			// inserting empty row ensure minimum column sizes
			emptyRow := []interface{}{}
			for _, col := range cols {
				emptyRow = append(emptyRow, strings.Repeat("-", toSize(col)))
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
			if slices.Contains(display, pktDropDisplay) && slices.Contains(display, dnsDisplay) && slices.Contains(display, rttDisplay) {
				display = []string{rawDisplay}
			} else if slices.Contains(display, rawDisplay) {
				display = []string{standardDisplay}
			} else if slices.Contains(display, standardDisplay) {
				display = []string{pktDropDisplay}
			} else if slices.Contains(display, pktDropDisplay) {
				display = []string{dnsDisplay}
			} else if slices.Contains(display, dnsDisplay) {
				display = []string{rttDisplay}
			} else {
				display = []string{pktDropDisplay, dnsDisplay, rttDisplay}
			}
		} else if key == keyboard.KeyArrowLeft {
			if slices.Contains(display, pktDropDisplay) && slices.Contains(display, dnsDisplay) && slices.Contains(display, rttDisplay) {
				display = []string{rttDisplay}
			} else if slices.Contains(display, rttDisplay) {
				display = []string{dnsDisplay}
			} else if slices.Contains(display, dnsDisplay) {
				display = []string{pktDropDisplay}
			} else if slices.Contains(display, pktDropDisplay) {
				display = []string{standardDisplay}
			} else if slices.Contains(display, standardDisplay) {
				display = []string{rawDisplay}
			} else {
				display = []string{pktDropDisplay, dnsDisplay, rttDisplay}
			}
		} else if key == keyboard.KeyPgup {
			if slices.Contains(enrichement, zoneEnrichment) && slices.Contains(enrichement, hostEnrichment) && slices.Contains(enrichement, ownerEnrichment) {
				enrichement = []string{noEnrichment}
			} else if slices.Contains(enrichement, noEnrichment) {
				enrichement = []string{resourceEnrichment}
			} else if slices.Contains(enrichement, resourceEnrichment) {
				enrichement = []string{ownerEnrichment}
			} else if slices.Contains(enrichement, ownerEnrichment) {
				enrichement = []string{hostEnrichment}
			} else if slices.Contains(enrichement, hostEnrichment) {
				enrichement = []string{zoneEnrichment}
			} else {
				enrichement = []string{zoneEnrichment, hostEnrichment, ownerEnrichment, resourceEnrichment}
			}
		} else if key == keyboard.KeyPgdn {
			if slices.Contains(enrichement, zoneEnrichment) && slices.Contains(enrichement, hostEnrichment) && slices.Contains(enrichement, ownerEnrichment) {
				enrichement = []string{zoneEnrichment}
			} else if slices.Contains(enrichement, zoneEnrichment) {
				enrichement = []string{hostEnrichment}
			} else if slices.Contains(enrichement, hostEnrichment) {
				enrichement = []string{ownerEnrichment}
			} else if slices.Contains(enrichement, ownerEnrichment) {
				enrichement = []string{resourceEnrichment}
			} else if slices.Contains(enrichement, resourceEnrichment) {
				enrichement = []string{noEnrichment}
			} else {
				enrichement = []string{zoneEnrichment, hostEnrichment, ownerEnrichment, resourceEnrichment}
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
