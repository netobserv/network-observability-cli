package cmd

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/navidys/tvxwidgets"
	"github.com/rivo/tview"
)

const (
	// 24h hh:mm:ss: 14:23:20
	HHMMSS24h = "15:04:05"
)

type Graph struct {
	Plot *tvxwidgets.Plot

	Query   Query
	Labels  []string
	Legends []map[string]string
	Data    [][]float64
}

var (
	legendView      *tview.Flex
	graphsContainer = tview.NewFlex().SetDirection(tview.FlexRow)
	panelsTextView  = tview.NewTextView()

	selectedPanels = []string{}
	graphs         = []Graph{}
	colors         = []tcell.Color{
		tcell.ColorWhite,
		tcell.ColorPeru,
		tcell.ColorRebeccaPurple,
		tcell.ColorChocolate,
		tcell.ColorGold,
		tcell.ColorGreen,
		tcell.ColorBlue,
		tcell.ColorMaroon,
		tcell.ColorAquaMarine,
		tcell.ColorDarkSeaGreen,
		tcell.ColorOrange,
		tcell.ColorBisque,
		tcell.ColorTeal,
		tcell.ColorPurple,
		tcell.ColorMintCream,
		tcell.ColorMistyRose,
		tcell.ColorSeaGreen,
		tcell.ColorSalmon,
		tcell.ColorMidnightBlue,
		tcell.ColorDeepSkyBlue,
		tcell.ColorFloralWhite,
		tcell.ColorMediumSeaGreen,
		tcell.ColorBlanchedAlmond,
		tcell.ColorDarkKhaki,
		tcell.ColorHoneydew,
	} // TODO: limit sample size to avoid color indexing crash
)

func createMetricDisplay() {
	updateShowMetricCount()
	updateGraphs(false)

	app = tview.NewApplication().
		SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			//nolint:exhaustive
			switch event.Key() {
			case tcell.KeyCtrlC:
				log.Info("Ctrl-C pressed, exiting program.")
				if app != nil {
					app.Stop()
				}
			case tcell.KeyCtrlSpace:
				pause(!paused)
			default:
				// nothing to do here
			}
			return event
		}).
		SetRoot(getPages(), true).
		EnableMouse(true)

	go hearbeat()

	errAdvancedDisplay = app.Run()
	if errAdvancedDisplay != nil {
		log.Errorf("Can't display advanced UI: %v", errAdvancedDisplay)
	}
}

func getMetricTop() tview.Primitive {
	topView := tview.NewFlex().SetDirection(tview.FlexRow)
	topView.AddItem(getInfoRow(), 1, 0, false)
	countRow := getCountRow(false)
	// add time range dropdown
	countRow.AddItem(tview.NewDropDown().
		SetLabel("Time range ").
		SetOptions(durations, nil).
		SetCurrentOption(selectedDuration).
		SetSelectedFunc(func(_ string, index int) {
			selectedDuration = index
			updateShowMetricCount()
			updateGraphs(true)
		}).
		SetFieldWidth(5), 16, 0, false)
	topView.AddItem(countRow, 1, 0, false)

	// panels row containing cycles and custom panels picker
	panelsRow := tview.NewFlex().SetDirection(tview.FlexColumn)

	// panels
	panelsRow.AddItem(panelsTextView, 0, 1, false)
	if len(selectedPanels) == 0 {
		panelsRow.
			AddItem(tview.NewButton("←").SetSelectedFunc(func() {
				panels.prev()
				updatePanels(true)
				updateScreen()
			}), 5, 0, false).
			AddItem(tview.NewButton("→").SetSelectedFunc(func() {
				panels.next()
				updatePanels(true)
				updateScreen()
			}), 5, 0, false)
	} else {
		panelsRow.
			AddItem(tview.NewButton("⟲").SetSelectedFunc(func() {
				selectedPanels = []string{}
				panels.current = defaultPanelsIndex
				updatePanels(true)
				updateScreen()
			}), 10, 0, false)
	}
	panelsRow.AddItem(tview.NewTextView(), 0, 2, false)
	if !paused {
		updatePanels(false)
	}

	// add panels modal button
	panelsRow.AddItem(tview.NewButton(" Manage panels ").SetSelectedFunc(func() {
		showPopup = true
		app.SetRoot(getPages(), true)
	}), 16, 0, false)
	topView.AddItem(panelsRow, 1, 0, false)
	return topView
}

