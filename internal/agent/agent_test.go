package agent

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/jmaister/spec-kit-cli/internal/config"
)

func TestClaudeParser(t *testing.T) {
	p := (&Claude{}).Parser()
	evs := p.Parse(`{"type":"assistant","message":{"content":[{"type":"text","text":"Working on T005"},{"type":"tool_use","name":"Edit","input":{"file_path":"a.go"}}]}}`)
	if len(evs) != 2 || evs[0].Kind != EvText || evs[1].Text != "Edit a.go" {
		t.Fatalf("got %+v", evs)
	}
	evs = p.Parse(`{"type":"result","subtype":"success","is_error":false,"total_cost_usd":0.5,"num_turns":3,"duration_ms":2000,"permission_denials":[{"tool_name":"Bash","tool_input":{"command":"go test"}}]}`)
	if len(evs) != 2 || evs[0].Kind != EvError || evs[1].Kind != EvResult || evs[1].CostUSD != 0.5 {
		t.Fatalf("got %+v", evs)
	}
}

func TestClaudeCommand(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".claude/commands"), 0o755)
	os.WriteFile(filepath.Join(root, ".claude/commands/speckit.plan.md"), []byte("x"), 0o644)
	cmd, _ := (&Claude{Bin: "claude"}).Command(context.Background(), Request{Root: root, Step: "plan", Args: "use Go", Model: "opus", Perm: "acceptEdits", Feature: "001-x"})
	if cmd.Args[2] != "/speckit.plan use Go" || !slices.Contains(cmd.Args, "opus") {
		t.Fatalf("args %q", cmd.Args)
	}
	if !slices.Contains(cmd.Env, "SPECIFY_FEATURE=001-x") {
		t.Error("missing SPECIFY_FEATURE")
	}
}

func TestTemplateAgent(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".github/prompts"), 0o755)
	os.WriteFile(filepath.Join(root, ".github/prompts/speckit.tasks.prompt.md"), []byte("---\ndescription: x\n---\nDo tasks: $ARGUMENTS"), 0o644)
	a := &TemplateAgent{Def: config.CustomAgent{ID: "t", Command: "tool", Args: []string{"-p", "{prompt}", "--model", "{model}"}, PromptDir: []string{".github/prompts"}}}
	cmd, _ := a.Command(context.Background(), Request{Root: root, Step: "tasks", Args: "fast"})
	if want := []string{"tool", "-p", "Do tasks: fast"}; !slices.Equal(cmd.Args, want) {
		t.Fatalf("args %q want %q", cmd.Args, want)
	}
	cmd, _ = a.Command(context.Background(), Request{Root: root, Step: "tasks", Model: "gpt-5"})
	if cmd.Args[len(cmd.Args)-1] != "gpt-5" {
		t.Fatalf("args %q", cmd.Args)
	}
}
