package display

import (
	"time"
)

// Runtime coordinates display refreshes without depending on collector state.
// The collector supplies the small callbacks needed for capture-specific work.
type Runtime struct {
	Ended       func() bool
	IsMetrics   func() bool
	FPS         func() int
	UpdateState func()
	UpdateData  func()
	Draw        func()
	PrintRows   func()
}

func (r Runtime) Heartbeat() {
	for {
		if r.Ended() {
			return
		}
		r.UpdateState()
		r.UpdateData()
		r.Draw()
		time.Sleep(time.Second / time.Duration(max(1, r.FPS())))
	}
}

func (r Runtime) BackgroundHeartbeat() {
	for {
		if r.Ended() {
			return
		}
		if !r.IsMetrics() {
			r.UpdateData()
			r.PrintRows()
		}
		time.Sleep(5 * time.Second)
	}
}
