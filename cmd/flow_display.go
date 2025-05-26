package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"slices"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/jpillora/sizestr"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type TableData struct {
	cols  []string
	flows []config.GenericMap
	tview.TableContentReadOnly
}

const (
	keepCount              = 100 // flows to keep in memory
	defaultShowCount       = 30  // flows to display
	defaultFramesPerSecond = 5   // frames per second
	defaultExtraWidth      = 5   // additionnal column width
)

var (
	regexes     = []string{}
	lastFlows   = []config.GenericMap{}
	suggestions = []string{}

	showCount       = defaultShowCount
	framesPerSecond = defaultFramesPerSecond
	extraWidth      = defaultExtraWidth

	app          *tview.Application
	mainView     *tview.Flex
	tableView    *tview.Table
	durationText = tview.NewTextView()
	sizeText     = tview.NewTextView()
	tableData    = &TableData{
		cols:  []string{},
		flows: []config.GenericMap{},
	}

	errAdvancedDisplay error
)

func createDisplay() {
	app = tview.NewApplication().
		SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			switch event.Key() {
			case tcell.KeyCtrlC:
				log.Info("Ctrl-C pressed, exiting program.")
				if app != nil {
					app.Stop()
				}
			default:
				// nothing to do here
			}

			return event
		}).
		SetRoot(getMain(), true).
		EnableMouse(true)

	errAdvancedDisplay = app.Run()
	if errAdvancedDisplay == nil {
		go hearbeat()
	} else {
		fmt.Printf("Can't display advanced UI: %v", errAdvancedDisplay)
		done := make(chan os.Signal, 1)
		signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)
		<-done
	}
}

func getMain() tview.Primitive {
	mainView = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(getTop(), 5, 0, false).
		AddItem(getTable(), 0, 1, false).
		AddItem(getBottom(), 2, 0, false)
	return mainView
}

func getTop() tview.Primitive {
	flexView := tview.NewFlex().SetDirection(tview.FlexRow)

	// info row
	fpsText := tview.NewTextView().SetText(getFPSText())
	infoRow := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(tview.NewTextView().SetText(getCaptureTypeText()), 0, 1, false).
		AddItem(tview.NewTextView().SetText(getLogLevelText()), 0, 1, false).
		AddItem(durationText.SetText(getDurationText()), 0, 1, false).
		AddItem(sizeText.SetText(getSizeText()), 0, 1, false).
		AddItem(fpsText, 0, 1, false).
		AddItem(tview.NewButton("-").SetSelectedFunc(func() {
			if framesPerSecond > 1 {
				framesPerSecond--
			}
			fpsText.SetText(getFPSText())
		}), 5, 0, false).
		AddItem(tview.NewButton("+").SetSelectedFunc(func() {
			framesPerSecond++
			fpsText.SetText(getFPSText())
		}), 5, 0, false)

	flexView.AddItem(infoRow, 0, 1, false)

	// flows count
	flowCountTextView := tview.NewTextView().SetText(getShowCountText())
	flowsCountRow := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(flowCountTextView, 0, 1, false).
		AddItem(tview.NewButton("-").SetSelectedFunc(func() {
			if showCount > 5 {
				showCount--
			}
			flowCountTextView.SetText(getShowCountText())
			updateScreen()
		}), 5, 0, false).
		AddItem(tview.NewButton("+").SetSelectedFunc(func() {
			showCount++
			flowCountTextView.SetText(getShowCountText())
			updateScreen()
		}), 5, 0, false)

	flexView.AddItem(flowsCountRow, 0, 1, false)

	// display: TODO: replace with dropdowns or popup
	displayTextView := tview.NewTextView().SetText(getDisplayText())
	displayRow := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(displayTextView, 0, 1, false).
		AddItem(tview.NewButton("←").SetSelectedFunc(func() {
			display.prev()
			displayTextView.SetText(getDisplayText())
			updateScreen()
		}), 5, 0, false).
		AddItem(tview.NewButton("→").SetSelectedFunc(func() {
			display.next()
			displayTextView.SetText(getDisplayText())
			updateScreen()
		}), 5, 0, false)

	flexView.AddItem(displayRow, 0, 1, false)

	// enrichment: TODO: replace with dropdowns or popup
	enrichmentTextView := tview.NewTextView().SetText(getEnrichmentText())
	enrichmentRow := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(enrichmentTextView, 0, 1, false)
	if display.getCurrentItem().name != rawDisplay {
		enrichmentRow.
			AddItem(tview.NewButton("←").SetSelectedFunc(func() {
				enrichment.prev()
				enrichmentTextView.SetText(getEnrichmentText())
				updateScreen()
			}), 5, 0, false).
			AddItem(tview.NewButton("→").SetSelectedFunc(func() {
				enrichment.next()
				enrichmentTextView.SetText(getEnrichmentText())
				updateScreen()
			}), 5, 0, false)
	}
	flexView.AddItem(enrichmentRow, 0, 1, false)

	return flexView
}

