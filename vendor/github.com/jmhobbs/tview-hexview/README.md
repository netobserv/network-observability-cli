# tview-hexview

A simple hexviewer widget for [tview](https://github.com/rivo/tview)

It supports basic keyboard navigation, up, down, page up, page down, home, end.

![Screenshot of HexView widget](https://github.com/jmhobbs/tview-hexview/raw/doc/screenshot.png)

## Example

```golang
package main

import (
	"io/ioutil"
	"os"

	hex "github.com/jmhobbs/tview-hexview"
	"github.com/rivo/tview"
)

func main() {

	bytes, err := ioutil.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}

	grid := tview.NewGrid().
		SetRows(5, 0, 5).
		SetColumns(0).
		SetBorders(true)

	over := tview.NewTextView().SetText("Widgets Over").SetTextAlign(tview.AlignCenter)
	hex := hex.NewHexView(bytes)
	under := tview.NewTextView().SetText("Widgets Under").SetTextAlign(tview.AlignCenter)

	grid.AddItem(over, 0, 0, 1, 1, 0, 0, false)
	grid.AddItem(hex, 1, 0, 1, 1, 0, 0, true)
	grid.AddItem(under, 2, 0, 1, 1, 0, 0, false)

	app := tview.NewApplication().SetRoot(grid, true)
	if err := app.Run(); err != nil {
		panic(err)
	}
}
```

## Todo / Future Enhancements

- Better tests
- Improved outputs
  - offset on/off
  - byte group count
  - output base (hex, dec, etc)
  - color on/off
  - ascii view on/off
- Optionally take data from a io.ReadSeeker
