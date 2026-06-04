package tui

import "github.com/charmbracelet/bubbles/key"

// GlobalKeyMap 全局快捷键
type GlobalKeyMap struct {
	Quit key.Binding
	Help key.Binding
}

// GlobalKeys 全局快捷键实例
var GlobalKeys = GlobalKeyMap{
	Quit: key.NewBinding(
		key.WithKeys("q"),
		key.WithHelp("q", "quit"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
}

// BookshelfKeyMap 书架快捷键
type BookshelfKeyMap struct {
	Up          key.Binding
	Down        key.Binding
	Enter       key.Binding
	Search      key.Binding
	Delete      key.Binding
	Refresh     key.Binding
	GoTop       key.Binding
	GoBottom    key.Binding
	Desc        key.Binding
	Pin         key.Binding
	Redraw      key.Binding
	Continue    key.Binding
	StopCrawl   key.Binding
	Nuke        key.Binding
	FillMissing key.Binding
}

// BookshelfKeys 书架快捷键实例
var BookshelfKeys = BookshelfKeyMap{
	Up: key.NewBinding(
		key.WithKeys("up"),
		key.WithHelp("↑", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down"),
		key.WithHelp("↓", "down"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "open"),
	),
	Search: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "search"),
	),
	Delete: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "delete"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "refresh"),
	),
	GoTop: key.NewBinding(
		key.WithKeys("g"),
		key.WithHelp("g", "top"),
	),
	GoBottom: key.NewBinding(
		key.WithKeys("G"),
		key.WithHelp("G", "bottom"),
	),
	Desc: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "description"),
	),
	Pin: key.NewBinding(
		key.WithKeys("p"),
		key.WithHelp("p", "pin"),
	),
	Redraw: key.NewBinding(
		key.WithKeys("R"),
		key.WithHelp("R", "redraw"),
	),
	Continue: key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", "continue download"),
	),
	StopCrawl: key.NewBinding(
		key.WithKeys("x"),
		key.WithHelp("x", "stop download"),
	),
	Nuke: key.NewBinding(
		key.WithKeys("T"),
		key.WithHelp("T", "clear all data"),
	),
	FillMissing: key.NewBinding(
		key.WithKeys("f"),
		key.WithHelp("f", "fill missing"),
	),
}

// ReaderKeyMap 阅读器快捷键
type ReaderKeyMap struct {
	ScrollUp      key.Binding
	ScrollDown    key.Binding
	PageUp        key.Binding
	PageDown      key.Binding
	PrevChapter   key.Binding
	NextChapter   key.Binding
	GoStart       key.Binding
	GoEnd         key.Binding
	ChapterPicker key.Binding
	Back          key.Binding
	Bookmark      key.Binding
	FillMissing   key.Binding
}

// ReaderKeys 阅读器快捷键实例
var ReaderKeys = ReaderKeyMap{
	ScrollUp: key.NewBinding(
		key.WithKeys("up"),
		key.WithHelp("↑", "scroll up"),
	),
	ScrollDown: key.NewBinding(
		key.WithKeys("down"),
		key.WithHelp("↓", "scroll down"),
	),
	PageUp: key.NewBinding(
		key.WithKeys("b"),
		key.WithHelp("b", "page up"),
	),
	PageDown: key.NewBinding(
		key.WithKeys(" "),
		key.WithHelp("space", "page down"),
	),
	PrevChapter: key.NewBinding(
		key.WithKeys("left"),
		key.WithHelp("←", "prev chapter"),
	),
	NextChapter: key.NewBinding(
		key.WithKeys("right"),
		key.WithHelp("→", "next chapter"),
	),
	GoStart: key.NewBinding(
		key.WithKeys("g"),
		key.WithHelp("g", "chapter start"),
	),
	GoEnd: key.NewBinding(
		key.WithKeys("G"),
		key.WithHelp("G", "chapter end"),
	),
	ChapterPicker: key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", "chapters"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back"),
	),
	Bookmark: key.NewBinding(
		key.WithKeys("m"),
		key.WithHelp("m", "bookmark"),
	),
	FillMissing: key.NewBinding(
		key.WithKeys("f"),
		key.WithHelp("f", "fill missing"),
	),
}

// ChapterPickerKeyMap 章节选择快捷键
type ChapterPickerKeyMap struct {
	Close    key.Binding
	Confirm  key.Binding
	Up       key.Binding
	Down     key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Filter   key.Binding
}

// ChapterPickerKeys 章节选择快捷键实例
var ChapterPickerKeys = ChapterPickerKeyMap{
	Close: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "close"),
	),
	Confirm: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "jump"),
	),
	Up: key.NewBinding(
		key.WithKeys("up"),
		key.WithHelp("↑", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down"),
		key.WithHelp("↓", "down"),
	),
	PageUp: key.NewBinding(
		key.WithKeys("left"),
		key.WithHelp("←", "page up"),
	),
	PageDown: key.NewBinding(
		key.WithKeys("right"),
		key.WithHelp("→", "page down"),
	),
	Filter: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "filter"),
	),
}

// SearchKeyMap 搜索弹层快捷键
type SearchKeyMap struct {
	Close      key.Binding
	Confirm    key.Binding
	Up         key.Binding
	Down       key.Binding
	Tab        key.Binding
	Background key.Binding
}

// SearchKeys 搜索弹层快捷键实例
var SearchKeys = SearchKeyMap{
	Close: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "close"),
	),
	Confirm: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "confirm"),
	),
	Up: key.NewBinding(
		key.WithKeys("up"),
		key.WithHelp("↑", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down"),
		key.WithHelp("↓", "down"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "switch"),
	),
	Background: key.NewBinding(
		key.WithKeys("b"),
		key.WithHelp("b", "background download"),
	),
}
