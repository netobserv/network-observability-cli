package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jpillora/sizestr"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/write/grpc"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/write/grpc/genericmap"

	"github.com/eiannone/keyboard"
	"github.com/fatih/color"
	"github.com/rodaine/table"
	"github.com/spf13/cobra"
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

	features = displayLoop{
		current: featureDefaultIndex,
		all: []displayLoopItem{
			{name: "Raw"},
			{name: "Standard", columns: []string{"Dir", "Interfaces", "Proto", "Dscp", "Bytes", "Packets"}},
			{name: "PktDrop", columns: []string{"DropBytes", "DropPackets", "DropState", "DropCause"}},
			{name: "DNS", columns: []string{"DnsId", "DnsLatency", "DnsRCode", "DnsErrno"}},
			{name: "RTT", columns: []string{"RTT"}},
			{name: "NetworkEvents", columns: []string{"NetworkEvents"}},
			{group: []string{"PktDrop", "DNS", "RTT", "NetworkEvents"}},
		},
	}

	enrichment = displayLoop{
		current: enrichmentDefaultIndex,
		all: []displayLoopItem{
			{name: "None", columns: []string{"SrcAddr", "SrcPort", "DstAddr", "DstPort"}},
			{name: "Zone", columns: []string{"SrcZone", "DstZone"}},
			{name: "Host", columns: []string{"SrcHostName", "SrcHostName"}},
			{name: "Owner", columns: []string{"SrcOwnerName", "SrcOwnerType", "DstOwnerName", "DstOwnerType"}},
			{name: "Resource", columns: []string{"SrcName", "SrcType", "DstName", "DstType"}},
			{group: []string{"Zone", "Host", "Owner", "Resource"}},
		},
	}
)

const (
	featureDefaultIndex    = 6 // All feats.
	enrichmentDefaultIndex = 4 // Resources
)

func runFlowCapture(_ *cobra.Command, _ []string) {
	go func() {
		scanner()
		// scanner returns on exit request
		os.Exit(0)
	}()

	captureType = "Flow"
	wg := sync.WaitGroup{}
	wg.Add(len(ports))
	for i := range ports {
		go func(idx int) {
			defer wg.Done()
			err := runFlowCaptureOnAddr(ports[idx], nodes[idx])
			if err != nil {
				// Only fatal errors are returned here
				log.Fatal(err)
			}
		}(i)
	}
	wg.Wait()
}

func runFlowCaptureOnAddr(port int, filename string) error {
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
	log.Trace("Created flow folder")

	f, err = os.Create("./output/flow/" + filename + ".json")
	if err != nil {
		log.Errorf("Create file %s failed: %v", filename, err.Error())
		log.Fatal(err)
	}
	defer f.Close()
	log.Trace("Created json file")

	// Initialize sqlite DB
	db := initFlowDB(filename)
	log.Trace("Initialized database")

	flowPackets := make(chan *genericmap.Flow, 100)
	collector, err := grpc.StartCollector(port, flowPackets)
	if err != nil {
		return fmt.Errorf("StartCollector failed: %w", err)
	}
	log.Trace("Started collector")

	go func() {
		<-utils.ExitChannel()
		log.Trace("Ending collector")
		close(flowPackets)
		collector.Close()
		db.Close()
		log.Trace("Done")
	}()

	log.Trace("Ready ! Waiting for flows...")
	for fp := range flowPackets {
		if !captureStarted {
			log.Tracef("Received first %d flows", len(flowPackets))
		}

		if stopReceived {
			log.Trace("Stop received")
			return nil
		}
		// parse and display flow async
		go parseGenericMapAndDisplay(fp.GenericMap.Value)

		// Write flows to sqlite DB
		err = queryFlowDB(fp.GenericMap.Value, db)
		if err != nil {
			log.Error("Error while writing to DB:", err.Error())
		}
		if !captureStarted {
			log.Trace("Wrote flows to DB")
		}

		// append new line between each record to read file easilly
		bytes, err := f.Write(append(fp.GenericMap.Value, []byte(",\n")...))
		if err != nil {
			return err
		}
		if !captureStarted {
			log.Trace("Wrote flows to json")
		}

		// terminate capture if max bytes reached
		totalBytes += int64(bytes)
		if totalBytes > maxBytes {
			log.Infof("Capture reached %s, exiting now...", sizestr.ToString(maxBytes))
			return nil
		}

		// terminate capture if max time reached
		now := currentTime()
		duration := now.Sub(startupTime)
		if int(duration) > int(maxTime) {
			log.Infof("Capture reached %s, exiting now...", maxTime)
			return nil
		}

		captureStarted = true
	}
	return nil
}

