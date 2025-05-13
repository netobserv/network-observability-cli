package cmd

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
	"github.com/jpillora/sizestr"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/write/grpc"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/write/grpc/genericmap"
	"github.com/spf13/cobra"
)

var pktCmd = &cobra.Command{
	Use:   "get-packets",
	Short: "",
	Long:  "",
	Run:   runPacketCapture,
}

func runPacketCapture(_ *cobra.Command, _ []string) {
	go scanner()

	captureType = "Packet"
	wg := sync.WaitGroup{}
	wg.Add(len(ports))
	for i := range ports {
		go func(idx int) {
			defer wg.Done()
			err := runPacketCaptureOnAddr(ports[idx], nodes[idx])
			if err != nil {
				// Only fatal error are returned
				log.Fatal(err)
			}
		}(i)
	}
	wg.Wait()
}

func runPacketCaptureOnAddr(port int, filename string) error {
	if len(filename) > 0 {
		log.Infof("Starting Packet Capture for %s...", filename)
	} else {
		log.Infof("Starting Packet Capture...")
		filename = strings.ReplaceAll(
			currentTime().UTC().Format(time.RFC3339),
			":", "") // get rid of offensive colons
	}

	err := os.MkdirAll("./output/pcap/", 0700)
	if err != nil {
		log.Errorf("Create directory failed: %v", err.Error())
		log.Fatal(err)
	}
	log.Trace("Created pcap folder")

	f, err := os.Create("./output/pcap/" + filename + ".pcapng")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	log.Trace("Created pcapng file")

	ngw, err := pcapgo.NewNgWriter(f, layers.LinkTypeEthernet)
	if err != nil {
		log.Error("Error while creating writer", err)
		return nil
	}
	defer ngw.Flush()
	log.Trace("Wrote pcap section header & interface")

	flowPackets := make(chan *genericmap.Flow, 100)
	collector, err := grpc.StartCollector(port, flowPackets)
	if err != nil {
		return fmt.Errorf("StartCollector failed: %w", err)
	}
	log.Trace("Started collector")
	collectorStarted = true

	go func() {
		<-utils.ExitChannel()
		log.Trace("Ending collector")
		close(flowPackets)
		collector.Close()
		log.Trace("Done")
	}()

	var srcComment strings.Builder
	var dstComment strings.Builder
	var commonComment strings.Builder

	log.Trace("Ready ! Waiting for packets...")
	go hearbeat()
	for fp := range flowPackets {
		if !captureStarted {
			log.Tracef("Received first %d packets", len(flowPackets))
		}

		if stopReceived {
			log.Trace("Stop received")
			return nil
		}

		genericMap := config.GenericMap{}
		err := json.Unmarshal(fp.GenericMap.Value, &genericMap)
		if err != nil {
			log.Error("Error while parsing json", err)
			return nil
		}
		if !captureStarted {
			log.Tracef("Parsed genericMap %v", genericMap)
		}

		data, ok := genericMap["Data"]
		if ok {
			// clear generic map data
			delete(genericMap, "Data")
			if !captureStarted {
				log.Trace("Deleted data")
			}

			// display as flow async
			go AppendFlow(genericMap)

			// Get capture timestamp
			ts := time.Unix(int64(genericMap["Time"].(float64)), 0)

			// Decode b64 encoded data
			b, err := base64.StdEncoding.DecodeString(data.(string))
			if err != nil {
				log.Error("Error while decoding data", err)
				return nil
			}
			// sort generic map keys to keep comments ordered
			keys := make([]string, 0, len(genericMap))
			for k := range genericMap {
				// ignore time field
				if k == "Time" {
					continue
				}
				keys = append(keys, k)

			}
			sort.Strings(keys)

			// generate comments per category
			srcComment.WriteString("Source\n")
			dstComment.WriteString("Destination\n")
			commonComment.WriteString("Common\n")
			for _, k := range keys {
				id := toColID(k)
				str := fmt.Sprintf("%s: %v\n", ToTableColName(id), toDisplayValue(genericMap, id, k))
				if strings.HasPrefix(k, "Src") {
					srcComment.WriteString(str)
				} else if strings.HasPrefix(k, "Dst") {
					dstComment.WriteString(str)
				} else {
					commonComment.WriteString(str)
				}
			}

			// write enriched data as interface
			if err := ngw.WritePacket(gopacket.CaptureInfo{
				Timestamp:     ts,
				Length:        len(b),
				CaptureLength: len(b),
				Comments: []string{
					srcComment.String(),
					dstComment.String(),
					commonComment.String(),
				},
			}, b); err != nil {
				log.Error("Error while writing packet", err)
				return nil
			}

			srcComment.Reset()
			dstComment.Reset()
			commonComment.Reset()
		} else {
			if !captureStarted {
				log.Trace("Data is missing")
			}

			// display as flow async
			go AppendFlow(genericMap)
		}

		// terminate capture if max bytes reached
		totalBytes += int64(len(fp.GenericMap.Value))
		if totalBytes > maxBytes {
			if exit := onLimitReached(); exit {
				log.Infof("Capture reached %s, exiting now...", sizestr.ToString(maxBytes))
				return nil
			}
		}

		// terminate capture if max time reached
		now := currentTime()
		duration := now.Sub(startupTime)
		if int(duration) > int(maxTime) {
			if exit := onLimitReached(); exit {
				log.Infof("Capture reached %s, exiting now...", maxTime)
				return nil
			}
		}

		captureStarted = true
	}
	return nil
}
