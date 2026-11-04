package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/jmaister/spec-kit-cli/internal/agent"
	"github.com/jmaister/spec-kit-cli/internal/speckit"
)

func (m *Model) bodyHeight() int { return max(6, m.h-4) }

func (m *Model) View() string {
	if m.w == 0 {
		return "loading…"
	}
	h := m.bodyHeight()
	var body string
	switch m.overlay {
	case ovHelp:
		body = m.place(h, m.viewHelp())
	case ovInput:
		body = m.place(h, m.viewInput())
	case ovPicker:
		body = m.place(h, m.viewPicker())
	case ovConfirm:
		body = m.place(h, sModal.Render(m.confirmMsg+"\n\n"+sKey.Render("y")+" yes   "+sKey.Render("n")+" no"))
	default:
		switch m.screen {
		case scrDashboard:
			body = m.viewDashboard(h)
		case scrTasks:
			body = m.viewTasks(h)
		case scrLogs:
			body = m.viewLogs(h)
		}
	}
	body = clipLines(body, h)
	if n := lipgloss.Height(body); n < h {
		body += strings.Repeat("\n", h-n)
	}
	return m.viewHeader() + "\n" + body + "\n" + m.viewFooter()
}

func (m *Model) place(h int, s string) string {
	return lipgloss.Place(m.w, h, lipgloss.Center, lipgloss.Center, s)
}

// ---------- header / footer ----------

func (m *Model) viewHeader() string {
	name := filepath.Base(m.proj.Root)
	left := sTitle.Render("◆ speckit") + " " + sMuted.Render(name)
	ag := m.agent()
	agentS := ag.Name()
	if !ag.Available() {
		agentS = sErr.Render(agentS + " (missing)")
	}
	right := sMuted.Render("agent ") + agentS +
		sMuted.Render("  model ") + sAccent.Render(displayModel(m.cfg.Model(ag.ID())))
	if pm := m.permMode(); pm != "" {
		right += sMuted.Render("  perm ") + permStyle(pm).Render(pm)
	}
	if n := m.jobs.RunningCount(); n > 0 {
		right += "  " + m.spin.View() + sAccent2.Render(fmt.Sprintf(" %d running", n))
	}
	line1 := spread(left, right, m.w)

	var tabs []string
	for i, n := range screenNames {
		label := fmt.Sprintf("%s", n)
		if screen(i) == m.screen {
			tabs = append(tabs, sTabActive.Render(label))
		} else {
			tabs = append(tabs, sTabInactive.Render(label))
		}
	}
	return line1 + "\n" + strings.Join(tabs, sFaint.Render("│"))
}

func permStyle(pm string) lipgloss.Style {
	switch pm {
	case "bypassPermissions":
		return sErr
	case "plan":
		return sMuted
	}
	return sWarn
}

func (m *Model) viewFooter() string {
	var flash string
	if m.flash != "" {
		if m.flashErr {
			flash = sErr.Render(m.flash)
		} else {
			flash = sAccent2.Render(m.flash)
		}
	}
	var hints [][2]string
	switch m.screen {
	case scrDashboard:
		hints = [][2]string{{"↑↓", "feature"}, {"enter", "tasks"}, {"1-8", "run step"}, {"i", "implement"}, {"n", "new"}, {"e", "edit"}}
	case scrTasks:
		hints = [][2]string{{"↑↓", "move"}, {"space", "toggle"}, {"x", "run task"}, {"X", "run phase"}, {"enter", "fold"}, {"f", "hide done"}, {"e", "edit"}}
	case scrLogs:
		hints = [][2]string{{"↑↓", "job"}, {"pgup/pgdn", "scroll"}, {"F", "follow"}, {"c", "clear"}, {"^x", "cancel"}}
	}
	hints = append(hints, [][2]string{{"tab", "screen"}, {"m", "model"}, {"a", "agent"}, {"l", "logs"}, {"h", "help"}, {"q", "quit"}}...)
	var parts []string
	for _, h := range hints {
		parts = append(parts, sKey.Render(h[0])+" "+sMuted.Render(h[1]))
	}
	return truncate(flash, m.w) + "\n" + truncate(strings.Join(parts, sFaint.Render(" · ")), m.w)
}

// ---------- dashboard ----------

