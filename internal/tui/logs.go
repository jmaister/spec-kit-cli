package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jmaister/spec-kit-cli/internal/agent"
	"github.com/jmaister/spec-kit-cli/internal/runner"
)

const jobListWidth = 34

func (m *Model) layoutLogs() {
	m.vp.Width = max(20, m.w-jobListWidth-8)
	m.vp.Height = max(3, m.bodyHeight()-5)
}

func (m *Model) selectedJob() *runner.Job {
	jobs := m.jobs.Jobs()
	if len(jobs) == 0 {
		return nil
	}
	if m.jobIdx >= len(jobs) || m.jobIdx < 0 {
		m.jobIdx = len(jobs) - 1
	}
	return jobs[m.jobIdx]
}

func (m *Model) refreshLogs() {
	// Auto-select new jobs while following.
	if n := len(m.jobs.Jobs()); n != m.lastJobCount {
		if m.follow && n > m.lastJobCount {
			m.jobIdx = n - 1
		}
		m.lastJobCount = n
	}
	if m.screen != scrLogs {
		return
	}
	j := m.selectedJob()
	if j == nil {
		m.vp.SetContent(sMuted.Render("No jobs yet. Run a workflow step (s, or → and enter on the dashboard) or create a feature (n)."))
		return
	}
	atBottom := m.vp.AtBottom()
	m.vp.SetContent(renderLogLines(j.Lines(), m.vp.Width))
	if m.follow || atBottom {
		m.vp.GotoBottom()
	}
}

func renderLogLines(lines []runner.Line, width int) string {
	var b strings.Builder
	wrap := lipgloss.NewStyle().Width(max(10, width-9))
	for _, l := range lines {
		ts := sFaint.Render(l.At.Format("15:04:05") + " ")
		var txt string
		switch l.Kind {
		case agent.EvInfo:
			txt = sMuted.Render(l.Text)
		case agent.EvTool:
			txt = sAccent2.Render("⚙ " + l.Text)
		case agent.EvToolResult:
			txt = sMuted.Render(l.Text)
		case agent.EvError:
			txt = sErr.Render(l.Text)
		case agent.EvResult:
			txt = sOK.Bold(true).Render("● " + l.Text)
		default:
			txt = l.Text
		}
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, ts, wrap.Render(txt)))
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m *Model) updateLogs(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := m.keys
	n := len(m.jobs.Jobs())
	switch {
	case key.Matches(msg, k.Back):
		m.screen = scrDashboard
		return m, nil
	case key.Matches(msg, k.Up):
		if m.jobIdx > 0 {
			m.jobIdx--
			m.follow = false
		}
	case key.Matches(msg, k.Down):
		if m.jobIdx < n-1 {
			m.jobIdx++
		}
	case key.Matches(msg, k.Follow):
		m.follow = !m.follow
		m.setFlash(false, "follow: %v", m.follow)
	case key.Matches(msg, k.ClearJobs):
		m.jobs.ClearFinished()
		m.lastJobCount = len(m.jobs.Jobs())
		m.jobIdx = 0
	case key.Matches(msg, k.PageUp), msg.String() == "u":
		m.follow = false
		m.vp.HalfPageUp()
		return m, nil
	case key.Matches(msg, k.PageDown), msg.String() == "d":
		m.vp.HalfPageDown()
		return m, nil
	case key.Matches(msg, k.Top):
		m.follow = false
		m.vp.GotoTop()
		return m, nil
	case key.Matches(msg, k.Bottom):
		m.vp.GotoBottom()
		return m, nil
	}
	m.refreshLogs()
	return m, nil
}

func (m *Model) viewLogs(h int) string {
	jobs := m.jobs.Jobs()
	var list []string
	list = append(list, sHeading.Render(fmt.Sprintf("Jobs (%d)", len(jobs))), "")
	if len(jobs) == 0 {
		list = append(list, sMuted.Render("none yet"))
	}
	inner := jobListWidth - 4
	for i, j := range jobs {
		s := j.Snapshot()
		var icon string
		switch s.Status {
		case runner.Running:
			icon = m.spin.View()
		case runner.Succeeded:
			icon = sOK.Render(icDone)
		case runner.Failed:
			icon = sErr.Render("✗")
		case runner.Canceled:
			icon = sWarn.Render("■")
		}
		title := truncate(fmt.Sprintf("#%d %s", j.ID, j.Title), inner-2)
		meta := sMuted.Render(fmt.Sprintf("   %s · %s", fmtDur(j.Duration()), displayModel(j.Model)))
		if s.Cost > 0 {
			meta += sMuted.Render(fmt.Sprintf(" · $%.3f", s.Cost))
		}
		line := icon + " " + title
		if i == m.jobIdx {
			line = sSelected.Width(inner).Render(line)
		}
		list = append(list, line, truncate(meta, inner+20), "")
	}
	left := sPane.Width(jobListWidth - 2).Height(h - 2).Render(clipLines(strings.Join(list, "\n"), h-2))

	var head string
	if j := m.selectedJob(); j != nil {
		head = sHeading.Render(truncate(j.Title, m.vp.Width-20)) + "  " + sMuted.Render(j.Snapshot().Status.String())
		if j.LogPath != "" {
			head += "\n" + sFaint.Render(truncate("raw: "+j.LogPath, m.vp.Width))
		}
	} else {
		head = sHeading.Render("Logs") + "\n"
	}
	follow := ""
	if m.follow {
		follow = sAccent2.Render(" ⇣ follow")
	}
	scroll := sMuted.Render(fmt.Sprintf("%3.0f%%", m.vp.ScrollPercent()*100)) + follow
	right := sPaneFocus.Width(m.w - jobListWidth - 4).Height(h - 2).
		Render(head + "\n" + m.vp.View() + "\n" + scroll)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func fmtDur(d time.Duration) string {
	d = d.Round(time.Second)
	if d < time.Minute {
		return d.String()
	}
	return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
}
