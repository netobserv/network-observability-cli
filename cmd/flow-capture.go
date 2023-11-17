package cmd

import (
	"bufio"
	"encoding/json"
	"net"
	"net/textproto"
	"os"
	"os/exec"
	"sort"
	"time"

	"github.com/jpillora/sizestr"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/spf13/cobra"

	"github.com/fatih/color"
	"github.com/rodaine/table"
)

const (
	flowsToShow = 40
)

var flowCmd = &cobra.Command{
	Use:   "get-flows",
	Short: "",
	Long:  "",
	Run:   runFlowCapture,
}

var lastFlows = []config.GenericMap{}

func runFlowCapture(cmd *cobra.Command, args []string) {
	wg.Add(len(addresses))
	for i, _ := range addresses {
		go runFlowCaptureOnAddr(addresses[i], nodes[i])
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
			go manageFlowTable(line)
		}
	}
}

func manageFlowTable(line []byte) {
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
	if len(lastFlows) > flowsToShow {
		lastFlows = lastFlows[len(lastFlows)-flowsToShow:]
	}

	// don't refresh terminal too often to avoid blinking
	now := time.Now()
	if int(now.Sub(lastRefresh)) > int(maxRefreshRate) {
		lastRefresh = now

		// clear terminal to render table properly
		c := exec.Command("clear")
		c.Stdout = os.Stdout
		c.Run()

		log.Infof("Running network-observability-cli as Flow Capture\nLog level: %s\nFilters: %s\n", logLevel, filter)

		// recreate table from scratch
		headerFmt := color.New(color.BgHiBlue, color.Bold).SprintfFunc()
		columnFmt := color.New(color.FgHiYellow).SprintfFunc()
		tbl := table.New(
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
			"DropBytes",
			"Packets",
			"DropPackets",
			"DropState",
			"DropCause",
			"DnsId",
			"DnsLatencyMs",
			"DnsRCode",
			"DnsErrno",
			"RTT",
		)
		tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)

		// append most recent rows
		for _, flow := range lastFlows {
			tbl.AddRow(
				time.UnixMilli(int64(flow["TimeFlowEndMs"].(float64))).Format("15:04:05.000000"),
				flow["SrcAddr"],
				flow["SrcPort"],
				flow["DstAddr"],
				flow["DstPort"],
				flow["FlowDirection"],
				flow["Interface"],
				flow["Proto"],
				flow["Dscp"],
				ToPacketCount(flow, "Bytes"),
				ToPacketCount(flow, "PktDropBytes"),
				flow["Packets"],
				flow["PktDropPackets"],
				flow["PktDropLatestState"],
				flow["PktDropLatestDropCause"],
				flow["DnsId"],
				ToDuration(flow, "DnsLatencyMs", time.Millisecond),
				flow["DnsRCode"],
				flow["DnsErrno"],
				ToDuration(flow, "TimeFlowRttNs", time.Nanosecond),
			)
		}

		// print table
		tbl.Print()
	}

	// unlock
	mutex.Unlock()
}

func ToPacketCount(genericMap config.GenericMap, fieldName string) interface{} {
	v, ok := genericMap[fieldName]
	if ok {
		return sizestr.ToString(int64(v.(float64)))
	}
	return nil
}

func ToDuration(genericMap config.GenericMap, fieldName string, factor time.Duration) interface{} {
	v, ok := genericMap[fieldName]
	if ok {
		return (time.Duration(int64(v.(float64))) * factor).String()
	}
	return nil
}
