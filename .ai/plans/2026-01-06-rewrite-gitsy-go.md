# Rewrite gitsy as a Go CLI

## Summary

Rewrite the current TypeScript/Ink CLI as a Go CLI in one conversion. Preserve the core command behavior and Git workflows, but remove fullscreen mode. The new CLI should be delightful in static terminal output, using Go CLI/TUI dependencies where useful. Functional compatibility is enough: same intent, flags, discovery, status, fetch, sync, and install story, without needing exact character-for-character output parity.

## User Decisions

- `--fullscreen` will be removed.
- Go dependencies are acceptable for polished terminal output.
- Functional output compatibility is enough; exact Ink/table rendering parity is not required.
- Convert in one shot instead of keeping TypeScript and Go side by side.
- Installation should support GitHub URL install and local directory build only.

## Key Implementation Changes

1. Create a Go module.
   - Add `go.mod`.
   - Use `cmd/gitsy/main.go` for the executable.
   - Use internal packages:
     - `internal/args`
     - `internal/git`
     - `internal/discover`
     - `internal/status`
     - `internal/ui`
     - `internal/version`

2. Port CLI flags.
   - Keep:
     - `--all`
     - `--max-depth <n>`
     - `--dir <path>`
     - `--verbose`
     - `--no-fetch`
     - `--sync`
     - `--help`
     - `--version`
   - Remove:
     - `--fullscreen`
   - Preserve current defaults:
     - `--max-depth` defaults to `3`
     - scan starts at current working directory unless `--dir` is set
     - fetch runs by default
     - clean repos are hidden unless `--all`

3. Port Git discovery.
   - Recursively scan child directories for `.git` directories and `.git` files.
   - Continue skipping generated/cache directories like `node_modules`, `dist`, `.next`, `target`, etc.
   - Continue excluding the current directory's root `.git`.
   - Use `git rev-parse --show-toplevel` to verify candidates.
   - Use `git worktree list --porcelain` to include linked worktrees.
   - Deduplicate repositories by real path.
   - Preserve display names:
     - relative paths inside scan dir
     - absolute paths outside scan dir

4. Port Git operations.
   - Wrap `git` commands with `os/exec`.
   - Preserve:
     - `git status --short --branch --ahead-behind`
     - `git fetch --all`
     - `git merge --ff-only`
     - `git worktree list --porcelain`
     - `git rev-parse --show-toplevel`
   - Use `context.WithTimeout` for fetch, matching the current 30-second timeout.

5. Port status parsing.
   - Preserve branch parsing for:
     - branch name
     - upstream
     - ahead count
     - behind count
     - gone upstream
   - Preserve status categories:
     - modified
     - staged
     - untracked
     - deleted
     - renamed
     - conflict
     - other
   - Preserve fast-forward eligibility:
     - branch exists
     - upstream exists
     - upstream not gone
     - behind > 0
     - ahead == 0
     - no local status items

6. Build a polished static terminal UI.
   - Use dependencies for width-aware table rendering and styling.
   - Recommended:
     - `github.com/charmbracelet/lipgloss` for styled terminal output
     - `github.com/mattn/go-runewidth` if needed for precise visible-width truncation
   - Render a concise table with repo name and status.
   - Keep visual indicators for:
     - clean
     - fetching/stale
     - ahead/behind
     - upstream gone
     - sync success/failure
     - file change categories

7. Port fetch/status concurrency.
   - Discover repos first.
   - Start fetch/status work concurrently per repo.
   - If `--no-fetch`, skip network calls and use local status only.
   - If `--sync`, always fetch, then fast-forward only safe repos.
   - Static mode should print final results after all repos complete.

8. Replace the test suite with Go tests.
   - Port current test coverage:
     - args parsing
     - discovery
     - worktree parsing
     - display names
     - status parsing
     - row/table model formatting
     - real Git integration tests
   - Add/keep integration coverage for:
     - `--no-fetch`
     - `--sync` fast-forwarding a strictly-behind clone
     - refusing sync on diverged branches

9. Update docs and remove TypeScript.
   - Rewrite `README.md` install instructions:
     - GitHub install: `go install github.com/flexdinesh/gitsy/cmd/gitsy@latest`
     - Local build: `go build -o gitsy ./cmd/gitsy`
   - Remove Node/pnpm references.
   - Delete:
     - `src/`
     - `test/*.ts`
     - `package.json`
     - `pnpm-lock.yaml`
     - `pnpm-workspace.yaml`
     - `tsconfig.json`
     - `.node-version`
   - Keep decision logs unless explicitly requested otherwise.

## Tests And Verification

Run:

```bash
go test ./...
go build ./cmd/gitsy
```

Manual checks:

```bash
./gitsy --help
./gitsy --version
./gitsy --no-fetch
./gitsy --all --no-fetch
./gitsy --max-depth 5 --no-fetch
./gitsy --dir /some/workspace --no-fetch
./gitsy --sync
```

Also verify from a temporary Git workspace:

- clean child repo is hidden by default
- clean child repo appears with `--all`
- dirty repo appears by default
- linked worktree is discovered
- strictly-behind clean clone is fast-forwarded by `--sync`
- diverged clone is not fast-forwarded

## Tradeoffs And Risks

- Removing fullscreen simplifies the rewrite substantially, but users relying on scrollable alternate-screen mode lose that workflow.
- Functional compatibility allows a better Go-native UI, but snapshots from the old Ink output should not be treated as golden output.
- Concurrent `git fetch --all` can still be slow or network-sensitive across many repos; `--no-fetch` remains the escape hatch.
- `go install github.com/flexdinesh/gitsy/cmd/gitsy@latest` uses the command package path because the executable lives under `cmd/gitsy`.
- Terminal styling dependencies improve presentation but add dependency maintenance.

## Execution Guidance

If implementation deviates from this plan, update this saved plan before continuing and surface the deviation clearly. In particular, any change to flags, sync behavior, fetch behavior, install method, or removal scope should be treated as a plan change.
