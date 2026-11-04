package speckit

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Task is one checkbox line of tasks.md, e.g.
//
//   - [ ] T012 [P] [US1] Create User model in src/models/user.py
type Task struct {
	ID       string // T012 (may be empty if the line has no ID)
	Text     string // description without ID and markers
	Raw      string // full text after the checkbox
	Done     bool
	Parallel bool   // [P]
	Story    string // US1, ...
	Line     int    // 0-based line number in tasks.md
	Phase    int    // index into TaskFile.Phases
}

type Phase struct {
	Title string
	Tasks []int // indexes into TaskFile.Tasks
}

func (p Phase) Progress(tf *TaskFile) (done, total int) {
	for _, i := range p.Tasks {
		total++
		if tf.Tasks[i].Done {
			done++
		}
	}
	return
}

type TaskFile struct {
	Path   string
	Phases []Phase
	Tasks  []Task
}

func (tf *TaskFile) Progress() (done, total int) {
	if tf == nil {
		return 0, 0
	}
	for _, t := range tf.Tasks {
		total++
		if t.Done {
			done++
		}
	}
	return
}

// FindTask returns the task index with the given ID or -1.
func (tf *TaskFile) FindTask(id string) int {
	if tf == nil {
		return -1
	}
	for i, t := range tf.Tasks {
		if t.ID == id {
			return i
		}
	}
	return -1
}

// FirstPending returns the index of the first unchecked task or -1.
func (tf *TaskFile) FirstPending() int {
	if tf == nil {
		return -1
	}
	for i, t := range tf.Tasks {
		if !t.Done {
			return i
		}
	}
	return -1
}

var (
	headingRe = regexp.MustCompile(`^(#{2,4})\s+(.+?)\s*$`)
	checkRe   = regexp.MustCompile(`^(\s*[-*]\s+\[)([ xX])(\]\s+)(.*)$`)
	idRe      = regexp.MustCompile(`^(T\d+)\b\s*`)
	markerRe  = regexp.MustCompile(`^\[([^\]]+)\]\s*`)
	TaskIDRe  = regexp.MustCompile(`\bT\d{3,}\b`)
)

// ParseTasks reads and parses a spec-kit tasks.md file.
func ParseTasks(path string) (*TaskFile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseTasks(path, string(b)), nil
}

func parseTasks(path, content string) *TaskFile {
	tf := &TaskFile{Path: path}
	cur := -1
	ensurePhase := func(title string) {
		tf.Phases = append(tf.Phases, Phase{Title: title})
		cur = len(tf.Phases) - 1
	}
	for n, line := range strings.Split(content, "\n") {
		line = strings.TrimRight(line, "\r")
		if m := headingRe.FindStringSubmatch(line); m != nil {
			title := strings.TrimSpace(m[2])
			// Every "Phase ..." heading opens a phase. Other level-2 headings also
			// do, so tasks never end up under an unrelated phase.
			if strings.HasPrefix(strings.ToLower(title), "phase") || len(m[1]) == 2 {
				ensurePhase(title)
			}
			continue
		}
		m := checkRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if cur < 0 {
			ensurePhase("Tasks")
		}
		t := Task{Raw: m[4], Done: m[2] != " ", Line: n, Phase: cur}
		rest := m[4]
		if id := idRe.FindStringSubmatch(rest); id != nil {
			t.ID = id[1]
			rest = rest[len(id[0]):]
		}
		for {
			mk := markerRe.FindStringSubmatch(rest)
			if mk == nil {
				break
			}
			switch v := mk[1]; {
			case v == "P":
				t.Parallel = true
			case strings.HasPrefix(v, "US") || strings.HasPrefix(v, "Story"):
				t.Story = v
			}
			rest = rest[len(mk[0]):]
		}
		t.Text = rest
		tf.Tasks = append(tf.Tasks, t)
		tf.Phases[cur].Tasks = append(tf.Phases[cur].Tasks, len(tf.Tasks)-1)
	}
	// Drop phases without tasks (e.g. "## Notes", "## Dependencies").
	var phases []Phase
	remap := map[int]int{}
	for i, p := range tf.Phases {
		if len(p.Tasks) > 0 {
			remap[i] = len(phases)
			phases = append(phases, p)
		}
	}
	for i := range tf.Tasks {
		tf.Tasks[i].Phase = remap[tf.Tasks[i].Phase]
	}
	tf.Phases = phases
	return tf
}

// SetTaskDone rewrites the checkbox of a task in place. It re-reads the file and
// verifies the line still holds the same task, so concurrent edits by the agent
// are not clobbered.
func SetTaskDone(path string, t Task, done bool) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(b), "\n")
	idx := t.Line
	matches := func(i int) bool {
		if i < 0 || i >= len(lines) {
			return false
		}
		m := checkRe.FindStringSubmatch(strings.TrimRight(lines[i], "\r"))
		return m != nil && m[4] == t.Raw
	}
	if !matches(idx) {
		idx = -1
		for i := range lines {
			if matches(i) {
				idx = i
				break
			}
		}
		if idx < 0 {
			return fmt.Errorf("task %q not found in %s (file changed?)", t.ID, path)
		}
	}
	mark := " "
	if done {
		mark = "X"
	}
	lines[idx] = checkRe.ReplaceAllString(lines[idx], "${1}"+mark+"${3}${4}")
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
}
