package agent

import (
	"context"
	"os/exec"
	"strings"

	"github.com/jmaister/spec-kit-cli/internal/config"
)

// TemplateAgent runs any CLI from an argument template. The spec-kit command
// file installed for the agent is expanded into a plain prompt, so it works
// with tools that can't run custom slash commands headlessly.
//
// Placeholders: {prompt}, {model}. An argument containing {model} is removed
// together with the preceding flag when no model is set, e.g.
// ["--model", "{model}"] disappears entirely.
type TemplateAgent struct {
	Def config.CustomAgent
}

func (t *TemplateAgent) ID() string                { return t.Def.ID }
func (t *TemplateAgent) Name() string              { return t.Def.Name }
func (t *TemplateAgent) Models() []string          { return t.Def.Models }
func (t *TemplateAgent) PermissionModes() []string { return nil }
func (t *TemplateAgent) Parser() LineParser        { return PlainParser{} }

func (t *TemplateAgent) InitAI() string {
	if t.Def.InitAI != "" {
		return t.Def.InitAI
	}
	return t.Def.ID
}

func (t *TemplateAgent) Available() bool {
	_, err := exec.LookPath(t.Def.Command)
	return err == nil
}

func (t *TemplateAgent) Command(ctx context.Context, req Request) (*exec.Cmd, error) {
	prompt, ok := PromptFromCommandFile(req.Root, t.Def.PromptDir, req.Step, req.Args)
	if !ok {
		prompt = fallbackPrompt(req.Step, req.Args)
	}
	var args []string
	for i := 0; i < len(t.Def.Args); i++ {
		a := t.Def.Args[i]
		if strings.Contains(a, "{model}") && req.Model == "" {
			if len(args) > 0 && strings.HasPrefix(args[len(args)-1], "-") {
				args = args[:len(args)-1]
			}
			continue
		}
		a = strings.ReplaceAll(a, "{model}", req.Model)
		a = strings.ReplaceAll(a, "{prompt}", prompt)
		args = append(args, a)
	}
	cmd := exec.CommandContext(ctx, t.Def.Command, args...)
	cmd.Dir = req.Root
	cmd.Env = featureEnv(req)
	return cmd, nil
}

// Copilot CLI: `copilot -p <prompt> --allow-all-tools [--model m]`.
func init() {
	Register(&TemplateAgent{Def: config.CustomAgent{
		ID:        "copilot",
		Name:      "GitHub Copilot CLI",
		Command:   "copilot",
		Args:      []string{"-p", "{prompt}", "--allow-all-tools", "--model", "{model}"},
		Models:    []string{"claude-sonnet-4.5", "claude-opus-4.5", "gpt-5", "gpt-5.1-codex"},
		InitAI:    "copilot",
		PromptDir: []string{".github/skills", ".github/prompts", ".github/agents"},
	}})
}

// RegisterCustom registers user-defined agents from the config file.
func RegisterCustom(defs []config.CustomAgent) {
	for _, d := range defs {
		if d.ID == "" || d.Command == "" {
			continue
		}
		if d.Name == "" {
			d.Name = d.ID
		}
		if len(d.PromptDir) == 0 {
			d.PromptDir = []string{"." + d.ID + "/skills", "." + d.ID + "/commands", ".github/prompts"}
		}
		Register(&TemplateAgent{Def: d})
	}
}
