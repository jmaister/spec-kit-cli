package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jmaister/spec-kit-cli/internal/speckit"
)

type taskRow struct {
	phase int
	task  int // -1 for phase header
}

func (m *Model) phaseKey(f *speckit.Feature, p int) string {
	return f.Name + "/" + f.Tasks.Phases[p].Title
}

func (m *Model) taskRows(f *speckit.Feature) []taskRow {
	if f == nil || f.Tasks == nil {
		return nil
	}
	var rows []taskRow
	for pi, p := range f.Tasks.Phases {
		rows = append(rows, taskRow{phase: pi, task: -1})
		if m.collapsed[m.phaseKey(f, pi)] {
			continue
		}
		for _, ti := range p.Tasks {
			if m.hideDone && f.Tasks.Tasks[ti].Done {
				continue
			}
			rows = append(rows, taskRow{phase: pi, task: ti})
		}
	}
	return rows
}

func (m *Model) updateTasks(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := m.keys
	f := m.feature()
	if id, ok := m.stepKey(msg); ok {
		return m, m.openStepInput(id)
	}
	if key.Matches(msg, k.Back) {
		m.screen = scrDashboard
		return m, nil
	}
	if key.Matches(msg, k.Implement) {
		return m, m.openStepInput("implement")
	}
	rows := m.taskRows(f)
	if len(rows) == 0 {
		if key.Matches(msg, k.Edit) && f != nil && f.Has("spec.md") {
			return m, m.openEditor(f.Path("spec.md"), 0)
		}
		return m, nil
	}
	if m.taskCursor >= len(rows) {
		m.taskCursor = len(rows) - 1
	}
	row := rows[m.taskCursor]
	page := max(1, m.bodyHeight()-6)
	switch {
	case key.Matches(msg, k.Up):
		m.taskCursor = max(0, m.taskCursor-1)
	case key.Matches(msg, k.Down):
		m.taskCursor = min(len(rows)-1, m.taskCursor+1)
	case key.Matches(msg, k.PageUp):
		m.taskCursor = max(0, m.taskCursor-page)
	case key.Matches(msg, k.PageDown):
		m.taskCursor = min(len(rows)-1, m.taskCursor+page)
	case key.Matches(msg, k.Top):
		m.taskCursor = 0
	case key.Matches(msg, k.Bottom):
		m.taskCursor = len(rows) - 1
	case key.Matches(msg, k.HideDone):
		m.hideDone = !m.hideDone
		m.setFlash(false, "hide completed: %v", m.hideDone)
	case key.Matches(msg, k.Enter), msg.String() == "left", msg.String() == "right":
		pk := m.phaseKey(f, row.phase)
		m.collapsed[pk] = !m.collapsed[pk]
		if m.collapsed[pk] {
			// Move cursor onto the header of the collapsed phase.
			for i, r := range m.taskRows(f) {
				if r.phase == row.phase && r.task == -1 {
					m.taskCursor = i
				}
			}
		}
	case key.Matches(msg, k.Toggle):
		if row.task < 0 {
			return m, m.togglePhase(f, row.phase)
		}
		t := f.Tasks.Tasks[row.task]
		if err := speckit.SetTaskDone(f.Tasks.Path, t, !t.Done); err != nil {
			m.setFlash(true, "%v", err)
		}
		m.reload()
	case key.Matches(msg, k.RunTask):
		if row.task < 0 {
			return m, m.runPhase(f, row.phase)
		}
		t := f.Tasks.Tasks[row.task]
		if t.ID == "" {
			m.setFlash(true, "task has no ID")
			return m, nil
		}
		args := fmt.Sprintf("Implement ONLY task %s (%s). Do not start other tasks. Mark it as [X] in tasks.md when complete.", t.ID, t.Text)
		return m, m.openInput(pendingRun{step: "implement", feature: f.Name, taskIDs: []string{t.ID},
			title: "Implement " + t.ID + " · " + f.Name}, args)
	case key.Matches(msg, k.RunPhase):
		return m, m.runPhase(f, row.phase)
	case key.Matches(msg, k.Edit):
		line := 0
		if row.task >= 0 {
			line = f.Tasks.Tasks[row.task].Line + 1
		}
		return m, m.openEditor(f.Tasks.Path, line)
	}
	return m, nil
}

// togglePhase marks all tasks of a phase done, or all undone if already done.
func (m *Model) togglePhase(f *speckit.Feature, pi int) tea.Cmd {
	p := f.Tasks.Phases[pi]
	done, total := p.Progress(f.Tasks)
	target := done < total
	for _, ti := range p.Tasks {
		t := f.Tasks.Tasks[ti]
		if t.Done != target {
			if err := speckit.SetTaskDone(f.Tasks.Path, t, target); err != nil {
				m.setFlash(true, "%v", err)
				break
			}
		}
	}
	m.reload()
	return nil
}

func (m *Model) runPhase(f *speckit.Feature, pi int) tea.Cmd {
	p := f.Tasks.Phases[pi]
	var ids []string
	for _, ti := range p.Tasks {
		if t := f.Tasks.Tasks[ti]; !t.Done && t.ID != "" {
			ids = append(ids, t.ID)
		}
	}
	if len(ids) == 0 {
		m.setFlash(false, "phase already complete")
		return nil
	}
	args := fmt.Sprintf("Implement ONLY the pending tasks of %q: %s. Respect [P] parallel markers and dependencies. Do not start tasks of other phases. Mark each as [X] in tasks.md when complete.",
		p.Title, strings.Join(ids, ", "))
	return m.openInput(pendingRun{step: "implement", feature: f.Name, taskIDs: ids,
		title: "Implement " + shortPhase(p.Title) + " · " + f.Name}, args)
}

func shortPhase(t string) string {
	if i := strings.Index(t, ":"); i > 0 {
		return t[:i]
	}
	return t
}
