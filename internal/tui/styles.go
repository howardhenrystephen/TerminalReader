package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// 颜色常量
const (
	ColorBg        = "#1e1e2e"
	ColorText      = "#cdd6f4"
	ColorAccent    = "#89b4fa"
	ColorHighlight = "#f38ba8"
	ColorSuccess   = "#a6e3a1"
	ColorError     = "#f38ba8"
	ColorMuted     = "#6c7086"
	ColorSubtext   = "#a6adc8"
)

// 全局样式
var (
	BaseStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorText))

	TitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorAccent)).
			Bold(true).
			MarginLeft(2)

	DescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorSubtext)).
			MarginLeft(2)

	HelpKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorAccent)).
			Bold(true)

	HelpDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorSubtext))

	HelpSepStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorMuted))

	DialogBoxStyle = lipgloss.NewStyle().
			Padding(1, 2)

	ToastSuccessStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorSuccess)).
				Bold(true).
				Padding(0, 1)

	ToastErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorError)).
			Bold(true).
			Padding(0, 1)

	ReaderTextStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorText)).
			Padding(0, 2)

	ReaderHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorAccent)).
				Bold(true).
				Padding(0, 2)

	FooterStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorMuted)).
			Padding(0, 1)
)

// WindowSize 记录终端尺寸
type WindowSize struct {
	Width  int
	Height int
}
