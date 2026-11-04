# sk — a TUI for spec-kit

A terminal UI written in Go ([Bubble Tea](https://github.com/charmbracelet/bubbletea)) for working with
[GitHub spec-kit](https://github.com/github/spec-kit) without memorizing commands. Browse your
features and see at a glance which stage each one is in, which `tasks.md` phase is being worked on,
and how many tasks are done. Then run the next Spec-Driven Development step with a single key.

```
◆ sk demo                                                  agent Claude Code  model sonnet  perm acceptEdits  ⣽ 1 running
 Dashboard │ Tasks │ Logs
╭──────────────────────────────────────────────╮╭──────────────────────────────────────────────────────────────╮
│ Features (4)                                 ││ 001-photo-albums                                             │
│ ✔ constitution                               ││ Workflow                                                     │
│                                              ││   ✔ Constitution  Project principles & governance            │
│ ▸ 001-photo-albums ⣽                         ││   ✔ Specify       Create a new feature spec                  │
│   Implementing · Phase 3: User Story 1       ││   ✔ Plan          Technical plan: stack, architecture        │
│   ██████████████░░░░░░░░░░░░░░ 6/13  46%     ││   ✔ Tasks         Break the plan into actionable tasks       │
│                                              ││ ▸ ⣽ Implement     running…                                   │
│   002-sharing                                ││                                                              │
│   Specified                                  ││ Progress  █████████░░░░░░░░░░░  46%  6/13                    │
│   ░░░░░░░░░░░░░░░░░░░░░░░░░░░░  no tasks     ││  ✔ Phase 1: Setup          ████████ 3/3                      │
│                                              ││  ⣽ Phase 3: User Story 1   ░░░░░░░░ 0/5                      │
│   004-search                                 ││                                                              │
│   Done                                       ││ Next: Implement — press s to run it, or → to pick a step     │
│   ████████████████████████████ 2/2 100%      ││                                                              │
╰──────────────────────────────────────────────╯╰──────────────────────────────────────────────────────────────╯
```

## Install

```bash
make install                                               # → ~/.local/bin/sk (override with PREFIX=/some/dir)
go install github.com/jmaister/spec-kit-cli/cmd/sk@latest  # → $(go env GOPATH)/bin, which must be on your PATH
sk                                                         # run it in your project directory
sk -C ~/code/app -agent claude -model opus
```

Requirements: the agent CLI (`claude`, `copilot`, …). Initializing a project (`I`) also needs
spec-kit's `specify` CLI, or one of `uv`/`uvx`/`pipx` to run it without installing:

```bash
curl -LsSf https://astral.sh/uv/install.sh | sh
uv tool install specify-cli --from git+https://github.com/github/spec-kit.git
```

## Keyboard shortcuts

| Key | Action |
|---|---|
| `tab` / `shift+tab` | switch screen (Dashboard → Tasks → Logs) |
| `n` | new feature (specify) |
| `s` | run the next suggested workflow step for the selected feature |
| `i` | implement the whole feature |
| `m` / `a` / `p` | choose model / agent / permission mode |
| `l` · `h`/`?` · `r` · `I` · `ctrl+x` · `q` | logs · help · refresh · `specify init` · cancel job · quit |
| **Dashboard** `↑↓` | move through features (stage + task progress bar for each) |
| **Dashboard** `→` / `←` | focus the workflow steps / go back to features |
| **Dashboard** `enter` | on a feature: open its tasks · on a step: run it |
| **Dashboard** `e` | open spec.md in `$EDITOR` |
| **Tasks** `space` | toggle a task checkbox (on a phase header: all of its tasks) |
| **Tasks** `x` / `X` | run only the selected task / phase |
| **Tasks** `enter` `f` `e` | fold phase · hide completed · open `tasks.md` in `$EDITOR` at that line |
| **Logs** `↑↓` `pgup/pgdn` `F` `c` | select job · scroll · follow · clear finished |

Before any step runs, a dialog shows optional arguments and the exact command that will be
executed, so you learn the workflow without having to remember it.

## How it works

- **Feature stages**: each feature shows where it is in the workflow: *New → Specified →
  Clarified → Planned → Ready to implement → Implementing → Done*. While it's being implemented,
  the first `tasks.md` phase with pending tasks is shown next to the stage.
- **State comes from disk**: `specs/NNN-*/` (spec.md, plan.md, tasks.md, checklists/…) and
  `.specify/memory/constitution.md` are re-read every 1.5 s. When the agent checks off `[X]` in
  `tasks.md`, the progress bars update live. Steps that leave no artifact behind (analyze) are
  recorded in `.specify/speckit-tui.json`.
- **Active feature**: commands run with `SPECIFY_FEATURE_DIRECTORY=specs/<feature>` (plus
  `SPECIFY_FEATURE=<feature>` for older spec-kit versions). spec-kit's scripts use these to pick the
  feature even when you're not on its git branch.
- **spec-kit versions**: `I` runs `specify init --integration <agent> --non-interactive`, or
  `--ai` if the installed `specify` is an older release. Commands are detected from the installed
  layout: skills `/speckit-plan` (current), `/speckit.plan` or `/plan`.
- **Model shown**: the model you pick with `m`. Otherwise it's the agent's own setting (for Claude:
  `ANTHROPIC_MODEL`, `.claude/settings.local.json`, `.claude/settings.json`, `~/.claude/settings.json`).
  Jobs show the model the agent actually reported.
- **Per-task spinner**: during `implement`, sk tracks the last `T###` the agent mentioned.
- **Logs**: readable output (text, tool calls, errors, cost). The raw log is saved in
  `~/.cache/speckit-tui/logs/`.
- **Permissions (Claude)**: the default is `acceptEdits`, plus access to `.specify/scripts`.
  `implement` usually needs to run tests and builds, so switch with `p` to `auto` or
  `bypassPermissions` if needed. Denied tool calls show up in red in the logs.

## Agents (extensible)

`internal/agent` defines the `Agent` interface (`Command`, `Parser`, `Models`, `PermissionModes`,
`InitAI`). Built in:

- **claude** — `claude -p "/speckit-<step> …" --output-format stream-json --verbose --model …`
- **copilot** — `copilot -p <prompt> --allow-all-tools --model …`. The prompt is built by expanding
  `.github/skills/speckit-<step>/SKILL.md` (or `.github/prompts/speckit.<step>.prompt.md`) with `$ARGUMENTS`.

To add another CLI without writing Go, declare it in `~/.config/speckit-tui/config.json`:

```json
{
  "agent": "mycli",
  "models": { "claude": "opus", "mycli": "some-model" },
  "permission_mode": "acceptEdits",
  "custom_agents": [
    {
      "id": "mycli",
      "name": "My agent CLI",
      "command": "mycli",
      "args": ["--prompt", "{prompt}", "--yes", "--model", "{model}"],
      "models": ["some-model", "other-model"],
      "init_ai": "generic",
      "prompt_dirs": [".mycli/skills", ".mycli/commands"]
    }
  ]
}
```

When no model is selected, `{model}` and the flag before it are dropped. `prompt_dirs` are searched
for Markdown command files (`speckit-<step>/SKILL.md`, `speckit.<step>.md`, …). If none is found,
a generic "run the spec-kit `<step>` step" prompt is used. For richer integrations, such as
parsing streaming JSON like the Claude agent does, implement `agent.Agent` and call
`agent.Register` from an `init()`.

## Layout

```
cmd/sk/main.go           flags and startup
internal/config          preferences (~/.config/speckit-tui/config.json)
internal/speckit         project model: features, stages, tasks.md (parse/toggle), workflow
internal/agent           Agent interface + claude, copilot, template-based agents
internal/runner          background jobs, logs, process-group cancellation
internal/tui             Bubble Tea: screens, overlays, keymap, styles
```
