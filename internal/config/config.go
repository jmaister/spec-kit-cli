// Package config persists user preferences (selected agent, model per agent,
// permission mode and user-defined agents).
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// CustomAgent describes an agent CLI driven purely by an argument template, so
// new tools can be plugged in from the config file without writing Go code.
//
// Placeholders available in Args: {prompt}, {model}.
// Args containing {model} are dropped when no model is selected.
type CustomAgent struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Command   string   `json:"command"`
	Args      []string `json:"args"`
	Models    []string `json:"models,omitempty"`
	InitAI    string   `json:"init_ai,omitempty"`     // value for `specify init --integration`
	PromptDir []string `json:"prompt_dirs,omitempty"` // where spec-kit installs this agent's command files
}

type Config struct {
	Agent          string            `json:"agent"`
	Models         map[string]string `json:"models"`          // agent id -> model ("" = agent default)
	PermissionMode string            `json:"permission_mode"` // agent-specific meaning
	CustomAgents   []CustomAgent     `json:"custom_agents,omitempty"`

	path string
}

func Default() *Config {
	return &Config{
		Agent:          "claude",
		Models:         map[string]string{},
		PermissionMode: "acceptEdits",
	}
}

// Path returns the config file location (~/.config/speckit-tui/config.json).
func Path() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "speckit-tui", "config.json")
}

func Load() (*Config, error) {
	c := Default()
	c.path = Path()
	b, err := os.ReadFile(c.path)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(b, c); err != nil {
		return c, err
	}
	if c.Models == nil {
		c.Models = map[string]string{}
	}
	return c, nil
}

func (c *Config) Save() error {
	if c.path == "" {
		c.path = Path()
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.path, b, 0o644)
}

func (c *Config) Model(agentID string) string { return c.Models[agentID] }

func (c *Config) SetModel(agentID, model string) { c.Models[agentID] = model }
