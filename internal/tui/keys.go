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
	Up       key.Binding
	Down     key.Binding
	Enter    key.Binding
	Search   key.Binding
	Delete   key.Binding
	Refresh  key.Binding
	GoTop    key.Binding
	GoBottom key.Binding
	Desc     key.Binding
	Pin      key.Binding
	Redraw   key.Binding
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
		key.WithKeys("enter", "l"),
		key.WithHelp("enter/l", "open"),
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
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "scroll up"),
	),
	ScrollDown: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "scroll down"),
	),
	PageUp: key.NewBinding(
		key.WithKeys("b", "pgup"),
		key.WithHelp("b/pgup", "page up"),
	),
	PageDown: key.NewBinding(
		key.WithKeys(" ", "f", "pgdown"),
		key.WithHelp("space/f/pgdn", "page down"),
	),
	PrevChapter: key.NewBinding(
		key.WithKeys("left", "h", "p"),
		key.WithHelp("←/h/p", "prev chapter"),
	),
	NextChapter: key.NewBinding(
		key.WithKeys("right", "l", "n"),
		key.WithHelp("→/l/n", "next chapter"),
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
		key.WithKeys("esc", "q"),
		key.WithHelp("esc/q", "back"),
	),
	Bookmark: key.NewBinding(
		key.WithKeys("m"),
		key.WithHelp("m", "bookmark"),
	),
}

// ChapterPickerKeyMap 章节选择快捷键
type ChapterPickerKeyMap struct {
	Close   key.Binding
	Confirm key.Binding
	Up      key.Binding
	Down    key.Binding
	Filter  key.Binding
}

// ChapterPickerKeys 章节选择快捷键实例
var ChapterPickerKeys = ChapterPickerKeyMap{
	Close: key.NewBinding(
		key.WithKeys("esc", "ctrl+c"),
		key.WithHelp("esc", "close"),
	),
	Confirm: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "jump"),
	),
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
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
		key.WithKeys("esc", "ctrl+c"),
		key.WithHelp("esc", "close"),
	),
	Confirm: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "confirm"),
	),
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
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
