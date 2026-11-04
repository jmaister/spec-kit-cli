package tui

import "github.com/charmbracelet/lipgloss"

var (
	cAccent  = lipgloss.AdaptiveColor{Light: "#0B63C4", Dark: "#61AFEF"}
	cAccent2 = lipgloss.AdaptiveColor{Light: "#0F7B6C", Dark: "#4FD1B5"}
	cMuted   = lipgloss.AdaptiveColor{Light: "#7A7A85", Dark: "#7C7C8A"}
	cFaint   = lipgloss.AdaptiveColor{Light: "#C9C9D1", Dark: "#3A3A46"}
	cText    = lipgloss.AdaptiveColor{Light: "#1E1E24", Dark: "#E6E6EE"}
	cOK      = lipgloss.AdaptiveColor{Light: "#1A7F37", Dark: "#56D364"}
	cWarn    = lipgloss.AdaptiveColor{Light: "#9A6700", Dark: "#E3B341"}
	cErr     = lipgloss.AdaptiveColor{Light: "#CF222E", Dark: "#FF7B72"}
	cSelBg   = lipgloss.AdaptiveColor{Light: "#DCE8F7", Dark: "#24303F"}
	cTabFg   = lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#0B1220"}
	// Pane borders are always visible; the focused one is brighter.
	cBorder      = lipgloss.AdaptiveColor{Light: "#A3A3AD", Dark: "#5C5C68"}
	cBorderFocus = lipgloss.AdaptiveColor{Light: "#1E1E24", Dark: "#F2F2F7"}

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
	sHint     = lipgloss.NewStyle().Foreground(cText)
	sBadge    = lipgloss.NewStyle().Foreground(cAccent).Bold(true)

	sTabActive   = lipgloss.NewStyle().Bold(true).Foreground(cTabFg).Background(cAccent).Padding(0, 1)
	sTabInactive = lipgloss.NewStyle().Foreground(cMuted).Padding(0, 1)

	sPane      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(cBorder).Padding(0, 1)
	sPaneFocus = sPane.BorderForeground(cBorderFocus)

	sModal = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(cBorderFocus).Padding(1, 2)
)

const (
	icDone    = "✔"
	icPending = "○"
	icPartial = "◐"
	icQueued  = "◌"
	icBoxOn   = "[x]"
	icBoxOff  = "[ ]"
)
