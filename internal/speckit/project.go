// Package speckit understands the on-disk layout of a spec-kit project:
//
//	.specify/memory/constitution.md
//	specs/NNN-feature/{spec.md,plan.md,tasks.md,research.md,checklists/...}
package speckit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Project struct {
	Root         string
	Initialized  bool // .specify/ exists
	Constitution bool // constitution filled in (not the raw template)
	Features     []*Feature
	State        *State
}

type Feature struct {
	Name  string // directory name, e.g. 001-photo-albums
	Dir   string
	Files map[string]bool // spec.md, plan.md, tasks.md, research.md, data-model.md, quickstart.md, contracts, checklists
	Tasks *TaskFile
	// HasClarifications is true when spec.md contains a Clarifications section.
	HasClarifications bool
	ModTime           time.Time
}

func (f *Feature) Has(name string) bool { return f.Files[name] }

func (f *Feature) Path(name string) string { return filepath.Join(f.Dir, name) }

// Load scans root for a spec-kit project. It never fails on a non spec-kit
// directory; Initialized is simply false.
func Load(root string) (*Project, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	p := &Project{Root: abs}
	if st, err := os.Stat(filepath.Join(abs, ".specify")); err == nil && st.IsDir() {
		p.Initialized = true
	}
	p.State = LoadState(abs)
	if b, err := os.ReadFile(filepath.Join(abs, ".specify", "memory", "constitution.md")); err == nil {
		// The untouched template is full of [PLACEHOLDER] tokens.
		p.Constitution = !strings.Contains(string(b), "[PROJECT_NAME]") && !strings.Contains(string(b), "[PRINCIPLE_1_NAME]")
	}
	entries, err := os.ReadDir(filepath.Join(abs, "specs"))
	if err != nil {
		return p, nil
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p.Features = append(p.Features, loadFeature(filepath.Join(abs, "specs", e.Name())))
	}
	sort.Slice(p.Features, func(i, j int) bool { return p.Features[i].Name < p.Features[j].Name })
	return p, nil
}

func loadFeature(dir string) *Feature {
	f := &Feature{Name: filepath.Base(dir), Dir: dir, Files: map[string]bool{}}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			sub, _ := os.ReadDir(filepath.Join(dir, name))
			f.Files[name] = len(sub) > 0
		} else {
			f.Files[name] = true
		}
		if info, err := e.Info(); err == nil && info.ModTime().After(f.ModTime) {
			f.ModTime = info.ModTime()
		}
	}
	if f.Has("spec.md") {
		if b, err := os.ReadFile(f.Path("spec.md")); err == nil {
			f.HasClarifications = strings.Contains(string(b), "## Clarifications")
		}
	}
	if f.Has("tasks.md") {
		f.Tasks, _ = ParseTasks(f.Path("tasks.md"))
	}
	return f
}

// FeatureByName returns the feature or nil.
func (p *Project) FeatureByName(name string) *Feature {
	for _, f := range p.Features {
		if f.Name == name {
			return f
		}
	}
	return nil
}

// State keeps track of steps that leave no artifact behind (e.g. analyze),
// stored in .specify/speckit-tui.json.
type State struct {
	// Runs maps "feature/step" -> last successful run.
	Runs map[string]time.Time `json:"runs"`
	path string
}

func LoadState(root string) *State {
	s := &State{Runs: map[string]time.Time{}, path: filepath.Join(root, ".specify", "speckit-tui.json")}
	if b, err := os.ReadFile(s.path); err == nil {
		_ = json.Unmarshal(b, s)
		if s.Runs == nil {
			s.Runs = map[string]time.Time{}
		}
	}
	return s
}

func (s *State) Record(feature, step string) error {
	s.Runs[feature+"/"+step] = time.Now()
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(s, "", "  ")
	return os.WriteFile(s.path, b, 0o644)
}

func (s *State) Ran(feature, step string) bool {
	_, ok := s.Runs[feature+"/"+step]
	return ok
}
