package speckit

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const sample = `# Tasks: Photo Albums

## Format: ` + "`[ID] [P?] [Story] Description`" + `

- **[P]**: Can run in parallel

## Phase 1: Setup (Shared Infrastructure)

- [X] T001 Create project structure per implementation plan
- [x] T002 [P] Configure linting in .golangci.yml

## Phase 2: Foundational

- [ ] T003 Setup database schema in db/schema.sql

## Phase 3: User Story 1 - Create albums (Priority: P1) 🎯 MVP

### Tests for User Story 1

- [ ] T004 [P] [US1] Contract test for POST /albums in tests/contract/albums_test.go

### Implementation for User Story 1

- [ ] T005 [US1] Create Album model in internal/models/album.go

## Dependencies

- Setup before everything
`

func TestParseTasks(t *testing.T) {
	tf := parseTasks("tasks.md", sample)
	if len(tf.Phases) != 3 {
		t.Fatalf("phases = %d, want 3: %+v", len(tf.Phases), tf.Phases)
	}
	if got := tf.Phases[2].Title; got != "Phase 3: User Story 1 - Create albums (Priority: P1) 🎯 MVP" {
		t.Errorf("phase title = %q", got)
	}
	if len(tf.Tasks) != 5 {
		t.Fatalf("tasks = %d", len(tf.Tasks))
	}
	t4 := tf.Tasks[3]
	if t4.ID != "T004" || !t4.Parallel || t4.Story != "US1" || t4.Phase != 2 || t4.Done {
		t.Errorf("T004 parsed wrong: %+v", t4)
	}
	if t4.Text != "Contract test for POST /albums in tests/contract/albums_test.go" {
		t.Errorf("text = %q", t4.Text)
	}
	if d, n := tf.Progress(); d != 2 || n != 5 {
		t.Errorf("progress %d/%d", d, n)
	}
	if tf.FirstPending() != 2 {
		t.Errorf("first pending = %d", tf.FirstPending())
	}
}

func TestSetTaskDone(t *testing.T) {
	p := filepath.Join(t.TempDir(), "tasks.md")
	if err := os.WriteFile(p, []byte(sample), 0o644); err != nil {
		t.Fatal(err)
	}
	tf, _ := ParseTasks(p)
	if err := SetTaskDone(p, tf.Tasks[2], true); err != nil {
		t.Fatal(err)
	}
	if err := SetTaskDone(p, tf.Tasks[0], false); err != nil {
		t.Fatal(err)
	}
	tf2, _ := ParseTasks(p)
	if !tf2.Tasks[2].Done || tf2.Tasks[0].Done {
		t.Errorf("toggle failed: %+v", tf2.Tasks[:3])
	}
	// Line shifted by an external edit: still found by content.
	b, _ := os.ReadFile(p)
	os.WriteFile(p, append([]byte("\n\n"), b...), 0o644)
	if err := SetTaskDone(p, tf2.Tasks[4], true); err != nil {
		t.Fatal(err)
	}
	tf3, _ := ParseTasks(p)
	if !tf3.Tasks[4].Done {
		t.Error("T005 should be done")
	}
}

func TestStage(t *testing.T) {
	p := &Project{Constitution: true, State: &State{Runs: map[string]time.Time{}}}
	f := &Feature{Name: "001-x", Files: map[string]bool{}}
	check := func(wantStage, wantPhase string) {
		t.Helper()
		if s, ph := p.Stage(f); s != wantStage || ph != wantPhase {
			t.Errorf("Stage = %q/%q, want %q/%q", s, ph, wantStage, wantPhase)
		}
	}
	check("New", "")
	f.Files["spec.md"] = true
	check("Specified", "")
	f.Files["plan.md"] = true
	check("Planned", "")
	f.Files["tasks.md"] = true
	f.Tasks = parseTasks("tasks.md", sample)
	check("Implementing", "Phase 2: Foundational") // sample has phase 1 done
	for i := range f.Tasks.Tasks {
		f.Tasks.Tasks[i].Done = true
	}
	check("Done", "")
}