func updateShowMetricCount() {
	showCount = metricCounts[durations[selectedDuration]]
	countTextView.SetText(getShowCountText())
}

func getGraphs() tview.Primitive {
	graphsContainer.Clear()

	var flex *tview.Flex
	for index := range graphs {
		plot := getPlot(toMetricName(graphs[index].Query.PromQL, 0))
		graphs[index].Plot = plot

		if index%2 == 0 {
			flex = tview.NewFlex().SetDirection(tview.FlexColumn)
			flex.SetRect(0, 0, 100, 15)
			graphsContainer.AddItem(flex, 0, 1, false)
		}
		flex.AddItem(plot, 0, 1, false)
	}

	return graphsContainer
}

func getMetricMain() tview.Primitive {
	mainView = tview.NewFlex().SetDirection(tview.FlexRow)
	mainView.AddItem(getMetricTop(), 3, 0, false)
	mainView.AddItem(getGraphs(), 0, 1, false)

	return mainView
}

func getLegends(title string, labels []string, legends []map[string]string, data [][]float64) tview.Primitive {
	legendView = tview.NewFlex().SetDirection(tview.FlexRow)
	legendView.SetBorder(true)
	legendView.SetTitle(fmt.Sprintf("%s legend", title))

	table := tview.NewTable()
	table.SetCell(0, 0, tview.NewTableCell("   ").SetTextColor(tcell.ColorWhite).SetBackgroundColor(tcell.ColorBlue))
	for i, label := range labels {
		table.SetCell(0, i+1, tview.NewTableCell(ellipsizeAndPad(label, 50)).SetTextColor(tcell.ColorWhite).SetBackgroundColor(tcell.ColorBlue))
	}
	table.SetCell(0, len(labels)+1, tview.NewTableCell("Value").SetTextColor(tcell.ColorWhite).SetBackgroundColor(tcell.ColorBlue))

	for i := range legends {
		table.SetCell(i+1, 0, tview.NewTableCell("•••").SetTextColor(colors[i]))
		for j, label := range labels {
			table.SetCell(i+1, j+1, tview.NewTableCell(ellipsizeAndPad(legends[i][label], 50)))
		}
		table.SetCell(i+1, len(labels)+1, tview.NewTableCell(ellipsizeAndPad(fmt.Sprintf("%.2f", data[i][len(data[i])-1]), 50)))
	}

	legendView.AddItem(table, 0, 1, false)

	return legendView

}

func getMetricsModal() tview.Primitive {
	availablePanels := []string{}
	for _, p := range panels.all {
		availablePanels = append(availablePanels, p.ids...)
	}

	content := tview.NewFlex().SetDirection(tview.FlexRow)
	content.SetBorder(true).SetTitle("Manage panels")

	content.AddItem(tview.NewTextView().
		SetText("Highlight a panel and select / unselect it pressing the `Enter` key."), 2, 0, false)

	panelsTable := tview.NewTable()

	setCell := func(i int, panel string) {
		checkedStr := "[   ]"
		if slices.Contains(selectedPanels, panel) {
			checkedStr = "[ X ]"
		}
		panelsTable.SetCell(i, 0, tview.NewTableCell(checkedStr))
		panelsTable.SetCell(i, 1, tview.NewTableCell(toMetricName(panel, 140)))
	}

	setTableContent := func() {
		for i, p := range availablePanels {
			setCell(i, p)
		}
	}

	onSelect := func(row int) {
		if row < 0 || row >= len(availablePanels) {
			return
		}
		p := availablePanels[row]
		for i, v := range selectedPanels {
			// remove id if found
			if v == p {
				selectedPanels = append(selectedPanels[:i], selectedPanels[i+1:]...)
				setCell(row, p)
				return
			}
		}
		// else add it to selection
		selectedPanels = append(selectedPanels, p)
		setCell(row, p)
		updatePanels(true)
	}

	panelsTable.SetSelectable(true, false).
		SetSelectedFunc(func(row, _ int) {
			onSelect(row)
		})

	setTableContent()
	content.AddItem(panelsTable, 0, 1, true)

	buttons := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(tview.NewButton("Reset").SetSelectedFunc(func() {
			selectedPanels = []string{}
			setTableContent()
			updatePanels(false)
		}), 0, 1, false).
		AddItem(tview.NewTextView(), 1, 0, false).
		AddItem(tview.NewButton("Save").SetSelectedFunc(func() {
			updateGraphs(true)
			updateScreen()
		}), 0, 1, false)
	content.AddItem(buttons, 1, 0, false)

	return getModal(content, 150, 30)
}

