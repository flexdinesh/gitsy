# Remove --all Flag

## Summary

Remove the clean-repo filtering mode entirely. `gitsy` will show all discovered repos by default, and `--all` will be removed from the CLI.

## Key Implementation Changes

1. `internal/args`
   - Remove `Options.All`.
   - Remove `--all` from `args.Usage`.
   - Remove parser handling for `--all`, so it becomes an unknown argument.
   - Update tests:
     - defaults no longer check `All`
     - supported flags test removes `--all`
     - missing `--dir` test should use another flag, e.g. `--dir --verbose`, instead of `--dir --all`

2. `cmd/gitsy/main.go`
   - Stop passing `options.All` into `tui.Run`.
   - Update `tui.Run` call signature accordingly.

3. `internal/tui`
   - Remove `showAll` from `Model`, `Run`, `NewModel`, and `newModel`.
   - Call `ui.Title`, `ui.BuildRows`, and `ui.EmptyMessage` without filtering state.
   - Update TUI tests that currently pass `showAll`.
   - Replace `TestViewFiltersCompletedCleanReposWhenAllIsFalse` with a test asserting clean completed repos remain visible.

4. `internal/ui`
   - Remove `showAll` parameters from:
     - `EmptyMessage`
     - `Title`
     - `BuildRows`
     - `RowsForRepo`
     - `countVisible`
     - remove `filterLabel`
   - Remove the clean-repo filtering condition.
   - Title should likely become `gitsy • N/N all repos`.
   - Empty message for discovered repos should only happen if there are zero repos.

5. Docs
   - Update `README.md` usage examples to say default `gitsy` shows all repositories.
   - Remove the `gitsy --all` example.
   - Update repo `AGENTS.md` command examples to remove `go run ./cmd/gitsy --all`.

## Tests / Verification

Run:

```bash
go test ./...
go build -o bin/gitsy ./cmd/gitsy
```

Targeted tests while editing:

```bash
go test ./internal/args ./internal/ui ./internal/tui ./cmd/gitsy
```

## Decisions Made

- Default behavior changes from "hide clean repos" to "show all repos."
- `--all` is removed instead of kept as a no-op.
- Existing UI should keep showing clean repos with the existing `✓ clean` styling.

## Tradeoffs And Risks

- Existing scripts or aliases using `gitsy --all` will now fail with `Unknown argument: --all`. That matches "no need for a `--all` cli arg," but it is a compatibility break.
- Removing `showAll` simplifies the UI path and avoids carrying a permanent no-op option.

## Execution Guidance

If implementation deviates from this plan, update this saved plan file to reflect the latest approved plan and surface the deviation before continuing.
