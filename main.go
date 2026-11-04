// speckit is a terminal UI for GitHub spec-kit projects: see the workflow,
// features, phases and tasks at a glance, and run spec-kit commands through an
// AI agent CLI (Claude Code by default) without memorizing them.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jmaister/spec-kit-cli/internal/agent"
	"github.com/jmaister/spec-kit-cli/internal/config"
	"github.com/jmaister/spec-kit-cli/internal/tui"
)

var version = "dev"

func main() {
	dir := flag.String("C", ".", "project directory")
	agentID := flag.String("agent", "", "AI agent to use (claude, copilot, or a custom agent id)")
	model := flag.String("model", "", "model for the selected agent (saved to config)")
	showVersion := flag.Bool("version", false, "print version")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: speckit [flags] [dir]\n\nFlags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nConfig: %s\n", config.Path())
	}
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}
	if flag.NArg() > 0 {
		*dir = flag.Arg(0)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: config: %v\n", err)
	}
	agent.RegisterCustom(cfg.CustomAgents)
	if *agentID != "" {
		if _, ok := agent.Get(*agentID); !ok {
			fmt.Fprintf(os.Stderr, "unknown agent %q\n", *agentID)
			os.Exit(2)
		}
		cfg.Agent = *agentID
	}
	if *model != "" {
		cfg.SetModel(cfg.Agent, *model)
		_ = cfg.Save()
	}

	m, err := tui.New(cfg, *dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
