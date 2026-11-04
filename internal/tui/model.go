// Package tui is the Bubble Tea interface.
package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jmaister/spec-kit-cli/internal/agent"
	"github.com/jmaister/spec-kit-cli/internal/config"
	"github.com/jmaister/spec-kit-cli/internal/runner"
	"github.com/jmaister/spec-kit-cli/internal/speckit"
)

type screen int

const (
	scrDashboard screen = iota
	scrTasks
	scrLogs
	numScreens
)

var screenNames = []string{"Dashboard", "Tasks", "Logs"}

type overlay int

const (
	ovNone overlay = iota
	ovHelp
	ovInput
	ovPicker
	ovConfirm
)

type pickerKind int

const (
	pickModel pickerKind = iota
	pickAgent
)

// pendingRun is what the input overlay will execute.
type pendingRun struct {
	step    string
	feature string
	taskIDs []string
	title   string
}

type Model struct {
	cfg  *config.Config
	proj *speckit.Project
	jobs *runner.Manager
	keys keyMap

	w, h    int
	screen  screen
	overlay overlay
	spin    spinner.Model

	featIdx    int
	stepsFocus bool // dashboard: workflow pane focused
	stepIdx    int

	// tasks screen
	taskCursor int
	collapsed  map[string]bool // "feature/phase title"
	hideDone   bool

	// logs screen
	jobIdx       int
	vp           viewport.Model
	follow       bool
	lastJobCount int

	// input overlay
	input   textarea.Model
	pending pendingRun

	// picker overlay
	picker      pickerKind
	pickItems   []string
	pickIdx     int
	customMode  bool
	customInput textinput.Model

	// confirm overlay
	confirmMsg string
	confirmFn  func() tea.Cmd

	flash    string
	flashErr bool
	flashAt  time.Time
}

type (
	tickMsg       time.Time
	jobUpdateMsg  runner.Update
	editorDoneMsg struct{ err error }
)

func New(cfg *config.Config, root string) (*Model, error) {
	proj, err := speckit.Load(root)
	if err != nil {
		return nil, err
	}
	sp := spinner.New(spinner.WithSpinner(spinner.Spinner{
		Frames: []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}, FPS: time.Second / 10,
	}), spinner.WithStyle(sAccent2))

	ta := textarea.New()
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.KeyMap.InsertNewline = key.NewBinding(key.WithKeys("alt+enter", "ctrl+j"))
	ta.SetHeight(5)

	ti := textinput.New()
	ti.Placeholder = "model id, e.g. claude-sonnet-5-5"

	m := &Model{
		cfg: cfg, proj: proj, jobs: runner.NewManager(), keys: newKeyMap(),
		spin: sp, collapsed: map[string]bool{}, follow: true,
		input: ta, customInput: ti, vp: viewport.New(80, 20),
	}
	if _, ok := agent.Get(cfg.Agent); !ok {
		cfg.Agent = "claude"
	}
	// Select the most recently modified feature.
	for i, f := range proj.Features {
		if f.ModTime.After(proj.Features[m.featIdx].ModTime) {
			m.featIdx = i
		}
	}
	return m, nil
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.spin.Tick, tick(), m.waitJob())
}

