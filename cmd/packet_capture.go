package cmd

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/gopacket/layers"
	"github.com/jpillora/sizestr"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/write/grpc"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/write/grpc/genericmap"
	"github.com/ryankurte/go-pcapng"
	"github.com/ryankurte/go-pcapng/types"
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

func runPacketCapture(_ *cobra.Command, _ []string) {
	go scanner()

	captureType = "Packet"
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
	pw, err := pcapng.NewFileWriter("./output/pcap/" + filename + ".pcapng")
	if err != nil {
		log.Errorf("Create file %s failed: %v", filename, err.Error())
		log.Fatal(err)
	}
	// write pcap file header
	so := types.SectionHeaderOptions{
		Comment:     filename,
		Application: "netobserv-cli",
	}
	err = pw.WriteSectionHeader(so)
	if err != nil {
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
		if stopReceived {
			return
		}
		genericMap := config.GenericMap{}
		err := json.Unmarshal(fp.GenericMap.Value, &genericMap)
		if err != nil {
			log.Error("Error while parsing json", err)
			return
		}

		data, ok := genericMap["Data"]
		if ok {
			// clear generic map data
			delete(genericMap, "Data")

			// display as flow async
			go manageFlowsDisplay(genericMap)

			// Get capture timestamp
			ts := time.Unix(int64(genericMap["Time"].(float64)), 0)

			// Decode b64 encoded data
			b, err := base64.StdEncoding.DecodeString(data.(string))
			if err != nil {
				log.Error("Error while decoding data", err)
				return
			}

			// write enriched data as interface
			writeEnrichedData(pw, &genericMap)

			// then append packet to file
			err = pw.WriteEnhancedPacketBlock(0, ts, b, types.EnhancedPacketOptions{})
			if err != nil {
				log.Fatal(err)
			}
		} else {
			// display as flow async
			go manageFlowsDisplay(genericMap)
		}

		// terminate capture if max bytes reached
		totalBytes = totalBytes + int64(len(fp.GenericMap.Value))
		if totalBytes > maxBytes {
			log.Infof("Capture reached %s, exiting now...", sizestr.ToString(maxBytes))
			return
		}

		// terminate capture if max time reached
		now := currentTime()
		duration := now.Sub(startupTime)
		if int(duration) > int(maxTime) {
			log.Infof("Capture reached %s, exiting now...", maxTime)
			return
		}
	}
}

func writeEnrichedData(pw *pcapng.FileWriter, genericMap *config.GenericMap) {
	var io types.InterfaceOptions
	srcType := toText(*genericMap, "SrcK8S_Type").(string)
	if srcType != emptyText {
		io = types.InterfaceOptions{
			Name: fmt.Sprintf(
				"%s: %s -> %s: %s",
				srcType,
				toText(*genericMap, "SrcK8S_Name"),
				toText(*genericMap, "DstK8S_Type"),
				toText(*genericMap, "DstK8S_Name")),
			Description: fmt.Sprintf(
				"%s: %s Namespace: %s -> %s: %s Namespace: %s",
				toText(*genericMap, "SrcK8S_OwnerType"),
				toText(*genericMap, "SrcK8S_OwnerName"),
				toText(*genericMap, "SrcK8S_Namespace"),
				toText(*genericMap, "DstK8S_OwnerType"),
				toText(*genericMap, "DstK8S_OwnerName"),
				toText(*genericMap, "DstK8S_Namespace"),
			),
		}
	} else {
		io.Name = "Unknown resource"
		io = types.InterfaceOptions{
			Name: "Unknown kubernetes resource",
		}
	}
	err := pw.WriteInterfaceDescription(uint16(layers.LinkTypeEthernet), io)
	if err != nil {
		log.Fatal(err)
	}
}
