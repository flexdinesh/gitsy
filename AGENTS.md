# AGENTS.md

## What This Repo Is

`gitsy` is a small Go CLI that scans a directory for child Git repositories and linked worktrees, fetches upstream metadata by default, and shows a compact Bubble Tea status table. It is meant to make multi-repo workspaces easy to inspect and optionally fast-forward with `--sync`.

The main entry point is `cmd/gitsy/main.go`. Most behavior lives in focused internal packages:

- `internal/args`: command-line parsing and usage text
- `internal/discover`: repository and worktree discovery
- `internal/git`: thin wrappers around Git commands
- `internal/inspect`: fetch/status/sync orchestration
- `internal/status`: parsing `git status --short --branch --ahead-behind`
- `internal/ui`: display rows and status formatting
- `internal/tui`: Bubble Tea model, table, spinner, layout, and rendering

## Commands

Run these from the repo root.

```bash
# Run the full test suite before and after behavior changes.
go test ./...

# Build the CLI locally.
go build -o bin/gitsy ./cmd/gitsy

# Install the local build into your Go bin path.
go install ./cmd/gitsy

# Try the CLI against the current directory.
go run ./cmd/gitsy

# Try useful modes while developing.
go run ./cmd/gitsy --all
go run ./cmd/gitsy --no-fetch
go run ./cmd/gitsy --verbose
go run ./cmd/gitsy --sync

# Keep module files tidy after dependency changes.
go mod tidy
```

Use targeted tests while iterating, then run `go test ./...` before calling work done.

```bash
go test ./internal/status
go test ./internal/tui
go test ./cmd/gitsy
```

## Development Patterns

- Keep packages small and boring. Prefer extending the existing internal package that owns the behavior.
- Keep Git execution inside `internal/git`; callers should not shell out directly.
- Keep command-line parsing in `internal/args`; update `args.Usage` and tests when adding flags.
- Keep raw status parsing in `internal/status`; keep display wording and row shaping in `internal/ui`.
- Keep Bubble Tea state transitions in `internal/tui`; avoid mixing Git calls or parsing logic into rendering code.
- Prefer context-aware functions for work that can block or be cancelled.
- Preserve deterministic ordering for discovered repos and rendered rows.
- Treat warnings as optional user-facing diagnostics controlled by `--verbose`.
- Add focused tests near the changed package. Use integration tests only when behavior crosses package boundaries.
- Do not overwrite user work. Check `git status --short` before editing and avoid unrelated cleanup.

## Code Standards

- Use idiomatic Go, `gofmt`, simple names, and explicit errors.
- Favor standard library code unless a dependency already exists for the job.
- Keep comments rare and useful; explain non-obvious behavior, not the syntax.
- Use table-driven tests for parsers, formatters, and CLI argument behavior.
- Avoid global mutable state unless it is effectively configuration, like ignored directory names.
- Keep terminal output compact and stable because tests and users rely on exact wording.

## Dependencies

This project intentionally keeps dependencies light. Direct UI dependencies are:

- `github.com/charmbracelet/bubbletea`
- `github.com/charmbracelet/bubbles`
- `github.com/charmbracelet/lipgloss`
- `github.com/mattn/go-runewidth`

When adding or updating dependencies:

```bash
go get github.com/charmbracelet/bubbletea@latest
go get github.com/charmbracelet/bubbles@latest
go get github.com/charmbracelet/lipgloss@latest
go mod tidy
go test ./...
```

Only add new dependencies when they clearly reduce complexity or match the existing TUI stack.

## Bubble Tea, Bubbles, And Lip Gloss

- Follow Bubble Tea's model/update/view shape: `Init` starts commands, `Update` mutates state and returns commands, `View` renders from state.
- Keep `tea.Cmd` work small, cancellable, and free of rendering concerns.
- Use `tea.Batch` when starting independent commands.
- Keep rendering pure. `View` should derive output from model state and should not run Git commands, mutate external state, or perform I/O.
- Use Bubbles components for standard terminal widgets instead of custom controls where practical. This repo currently uses `spinner` and `table`.
- Keep Lip Gloss styles close to the rendering code that uses them unless a style is shared enough to justify extraction.
- Use `runewidth` or Bubble/Lip Gloss width helpers when aligning terminal text; do not assume byte length equals display width.
- Be careful with Unicode symbols. They are fine for compact status output, but tests should cover exact strings and width-sensitive layout.
- Always test TUI changes with small terminal sizes, empty results, loading rows, clean repos, changed repos, failed status, stale fetch, and `--sync` outcomes.
