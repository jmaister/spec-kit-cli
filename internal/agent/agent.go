// Package agent abstracts the AI coding CLI (Claude Code, Copilot CLI, ...)
// used to run spec-kit commands. Adding a new tool means implementing Agent and
// calling Register, or declaring a custom agent in the config file.
package agent

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Request is one spec-kit command execution.
type Request struct {
	Root    string // project root (working directory)
	Step    string // spec-kit step id: specify, plan, tasks, implement...
	Args    string // user text passed as $ARGUMENTS
	Model   string // "" = agent default
	Perm    string // permission mode (agent specific)
	Feature string // feature dir name, exported as SPECIFY_FEATURE
}

type EventKind int

const (
	EvInfo EventKind = iota
	EvText
	EvTool
	EvToolResult
	EvError
	EvResult
)

// Event is a human friendly log entry produced from the agent's raw output.
type Event struct {
	Kind EventKind
	Text string
	// Optional metadata (set on EvResult).
	CostUSD float64
	IsError bool
	Model   string // actual model reported by the agent (session start)
}

type Agent interface {
	ID() string   // stable id stored in config: "claude"
	Name() string // display name
	// Models returns suggested model identifiers. "" (agent default) is
	// always offered by the UI, and users can type any custom value.
	Models() []string
	// PermissionModes lists accepted values for Request.Perm (may be nil).
	PermissionModes() []string
	// InitAI is the spec-kit integration key: `specify init --integration <x>`.
	InitAI() string
	// Available reports whether the CLI binary is installed.
	Available() bool
	// Command builds the process that runs the request.
	Command(ctx context.Context, req Request) (*exec.Cmd, error)
	// Parser returns a fresh stateful parser turning stdout lines into events.
	Parser() LineParser
}

// DefaultModeler is implemented by agents that can tell which model they use
// when none is passed explicitly (e.g. from their own settings files).
type DefaultModeler interface {
	DefaultModel(root string) string
}

// ResolveModel returns the model that will actually be used: the selected one,
// or the agent's configured default. "" means unknown.
func ResolveModel(a Agent, root, selected string) string {
	if selected != "" {
		return selected
	}
	if d, ok := a.(DefaultModeler); ok {
		return d.DefaultModel(root)
	}
	return ""
}

type LineParser interface {
	Parse(line string) []Event
}

var registry = map[string]Agent{}

func Register(a Agent) { registry[a.ID()] = a }

func Get(id string) (Agent, bool) {
	a, ok := registry[id]
	return a, ok
}

// All returns registered agents sorted by id (claude first).
func All() []Agent {
	var out []Agent
	for _, a := range registry {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ID() == "claude" {
			return true
		}
		if out[j].ID() == "claude" {
			return false
		}
		return out[i].ID() < out[j].ID()
	})
	return out
}

// PromptFromCommandFile turns a spec-kit command template installed for an
// agent (markdown with YAML front matter and $ARGUMENTS) into a plain prompt.
// It's the portable way to run spec-kit with CLIs that can't invoke custom
// slash commands in non-interactive mode.
func PromptFromCommandFile(root string, dirs []string, step, args string) (string, bool) {
	names := []string{
		"speckit-" + step + "/SKILL.md", // skills layout (current spec-kit)
		"speckit." + step + ".prompt.md", "speckit." + step + ".agent.md", "speckit." + step + ".md",
		step + ".prompt.md", step + ".md",
	}
	for _, d := range dirs {
		for _, n := range names {
			b, err := os.ReadFile(filepath.Join(root, d, n))
			if err != nil {
				continue
			}
			body := stripFrontMatter(string(b))
			if strings.Contains(body, "$ARGUMENTS") {
				body = strings.ReplaceAll(body, "$ARGUMENTS", args)
			} else if args != "" {
				body += "\n\nUser input:\n\n" + args
			}
			return body, true
		}
	}
	return "", false
}

func stripFrontMatter(s string) string {
	if !strings.HasPrefix(s, "---") {
		return s
	}
	rest := s[3:]
	if i := strings.Index(rest, "\n---"); i >= 0 {
		return strings.TrimLeft(rest[i+4:], "\r\n")
	}
	return s
}

// fallbackPrompt is used when no command file is found for an agent.
func fallbackPrompt(step, args string) string {
	p := "Run the spec-kit '" + step + "' step (Spec-Driven Development) for this repository, following the templates and scripts in .specify/."
	if args != "" {
		p += "\n\nUser input:\n\n" + args
	}
	return p
}

// featureEnv returns the environment for a request so spec-kit scripts target
// the selected feature regardless of git branch or .specify/feature.json:
// current scripts resolve it from SPECIFY_FEATURE_DIRECTORY, older ones from
// SPECIFY_FEATURE.
func featureEnv(req Request) []string {
	env := os.Environ()
	if req.Feature != "" {
		env = append(env, "SPECIFY_FEATURE="+req.Feature,
			"SPECIFY_FEATURE_DIRECTORY="+filepath.Join("specs", req.Feature))
	}
	return env
}

// PlainParser treats every non-empty line as text.
type PlainParser struct{}

func (PlainParser) Parse(line string) []Event {
	if strings.TrimSpace(line) == "" {
		return nil
	}
	return []Event{{Kind: EvText, Text: line}}
}