func (m *Model) viewDashboard(h int) string {
	if !m.proj.Initialized {
		msg := sHeading.Render("This directory is not a spec-kit project") + "\n\n" +
			sMuted.Render(truncate(m.proj.Root, m.w-12)) + "\n\n" +
			"Press " + sKey.Render("I") + " to run " + sAccent.Render("specify init --here --ai "+m.agent().InitAI()) + "\n" +
			"Press " + sKey.Render("a") + " to choose another agent, " + sKey.Render("h") + " for help."
		return m.place(h, sModal.Render(msg))
	}
	lw := clamp(m.w*38/100, 30, 56)
	left := sPane.Width(lw - 2).Height(h - 2).Render(clipLines(m.viewFeatureList(lw-4, h-2), h-2))
	right := sPaneFocus.Width(m.w - lw - 4).Height(h - 2).Render(clipLines(m.viewFeatureDetail(m.w-lw-6), h-2))
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (m *Model) viewFeatureList(w, h int) string {
	var lines []string
	cons := sOK.Render(icDone + " constitution")
	if m.stepRunning("", "constitution") {
		cons = m.spin.View() + sAccent2.Render(" constitution")
	} else if !m.proj.Constitution {
		cons = sWarn.Render(icPending+" constitution missing") + sMuted.Render(" (1)")
	}
	lines = append(lines, sHeading.Render(fmt.Sprintf("Features (%d)", len(m.proj.Features))), cons, "")
	if m.stepRunning("", "specify") {
		lines = append(lines, m.spin.View()+sAccent2.Render(" creating new feature…"), "")
	}
	if len(m.proj.Features) == 0 {
		lines = append(lines, sMuted.Render("No features yet."), "", "Press "+sKey.Render("n")+" to specify one.")
	}
	header := len(lines)
	per := 3
	visible := max(1, (h-header)/per)
	start := 0
	if m.featIdx >= visible {
		start = m.featIdx - visible + 1
	}
	for i := start; i < len(m.proj.Features) && i < start+visible; i++ {
		f := m.proj.Features[i]
		running := len(m.runningJobsFor(f.Name)) > 0
		icon := " "
		if running {
			icon = m.spin.View()
		}
		name := truncate(f.Name, w-4)
		var row1 string
		if i == m.featIdx {
			row1 = sSelected.Width(w).Render(sAccent.Render("▸ ") + name + " " + icon)
		} else {
			row1 = "  " + name + " " + icon
		}
		row2 := "  " + m.stepIcons(f) + "  " + m.taskBar(f, max(6, w-22))
		lines = append(lines, row1, row2, "")
	}
	if start+visible < len(m.proj.Features) {
		lines = append(lines, sMuted.Render(fmt.Sprintf("  ↓ %d more", len(m.proj.Features)-start-visible)))
	}
	return strings.Join(lines, "\n")
}

// stepIcons renders a compact pipeline: one glyph per workflow step.
func (m *Model) stepIcons(f *speckit.Feature) string {
	var b strings.Builder
	for _, s := range speckit.Steps {
		b.WriteString(m.stepGlyph(f, s))
	}
	return b.String()
}

func (m *Model) stepGlyph(f *speckit.Feature, s speckit.Step) string {
	if m.stepRunning(f.Name, s.ID) && !s.ProjectScope {
		return m.spin.View()
	}
	switch m.proj.Status(f, s.ID) {
	case speckit.StepDone:
		return sOK.Render(icDone)
	case speckit.StepPartial:
		return sWarn.Render(icPartial)
	}
	if s.Optional {
		return sFaint.Render(icPending)
	}
	return sMuted.Render(icPending)
}

func (m *Model) taskBar(f *speckit.Feature, w int) string {
	done, total := f.Tasks.Progress()
	if total == 0 {
		return sFaint.Render("no tasks")
	}
	return bar(float64(done)/float64(total), w) + sMuted.Render(fmt.Sprintf(" %d/%d", done, total))
}

func (m *Model) viewFeatureDetail(w int) string {
	f := m.feature()
	var lines []string
	if f == nil {
		lines = append(lines, sHeading.Render("Getting started"), "")
		for i, s := range speckit.Steps[:2] {
			lines = append(lines, m.stepLine(nil, i, s, w))
		}
		lines = append(lines, "", sMuted.Render("Press ")+sKey.Render("n")+sMuted.Render(" to describe your first feature."))
		return strings.Join(lines, "\n")
	}
	lines = append(lines, sTitle.Render(f.Name), sFaint.Render(truncate(relPath(m.proj.Root, f.Dir), w)), "")
	lines = append(lines, sHeading.Render("Workflow"))
	for i, s := range speckit.Steps {
		lines = append(lines, m.stepLine(f, i, s, w))
	}
	if next := m.proj.NextStep(f); next != "" {
		idx := 0
		for i, s := range speckit.Steps {
			if s.ID == next {
				idx = i + 1
			}
		}
		lines = append(lines, "", sMuted.Render("Next → press ")+sKey.Render(fmt.Sprint(idx))+sMuted.Render(" to run ")+sAccent.Render(stepTitle(next)))
	} else {
		lines = append(lines, "", sOK.Render("✔ Feature complete"))
	}

	if f.Tasks != nil && len(f.Tasks.Tasks) > 0 {
		done, total := f.Tasks.Progress()
		lines = append(lines, "", sHeading.Render("Progress")+"  "+
			bar(float64(done)/float64(total), max(10, w-24))+sMuted.Render(fmt.Sprintf(" %3.0f%%  %d/%d", 100*float64(done)/float64(total), done, total)))
		active, _ := m.taskActivity(f)
		nameW := clamp(w/2, 16, 40)
		for _, p := range f.Tasks.Phases {
			d, t := p.Progress(f.Tasks)
			icon := " "
			for _, ti := range p.Tasks {
				if active[f.Tasks.Tasks[ti].ID] {
					icon = m.spin.View()
				}
			}
			if d == t && icon == " " {
				icon = sOK.Render(icDone)
			}
			title := lipgloss.NewStyle().Width(nameW).Render(truncate(p.Title, nameW))
			lines = append(lines, fmt.Sprintf(" %s %s %s %s", icon, title, bar(float64(d)/float64(t), max(6, w-nameW-14)), sMuted.Render(fmt.Sprintf("%d/%d", d, t))))
		}
	}

	var arts []string
	for _, n := range []string{"spec.md", "plan.md", "research.md", "data-model.md", "quickstart.md", "contracts", "checklists", "tasks.md"} {
		if f.Has(n) {
			arts = append(arts, sOK.Render("●")+" "+n)
		} else {
			arts = append(arts, sFaint.Render("○ "+n))
		}
	}
	lines = append(lines, "", sHeading.Render("Artifacts"), wrapJoin(arts, "  ", w))
	return strings.Join(lines, "\n")
}

func (m *Model) stepLine(f *speckit.Feature, i int, s speckit.Step, w int) string {
	var icon string
	name := ""
	if f != nil {
		name = f.Name
	}
	running := m.stepRunning(name, s.ID)
	if running {
		icon = m.spin.View()
	} else {
		switch m.proj.Status(f, s.ID) {
		case speckit.StepDone:
			icon = sOK.Render(icDone)
		case speckit.StepPartial:
			icon = sWarn.Render(icPartial)
		default:
			icon = sMuted.Render(icPending)
		}
	}
	title := lipgloss.NewStyle().Width(13).Render(s.Title)
	if running {
		title = sAccent2.Bold(true).Width(13).Render(s.Title)
	}
	desc := s.Description
	if s.Optional {
		desc = "(optional) " + desc
	}
	if running {
		desc = "running…"
	}
	return fmt.Sprintf(" %s %s %s %s", sKey.Render(fmt.Sprint(i+1)), icon, title, sMuted.Render(truncate(desc, max(5, w-22))))
}

// ---------- tasks screen ----------

func (m *Model) viewTasks(h int) string {
	f := m.feature()
	pane := sPaneFocus.Width(m.w - 2).Height(h - 2)
	inner := m.w - 6
	if f == nil {
		return pane.Render(sMuted.Render("No feature selected. Press n to create one."))
	}
	head := sTitle.Render(f.Name)
	if f.Tasks == nil || len(f.Tasks.Tasks) == 0 {
		msg := head + "\n\n" + sMuted.Render("No tasks.md yet. ")
		if m.stepRunning(f.Name, "tasks") {
			msg += m.spin.View() + sAccent2.Render(" generating tasks…")
		} else {
			msg += sMuted.Render("Press ") + sKey.Render("6") + sMuted.Render(" to generate tasks (after plan: ") + sKey.Render("4") + sMuted.Render(").")
		}
		return pane.Render(msg)
	}
	done, total := f.Tasks.Progress()
	head = spread(head, bar(float64(done)/float64(total), clamp(inner/3, 10, 40))+
		sMuted.Render(fmt.Sprintf(" %3.0f%% %d/%d", 100*float64(done)/float64(total), done, total)), inner)

	rows := m.taskRows(f)
	if m.taskCursor >= len(rows) {
		m.taskCursor = max(0, len(rows)-1)
	}
	active, queued := m.taskActivity(f)
	avail := h - 2 - 2
	start := 0
	if m.taskCursor >= avail {
		start = m.taskCursor - avail + 1
	}
	var lines []string
	lines = append(lines, head, "")
	for i := start; i < len(rows) && i < start+avail; i++ {
		r := rows[i]
		var line string
		if r.task < 0 {
			p := f.Tasks.Phases[r.phase]
			d, t := p.Progress(f.Tasks)
			arrow := "▾"
			if m.collapsed[m.phaseKey(f, r.phase)] {
				arrow = "▸"
			}
			spin := ""
			for _, ti := range p.Tasks {
				if active[f.Tasks.Tasks[ti].ID] {
					spin = " " + m.spin.View()
				}
			}
			label := sHeading.Render(arrow+" "+truncate(p.Title, inner-30)) + spin
			line = spread(label, bar(float64(d)/float64(t), 14)+sMuted.Render(fmt.Sprintf(" %d/%d", d, t)), inner)
		} else {
			t := f.Tasks.Tasks[r.task]
			status := "  "
			switch {
			case active[t.ID]:
				status = m.spin.View() + " "
			case queued[t.ID]:
				status = sMuted.Render(icQueued) + " "
			}
			box := sMuted.Render(icBoxOff)
			if t.Done {
				box = sOK.Render(icBoxOn)
			}
			var tags string
			if t.ID != "" {
				tags += sAccent.Render(t.ID) + " "
			}
			if t.Parallel {
				tags += sBadge.Render("[P]") + " "
			}
			if t.Story != "" {
				tags += sAccent2.Render("["+t.Story+"]") + " "
			}
			text := truncate(t.Text, max(10, inner-lipgloss.Width(tags)-10))
			if t.Done {
				text = sMuted.Strikethrough(true).Render(text)
			} else if active[t.ID] {
				text = sAccent2.Render(text)
			}
			line = "  " + status + box + " " + tags + text
		}
		if i == m.taskCursor {
			line = sSelected.Width(inner).Render(line)
		}
		lines = append(lines, line)
	}
	return pane.Render(strings.Join(lines, "\n"))
}

// ---------- overlays ----------

func (m *Model) viewHelp() string {
	sec := func(title string, rows [][2]string) string {
		var b strings.Builder
		b.WriteString(sHeading.Render(title) + "\n")
		for _, r := range rows {
			b.WriteString(fmt.Sprintf("  %s %s\n", sKey.Width(11).Render(r[0]), r[1]))
		}
		return b.String()
	}
	global := sec("Global", [][2]string{
		{"tab / S-tab", "switch screen"}, {"n", "new feature (/speckit.specify)"}, {"1-8", "run workflow step"},
		{"i", "implement whole feature"}, {"m", "choose model"}, {"a", "choose agent (claude, copilot…)"},
		{"p", "cycle permission mode"}, {"l", "logs"}, {"I", "init spec-kit in this dir"},
		{"r", "refresh"}, {"^x", "cancel running job"}, {"h / ?", "this help"}, {"q", "quit"},
	})
	dash := sec("Dashboard", [][2]string{{"↑↓ / jk", "select feature"}, {"enter", "open tasks"}, {"e", "edit spec.md"}})
	tasks := sec("Tasks", [][2]string{
		{"space", "toggle checkbox (on phase: all)"}, {"x", "run selected task / phase"}, {"X", "run whole phase"},
		{"enter ←→", "fold / unfold phase"}, {"f", "hide completed"}, {"e", "edit tasks.md at line"}, {"esc", "back"},
	})
	logs := sec("Logs", [][2]string{{"↑↓", "select job"}, {"pgup/pgdn", "scroll"}, {"g / G", "top / bottom"}, {"F", "follow output"}, {"c", "clear finished jobs"}})
	var steps strings.Builder
	steps.WriteString(sHeading.Render("Spec-Driven workflow") + "\n")
	for i, s := range speckit.Steps {
		opt := ""
		if s.Optional {
			opt = sMuted.Render(" (optional)")
		}
		steps.WriteString(fmt.Sprintf("  %s %s%s\n     %s\n", sKey.Render(fmt.Sprint(i+1)), sAccent.Render("/speckit."+s.ID), opt, sMuted.Render(s.Description)))
	}
	col1 := global + "\n" + dash
	col2 := tasks + "\n" + logs
	content := lipgloss.JoinHorizontal(lipgloss.Top, col1, "    ", col2)
	if m.w > 150 {
		content = lipgloss.JoinHorizontal(lipgloss.Top, content, "    ", steps.String())
	} else if m.h > 50 {
		content += "\n" + steps.String()
	}
	return sModal.Render(sTitle.Render("speckit — keyboard shortcuts") + "\n\n" + content + "\n" + sMuted.Render("esc to close"))
}

func (m *Model) viewInput() string {
	p := m.pending
	s, _ := speckit.StepByID(p.step)
	ag := m.agent()
	var b strings.Builder
	b.WriteString(sTitle.Render("▶ "+p.title) + "\n")
	b.WriteString(sMuted.Render(s.Description) + "\n\n")
	label := "Arguments (optional)"
	if s.InputRequired {
		label = "Description (required)"
	}
	b.WriteString(sHeading.Render(label) + "\n")
	b.WriteString(m.input.View() + "\n\n")
	b.WriteString(sMuted.Render("agent ") + ag.Name() + sMuted.Render(" · model ") + sAccent.Render(displayModel(m.cfg.Model(ag.ID()))))
	if pm := m.permMode(); pm != "" {
		b.WriteString(sMuted.Render(" · perm ") + permStyle(pm).Render(pm))
	}
	if p.feature != "" {
		b.WriteString(sMuted.Render(" · SPECIFY_FEATURE=") + p.feature)
	}
	b.WriteString("\n" + sFaint.Render(truncate(previewCommand(ag, s.ID, m.input.Value()), min(90, m.w-12))) + "\n\n")
	b.WriteString(sKey.Render("enter") + sMuted.Render(" run  ") + sKey.Render("alt+enter") + sMuted.Render(" newline  ") + sKey.Render("esc") + sMuted.Render(" cancel"))
	return sModal.Width(min(96, m.w-4)).Render(b.String())
}

func previewCommand(ag agent.Agent, step, args string) string {
	args = strings.Join(strings.Fields(args), " ")
	if ag.ID() == "claude" {
		c := "$ claude -p \"/speckit." + step
		if args != "" {
			c += " " + args
		}
		return c + "\" --output-format stream-json …"
	}
	return "$ " + ag.ID() + " -p <speckit." + step + " prompt> …"
}

func (m *Model) viewPicker() string {
	var b strings.Builder
	ag := m.agent()
	switch m.picker {
	case pickModel:
		b.WriteString(sTitle.Render("Model for "+ag.Name()) + "\n\n")
	case pickAgent:
		b.WriteString(sTitle.Render("AI agent") + "\n\n")
	}
	if m.customMode {
		b.WriteString("Custom model id:\n" + m.customInput.View() + "\n\n" + sMuted.Render("enter save · esc back"))
		return sModal.Width(60).Render(b.String())
	}
	for i, it := range m.pickItems {
		var label string
		current := false
		switch m.picker {
		case pickModel:
			label = it
			if it == "" {
				label = "default " + sMuted.Render("(agent's own default)")
			}
			current = it == m.cfg.Model(ag.ID()) && it != customItem
		case pickAgent:
			a, _ := agent.Get(it)
			avail := sOK.Render("✔ installed")
			if !a.Available() {
				avail = sErr.Render("✗ not found")
			}
			label = fmt.Sprintf("%-22s %s", a.Name(), avail)
			current = it == m.cfg.Agent
		}
		mark := "  "
		if current {
			mark = sOK.Render("● ")
		}
		line := mark + label
		if i == m.pickIdx {
			line = sSelected.Width(52).Render(sAccent.Render("▸") + line)
		} else {
			line = " " + line
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n" + sMuted.Render("↑↓ select · enter choose · esc close"))
	return sModal.Width(60).Render(b.String())
}

// ---------- helpers ----------

func bar(pct float64, w int) string {
	p := progress.New(progress.WithGradient("#7C5CFF", "#3DDC97"), progress.WithoutPercentage(), progress.WithWidth(w))
	p.Full, p.Empty = '█', '░'
	p.EmptyColor = "#44444F"
	return p.ViewAs(pct)
}

func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	return ansi.Truncate(s, w, "…")
}

func clipLines(s string, h int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > h {
		lines = lines[:max(0, h)]
	}
	return strings.Join(lines, "\n")
}

func spread(left, right string, w int) string {
	gap := w - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		return truncate(left+" "+right, w)
	}
	return left + strings.Repeat(" ", gap) + right
}

func wrapJoin(items []string, sep string, w int) string {
	var lines []string
	cur := ""
	for _, it := range items {
		if cur != "" && lipgloss.Width(cur+sep+it) > w {
			lines = append(lines, cur)
			cur = ""
		}
		if cur != "" {
			cur += sep
		}
		cur += it
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return strings.Join(lines, "\n")
}

func clamp(v, lo, hi int) int { return max(lo, min(hi, v)) }

func relPath(root, p string) string {
	if r, err := filepath.Rel(root, p); err == nil {
		return r
	}
	return p
}
