package tui

import "github.com/charmbracelet/lipgloss"

var (
	Orange     = lipgloss.Color("#FF7700")
	Yellow     = lipgloss.Color("#FFB347")
	Gold       = lipgloss.Color("#FFD700")
	Green      = lipgloss.Color("#00CC66")
	Red        = lipgloss.Color("#FF4444")
	WarmWhite  = lipgloss.Color("#F0E6D0")
	Grey       = lipgloss.Color("#888888")

	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Orange).
			Padding(0, 1)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(Yellow).
			Padding(0, 1)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(Green).
			Bold(true)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(Red).
			Bold(true)

	InfoStyle = lipgloss.NewStyle().
			Foreground(WarmWhite)

	MutedStyle = lipgloss.NewStyle().
			Foreground(Grey)

	HighlightStyle = lipgloss.NewStyle().
			Foreground(Gold)

	ProgressBarEmpty = lipgloss.Color("#333333")
	ProgressBarFill  = Orange

	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Orange).
			Padding(1, 2)

	spinnerStyle = lipgloss.NewStyle().Foreground(Orange)
)