func getTable() *tview.Table {
	tableView = tview.NewTable().
		SetBorders(false).
		SetSelectable(true, true).
		SetSelectedFunc(func(_, _ int) {
			if app != nil {
				app.Sync()
			}
		}).
		SetContent(tableData)
	return tableView
}

func getBottom() tview.Primitive {
	flexView := tview.NewFlex().SetDirection(tview.FlexColumn)

	textView := tview.NewTextView().SetText(getRegexesText())

	inputField := tview.NewInputField().
		SetLabel("Live table regexes: ").
		SetFieldWidth(30)

	inputField.SetAutocompleteFunc(func(currentText string) (entries []string) {
		if len(currentText) == 0 {
			return
		}
		for _, word := range suggestions {
			if strings.HasPrefix(strings.ToLower(word), strings.ToLower(currentText)) {
				entries = append(entries, word)
			}
		}
		if len(entries) <= 1 {
			entries = nil
		}
		return
	})
	// on any input event
	inputField.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			if len(inputField.GetText()) == 0 && len(regexes) > 0 {
				regexes = regexes[:len(regexes)-1]
			}
			textView.SetText(getRegexesText())
		default:
			// nothing to do here
		}
		return event
	})
	// after input event
	inputField.SetAutocompletedFunc(func(text string, _, source int) bool {
		if source != tview.AutocompletedNavigate {
			inputField.SetText(text)
		}
		return source == tview.AutocompletedEnter || source == tview.AutocompletedClick
	})
	// after autocomplete event
	inputField.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			text := inputField.GetText()
			if len(text) > 0 {
				regexes = append(regexes, text)
				inputField.SetText("")
			}
			textView.SetText(getRegexesText())
		default:
			// nothing to do here
		}
		updateScreen()
	})
	flexView.AddItem(inputField, 51, 0, true)

	flexView.AddItem(textView, 0, 1, false)

	return flexView
}

func getCaptureTypeText() string {
	return fmt.Sprintf("%s Capture", captureType)
}

func getLogLevelText() string {
	return fmt.Sprintf("Log level: %s", logLevel)
}

func getEnrichmentText() string {
	if display.getCurrentItem().name == rawDisplay {
		return "Enrichment: n/a\n"
	}
	return fmt.Sprintf("Enrichment: %s\n", enrichment.getCurrentItem().name)
}

func getDisplayText() string {
	return fmt.Sprintf("Display: %s\n", display.getCurrentItem().name)
}

func getShowCountText() string {
	return fmt.Sprintf("Showing last: %d\n", showCount)
}

func getFPSText() string {
	return fmt.Sprintf("FPS: %d", framesPerSecond)
}

func getSizeText() string {
	return fmt.Sprintf("Capture size: %s", sizestr.ToString(totalBytes))
}

func getDurationText() string {
	duration := currentTime().Sub(startupTime)
	return fmt.Sprintf("Duration: %s ", duration.Round(time.Second))
}

func getRegexesText() string {
	if len(regexes) > 0 {
		return fmt.Sprintf("Current filters: [%s]. Press enter to add a new one and backspace to remove last one", strings.Join(regexes, ","))
	}
	return "Press enter to match multiple regexes at once"
}

func AppendFlow(genericMap config.GenericMap) {
	if errAdvancedDisplay != nil {
		// simply print flow into logs
		log.Printf("%v\n", genericMap)
	} else {
		// lock since we are updating lastFlows concurrently
		mutex.Lock()

		// add new flow to the array
		lastFlows = append(lastFlows, genericMap)

		// sort flows according to time
		sort.Slice(lastFlows, func(i, j int) bool {
			if captureType == "Flow" {
				return toFloat64(lastFlows[i], "TimeFlowEndMs") < toFloat64(lastFlows[j], "TimeFlowEndMs")
			}
			return toFloat64(lastFlows[i], "Time") < toFloat64(lastFlows[j], "Time")
		})

		// limit flows kept in memory
		if len(lastFlows) > keepCount {
			lastFlows = lastFlows[len(lastFlows)-keepCount:]
		}

		mutex.Unlock()
	}
}

