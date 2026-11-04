package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Claude drives Claude Code in headless mode:
//
//	claude -p "/speckit.plan <args>" --output-format stream-json --verbose [--model m]
//
// spec-kit installs its commands in .claude/commands, so the slash command is
// invoked directly.
type Claude struct{ Bin string }

func init() { Register(&Claude{Bin: "claude"}) }

func (c *Claude) ID() string     { return "claude" }
func (c *Claude) Name() string   { return "Claude Code" }
func (c *Claude) InitAI() string { return "claude" }

func (c *Claude) Models() []string {
	return []string{"opus", "sonnet", "haiku", "fable",
		"claude-opus-5-5", "claude-sonnet-5-5", "claude-haiku-5-5", "claude-fable-5-1"}
}

func (c *Claude) PermissionModes() []string {
	return []string{"acceptEdits", "auto", "bypassPermissions", "dontAsk", "plan"}
}

func (c *Claude) Available() bool {
	_, err := exec.LookPath(c.Bin)
	return err == nil
}

// specKitTools are always allowed so spec-kit's helper scripts work in
// headless mode even under acceptEdits.
var specKitTools = []string{
	"Bash(.specify/scripts/*)", "Bash(bash .specify/scripts/*)", "Bash(sh .specify/scripts/*)",
	"Bash(git status:*)", "Bash(git branch:*)", "Bash(git checkout:*)", "Bash(git fetch:*)",
	"Bash(git ls-remote:*)", "Bash(git rev-parse:*)", "Bash(ls:*)", "Bash(mkdir:*)",
}

func (c *Claude) prompt(req Request) string {
	cmd := ClaudeSlashCommand(req.Root, req.Step)
	if req.Args != "" {
		cmd += " " + req.Args
	}
	return cmd
}

// ClaudeSlashCommand returns the slash command spec-kit installed for a step.
// The layout changed across spec-kit versions:
//
//	.claude/skills/speckit-plan/SKILL.md  -> /speckit-plan  (current: skills)
//	.claude/commands/speckit.plan.md      -> /speckit.plan
//	.claude/commands/plan.md              -> /plan          (oldest)
//
// Defaults to the current skills form when nothing is found.
func ClaudeSlashCommand(root, step string) string {
	switch {
	case fileExists(root, ".claude/skills/speckit-"+step+"/SKILL.md"):
		return "/speckit-" + step
	case fileExists(root, ".claude/commands/speckit."+step+".md"):
		return "/speckit." + step
	case fileExists(root, ".claude/commands/"+step+".md"):
		return "/" + step
	}
	return "/speckit-" + step
}

func fileExists(root, rel string) bool {
	_, err := os.Stat(filepath.Join(root, rel))
	return err == nil
}

func (c *Claude) Command(ctx context.Context, req Request) (*exec.Cmd, error) {
	args := []string{"-p", c.prompt(req), "--output-format", "stream-json", "--verbose"}
	if req.Model != "" {
		args = append(args, "--model", req.Model)
	}
	if req.Perm != "" {
		args = append(args, "--permission-mode", req.Perm)
	}
	args = append(args, "--allowedTools", strings.Join(specKitTools, ","))
	cmd := exec.CommandContext(ctx, c.Bin, args...)
	cmd.Dir = req.Root
	cmd.Env = featureEnv(req)
	return cmd, nil
}

func (c *Claude) Parser() LineParser { return &claudeParser{} }

type claudeParser struct{}

type claudeMsg struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`
	Model   string `json:"model"`
	Message struct {
		Content []struct {
			Type    string          `json:"type"`
			Text    string          `json:"text"`
			Name    string          `json:"name"`
			Input   json.RawMessage `json:"input"`
			IsError bool            `json:"is_error"`
			Content json.RawMessage `json:"content"`
		} `json:"content"`
	} `json:"message"`
	Result            string            `json:"result"`
	IsError           bool              `json:"is_error"`
	TotalCost         float64           `json:"total_cost_usd"`
	DurationMs        int64             `json:"duration_ms"`
	NumTurns          int               `json:"num_turns"`
	PermissionDenials []json.RawMessage `json:"permission_denials"`
}

func (p *claudeParser) Parse(line string) []Event {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}
	var m claudeMsg
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		return []Event{{Kind: EvText, Text: line}}
	}
	var out []Event
	switch m.Type {
	case "system":
		if m.Subtype == "init" {
			model := m.Model
			if model == "" {
				model = "default"
			}
			out = append(out, Event{Kind: EvInfo, Text: "session started · model " + model})
		}
	case "assistant":
		for _, c := range m.Message.Content {
			switch c.Type {
			case "text":
				if t := strings.TrimSpace(c.Text); t != "" {
					out = append(out, Event{Kind: EvText, Text: t})
				}
			case "tool_use":
				out = append(out, Event{Kind: EvTool, Text: c.Name + " " + summarizeToolInput(c.Name, c.Input)})
			}
		}
	case "user":
		for _, c := range m.Message.Content {
			if c.Type == "tool_result" && c.IsError {
				out = append(out, Event{Kind: EvError, Text: "tool error: " + firstLine(rawText(c.Content), 200)})
			}
		}
	case "result":
		for _, d := range m.PermissionDenials {
			var pd struct {
				ToolName  string          `json:"tool_name"`
				ToolInput json.RawMessage `json:"tool_input"`
			}
			_ = json.Unmarshal(d, &pd)
			out = append(out, Event{Kind: EvError, Text: "permission denied: " + pd.ToolName + " " +
				summarizeToolInput(pd.ToolName, pd.ToolInput) + "  (press p to change permission mode)"})
		}
		status := "finished"
		if m.IsError {
			status = "failed (" + m.Subtype + ")"
		}
		summary := fmt.Sprintf("%s · %d turns · %.1fs · $%.4f", status, m.NumTurns, float64(m.DurationMs)/1000, m.TotalCost)
		if r := strings.TrimSpace(m.Result); r != "" && m.IsError {
			summary += "\n" + r
		}
		out = append(out, Event{Kind: EvResult, Text: summary, CostUSD: m.TotalCost, IsError: m.IsError})
	}
	return out
}

func summarizeToolInput(tool string, raw json.RawMessage) string {
	var in map[string]any
	if err := json.Unmarshal(raw, &in); err != nil {
		return ""
	}
	if tool == "TodoWrite" {
		if todos, ok := in["todos"].([]any); ok {
			var parts []string
			for _, t := range todos {
				if tm, ok := t.(map[string]any); ok && tm["status"] == "in_progress" {
					parts = append(parts, fmt.Sprint(tm["content"]))
				}
			}
			if len(parts) > 0 {
				return "→ " + strings.Join(parts, "; ")
			}
			return fmt.Sprintf("(%d todos)", len(todos))
		}
	}
	for _, k := range []string{"command", "file_path", "path", "pattern", "url", "query", "description", "prompt"} {
		if v, ok := in[k].(string); ok && v != "" {
			return firstLine(v, 160)
		}
	}
	return ""
}

func rawText(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var parts []struct {
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &parts) == nil {
		var b strings.Builder
		for _, p := range parts {
			b.WriteString(p.Text)
		}
		return b.String()
	}
	return string(raw)
}

func firstLine(s string, max int) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i] + " …"
	}
	if r := []rune(s); len(r) > max {
		s = string(r[:max]) + "…"
	}
	return s
}
