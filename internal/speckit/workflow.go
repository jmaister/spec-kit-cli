package speckit

// Step is one spec-kit slash command in the Spec-Driven Development flow.
type Step struct {
	ID          string // command suffix: specify -> /speckit.specify
	Title       string
	Description string
	Optional    bool
	// ProjectScope steps don't belong to a feature (constitution, specify).
	ProjectScope bool
	// InputRequired: the command makes no sense without user text.
	InputRequired bool
	InputHint     string
}

var Steps = []Step{
	{ID: "constitution", Title: "Constitution", ProjectScope: true,
		Description: "Project principles & governance (.specify/memory/constitution.md)",
		InputHint:   "Principles to apply (code quality, testing, UX...). Empty = infer from repo"},
	{ID: "specify", Title: "Specify", ProjectScope: true, InputRequired: true,
		Description: "Create a new feature spec: the what & why (spec.md)",
		InputHint:   "Describe the feature you want to build"},
	{ID: "clarify", Title: "Clarify", Optional: true,
		Description: "Ask & record clarifications for underspecified areas",
		InputHint:   "Optional focus areas"},
	{ID: "plan", Title: "Plan",
		Description: "Technical plan: stack, architecture (plan.md, research.md, data-model.md)",
		InputHint:   "Tech stack and architecture choices"},
	{ID: "checklist", Title: "Checklist", Optional: true,
		Description: "Quality checklists for requirements (checklists/)",
		InputHint:   "Checklist domain, e.g. ux, security, api"},
	{ID: "tasks", Title: "Tasks",
		Description: "Break the plan into actionable tasks (tasks.md)",
		InputHint:   "Optional extra instructions"},
	{ID: "analyze", Title: "Analyze", Optional: true,
		Description: "Cross-artifact consistency & coverage analysis (read-only)",
		InputHint:   "Optional focus"},
	{ID: "implement", Title: "Implement",
		Description: "Execute tasks.md and check tasks off as they complete",
		InputHint:   "Optional: task IDs / phase / extra instructions"},
}

func StepByID(id string) (Step, bool) {
	for _, s := range Steps {
		if s.ID == id {
			return s, true
		}
	}
	return Step{}, false
}

type StepStatus int

const (
	StepPending StepStatus = iota
	StepDone
	StepPartial // implement started but not finished
)

// Status computes a step status from artifacts on disk. f may be nil for
// project-scoped steps.
func (p *Project) Status(f *Feature, stepID string) StepStatus {
	ran := func() bool { return f != nil && p.State != nil && p.State.Ran(f.Name, stepID) }
	switch stepID {
	case "constitution":
		if p.Constitution {
			return StepDone
		}
	case "specify":
		if f != nil && f.Has("spec.md") {
			return StepDone
		}
	case "clarify":
		if f != nil && (f.HasClarifications || ran()) {
			return StepDone
		}
	case "plan":
		if f != nil && f.Has("plan.md") {
			return StepDone
		}
	case "checklist":
		if f != nil && (f.Has("checklists") || ran()) {
			return StepDone
		}
	case "tasks":
		if f != nil && f.Has("tasks.md") {
			return StepDone
		}
	case "analyze":
		if ran() {
			return StepDone
		}
	case "implement":
		if f != nil && f.Tasks != nil {
			done, total := f.Tasks.Progress()
			if total > 0 && done == total {
				return StepDone
			}
			if done > 0 {
				return StepPartial
			}
		}
	}
	return StepPending
}

// Stage summarizes where a feature is in the workflow, e.g. "Planned" or
// "Implementing". phase is the first tasks.md phase with pending tasks (the
// one being worked on) and is empty when there are no pending tasks.
func (p *Project) Stage(f *Feature) (stage, phase string) {
	if f == nil {
		return "", ""
	}
	if f.Tasks != nil {
		for _, ph := range f.Tasks.Phases {
			if d, t := ph.Progress(f.Tasks); d < t {
				phase = ph.Title
				break
			}
		}
	}
	switch {
	case p.Status(f, "implement") == StepDone:
		return "Done", ""
	case p.Status(f, "implement") == StepPartial:
		return "Implementing", phase
	case f.Has("tasks.md"):
		return "Ready to implement", phase
	case f.Has("plan.md"):
		return "Planned", ""
	case f.Has("spec.md") && f.HasClarifications:
		return "Clarified", ""
	case f.Has("spec.md"):
		return "Specified", ""
	}
	return "New", ""
}

// NextStep suggests the next required step for a feature.
func (p *Project) NextStep(f *Feature) string {
	if !p.Constitution {
		return "constitution"
	}
	if f == nil {
		return "specify"
	}
	for _, s := range Steps {
		if s.Optional || s.ProjectScope {
			continue
		}
		if p.Status(f, s.ID) != StepDone {
			return s.ID
		}
	}
	return ""
}