func hearbeat() {
	for {
		if captureEnded {
			return
		}

		updateStatusTexts()
		updateTable()

		time.Sleep(time.Second / time.Duration(framesPerSecond))
	}
}

func updateStatusTexts() {
	durationText.SetText(getDurationText())
	sizeText.SetText(getSizeText())
}

func updateTable() {
	cols := []string{}
	if display.getCurrentItem().name == rawDisplay {
		cols = append(cols,
			rawDisplay,
		)
	} else {
		// main field, always show the end time
		cols = append(cols,
			"EndTime",
		)

		// enrichment fields
		if enrichment.getCurrentItem().name != noOptions {
			cols = append(cols, enrichment.getCurrentItem().ids...)
		} else {
			// TODO: add a new flag in the config to identify these as default non enriched fields
			cols = append(cols,
				"SrcAddr",
				"SrcPort",
				"DstAddr",
				"DstPort",
			)
		}

		// append interfaces and their directions between enrichment and features
		// this is useful for pkt drop, udns etc
		cols = append(cols,
			"Interfaces",
			"IfDirections",
		)

		// standard / feature fields
		if display.getCurrentItem().name != standardDisplay {
			for _, col := range cfg.Columns {
				if col.Field != "" && slices.Contains(display.getCurrentItem().ids, col.Feature) {
					cols = append(cols, col.ID)
				}
			}
		} else {
			// TODO: add a new flag in the config to identify these as default feature fields
			cols = append(cols,
				"FlowDirection",
				"Proto",
				"Dscp",
				"Bytes",
				"Packets",
			)
		}
	}

	// lastFlows may change during the render so we make a copy first
	lfCopy := make([]config.GenericMap, len(lastFlows))
	copy(lfCopy, lastFlows)

	// apply regexes to filter flows
	flows := []config.GenericMap{}
	if len(regexes) > 0 {
		// regexes may change during the render so we make a copy first
		rCopy := make([]string, len(regexes))
		copy(rCopy, regexes)

		for _, flow := range lfCopy {
			match := true
			for i := range rCopy {
				ok, _ := regexp.MatchString(rCopy[i], fmt.Sprintf("%v", flow))
				match = match && ok
				if !match {
					break
				}
			}
			if match {
				flows = append(flows, flow)
			}
		}
	} else {
		flows = lfCopy
	}

	// limit filtered flows to display size
	if len(flows) > showCount {
		flows = flows[len(flows)-showCount:]
	}

	suggestions = []string{}
	for _, flow := range flows {
		for k, v := range flow {
			if !slices.Contains(suggestions, k) {
				suggestions = append(suggestions, k)
			}

			valueStr := fmt.Sprintf("%v", v)
			if !slices.Contains(suggestions, valueStr) {
				suggestions = append(suggestions, valueStr)
			}
		}
	}

	// update tableData
	tableData.cols = cols
	tableData.flows = flows

	// refresh
	if app != nil {
		app.Draw()
	}
}

func updateScreen() {
	if app != nil {
		mainView.Clear()
		app.SetRoot(getMain(), true)
	}
}

func (d *TableData) GetCell(row, col int) *tview.TableCell {
	if len(d.cols) == 0 {
		return tview.NewTableCell("Initializing...")
	} else if row == -1 {
		return tview.NewTableCell("invalid row")
	} else if col == -1 || col >= len(d.cols) {
		return tview.NewTableCell("invalid col")
	}

	id := d.cols[col]
	width := ToColWidth(id)
	color := tcell.ColorWhite
	bgColor := tcell.ColorBlack
	if row == 0 {
		color = tcell.ColorWhite
		bgColor = tcell.ColorBlue
	} else if col == 0 {
		color = tcell.ColorYellow
		bgColor = tcell.ColorBlack
	} else if id == "EndTime" {
		color = tcell.ColorYellow
		bgColor = tcell.ColorWhite
	}
	if row == 0 {
		return tview.NewTableCell(ToColName(id)).
			SetTextColor(color).
			SetBackgroundColor(bgColor).
			SetAlign(tview.AlignLeft).
			SetMaxWidth(width)
	}
	index := row - 1
	if index < len(d.flows) {
		return tview.NewTableCell(ToColValue(d.flows[index], id)).
			SetTextColor(color).
			SetBackgroundColor(bgColor).
			SetAlign(tview.AlignLeft).
			SetMaxWidth(width)
	}

	// index out of bounds due to concurrent update
	return tview.NewTableCell("")
}

func (d *TableData) GetRowCount() int {
	return len(d.flows) + 1
}

func (d *TableData) GetColumnCount() int {
	return len(d.cols)
}
