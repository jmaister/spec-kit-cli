package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	NextScreen, PrevScreen      key.Binding
	Help, Logs, New, Model      key.Binding
	Agent, Perm, Refresh, Init  key.Binding
	Edit, Cancel, Quit          key.Binding
	Up, Down, Top, Bottom       key.Binding
	PageUp, PageDown            key.Binding
	Enter, Back                 key.Binding
	Left, Right, NextStep       key.Binding
	Implement                   key.Binding
	Toggle, RunTask, RunPhase   key.Binding
	HideDone, Follow, ClearJobs key.Binding
}

func newKeyMap() keyMap {
	b := func(keys []string, k, desc string) key.Binding {
		return key.NewBinding(key.WithKeys(keys...), key.WithHelp(k, desc))
	}
	return keyMap{
		NextScreen: b([]string{"tab"}, "tab", "next screen"),
		PrevScreen: b([]string{"shift+tab"}, "S-tab", "prev screen"),
		Help:       b([]string{"h", "?"}, "h/?", "help"),
		Logs:       b([]string{"l"}, "l", "logs"),
		New:        b([]string{"n"}, "n", "new feature"),
		Model:      b([]string{"m"}, "m", "model"),
		Agent:      b([]string{"a"}, "a", "agent"),
		Perm:       b([]string{"p"}, "p", "permission mode"),
		Refresh:    b([]string{"r"}, "r", "refresh"),
		Init:       b([]string{"I"}, "I", "init spec-kit"),
		Edit:       b([]string{"e"}, "e", "open in $EDITOR"),
		Cancel:     b([]string{"ctrl+x"}, "^x", "cancel job"),
		Quit:       b([]string{"q", "ctrl+c"}, "q", "quit"),
		Up:         b([]string{"up", "k"}, "↑/k", "up"),
		Down:       b([]string{"down", "j"}, "↓/j", "down"),
		Top:        b([]string{"g", "home"}, "g", "top"),
		Bottom:     b([]string{"G", "end"}, "G", "bottom"),
		PageUp:     b([]string{"pgup", "ctrl+u"}, "pgup", "page up"),
		PageDown:   b([]string{"pgdown", "ctrl+d"}, "pgdn", "page down"),
		Enter:      b([]string{"enter"}, "enter", "open / run"),
		Back:       b([]string{"esc"}, "esc", "back"),
		Left:       b([]string{"left"}, "←", "features"),
		Right:      b([]string{"right"}, "→", "workflow steps"),
		NextStep:   b([]string{"s"}, "s", "run next step"),
		Implement:  b([]string{"i"}, "i", "implement feature"),
		Toggle:     b([]string{" "}, "space", "toggle task"),
		RunTask:    b([]string{"x"}, "x", "run task"),
		RunPhase:   b([]string{"X"}, "X", "run phase"),
		HideDone:   b([]string{"f"}, "f", "hide done"),
		Follow:     b([]string{"F"}, "F", "follow"),
		ClearJobs:  b([]string{"c"}, "c", "clear finished"),
	}
}
