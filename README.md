# speckit — TUI para spec-kit

Terminal UI en Go ([Bubble Tea](https://github.com/charmbracelet/bubbletea)) para trabajar con
[GitHub spec-kit](https://github.com/github/spec-kit) sin memorizar comandos: ves features, fases y
tareas con barras de progreso, checkboxes y spinners en lo que se está ejecutando, y lanzas cada
paso del flujo Spec-Driven Development con una tecla.

```
◆ speckit demo                          agent Claude Code  model sonnet  perm acceptEdits  ⣽ 1 running
 Dashboard │ Tasks │ Logs
╭──────────────────────────────╮╭───────────────────────────────────────────────────────────╮
│ Features (2)                 ││ 001-photo-albums                                          │
│ ✔ constitution               ││ Workflow                                                  │
│ ▸ 001-photo-albums ⣽         ││  1 ✔ Constitution  Project principles & governance        │
│   ✔✔✔✔✔✔○◐  ████░░░░ 6/13    ││  ...                                                      │
│                              ││  8 ⣽ Implement     running…                               │
│   002-sharing                ││ Progress  █████████░░░░░░░░░░░  46%  6/13                 │
│   ✔✔○○○○○○  no tasks         ││  ✔ Phase 1: Setup          ████████ 3/3                   │
╰──────────────────────────────╯╰───────────────────────────────────────────────────────────╯
```

## Instalación

```bash
go install github.com/jmaister/spec-kit-cli@latest   # instala el binario "spec-kit-cli"; o: make install (→ "speckit")
speckit            # en el directorio del proyecto
speckit -C ~/code/app -agent claude -model opus
```

Requisitos: el CLI del agente (`claude`, `copilot`, …) y, para `I` (init), `specify` o `uvx`.

## Atajos

| Tecla | Acción |
|---|---|
| `tab` / `shift+tab` | cambiar de pantalla (Dashboard → Tasks → Logs) |
| `n` | nueva feature (`/speckit.specify`) |
| `1`–`8` | ejecutar paso: constitution, specify, clarify, plan, checklist, tasks, analyze, implement |
| `i` | implementar la feature entera |
| `m` / `a` / `p` | elegir modelo / agente / modo de permisos |
| `l` | logs · `h`/`?` ayuda · `r` refrescar · `I` `specify init` · `ctrl+x` cancelar job · `q` salir |
| **Tasks** `space` | marcar/desmarcar tarea (sobre una fase: todas) |
| **Tasks** `x` / `X` | ejecutar solo la tarea / la fase seleccionada |
| **Tasks** `enter` `f` `e` | plegar fase · ocultar hechas · abrir `tasks.md` en `$EDITOR` en esa línea |
| **Logs** `↑↓` `pgup/pgdn` `F` `c` | job · scroll · follow · limpiar terminados |

Antes de ejecutar cualquier paso se abre un diálogo con argumentos opcionales y el comando exacto que
se va a lanzar, para que aprendas el flujo sin tener que recordarlo.

## Cómo funciona

- **Estado desde disco**: se lee `specs/NNN-*/` (spec.md, plan.md, tasks.md, checklists/…) y
  `.specify/memory/constitution.md` cada 1.5 s, así que cuando el agente marca `[X]` en `tasks.md`
  la barra avanza en vivo. Los pasos sin artefacto (analyze) se registran en `.specify/speckit-tui.json`.
- **Feature activa**: los comandos se lanzan con `SPECIFY_FEATURE=<feature>`, que los scripts de
  spec-kit usan para elegir la feature aunque no estés en su rama git.
- **Spinner por tarea**: durante `implement` se sigue el último `T###` que menciona el agente.
- **Logs**: salida legible (texto, herramientas, errores, coste) y el log crudo en
  `~/.cache/speckit-tui/logs/`.
- **Permisos (Claude)**: por defecto `acceptEdits` + permiso para los scripts de `.specify/scripts`.
  `implement` suele necesitar ejecutar tests/builds: cambia con `p` a `auto` o `bypassPermissions`
  si lo necesitas. Las denegaciones aparecen en rojo en los logs.

## Agentes (extensible)

`internal/agent` define la interfaz `Agent` (`Command`, `Parser`, `Models`, `PermissionModes`,
`InitAI`). Incluidos:

- **claude** — `claude -p "/speckit.<step> …" --output-format stream-json --verbose --model …`
- **copilot** — `copilot -p <prompt> --allow-all-tools --model …`; el prompt se genera expandiendo
  `.github/prompts/speckit.<step>.prompt.md` con `$ARGUMENTS`.

Para añadir otro CLI sin escribir Go, decláralo en `~/.config/speckit-tui/config.json`:

```json
{
  "agent": "gemini",
  "models": { "claude": "opus", "gemini": "gemini-2.5-pro" },
  "permission_mode": "acceptEdits",
  "custom_agents": [
    {
      "id": "gemini",
      "name": "Gemini CLI",
      "command": "gemini",
      "args": ["-p", "{prompt}", "--yolo", "--model", "{model}"],
      "models": ["gemini-2.5-pro", "gemini-2.5-flash"],
      "init_ai": "gemini",
      "prompt_dirs": [".gemini/commands"]
    }
  ]
}
```

`{model}` (y el flag anterior) se omite si no hay modelo elegido. Para algo más rico (parsear JSON en
streaming como con Claude), implementa `agent.Agent` y llama a `agent.Register` en un `init()`.

## Estructura

```
main.go                  flags y arranque
internal/config          preferencias (~/.config/speckit-tui/config.json)
internal/speckit         modelo del proyecto: features, tasks.md (parse/toggle), workflow
internal/agent           interfaz Agent + claude, copilot, agentes por plantilla
internal/runner          jobs en background, logs, cancelación por grupo de procesos
internal/tui             Bubble Tea: pantallas, overlays, keymap, estilos
```
