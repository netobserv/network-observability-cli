package cmd

import (
	"database/sql"
	"encoding/json"
	"fmt"
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
	showCount = defaultFlowShowCount

	done := make(chan struct{})
	go func() {
		defer close(done)
		startFlowCollector()
	}()

	// Always enforce maxtime even when no flows arrive (maxTime was previously
	// only checked inside the receive loop).
	go enforceTimeLimit()

	if isBackground {
		go backgroundHearbeat()
		<-done
		return
	}

	uiStarted := time.Now()
	createFlowDisplay()

	// Previous bug: always calling requestCollectorStop() after createFlowDisplay()
	// aborted the collector when the TUI failed to start (no TTY / kubectl quirks),
	// leaving an empty output/flow/<capture>/ directory and Capture size 0.
	// Also treat an instant successful Run() return the same way (TTY half-broken).
	uiTooShort := errAdvancedDisplay == nil && time.Since(uiStarted) < 500*time.Millisecond
	if errAdvancedDisplay != nil || uiTooShort {
		reason := errAdvancedDisplay
		if reason == nil {
			reason = fmt.Errorf("UI exited immediately")
		}
		log.Warnf("UI unavailable (%v); continuing capture until --maxtime, --maxbytes, or signal", reason)
		<-done
		return
	}

	// TUI exited (Ctrl-C / quit): stop collector and wait so parquet Close/Flush runs
	// before the process exits and before shell copyOutput.
	requestCollectorStop()
	<-done
}

// enforceTimeLimit stops the collector when --maxtime is reached, even if no
// flows have been received yet (needed when the receive-loop check never runs).
func enforceTimeLimit() {
	timer := time.NewTimer(maxTime)
	defer timer.Stop()
	select {
	case <-timer.C:
		log.Infof("Capture reached %s, exiting now...", maxTime)
		if exit := onLimitReached(); exit || isBackground {
			requestCollectorStop()
		}
		if app != nil {
			app.Stop()
		}
	case <-collectorStopCh:
	}
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

	formats, err := parseOutputFormats(options)
	if err != nil {
		log.Fatalf("Invalid --format: %v", err)
	}
	log.Infof("Local output format(s): %s", formats)

	var (
		f  *os.File
		db *sql.DB
		pq *parquetFlowWriter
	)

	if formats.Parquet {
		pq, err = newParquetFlowWriter(filename, defaultParquetBatchSize)
		if err != nil {
			log.Fatalf("Creating parquet writer failed: %v", err)
		}
		defer func() {
			if closeErr := pq.Close(); closeErr != nil {
				log.Errorf("Closing parquet writer: %v", closeErr)
			} else if n := pq.BytesReceived(); n > 0 {
				log.Infof("Parquet capture complete: %s inbound, %s on disk under %s/flow/%s/",
					sizestr.ToString(n), sizestr.ToString(pq.BytesWritten()), outputRoot, filename)
			} else {
				log.Warnf("Parquet capture ended with 0 flows under %s/flow/%s/", outputRoot, filename)
			}
		}()
		log.Infof("Writing Parquet schema v1 under %s/flow/%s/ (Hive layout; flushed every %s and on exit)",
			outputRoot, filename, parquetFlushInterval)
	}

	if formats.JSON {
		// Create a text file to receive json chunks; the file will be fixed and renamed as json later, when pulled in shell.
		f, err = createOutputFile("flow", filename+".txt")
		if err != nil {
			log.Fatalf("Creating output file failed: %v", err)
		}
		defer f.Close()
		log.Debugf("Created flow logs txt file: %s", f.Name())
	}

	if formats.SQLite {
		db = initFlowDB(filename)
		log.Debug("Initialized database")
	}

	flowPackets := make(chan *genericmap.Flow, 100)
	collector, err := grpc.StartCollector(port, flowPackets)
	if err != nil {
		log.Errorf("StartCollector failed: %v", err.Error())
		return
	}
	log.Debug("Started collector")
	collectorStarted = true

	go func() {
		select {
		case <-collectorStopCh:
		case <-utils.ExitChannel():
		}
		log.Debug("Ending collector")
		close(flowPackets)
		collector.Close()
		if db != nil {
			db.Close()
		}
		log.Debug("Done")
	}()

	// Flush buffered parquet periodically so short captures still produce files
	// even if the process is later killed before a full batch.
	if pq != nil {
		go func() {
			ticker := time.NewTicker(parquetFlushInterval)
			defer ticker.Stop()
			for {
				select {
				case <-collectorStopCh:
					// Final Close is handled by defer on startFlowCollector.
					return
				case <-ticker.C:
					if flushErr := pq.Flush(); flushErr != nil {
						log.Errorf("Periodic parquet flush: %v", flushErr)
					}
				}
			}
		}()
	}

	log.Debug("Ready ! Waiting for flows...")
	for fp := range flowPackets {
		if !captureStarted {
			log.Debugf("Received first %d flows", len(flowPackets))
		}

		inbound := int64(len(fp.GenericMap.Value))

		// parse and display flow async
		go parseGenericMapAndAppendFlow(fp.GenericMap.Value)

		if pq != nil {
			if err = pq.AddJSON(fp.GenericMap.Value); err != nil {
				log.Error("Error while writing parquet:", err.Error())
			}
			if !captureStarted {
				log.Debug("Wrote flows to parquet buffer")
			}
		}

		if db != nil {
			if err = queryFlowDB(fp.GenericMap.Value, db); err != nil {
				log.Error("Error while writing to DB:", err.Error())
			}
			if !captureStarted {
				log.Debug("Wrote flows to DB")
			}
		}

		if f != nil {
			if _, writeErr := f.Write(append(fp.GenericMap.Value, []byte(",\n")...)); writeErr != nil {
				log.Error(writeErr)
				return
			}
			if !captureStarted {
				log.Debug("Wrote flows to json")
			}
		}

		// Count inbound JSON bytes once (not per sink) so Capture size / --maxbytes
		// stay correct for any format combination.
		totalBytes += inbound

		// terminate capture if max bytes reached
		if totalBytes > maxBytes {
			if exit := onLimitReached(); exit {
				log.Infof("Capture reached %s, exiting now...", sizestr.ToString(maxBytes))
				requestCollectorStop()
				return
			}
		}

		// terminate capture if max time reached
		now := currentTime()
		duration := now.Sub(startupTime)
		if duration > maxTime {
			if exit := onLimitReached(); exit {
				log.Infof("Capture reached %s, exiting now...", maxTime)
				requestCollectorStop()
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
