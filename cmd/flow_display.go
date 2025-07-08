package cmd

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/jpillora/sizestr"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"

	"github.com/eiannone/keyboard"
	"github.com/fatih/color"
	"github.com/rodaine/table"
)

const (
	defaultShowCount = 20
)

var (
	regexes   = []string{}
	lastFlows = []config.GenericMap{}
	showCount = defaultShowCount

	outputBuffer *bytes.Buffer
)

func AppendFlow(genericMap config.GenericMap) {
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
	if len(lastFlows) > showCount {
		lastFlows = lastFlows[len(lastFlows)-showCount:]
	}

	mutex.Unlock()
}

func hearbeat() {
	// render only 1 frame per second to avoid flickering effects due to kubectl exec
	ticker := time.NewTicker(time.Second)
	for range ticker.C {
		if captureEnded {
			return
		}
		updateTable()
	}

}

func updateTable() {
	// init the output buffer if not set
	if outputBuffer == nil {
		buf := bytes.Buffer{}
		outputBuffer = &buf
	} else if outputBuffer.Len() > 0 {
		// skip this frame if the buffer is not empty
		// previous frame had not been rendered !
		return
	}

	if allowClear {
		// clear terminal to render table properly
		writeBuf("\x1bc")
		// no wrap
		writeBuf("\033[?7l")
	}

	writeBuf("Running network-observability-cli as %s Capture\n", captureType)
	writeBuf("Log level: %s ", logLevel)
	writeBuf("Duration: %s ", currentTime().Sub(startupTime).Round(time.Second))
	writeBuf("Capture size: %s\n", sizestr.ToString(totalBytes))
	if len(strings.TrimSpace(options)) > 0 {
		writeBuf("Options: %s\n", options)
	}

	if totalBytes > 0 {
		if strings.Contains(options, "background=true") {
			writeBuf("Showing last: %d\n", showCount)
			writeBuf("Display: %s\n", display.getCurrentItem().name)
			writeBuf("Enrichment: %s\n", enrichment.getCurrentItem().name)
		} else {
			writeBuf("Showing last: %d Use Up / Down keyboard arrows to increase / decrease limit\n", showCount)
			writeBuf("Display: %s Use Left / Right keyboard arrows to cycle views\n", display.getCurrentItem().name)
			writeBuf("Enrichment: %s Use Page Up / Page Down keyboard keys to cycle enrichment scopes\n", enrichment.getCurrentItem().name)
		}

		if display.getCurrentItem().name == rawDisplay {
			outputBuffer.WriteString("Raw flow logs:\n")
			for _, flow := range lastFlows {
				writeBuf("%v\n", flow)
			}
			writeBuf("%s\n", strings.Repeat("-", 500))
		} else {
			// recreate table from scratch
			headerFmt := color.New(color.BgHiBlue, color.Bold).SprintfFunc()
			columnFmt := color.New(color.FgHiYellow).SprintfFunc()

			// main field, always show the end time
			colIDs := []string{
				"EndTime",
			}

			// enrichment fields
			if enrichment.getCurrentItem().name != noOptions {
				colIDs = append(colIDs, enrichment.getCurrentItem().ids...)
			} else {
				// TODO: add a new flag in the config to identify these as default non enriched fields
				colIDs = append(colIDs,
					"SrcAddr",
					"SrcPort",
					"DstAddr",
					"DstPort",
				)
			}

			// append interfaces and their directions between enrichment and features
			// this is useful for pkt drop, udns etc
			colIDs = append(colIDs,
				"Interfaces",
				"IfDirections",
			)

			// standard / feature fields
			if display.getCurrentItem().name != standardDisplay {
				for _, col := range cfg.Columns {
					if col.Field != "" && slices.Contains(display.getCurrentItem().ids, col.Feature) {
						colIDs = append(colIDs, col.ID)
					}
				}
			} else {
				// TODO: add a new flag in the config to identify these as default feature fields
				colIDs = append(colIDs,
					"FlowDirection",
					"Proto",
					"Dscp",
					"Bytes",
					"Packets",
				)
			}

			colInterfaces := make([]interface{}, len(colIDs))
			for i, id := range colIDs {
				colInterfaces[i] = ToTableColName(id)
			}
			tbl := table.New(colInterfaces...)
			tbl.WithWriter(outputBuffer)
			tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)

			// append most recent rows
			for _, flow := range lastFlows {
				tbl.AddRow(ToTableRow(flow, colIDs)...)
			}

			// inserting empty row ensure minimum column sizes
			emptyRow := []interface{}{}
			for _, id := range colIDs {
				emptyRow = append(emptyRow, strings.Repeat("-", ToTableColWidth(id)))
			}
			tbl.AddRow(emptyRow...)

			// print table
			tbl.Print()
		}

		if len(keyboardError) > 0 {
			writeBuf(keyboardError)
		} else {
			if len(regexes) > 0 {
				writeBuf("Live table filter: %s Press enter to match multiple regexes at once\n", regexes)
			} else {
				writeBuf("Type anything to filter incoming flows in view\n")
			}
		}
	} else {
		writeBuf("\n\nCollector is waiting for messages... Please wait.")
	}

	if allowClear {
		printBuf()
	}
}

func writeBuf(s string, a ...any) {
	if len(a) > 0 {
		fmt.Fprintf(outputBuffer, s, a...)
	} else {
		outputBuffer.WriteString(s)
	}
}

func printBuf() {
	if captureEnded {
		return
	}
	// write new display
	_, err := os.Stdout.Write(outputBuffer.Bytes())
	if err != nil {
		fmt.Printf("Error occured while writing stdout: %v", err)
	}
	// reset buffer
	outputBuffer.Reset()
}

// scanner returns true in case of normal exit (end of program execution) or false in case of error
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
			stopReceived = true
			return
		case key == keyboard.KeyArrowUp:
			showCount++
		case key == keyboard.KeyArrowDown:
			if showCount > 10 {
				showCount--
			}
		case key == keyboard.KeyArrowRight:
			display.next()
		case key == keyboard.KeyArrowLeft:
			display.prev()
		case key == keyboard.KeyPgup:
			enrichment.next()
		case key == keyboard.KeyPgdn:
			enrichment.prev()
		case key == keyboard.KeyBackspace || key == keyboard.KeyBackspace2:
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
		// force update to reduce the latency feeling due to low fps
		updateTable()
	}
}