func parseGenericMapAndDisplay(bytes []byte) {
	genericMap := config.GenericMap{}
	err := json.Unmarshal(bytes, &genericMap)
	if err != nil {
		log.Error("Error while parsing json", err)
		return
	}

	if !captureStarted {
		log.Tracef("Parsed genericMap %v", genericMap)
	}
	manageFlowsDisplay(genericMap)
}

func manageFlowsDisplay(genericMap config.GenericMap) {
	// lock since we are updating lastFlows concurrently
	mutex.Lock()

	lastFlows = append(lastFlows, genericMap)
	sort.Slice(lastFlows, func(i, j int) bool {
		if captureType == "Flow" {
			return toFloat64(lastFlows[i], "TimeFlowEndMs") < toFloat64(lastFlows[j], "TimeFlowEndMs")
		}
		return toFloat64(lastFlows[i], "Time") < toFloat64(lastFlows[j], "Time")
	})
	if len(regexes) > 0 {
		// regexes may change during the render so we make a copy first
		rCopy := make([]string, len(regexes))
		copy(rCopy, regexes)
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
	case "Time", "Interfaces", "SrcZone", "DstZone", "NetworkEvents":
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
	now := currentTime()
	if int(now.Sub(lastRefresh)) > int(maxRefreshRate) {
		lastRefresh = now
		resetTerminal()

		duration := now.Sub(startupTime)
		if outputBuffer == nil {
			fmt.Printf("Running network-observability-cli as %s Capture\n", captureType)
			fmt.Printf("Log level: %s ", logLevel)
			fmt.Printf("Duration: %s ", duration.Round(time.Second))
			fmt.Printf("Capture size: %s\n", sizestr.ToString(totalBytes))
			if len(strings.TrimSpace(filter)) > 0 {
				fmt.Printf("Filters: %s\n", filter)
			}
			fmt.Printf("Showing last: %d Use Up / Down keyboard arrows to increase / decrease limit\n", flowsToShow)
			fmt.Printf("Display: %s	Use Left / Right keyboard arrows to cycle views\n", strings.Join(features.getNames(), ","))
			fmt.Printf("Enrichment: %s	Use Page Up / Page Down keyboard keys to cycle enrichment scopes\n", strings.Join(enrichment.getNames(), ","))
		}

		if features.current == 0 { // Raw
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
			cols = append(cols, enrichment.getCols()...)
			cols = append(cols, features.getCols()...)

			colInterfaces := make([]interface{}, len(cols))
			for i, c := range cols {
				colInterfaces[i] = c
			}
			tbl := table.New(colInterfaces...)
			if outputBuffer != nil {
				tbl.WithWriter(outputBuffer)
			}
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

		if len(keyboardError) > 0 {
			fmt.Println(keyboardError)
		} else if outputBuffer == nil {
			if len(regexes) > 0 {
				fmt.Printf("Live table filter: %s Press enter to match multiple regexes at once\n", regexes)
			} else {
				fmt.Printf("Type anything to filter incoming flows in view\n")
			}
		}

	}
}

func scanner() {
	if err := keyboard.Open(); err != nil {
		keyboardError = fmt.Sprintf("Keyboard not supported %v", err)
		return
	}
	defer func() {
		_ = keyboard.Close()
	}()

	for {
		char, key, err := keyboard.GetKey()
		if err != nil {
			panic(err)
		}
		switch {
		case key == keyboard.KeyCtrlC, stopReceived:
			log.Info("Ctrl-C pressed, exiting program.")
			// exit program
			return
		case key == keyboard.KeyArrowUp:
			flowsToShow++
		case key == keyboard.KeyArrowDown:
			if flowsToShow > 10 {
				flowsToShow--
			}
		case key == keyboard.KeyArrowRight:
			features.next()
		case key == keyboard.KeyArrowLeft:
			features.prev()
		case key == keyboard.KeyPgup:
			enrichment.prev()
		case key == keyboard.KeyPgdn:
			enrichment.next()
		case key == keyboard.KeyBackspace, key == keyboard.KeyBackspace2:
			if len(regexes) > 0 {
				lastIndex := len(regexes) - 1
				if len(regexes[lastIndex]) > 0 {
					regexes[lastIndex] = regexes[lastIndex][:len(regexes[lastIndex])-1]
				} else {
					regexes = regexes[:lastIndex]
				}
			}
		case key == keyboard.KeyEnter:
			regexes = append(regexes, "")
		default:
			if len(regexes) == 0 {
				regexes = []string{string(char)}
			} else {
				lastIndex := len(regexes) - 1
				regexes[lastIndex] += string(char)
			}
		}
		lastRefresh = startupTime
		updateTable()
	}
}
