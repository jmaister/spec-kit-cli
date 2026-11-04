// Package runner executes agent processes in the background, collecting
// their output as log lines and notifying the UI.
package runner

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jmaister/spec-kit-cli/internal/agent"
	"github.com/jmaister/spec-kit-cli/internal/speckit"
)

type Status int

const (
	Running Status = iota
	Succeeded
	Failed
	Canceled
)

func (s Status) String() string {
	return [...]string{"running", "done", "failed", "canceled"}[s]
}

type Line struct {
	At   time.Time
	Kind agent.EventKind
	Text string
}

type Job struct {
	ID      int
	Title   string
	Step    string // spec-kit step or "init"
	Feature string
	Agent   string
	Model   string
	CmdLine string
	TaskIDs []string // tasks explicitly requested (implement)
	LogPath string

	mu       sync.Mutex
	lines    []Line
	status   Status
	started  time.Time
	ended    time.Time
	cost     float64
	lastTask string
	err      error
	cancel   context.CancelFunc
}

type Snapshot struct {
	Status   Status
	Started  time.Time
	Ended    time.Time
	Cost     float64
	LastTask string
	Err      error
	Lines    int
}

func (j *Job) Snapshot() Snapshot {
	j.mu.Lock()
	defer j.mu.Unlock()
	return Snapshot{j.status, j.started, j.ended, j.cost, j.lastTask, j.err, len(j.lines)}
}

func (j *Job) Lines() []Line {
	j.mu.Lock()
	defer j.mu.Unlock()
	return append([]Line(nil), j.lines...)
}

func (j *Job) Running() bool { return j.Snapshot().Status == Running }

func (j *Job) Duration() time.Duration {
	s := j.Snapshot()
	if s.Status == Running {
		return time.Since(s.Started)
	}
	return s.Ended.Sub(s.Started)
}

func (j *Job) Cancel() {
	j.mu.Lock()
	c := j.cancel
	j.mu.Unlock()
	if c != nil {
		c()
	}
}

func (j *Job) add(kind agent.EventKind, text string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	for _, l := range strings.Split(text, "\n") {
		j.lines = append(j.lines, Line{At: time.Now(), Kind: kind, Text: l})
	}
	// Track the task the agent is working on (implement).
	if ids := speckit.TaskIDRe.FindAllString(text, -1); len(ids) > 0 && (kind == agent.EvText || kind == agent.EvTool) {
		j.lastTask = ids[len(ids)-1]
	}
}

// Update is sent on Manager.Updates whenever a job produced output or ended.
type Update struct {
	JobID int
	Done  bool
}

type Manager struct {
	mu      sync.Mutex
	jobs    []*Job
	next    int
	Updates chan Update
	logDir  string
}

func NewManager() *Manager {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	return &Manager{Updates: make(chan Update, 256), logDir: filepath.Join(dir, "speckit-tui", "logs")}
}

func (m *Manager) Jobs() []*Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]*Job(nil), m.jobs...)
}

func (m *Manager) RunningCount() int {
	n := 0
	for _, j := range m.Jobs() {
		if j.Running() {
			n++
		}
	}
	return n
}

// ClearFinished drops finished jobs from the list.
func (m *Manager) ClearFinished() {
	m.mu.Lock()
	defer m.mu.Unlock()
	var keep []*Job
	for _, j := range m.jobs {
		if j.Running() {
			keep = append(keep, j)
		}
	}
	m.jobs = keep
}

func (m *Manager) notify(u Update) {
	select {
	case m.Updates <- u:
	default: // UI is behind; it re-renders everything on the next update anyway
	}
}

// Spec describes a job to start.
type Spec struct {
	Title   string
	Step    string
	Feature string
	Agent   string
	Model   string
	TaskIDs []string
	// Build creates the command bound to ctx.
	Build  func(ctx context.Context) (*exec.Cmd, error)
	Parser agent.LineParser
	// OnSuccess runs in the job goroutine after a successful exit.
	OnSuccess func()
}

func (m *Manager) Start(s Spec) (*Job, error) {
	ctx, cancel := context.WithCancel(context.Background())
	cmd, err := s.Build(ctx)
	if err != nil {
		cancel()
		return nil, err
	}
	setProcAttrs(cmd)
	m.mu.Lock()
	m.next++
	j := &Job{ID: m.next, Title: s.Title, Step: s.Step, Feature: s.Feature, Agent: s.Agent, Model: s.Model,
		TaskIDs: s.TaskIDs, CmdLine: shellQuote(cmd.Args), status: Running, started: time.Now(), cancel: cancel}
	m.jobs = append(m.jobs, j)
	m.mu.Unlock()

	var logW io.WriteCloser
	if err := os.MkdirAll(m.logDir, 0o755); err == nil {
		j.LogPath = filepath.Join(m.logDir, fmt.Sprintf("%s-job%d-%s.log", time.Now().Format("20060102-150405"), j.ID, s.Step))
		logW, _ = os.Create(j.LogPath)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	j.add(agent.EvInfo, "$ "+truncate(j.CmdLine, 400))
	if err := cmd.Start(); err != nil {
		j.finish(Failed, err)
		cancel()
		m.notify(Update{JobID: j.ID, Done: true})
		return j, err
	}

	var wg sync.WaitGroup
	var logMu sync.Mutex
	pump := func(r io.Reader, isErr bool) {
		defer wg.Done()
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 64*1024), 32*1024*1024)
		for sc.Scan() {
			line := sc.Text()
			if logW != nil {
				logMu.Lock()
				fmt.Fprintln(logW, line)
				logMu.Unlock()
			}
			if isErr {
				if strings.TrimSpace(line) != "" {
					j.add(agent.EvError, line)
				}
			} else {
				for _, ev := range s.Parser.Parse(line) {
					j.add(ev.Kind, ev.Text)
					if ev.Kind == agent.EvResult {
						j.mu.Lock()
						j.cost += ev.CostUSD
						if ev.IsError {
							j.err = fmt.Errorf("agent reported an error")
						}
						j.mu.Unlock()
					}
				}
			}
			m.notify(Update{JobID: j.ID})
		}
	}
	wg.Add(2)
	go pump(stdout, false)
	go pump(stderr, true)
	go func() {
		wg.Wait()
		err := cmd.Wait()
		if logW != nil {
			logW.Close()
		}
		st := Succeeded
		switch {
		case ctx.Err() != nil:
			st, err = Canceled, nil
		case err != nil:
			st = Failed
		default:
			j.mu.Lock()
			if j.err != nil {
				st, err = Failed, j.err
			}
			j.mu.Unlock()
		}
		if st == Succeeded && s.OnSuccess != nil {
			s.OnSuccess()
		}
		j.finish(st, err)
		cancel()
		m.notify(Update{JobID: j.ID, Done: true})
	}()
	return j, nil
}

func (j *Job) finish(st Status, err error) {
	j.mu.Lock()
	j.status, j.ended = st, time.Now()
	if err != nil {
		j.err = err
	}
	j.mu.Unlock()
	switch st {
	case Succeeded:
		j.add(agent.EvInfo, "✓ job finished")
	case Canceled:
		j.add(agent.EvError, "■ canceled by user")
	case Failed:
		j.add(agent.EvError, fmt.Sprintf("✗ job failed: %v", err))
	}
}

func shellQuote(args []string) string {
	out := make([]string, len(args))
	for i, a := range args {
		if a == "" || strings.ContainsAny(a, " \t\n'\"$*()|&;<>") {
			a = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
		}
		out[i] = a
	}
	return strings.Join(out, " ")
}

func truncate(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}
