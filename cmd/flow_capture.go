package cmd

import (
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/jpillora/sizestr"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/write/grpc"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/write/grpc/genericmap"

	"github.com/spf13/cobra"
)

var flowCmd = &cobra.Command{
	Use:   "get-flows",
	Short: "",
	Long:  "",
	Run:   runFlowCapture,
}

func runFlowCapture(_ *cobra.Command, _ []string) {
	capture = Flow
	go startFlowCollector()
	createFlowDisplay()
}

func startFlowCollector() {
	if len(filename) > 0 {
		log.Infof("Starting Flow Capture for %s...", filename)
	} else {
		log.Infof("Starting Flow Capture...")
		filename = strings.ReplaceAll(
			currentTime().UTC().Format(time.RFC3339),
			":", "") // get rid of offensive colons
	}

	var f *os.File
	err := os.MkdirAll("./output/flow/", 0700)
	if err != nil {
		log.Errorf("Create directory failed: %v", err.Error())
		log.Fatal(err)
	}
	log.Debug("Created flow folder")

	f, err = os.Create("./output/flow/" + filename + ".txt")
	if err != nil {
		log.Errorf("Create file %s failed: %v", filename, err.Error())
		log.Fatal(err)
	}
	defer f.Close()
	log.Debug("Created flow logs txt file")

	// Initialize sqlite DB
	db := initFlowDB(filename)
	log.Debug("Initialized database")

	flowPackets := make(chan *genericmap.Flow, 100)
	collector, err := grpc.StartCollector(port, flowPackets)
	if err != nil {
		log.Errorf("StartCollector failed: %v", err.Error())
		return
	}
	log.Debug("Started collector")
	collectorStarted = true

	go func() {
		<-utils.ExitChannel()
		log.Debug("Ending collector")
		close(flowPackets)
		collector.Close()
		db.Close()
		log.Debug("Done")
	}()

	log.Debug("Ready ! Waiting for flows...")
	go hearbeat()
	for fp := range flowPackets {
		if !captureStarted {
			log.Debugf("Received first %d flows", len(flowPackets))
		}

		if stopReceived {
			log.Debug("Stop received")
			return
		}
		// parse and display flow async
		go parseGenericMapAndAppendFlow(fp.GenericMap.Value)

		// Write flows to sqlite DB
		err = queryFlowDB(fp.GenericMap.Value, db)
		if err != nil {
			log.Error("Error while writing to DB:", err.Error())
		}
		if !captureStarted {
			log.Debug("Wrote flows to DB")
		}

		// append new line between each record to read file easilly
		bytes, err := f.Write(append(fp.GenericMap.Value, []byte(",\n")...))
		if err != nil {
			log.Error(err)
			return
		}
		if !captureStarted {
			log.Debug("Wrote flows to json")
		}

		// terminate capture if max bytes reached
		totalBytes += int64(bytes)
		if totalBytes > maxBytes {
			if exit := onLimitReached(); exit {
				log.Infof("Capture reached %s, exiting now...", sizestr.ToString(maxBytes))
				return
			}
		}

		// terminate capture if max time reached
		now := currentTime()
		duration := now.Sub(startupTime)
		if int(duration) > int(maxTime) {
			if exit := onLimitReached(); exit {
				log.Infof("Capture reached %s, exiting now...", maxTime)
				return
			}
		}

		captureStarted = true
	}
}

func parseGenericMapAndAppendFlow(bytes []byte) {
	genericMap := config.GenericMap{}
	err := json.Unmarshal(bytes, &genericMap)
	if err != nil {
		log.Error("Error while parsing json", err)
		return
	}

	if !captureStarted {
		log.Debugf("Parsed genericMap %v", genericMap)
	}
	AppendFlow(genericMap)
}
