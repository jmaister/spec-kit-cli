package tui

import "github.com/charmbracelet/lipgloss"

var (
	cAccent  = lipgloss.AdaptiveColor{Light: "#5A3FD9", Dark: "#9D8CFF"}
	cAccent2 = lipgloss.AdaptiveColor{Light: "#0F7B6C", Dark: "#4FD1B5"}
	cMuted   = lipgloss.AdaptiveColor{Light: "#7A7A85", Dark: "#7C7C8A"}
	cFaint   = lipgloss.AdaptiveColor{Light: "#C9C9D1", Dark: "#3A3A46"}
	cText    = lipgloss.AdaptiveColor{Light: "#1E1E24", Dark: "#E6E6EE"}
	cOK      = lipgloss.AdaptiveColor{Light: "#1A7F37", Dark: "#56D364"}
	cWarn    = lipgloss.AdaptiveColor{Light: "#9A6700", Dark: "#E3B341"}
	cErr     = lipgloss.AdaptiveColor{Light: "#CF222E", Dark: "#FF7B72"}
	cSelBg   = lipgloss.AdaptiveColor{Light: "#ECE8FF", Dark: "#2A2540"}

	sTitle    = lipgloss.NewStyle().Bold(true).Foreground(cAccent)
	sHeading  = lipgloss.NewStyle().Bold(true).Foreground(cText)
	sMuted    = lipgloss.NewStyle().Foreground(cMuted)
	sFaint    = lipgloss.NewStyle().Foreground(cFaint)
	sOK       = lipgloss.NewStyle().Foreground(cOK)
	sWarn     = lipgloss.NewStyle().Foreground(cWarn)
	sErr      = lipgloss.NewStyle().Foreground(cErr)
	sAccent   = lipgloss.NewStyle().Foreground(cAccent)
	sAccent2  = lipgloss.NewStyle().Foreground(cAccent2)
	sSelected = lipgloss.NewStyle().Background(cSelBg).Bold(true)
	sKey      = lipgloss.NewStyle().Foreground(cAccent2).Bold(true)
	sBadge    = lipgloss.NewStyle().Foreground(cAccent).Bold(true)

	sTabActive   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(cAccent).Padding(0, 1)
	sTabInactive = lipgloss.NewStyle().Foreground(cMuted).Padding(0, 1)

	sPane      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(cFaint).Padding(0, 1)
	sPaneFocus = sPane.BorderForeground(cAccent)

	sModal = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(cAccent).Padding(1, 2)
)

const (
	icDone    = "✔"
	icPending = "○"
	icPartial = "◐"
	icQueued  = "◌"
	icBoxOn   = "[x]"
	icBoxOff  = "[ ]"
)