func tick() tea.Cmd {
	return tea.Tick(1500*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m *Model) waitJob() tea.Cmd {
	ch := m.jobs.Updates
	return func() tea.Msg { return jobUpdateMsg(<-ch) }
}

// ---------- helpers ----------

func (m *Model) agent() agent.Agent {
	a, _ := agent.Get(m.cfg.Agent)
	return a
}

func (m *Model) feature() *speckit.Feature {
	if len(m.proj.Features) == 0 {
		return nil
	}
	if m.featIdx >= len(m.proj.Features) {
		m.featIdx = len(m.proj.Features) - 1
	}
	if m.featIdx < 0 {
		m.featIdx = 0
	}
	return m.proj.Features[m.featIdx]
}

func (m *Model) setFlash(err bool, format string, a ...any) {
	m.flash, m.flashErr, m.flashAt = fmt.Sprintf(format, a...), err, time.Now()
}

func (m *Model) reload() {
	var cur string
	if f := m.feature(); f != nil {
		cur = f.Name
	}
	p, err := speckit.Load(m.proj.Root)
	if err != nil {
		m.setFlash(true, "reload: %v", err)
		return
	}
	m.proj = p
	for i, f := range p.Features {
		if f.Name == cur {
			m.featIdx = i
		}
	}
}

func (m *Model) permMode() string {
	if slices.Contains(m.agent().PermissionModes(), m.cfg.PermissionMode) {
		return m.cfg.PermissionMode
	}
	return ""
}

func (m *Model) saveCfg() {
	if err := m.cfg.Save(); err != nil {
		m.setFlash(true, "saving config: %v", err)
	}
}

// runningJobsFor returns running jobs of a feature (or project-level when name == "").
func (m *Model) runningJobsFor(feature string) []*runner.Job {
	var out []*runner.Job
	for _, j := range m.jobs.Jobs() {
		if j.Running() && j.Feature == feature {
			out = append(out, j)
		}
	}
	return out
}

func (m *Model) stepRunning(feature, step string) bool {
	scope := feature
	if s, ok := speckit.StepByID(step); ok && s.ProjectScope {
		scope = ""
	}
	for _, j := range m.runningJobsFor(scope) {
		if j.Step == step {
			return true
		}
	}
	return false
}

// taskActivity returns task IDs being worked on (spinner) and queued ones.
func (m *Model) taskActivity(f *speckit.Feature) (active, queued map[string]bool) {
	active, queued = map[string]bool{}, map[string]bool{}
	if f == nil || f.Tasks == nil {
		return
	}
	pending := func(id string) bool {
		i := f.Tasks.FindTask(id)
		return i >= 0 && !f.Tasks.Tasks[i].Done
	}
	for _, j := range m.runningJobsFor(f.Name) {
		if j.Step != "implement" {
			continue
		}
		last := j.Snapshot().LastTask
		if len(j.TaskIDs) > 0 {
			cur := ""
			if slices.Contains(j.TaskIDs, last) && pending(last) {
				cur = last
			}
			for _, id := range j.TaskIDs {
				if !pending(id) {
					continue
				}
				if cur == "" {
					cur = id
				}
				if id == cur {
					active[id] = true
				} else {
					queued[id] = true
				}
			}
			continue
		}
		if last != "" && pending(last) {
			active[last] = true
		} else if i := f.Tasks.FirstPending(); i >= 0 {
			active[f.Tasks.Tasks[i].ID] = true
		}
	}
	return
}

// ---------- running things ----------

func (m *Model) startStep(p pendingRun, args string) tea.Cmd {
	ag := m.agent()
	if ag == nil {
		m.setFlash(true, "no agent selected")
		return nil
	}
	if !ag.Available() {
		m.setFlash(true, "%s CLI not found in PATH", ag.Name())
		return nil
	}
	req := agent.Request{
		Root: m.proj.Root, Step: p.step, Args: strings.TrimSpace(args),
		Model: m.cfg.Model(ag.ID()), Perm: m.permMode(), Feature: p.feature,
	}
	root, feat, step := m.proj.Root, p.feature, p.step
	title := p.title
	if title == "" {
		title = stepTitle(step)
	}
	_, err := m.jobs.Start(runner.Spec{
		Title: title, Step: step, Feature: feat, Agent: ag.ID(), Model: req.Model, TaskIDs: p.taskIDs,
		Build:  func(ctx context.Context) (*exec.Cmd, error) { return ag.Command(ctx, req) },
		Parser: ag.Parser(),
		OnSuccess: func() {
			if feat != "" {
				_ = speckit.LoadState(root).Record(feat, step)
			}
		},
	})
	if err != nil {
		m.setFlash(true, "start %s: %v", step, err)
		return nil
	}
	m.setFlash(false, "▶ %s started with %s — press l to watch logs", title, ag.Name())
	return nil
}

func stepTitle(id string) string {
	if s, ok := speckit.StepByID(id); ok {
		return s.Title
	}
	return id
}

// openStepInput opens the input overlay for a step.
func (m *Model) openStepInput(stepID string) tea.Cmd {
	s, ok := speckit.StepByID(stepID)
	if !ok {
		return nil
	}
	if !m.proj.Initialized {
		m.setFlash(true, "not a spec-kit project — press I to initialize")
		return nil
	}
	p := pendingRun{step: s.ID, title: s.Title}
	if !s.ProjectScope {
		f := m.feature()
		if f == nil {
			m.setFlash(true, "no feature yet — press n to create one")
			return nil
		}
		p.feature = f.Name
		p.title = s.Title + " · " + f.Name
	}
	if m.stepRunning(p.feature, s.ID) {
		m.setFlash(true, "%s is already running", p.title)
		return nil
	}
	return m.openInput(p, "")
}

func (m *Model) openInput(p pendingRun, prefill string) tea.Cmd {
	m.pending = p
	m.input.Reset()
	m.input.SetValue(prefill)
	if s, ok := speckit.StepByID(p.step); ok {
		m.input.Placeholder = s.InputHint
	}
	m.overlay = ovInput
	return m.input.Focus()
}

func (m *Model) startInit() tea.Cmd {
	ag := m.agent()
	bin, prefix, ok := speckit.SpecifyCommand()
	if !ok {
		m.setFlash(true, "specify CLI not found (nor uv/uvx/pipx). Install: %s", speckit.InstallHint)
		return nil
	}
	args := append(prefix, speckit.InitArgs(bin, prefix, ag.InitAI())...)
	root := m.proj.Root
	_, err := m.jobs.Start(runner.Spec{
		Title: "Init spec-kit (" + ag.InitAI() + ")", Step: "init", Agent: ag.ID(),
		Build: func(ctx context.Context) (*exec.Cmd, error) {
			c := exec.CommandContext(ctx, bin, args...)
			c.Dir = root
			c.Env = append(os.Environ(), "NO_COLOR=1", "TERM=dumb")
			return c, nil
		},
		Parser: agent.PlainParser{},
	})
	if err != nil {
		m.setFlash(true, "init: %v", err)
	} else {
		m.setFlash(false, "▶ initializing spec-kit…")
	}
	return nil
}

func (m *Model) openEditor(path string, line int) tea.Cmd {
	ed := os.Getenv("VISUAL")
	if ed == "" {
		ed = os.Getenv("EDITOR")
	}
	if ed == "" {
		ed = "vi"
	}
	parts := strings.Fields(ed)
	args := parts[1:]
	switch filepath.Base(parts[0]) {
	case "vi", "vim", "nvim", "nano", "emacs", "micro", "hx", "kak":
		if line > 0 {
			args = append(args, fmt.Sprintf("+%d", line))
		}
	}
	args = append(args, path)
	return tea.ExecProcess(exec.Command(parts[0], args...), func(err error) tea.Msg { return editorDoneMsg{err} })
}

// ---------- update ----------

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.input.SetWidth(min(90, m.w-12))
		m.layoutLogs()
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd

	case tickMsg:
		m.reload()
		if m.flash != "" && time.Since(m.flashAt) > 8*time.Second {
			m.flash = ""
		}
		m.refreshLogs()
		return m, tick()

	case jobUpdateMsg:
		if msg.Done {
			m.reload()
			for _, j := range m.jobs.Jobs() {
				if j.ID != msg.JobID {
					continue
				}
				s := j.Snapshot()
				switch s.Status {
				case runner.Succeeded:
					m.setFlash(false, "✔ %s finished (%s)", j.Title, j.Duration().Round(time.Second))
					if j.Step == "specify" {
						m.selectNewestFeature()
					}
				case runner.Failed:
					m.setFlash(true, "✗ %s failed — press l for logs", j.Title)
				}
			}
		}
		m.refreshLogs()
		return m, m.waitJob()

	case editorDoneMsg:
		if msg.err != nil {
			m.setFlash(true, "editor: %v", msg.err)
		}
		m.reload()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *Model) selectNewestFeature() {
	best := -1
	for i, f := range m.proj.Features {
		if best < 0 || f.ModTime.After(m.proj.Features[best].ModTime) {
			best = i
		}
	}
	if best >= 0 {
		m.featIdx = best
	}
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" && m.overlay != ovConfirm {
		return m, m.quit()
	}
	switch m.overlay {
	case ovHelp:
		if key.Matches(msg, m.keys.Back, m.keys.Help, m.keys.Quit) || msg.String() == "enter" {
			m.overlay = ovNone
		}
		return m, nil
	case ovInput:
		return m.updateInput(msg)
	case ovPicker:
		return m.updatePicker(msg)
	case ovConfirm:
		switch msg.String() {
		case "y", "Y", "enter":
			m.overlay = ovNone
			if m.confirmFn != nil {
				return m, m.confirmFn()
			}
		case "n", "N", "esc", "q", "ctrl+c":
			m.overlay = ovNone
		}
		return m, nil
	}

	k := m.keys
	switch {
	case key.Matches(msg, k.Quit):
		return m, m.quit()
	case key.Matches(msg, k.NextScreen):
		m.screen = (m.screen + 1) % numScreens
		m.refreshLogs()
		return m, nil
	case key.Matches(msg, k.PrevScreen):
		m.screen = (m.screen + numScreens - 1) % numScreens
		m.refreshLogs()
		return m, nil
	case key.Matches(msg, k.Help):
		m.overlay = ovHelp
		return m, nil
	case key.Matches(msg, k.Logs):
		if m.screen == scrLogs {
			m.screen = scrDashboard
		} else {
			m.screen = scrLogs
			m.refreshLogs()
		}
		return m, nil
	case key.Matches(msg, k.New):
		return m, m.openStepInput("specify")
	case key.Matches(msg, k.Model):
		m.openPicker(pickModel)
		return m, nil
	case key.Matches(msg, k.Agent):
		m.openPicker(pickAgent)
		return m, nil
	case key.Matches(msg, k.Perm):
		modes := m.agent().PermissionModes()
		if len(modes) == 0 {
			m.setFlash(false, "%s has no permission modes", m.agent().Name())
			return m, nil
		}
		i := slices.Index(modes, m.cfg.PermissionMode)
		m.cfg.PermissionMode = modes[(i+1)%len(modes)]
		m.saveCfg()
		m.setFlash(false, "permission mode: %s", m.cfg.PermissionMode)
		return m, nil
	case key.Matches(msg, k.Refresh):
		m.reload()
		m.setFlash(false, "refreshed")
		return m, nil
	case key.Matches(msg, k.Init):
		m.confirmMsg = fmt.Sprintf("Run `specify init --here --integration %s` in\n%s ?", m.agent().InitAI(), m.proj.Root)
		if m.proj.Initialized {
			m.confirmMsg += "\n\n" + sWarn.Render("Project already initialized: templates/scripts will be refreshed.")
		}
		m.confirmFn = m.startInit
		m.overlay = ovConfirm
		return m, nil
	case key.Matches(msg, k.Cancel):
		m.cancelJob()
		return m, nil
	}

	switch m.screen {
	case scrDashboard:
		return m.updateDashboard(msg)
	case scrTasks:
		return m.updateTasks(msg)
	case scrLogs:
		return m.updateLogs(msg)
	}
	return m, nil
}

