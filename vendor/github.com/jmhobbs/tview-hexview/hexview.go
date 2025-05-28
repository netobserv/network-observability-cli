package hexview

import (
	"fmt"
	"math"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// HexView is a tview widget to display data in a hex viewer style.
// By default it responds to up/down/pgup/pgdn/home/end for moving
// through the data.  It is fixed width but can handle variable heights.
type HexView struct {
	*tview.Box

	data      []byte
	rows      int
	rowOffset int

	pageDown bool
	pageUp   bool

	// Colorize is a function to determine which color to make a given byte.
	// If not set, will use DefaultColorizer.
	Colorize func(byte) tcell.Color
}

func NewHexView(data []byte) *HexView {
	box := tview.NewBox()
	return &HexView{
		Box:      box,
		data:     data,
		rows:     int(math.Ceil(float64(len(data)) / 16)),
		Colorize: DefaultColorizer,
	}
}

// DefaultColorize returns colors for a byte as follows:
//   - Gray if NULL
//   - DarkCyan if an ASCII printable
//   - Green if an ASCII space
//   - Purple if other ASCII characters
//   - Yellow if non-ASCII
func DefaultColorizer(c byte) tcell.Color {
	if c == 0x00 { // NULL
		return tcell.ColorGray
	} else if c >= 33 && c <= 126 { // ASCII Printables
		return tcell.ColorDarkCyan
	} else if c == 9 || c == 10 || c == 13 || c == 32 { // ASCII Spaces
		return tcell.ColorGreen
	} else if c <= 127 { // Other ASCII
		return tcell.ColorPurple
	}
	return tcell.ColorYellow
}

func (h *HexView) printableCharacter(c byte) string {
	if c >= 32 && c <= 126 {
		return string([]byte{c})
	}
	return "_"
}

func (h *HexView) Draw(screen tcell.Screen) {
	h.Box.DrawForSubclass(screen, h)

	x, y, width, height := h.GetInnerRect()

	if height == 0 || width == 0 {
		return
	}

	if h.pageDown {
		h.rowOffset += height
		h.pageDown = false
	} else if h.pageUp {
		h.rowOffset = max(h.rowOffset-height, 0)
		h.pageUp = false
	}

	// don't allow overscroll
	if h.rowOffset > h.rows-height {
		h.rowOffset = max(h.rows-height, 0)
	}

	var (
		baseX       int = x + 1 // where do we start drawing a row
		offsetWidth int = 10    // how wide is the offset portion
		byteWidth   int = 3     // how wide is a byte representation (incl. the space)
		indexOffset int         // byte offset in the data for start of row
		baseY       int         // Y value for row
	)

	for row := 0; row < height; row++ {
		if row+h.rowOffset >= h.rows {
			break
		}

		indexOffset = (row + h.rowOffset) * 16
		baseY = y + row

		// print offset, 8 hex digits
		tview.Print(screen, fmt.Sprintf("%08x |", indexOffset), baseX, baseY, offsetWidth, tview.AlignLeft, tcell.ColorDarkGray)

		// first 8 bytes
		for j := 0; j < 8; j++ {
			index := indexOffset + j
			if index >= len(h.data) {
				break
			}
			// hex pairs
			tview.Print(screen, fmt.Sprintf(" %02x", h.data[index]), baseX+offsetWidth+(j*byteWidth), baseY, byteWidth, tview.AlignLeft, h.Colorize(h.data[index]))
			// ascii
			tview.Print(screen, h.printableCharacter(h.data[index]), baseX+offsetWidth+4+j+(16*byteWidth), baseY, byteWidth, tview.AlignLeft, h.Colorize(h.data[index]))
		}

		// divider: byte blocks
		tview.Print(screen, " ┊", baseX+offsetWidth+(byteWidth*8), baseY, 2, tview.AlignLeft, tcell.ColorWhite)

		// second 8 bytes
		for j := 8; j < 16; j++ {
			index := indexOffset + j
			if index >= len(h.data) {
				break
			}
			// hex pairs
			tview.Print(screen, fmt.Sprintf(" %02x", h.data[index]), baseX+offsetWidth+2+(j*byteWidth), baseY, byteWidth, tview.AlignLeft, h.Colorize(h.data[index]))
			// ascii
			tview.Print(screen, h.printableCharacter(h.data[index]), baseX+offsetWidth+5+j+(16*byteWidth), baseY, byteWidth, tview.AlignLeft, h.Colorize(h.data[index]))
		}

		// divider: byte blocks from ascii
		tview.Print(screen, " |", baseX+offsetWidth+2+(byteWidth*16), baseY, 2, tview.AlignLeft, tcell.ColorWhite)
		// divider: ascii blocks
		tview.Print(screen, "┊", baseX+offsetWidth+12+(byteWidth*16), baseY, 1, tview.AlignLeft, tcell.ColorWhite)
		// divider: ascii close
		tview.Print(screen, "|", baseX+offsetWidth+20+(byteWidth*16), baseY, 1, tview.AlignLeft, tcell.ColorWhite)
	}
}

// SetData will replace the contents of the current data slice with a new one.
// After updating, you will need to queue a Draw.
func (h *HexView) SetData(data []byte) *HexView {
	h.data = data
	h.rows = int(math.Ceil(float64(len(data)) / 16))
	return h
}

func (h *HexView) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return h.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		switch event.Key() {
		case tcell.KeyUp:
			h.rowOffset = max(h.rowOffset-1, 0)
		case tcell.KeyDown:
			h.rowOffset = min(h.rowOffset+1, h.rows)
		case tcell.KeyHome:
			h.rowOffset = 0
		case tcell.KeyEnd:
			h.rowOffset = h.rows
		// todo: not sure how best to handle pages since we don't have height here.
		// Currently we just set this flag and fix it in Draw()
		case tcell.KeyPgDn:
			h.pageDown = true
		case tcell.KeyPgUp:
			h.pageUp = true
		}
	})
}

func (h *HexView) MouseHandler() func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (consumed bool, capture tview.Primitive) {
	return h.WrapMouseHandler(func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (consumed bool, capture tview.Primitive) {
		return false, nil
	})
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
