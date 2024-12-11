package cmd

import "slices"

type displayLoop struct {
	all     []displayLoopItem
	current int
}

type displayLoopItem struct {
	name    string
	columns []string
	group   []string
}

func (dl *displayLoop) prev() {
	dl.current += len(dl.all) - 1
	dl.current %= len(dl.all)
}

func (dl *displayLoop) next() {
	dl.current++
	dl.current %= len(dl.all)
}

func (dl *displayLoop) getCurrentItems() []displayLoopItem {
	current := dl.all[dl.current]
	if current.group == nil {
		return []displayLoopItem{current}
	}
	var items []displayLoopItem
	for _, item := range dl.all {
		if slices.Contains(current.group, item.name) {
			items = append(items, item)
		}
	}
	return items
}

func (dl *displayLoop) getNames() []string {
	var names []string
	items := dl.getCurrentItems()
	for _, item := range items {
		names = append(names, item.name)
	}
	return names
}

func (dl *displayLoop) getCols() []string {
	var cols []string
	items := dl.getCurrentItems()
	for _, item := range items {
		cols = append(cols, item.columns...)
	}
	return cols
}