func (m *Model) quit() tea.Cmd {
	if n := m.jobs.RunningCount(); n > 0 {
		m.confirmMsg = fmt.Sprintf("%d job(s) still running.\nQuit and cancel them?", n)
		m.confirmFn = func() tea.Cmd {
			for _, j := range m.jobs.Jobs() {
				j.Cancel()
			}
			return tea.Quit
		}
		m.overlay = ovConfirm
		return nil
	}
	return tea.Quit
}

// cancelJob cancels the selected job on the logs screen, otherwise the most
// recent running one.
func (m *Model) cancelJob() {
	jobs := m.jobs.Jobs()
	var target *runner.Job
	if m.screen == scrLogs && m.jobIdx < len(jobs) && jobs[m.jobIdx].Running() {
		target = jobs[m.jobIdx]
	} else {
		for i := len(jobs) - 1; i >= 0; i-- {
			if jobs[i].Running() {
				target = jobs[i]
				break
			}
		}
	}
	if target == nil {
		m.setFlash(false, "no running job")
		return
	}
	m.confirmMsg = fmt.Sprintf("Cancel job #%d %q?", target.ID, target.Title)
	m.confirmFn = func() tea.Cmd { target.Cancel(); return nil }
	m.overlay = ovConfirm
}

// dashSteps lists the workflow steps shown for the selected feature (only
// project-level steps when there is no feature yet).
func (m *Model) dashSteps() []speckit.Step {
	if m.feature() == nil {
		return speckit.Steps[:2]
	}
	return speckit.Steps
}

