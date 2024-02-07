package cmd

import (
	"net"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/jpillora/sizestr"
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
	NodeName    string
	PacketCount int64
	ByteCount   int64
}

var results = []PcapResult{}

func runPacketCapture(cmd *cobra.Command, args []string) {
	wg := sync.WaitGroup{}
	wg.Add(len(addresses))
	for i := range addresses {
		go func(idx int) {
			defer wg.Done()
			runPacketCaptureOnAddr(addresses[idx], nodes[idx])
		}(i)
	}
	wg.Wait()
}

func runPacketCaptureOnAddr(addr string, filename string) {
	log.Infof("Starting Packet Capture for %s...", filename)

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
	var f *os.File
	err = os.MkdirAll("./output/pcap/", 0700)
	if err != nil {
		log.Errorf("Create directory failed: %v", err.Error())
		log.Fatal(err)
	}
	f, err = os.Create("./output/pcap/" + filename)
	if err != nil {
		log.Errorf("Create file %s failed: %v", filename, err.Error())
		log.Fatal(err)
	}
	defer CleanupCapture(conn, f)
	for {
		received := make([]byte, 65535)
		n, err := conn.Read(received)
		if err != nil {
			log.Error("Read data failed:", err.Error())
			log.Fatal(err)
		}
		_, err = f.Write(received[:n])
		if err != nil {
			log.Fatal(err)
		}
		go managePcapTable(PcapResult{NodeName: filename, PacketCount: 1, ByteCount: int64(n)})
	}
}

func managePcapTable(result PcapResult) {
	// lock since we are updating results concurrently
	mutex.Lock()

	// find result in array
	found := false
	for i, r := range results {
		if r.NodeName == result.NodeName {
			found = true
			// update existing result
			results[i].PacketCount += result.PacketCount
			results[i].ByteCount += result.ByteCount
			break
		}
	}
	if !found {
		results = append(results, result)
	}

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

		log.Infof("Running network-observability-cli as Packet Capture\nLog level: %s\nFilters: %s\n", logLevel, filter)

		// recreate table from scratch
		headerFmt := color.New(color.BgHiBlue, color.Bold).SprintfFunc()
		columnFmt := color.New(color.FgHiYellow).SprintfFunc()
		tbl := table.New(
			"Node Name",
			"Packets",
			"Bytes",
		)
		tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt).WithPadding(10)

		for _, result := range results {
			tbl.AddRow(
				result.NodeName,
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
