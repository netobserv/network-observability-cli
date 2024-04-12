package cmd

import (
	"os"
	"strings"
	"sync"
	"time"

	"github.com/eiannone/keyboard"
	"github.com/fatih/color"
	"github.com/google/gopacket/layers"
	"github.com/jpillora/sizestr"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
	"github.com/netobserv/netobserv-ebpf-agent/pkg/exporter"
	grpc "github.com/netobserv/netobserv-ebpf-agent/pkg/grpc/packet"
	"github.com/netobserv/netobserv-ebpf-agent/pkg/pbpacket"
	"github.com/rodaine/table"
	"github.com/spf13/cobra"
)

var pktCmd = &cobra.Command{
	Use:   "get-packets",
	Short: "",
	Long:  "",
	Run:   runPacketCapture,
}

type PcapResult struct {
	Name        string
	PacketCount int64
	ByteCount   int64
}

var packets = []PcapResult{}

// Setting Snapshot length to 0 sets it to maximum packet size
var snapshotlen uint32

func runPacketCapture(_ *cobra.Command, _ []string) {
	go packetCaptureScanner()

	wg := sync.WaitGroup{}
	wg.Add(len(ports))
	for i := range ports {
		go func(idx int) {
			defer wg.Done()
			runPacketCaptureOnAddr(ports[idx], nodes[idx])
		}(i)
	}
	wg.Wait()
}

func runPacketCaptureOnAddr(port int, filename string) {
	if len(filename) > 0 {
		log.Infof("Starting Packet Capture for %s...", filename)
	} else {
		log.Infof("Starting Packet Capture...")
		filename = strings.Replace(
			currentTime().UTC().Format(time.RFC3339),
			":", "", -1) // get rid of offensive colons
	}

	var f *os.File
	err := os.MkdirAll("./output/pcap/", 0700)
	if err != nil {
		log.Errorf("Create directory failed: %v", err.Error())
		log.Fatal(err)
	}
	f, err = os.Create("./output/pcap/" + filename + ".pcap")
	if err != nil {
		log.Errorf("Create file %s failed: %v", filename, err.Error())
		log.Fatal(err)
	}
	// write pcap file header
	_, err = f.Write(exporter.GetPCAPFileHeader(snapshotlen, layers.LinkTypeEthernet))
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	flowPackets := make(chan *pbpacket.Packet, 100)
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
		go managePacketsDisplay(PcapResult{Name: filename, ByteCount: int64(len(fp.Pcap.Value)), PacketCount: 1})
		// append new line between each record to read file easilly
		_, err = f.Write(fp.Pcap.Value)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func managePacketsDisplay(result PcapResult) {
	// lock since we are updating results concurrently
	mutex.Lock()

	// find result in array
	found := false
	for i, r := range packets {
		if r.Name == result.Name {
			found = true
			// update existing result
			packets[i].PacketCount += result.PacketCount
			packets[i].ByteCount += result.ByteCount
			break
		}
	}
	if !found {
		packets = append(packets, result)
	}

	// don't refresh terminal too often to avoid blinking
	now := currentTime()
	if int(now.Sub(lastRefresh)) > int(maxRefreshRate) {
		lastRefresh = now
		resetTerminal()

		if outputBuffer == nil {
			log.Infof("Running network-observability-cli as Packet Capture\nLog level: %s\nFilters: %s\n", logLevel, filter)
		}

		// recreate table from scratch
		headerFmt := color.New(color.BgHiBlue, color.Bold).SprintfFunc()
		columnFmt := color.New(color.FgHiYellow).SprintfFunc()
		tbl := table.New(
			"Name",
			"Packets",
			"Bytes",
		)
		if outputBuffer != nil {
			tbl.WithWriter(outputBuffer)
		}
		tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt).WithPadding(10)

		for _, result := range packets {
			tbl.AddRow(
				result.Name,
				result.PacketCount,
				sizestr.ToString(result.ByteCount),
			)
		}

		// print table
		tbl.Print()
	}

	// unlock
	mutex.Unlock()
}

func packetCaptureScanner() {
	if err := keyboard.Open(); err != nil {
		panic(err)
	}
	defer func() {
		_ = keyboard.Close()
	}()

	for {
		_, key, err := keyboard.GetKey()
		if err != nil {
			panic(err)
		}
		if key == keyboard.KeyCtrlC {
			log.Info("Ctrl-C pressed, exiting program.")

			// exit program
			os.Exit(0)
		}
	}
}