// focusNextStep moves the workflow cursor onto the suggested next step.
func (m *Model) focusNextStep() {
	next := m.proj.NextStep(m.feature())
	for i, s := range m.dashSteps() {
		if s.ID == next {
			m.stepIdx = i
			return
		}
	}
}

func (m *Model) updateDashboard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := m.keys
	steps := m.dashSteps()
	switch {
	case key.Matches(msg, k.Right):
		if !m.stepsFocus {
			m.stepsFocus = true
			m.focusNextStep()
		}
		return m, nil
	case key.Matches(msg, k.Left), m.stepsFocus && key.Matches(msg, k.Back):
		m.stepsFocus = false
		return m, nil
	case key.Matches(msg, k.NextStep):
		next := m.proj.NextStep(m.feature())
		if next == "" {
			m.setFlash(false, "✔ feature complete — nothing left to run")
			return m, nil
		}
		return m, m.openStepInput(next)
	case key.Matches(msg, k.Implement):
		return m, m.openStepInput("implement")
	case key.Matches(msg, k.Edit):
		f := m.feature()
		if f == nil {
			return m, nil
		}
		for _, name := range []string{"spec.md", "plan.md", "tasks.md"} {
			if f.Has(name) {
				return m, m.openEditor(f.Path(name), 0)
			}
		}
		return m, nil
	}

	if m.stepsFocus {
		switch {
		case key.Matches(msg, k.Up):
			m.stepIdx = max(0, m.stepIdx-1)
		case key.Matches(msg, k.Down):
			m.stepIdx = min(len(steps)-1, m.stepIdx+1)
		case key.Matches(msg, k.Top):
			m.stepIdx = 0
		case key.Matches(msg, k.Bottom):
			m.stepIdx = len(steps) - 1
		case key.Matches(msg, k.Enter):
			if m.stepIdx < len(steps) {
				return m, m.openStepInput(steps[m.stepIdx].ID)
			}
		}
		return m, nil
	}

	switch {
	case key.Matches(msg, k.Up):
		if m.featIdx > 0 {
			m.featIdx--
			m.taskCursor = 0
		}
	case key.Matches(msg, k.Down):
		if m.featIdx < len(m.proj.Features)-1 {
			m.featIdx++
			m.taskCursor = 0
		}
	case key.Matches(msg, k.Top):
		m.featIdx, m.taskCursor = 0, 0
	case key.Matches(msg, k.Bottom):
		m.featIdx, m.taskCursor = max(0, len(m.proj.Features)-1), 0
	case key.Matches(msg, k.Enter):
		if m.feature() != nil {
			m.screen = scrTasks
		}
	}
	return m, nil
}