func getMetricShowCountText() string {
	return fmt.Sprintf("Showing %d points per graph", showCount)
}

func getPanelsText() string {
	if len(selectedPanels) > 0 {
		return "Custom panels"
	}
	return fmt.Sprintf("Display: %s\n", panels.getCurrentItem().name)
}

func getPlot(title string) *tvxwidgets.Plot {
	plot := tvxwidgets.NewPlot()
	plot.SetBorder(true)
	plot.SetTitle(title)
	plot.SetAxesColor(tcell.ColorWhite)
	plot.SetAxesLabelColor(tcell.ColorWhite)
	plot.SetPlotType(tvxwidgets.PlotTypeScatter)
	plot.SetMarker(tvxwidgets.PlotMarkerBraille)
	plot.SetLineColor(colors)
	return plot
}

func updateGraphs(query bool) {
	graphs = []Graph{}

	if len(selectedPanels) > 0 {
		for _, p := range selectedPanels {
			graphs = append(graphs, Graph{
				Query: Query{
					PromQL: p,
				},
			})
		}
	} else {
		for _, p := range panels.getCurrentItem().ids {
			graphs = append(graphs, Graph{
				Query: Query{
					PromQL: p,
				},
			})
		}
	}

	getGraphs()
	if query && client != nil {
		go queryGraphs(context.TODO(), *client)
	}
}

func updatePanels(query bool) {
	panelsTextView.SetText(getPanelsText())
	updateGraphs(query)
}

func updatePlots() {
	for index := range graphs {
		if graphs[index].Plot != nil {
			graphs[index].Plot.SetXAxisLabelFunc(func(i int) string {
				return graphs[index].Query.Range.Start.Add(time.Duration(i) * graphs[index].Query.Range.Step).Format(HHMMSS24h)
			})

			if len(graphs[index].Labels) > 0 {
				graphs[index].Plot.SetFocusFunc(func() {
					mainView.AddItem(getLegends(graphs[index].Query.PromQL, graphs[index].Labels, graphs[index].Legends, graphs[index].Data), len(graphs[index].Legends)+3, 0, false)
				})
				graphs[index].Plot.SetBlurFunc(func() {
					if legendView != nil {
						mainView.RemoveItem(legendView)
					}
				})
			}

			if graphs[index].Data != nil {
				graphs[index].Plot.SetData(graphs[index].Data)
			}
		}
	}
}

func appendMetrics(query *Query, matrix *Matrix, index int) {
	// Skip if paused, query / matrix are invalid or when graph array changed in between
	if paused || query == nil || matrix == nil || index >= len(graphs) || graphs[index].Query.PromQL != query.PromQL {
		return
	}

	// update query info
	graphs[index].Query = *query

	// then update data
	if len(*matrix) > 0 {
		labels := []string{}
		legends := make([]map[string]string, len(*matrix))
		data := make([][]float64, len(*matrix))
		for i, s := range *matrix {
			legends[i] = map[string]string{}
			for k, v := range s.Metric {
				if !slices.Contains(labels, string(k)) {
					labels = append(labels, string(k))
				}
				legends[i][string(k)] = string(v)
			}

			data[i] = make([]float64, len(s.Values))
			for j, v := range s.Values {
				data[i][j] = float64(v.Value)
			}
		}

		sort.Strings(labels)
		graphs[index].Labels = labels
		graphs[index].Legends = legends
		graphs[index].Data = data
	} else {
		graphs[index].Data = [][]float64{}
	}
}
