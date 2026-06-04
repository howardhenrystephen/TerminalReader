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
		key.WithKeys("q", "ctrl+c"),
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
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter", "l", "o"),
		key.WithHelp("enter/l/o", "open"),
	),
	Search: key.NewBinding(
		key.WithKeys("s", "/"),
		key.WithHelp("s//", "search"),
	),
	Delete: key.NewBinding(
		key.WithKeys("d", "delete", "x"),
		key.WithHelp("d/del/x", "delete"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("m", "r"),
		key.WithHelp("m/r", "refresh"),
	),
	GoTop: key.NewBinding(
		key.WithKeys("g", "home"),
		key.WithHelp("g/home", "top"),
	),
	GoBottom: key.NewBinding(
		key.WithKeys("G", "end"),
		key.WithHelp("G/end", "bottom"),
	),
	Desc: key.NewBinding(
		key.WithKeys("tab", "i"),
		key.WithHelp("tab/i", "description"),
	),
	Pin: key.NewBinding(
		key.WithKeys("p", "P"),
		key.WithHelp("p/P", "pin"),
	),
	Redraw: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "redraw"),
	),
	Continue: key.NewBinding(
		key.WithKeys("c", "t"),
		key.WithHelp("c/t", "continue download"),
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
		key.WithKeys("f", "F"),
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
}

// ReaderKeys 阅读器快捷键实例
var ReaderKeys = ReaderKeyMap{
	ScrollUp: key.NewBinding(
		key.WithKeys("up", "k", "w"),
		key.WithHelp("↑/k/w", "scroll up"),
	),
	ScrollDown: key.NewBinding(
		key.WithKeys("down", "j", "s"),
		key.WithHelp("↓/j/s", "scroll down"),
	),
	PageUp: key.NewBinding(
		key.WithKeys("b", "pgup", "B"),
		key.WithHelp("b/B/pgup", "page up"),
	),
	PageDown: key.NewBinding(
		key.WithKeys(" ", "f", "pgdown", "F"),
		key.WithHelp("space/f/F/pgdn", "page down"),
	),
	PrevChapter: key.NewBinding(
		key.WithKeys("left", "h", "p", "H"),
		key.WithHelp("←/h/H/p", "prev chapter"),
	),
	NextChapter: key.NewBinding(
		key.WithKeys("right", "l", "n", "L"),
		key.WithHelp("→/l/L/n", "next chapter"),
	),
	GoStart: key.NewBinding(
		key.WithKeys("g", "home"),
		key.WithHelp("g/home", "chapter start"),
	),
	GoEnd: key.NewBinding(
		key.WithKeys("G", "end"),
		key.WithHelp("G/end", "chapter end"),
	),
	ChapterPicker: key.NewBinding(
		key.WithKeys("c", "C", "tab"),
		key.WithHelp("c/C/tab", "chapters"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc", "q", "Q"),
		key.WithHelp("esc/q/Q", "back"),
	),
	Bookmark: key.NewBinding(
		key.WithKeys("m", "M", "b"),
		key.WithHelp("m/M/b", "bookmark"),
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
		key.WithKeys("esc", "ctrl+c", "q", "Q"),
		key.WithHelp("esc/q", "close"),
	),
	Confirm: key.NewBinding(
		key.WithKeys("enter", "l", "L"),
		key.WithHelp("enter/l", "jump"),
	),
	Up: key.NewBinding(
		key.WithKeys("up", "k", "w", "W"),
		key.WithHelp("↑/k/w", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j", "s", "S"),
		key.WithHelp("↓/j/s", "down"),
	),
	PageUp: key.NewBinding(
		key.WithKeys("left", "h", "pgup"),
		key.WithHelp("←/h/pgup", "page up"),
	),
	PageDown: key.NewBinding(
		key.WithKeys("right", "l", "pgdown"),
		key.WithHelp("→/l/pgdn", "page down"),
	),
	Filter: key.NewBinding(
		key.WithKeys("/", "f", "F"),
		key.WithHelp("//f", "filter"),
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
		key.WithKeys("esc", "ctrl+c", "q", "Q"),
		key.WithHelp("esc/q", "close"),
	),
	Confirm: key.NewBinding(
		key.WithKeys("enter", "l", "L"),
		key.WithHelp("enter/l", "confirm"),
	),
	Up: key.NewBinding(
		key.WithKeys("up", "k", "w", "W"),
		key.WithHelp("↑/k/w", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j", "s", "S"),
		key.WithHelp("↓/j/s", "down"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab", "t", "T"),
		key.WithHelp("tab/t", "switch"),
	),
	Background: key.NewBinding(
		key.WithKeys("b", "B", "bg"),
		key.WithHelp("b", "background download"),
	),
}