// ---------- input overlay ----------

func (m *Model) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.overlay = ovNone
		m.input.Blur()
		return m, nil
	case "enter", "ctrl+s":
		val := m.input.Value()
		if s, _ := speckit.StepByID(m.pending.step); s.InputRequired && strings.TrimSpace(val) == "" {
			m.setFlash(true, "%s needs a description", s.Title)
			return m, nil
		}
		m.overlay = ovNone
		m.input.Blur()
		return m, m.startStep(m.pending, val)
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// ---------- picker overlay ----------

const customItem = "custom…"

func (m *Model) openPicker(kind pickerKind) {
	m.picker, m.pickIdx, m.customMode = kind, 0, false
	switch kind {
	case pickModel:
		m.pickItems = append([]string{""}, m.agent().Models()...)
		m.pickItems = append(m.pickItems, customItem)
		cur := m.cfg.Model(m.agent().ID())
		if i := slices.Index(m.pickItems, cur); i >= 0 {
			m.pickIdx = i
		} else if cur != "" {
			m.pickItems = slices.Insert(m.pickItems, len(m.pickItems)-1, cur)
			m.pickIdx = len(m.pickItems) - 2
		}
	case pickAgent:
		m.pickItems = nil
		for i, a := range agent.All() {
			m.pickItems = append(m.pickItems, a.ID())
			if a.ID() == m.cfg.Agent {
				m.pickIdx = i
			}
		}
	}
	m.overlay = ovPicker
}

