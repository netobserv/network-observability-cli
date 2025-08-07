package cmd

import (
	"fmt"
	"time"

	"github.com/jpillora/sizestr"
	"github.com/rivo/tview"
)

const (
	defaultFramesPerSecond = 5 // frames per second
)

var (
	app             *tview.Application
	pages           *tview.Pages
	mainView        *tview.Flex
	playPauseButton *tview.Button

	durationText = tview.NewTextView()
	sizeText     = tview.NewTextView()

	showCount          = 1
	framesPerSecond    = defaultFramesPerSecond
	showPopup          bool
	paused             = false
	errAdvancedDisplay error
	focus              = ""
)

func getPages() *tview.Pages {
	if capture == Metric {
		pages = tview.NewPages().AddPage("main", getMetricMain(), true, true)

		if showPopup {
			pages = pages.AddPage("modal", getMetricsModal(), true, true)
		}
	} else {
		pages = tview.NewPages().AddPage("main", getFlowMain(), true, true)

		if showPopup {
			pages = pages.AddPage("modal", getColumnsModal(), true, true)
		}
	}

	return pages
}

// Returns a new primitive which puts the provided primitive in the center and
// sets its size to the given width and height.
func getModal(p tview.Primitive, width, height int) tview.Primitive {
	return tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(p, height, 1, true).
			AddItem(nil, 0, 1, false), width, 1, true).
		AddItem(nil, 0, 1, false)
}

func getInfoRow() tview.Primitive {
	playPauseButton = tview.NewButton(getPlayPauseText()).SetSelectedFunc(func() {
		pause(!paused)
	})
	infoRow := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(tview.NewTextView().SetText(getcaptureText()), 0, 1, false).
		AddItem(playPauseButton, 10, 0, false)
	if logLevel != "info" {
		infoRow.AddItem(tview.NewTextView().SetText(getLogLevelText()), 0, 1, false)
	}
	infoRow.AddItem(durationText.SetText(getDurationText()).SetTextAlign(tview.AlignCenter), 0, 1, false)
	if capture != Metric {
		infoRow.AddItem(sizeText.SetText(getSizeText()).SetTextAlign(tview.AlignCenter), 0, 1, false)
	}
	if logLevel == "debug" {
		fpsText := tview.NewTextView().SetText(getFPSText()).SetTextAlign(tview.AlignCenter)
		infoRow.
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
	}
	infoRow.AddItem(tview.NewTextView(), 16, 0, false)
	return infoRow
}

func getCountRow() tview.Primitive {
	countTextView := tview.NewTextView().SetText(getShowCountText())
	countRow := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(countTextView, 0, 1, false).
		AddItem(tview.NewButton("-").SetSelectedFunc(func() {
			if showCount > 5 {
				if capture == Metric {
					showCount -= 5
				} else {
					showCount--
				}
			}
			countTextView.SetText(getShowCountText())
			updateScreen()
		}), 5, 0, false).
		AddItem(tview.NewButton("+").SetSelectedFunc(func() {
			if capture == Metric {
				showCount += 5
			} else {
				showCount++
			}
			countTextView.SetText(getShowCountText())
			updateScreen()
		}), 5, 0, false).
		AddItem(tview.NewTextView(), 0, 2, false)
	countRow.AddItem(tview.NewTextView(), 16, 0, false)
	return countRow
}

func getShowCountText() string {
	if capture == Metric {
		return getMetricShowCountText()
	}
	return getFlowShowCountText()
}

func getFPSText() string {
	return fmt.Sprintf("FPS: %d", framesPerSecond)
}

func getPlayPauseText() string {
	if paused {
		return "⏸︎"
	}
	return "⏵︎"
}

func getDurationText() string {
	duration := currentTime().Sub(startupTime)
	return fmt.Sprintf("Duration: %s ", duration.Round(time.Second))
}

func getSizeText() string {
	return fmt.Sprintf("Capture size: %s", sizestr.ToString(totalBytes))
}

func updateStatusTexts() {
	durationText.SetText(getDurationText())
	sizeText.SetText(getSizeText())
}

func hearbeat() {
	for {
		if captureEnded {
			return
		}

		updateStatusTexts()
		if capture == Metric {
			updatePlots()
		} else {
			updateTableAndSuggestions()
		}

		// refresh
		if app != nil {
			app.Draw()
		}

		time.Sleep(time.Second / time.Duration(framesPerSecond))
	}
}

func pause(pause bool) {
	paused = pause
	playPauseButton.SetLabel(getPlayPauseText())
	updateScreen()
}

func updateScreen() {
	if app != nil {
		showPopup = false
		app.SetRoot(getPages(), true)
	}
}