func (m *Model) updatePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.customMode {
		switch msg.String() {
		case "esc":
			m.customMode = false
			m.customInput.Blur()
			return m, nil
		case "enter":
			v := strings.TrimSpace(m.customInput.Value())
			m.cfg.SetModel(m.agent().ID(), v)
			m.saveCfg()
			m.overlay, m.customMode = ovNone, false
			m.setFlash(false, "model: %s", displayModel(v))
			return m, nil
		}
		var cmd tea.Cmd
		m.customInput, cmd = m.customInput.Update(msg)
		return m, cmd
	}
	switch {
	case key.Matches(msg, m.keys.Back), key.Matches(msg, m.keys.Quit):
		m.overlay = ovNone
	case key.Matches(msg, m.keys.Up):
		if m.pickIdx > 0 {
			m.pickIdx--
		}
	case key.Matches(msg, m.keys.Down):
		if m.pickIdx < len(m.pickItems)-1 {
			m.pickIdx++
		}
	case key.Matches(msg, m.keys.Enter), key.Matches(msg, m.keys.Toggle):
		item := m.pickItems[m.pickIdx]
		switch m.picker {
		case pickModel:
			if item == customItem {
				m.customMode = true
				m.customInput.SetValue(m.cfg.Model(m.agent().ID()))
				return m, m.customInput.Focus()
			}
			m.cfg.SetModel(m.agent().ID(), item)
			m.setFlash(false, "model: %s", displayModel(item))
		case pickAgent:
			m.cfg.Agent = item
			a := m.agent()
			if modes := a.PermissionModes(); len(modes) > 0 && !slices.Contains(modes, m.cfg.PermissionMode) {
				m.cfg.PermissionMode = modes[0]
			}
			if !a.Available() {
				m.setFlash(true, "agent: %s (CLI not found in PATH!)", a.Name())
			} else {
				m.setFlash(false, "agent: %s", a.Name())
			}
		}
		m.saveCfg()
		m.overlay = ovNone
	}
	return m, nil
}

func displayModel(m string) string {
	if m == "" {
		return "default"
	}
	return m
}
