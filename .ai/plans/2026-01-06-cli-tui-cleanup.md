# CLI/TUI Cleanup 1-5

## Summary

Keep the current architecture, but tighten the execution model around bounded work, clean terminal ownership, cancellation, and simpler row rendering. Non-TTY output is intentionally out of scope.

## Key Implementation Changes

1. Bounded repo inspection concurrency
   - Replace one Bubble Tea command per repo with a bounded scheduler.
   - Use a fixed internal concurrency limit, likely `8`.
   - Keep results streaming back to the TUI as each repo completes.
   - Do not add a new CLI flag.

2. Collect warnings and print after exit
   - Add a warning collector owned by `run`.
   - Pass collector callbacks into discovery and inspection.
   - Do not write warnings directly to `stderr` while Bubble Tea is active.
   - After `tui.Run(...)` returns, print collected warnings to `stderr` if `--verbose` is set.
   - Preserve existing warning message text where practical.

3. Context cancellation through Git operations
   - Create a `context.Context` in `run`.
   - Pass it through `tui.Run`, model inspection, `inspect.Repo`, and `git` command helpers.
   - On `q` or Ctrl-C, cancel the context and quit the Bubble Tea program.
   - Update `git.Run`, `FetchAll`, `ShortStatus`, `TopLevel`, `WorktreeList`, and `FastForward` to use context-aware command execution.
   - Treat cancelled commands as failed/stale without panics or terminal corruption.

4. Make `ui.BuildRows` the single row API
   - Move row composition/filtering responsibility into `ui.BuildRows`.
   - Update TUI table rendering to call `BuildRows(...)` once and translate `ui.Row` to `table.Row`.
   - Remove or avoid direct TUI loops over `RowsForRepo`.
   - Decide whether separator rows should remain visible in the table; likely drop separators unless they render cleanly in `bubbles/table`.

5. Minimal `run` exit refactor
   - Keep `main` as the only place that calls `os.Exit`.
   - Change parse failure in `run` to return an error after writing usage, or return a small custom error that includes usage.
   - Keep behavior equivalent for users: parse errors still print `gitsy: ...`, usage, and exit `1`.
   - Do not introduce a full exit-code abstraction.

## Tests / Verification

- Update `internal/tui` tests for bounded scheduling and cancellation behavior.
- Update `internal/git` or `inspect` tests around context cancellation where feasible.
- Update `internal/ui`/`tui` tests so `BuildRows` is the canonical rendering path.
- Add or adjust `cmd/gitsy` tests if practical for parse failure no longer calling `os.Exit`.
- Run `go test ./...`.

## Decisions Made

- Concurrency: fixed internal limit, no flag.
- Warning handling: collect warnings and print after TUI exits.
- Cancellation: pressing quit should cancel running Git commands.
- Row API: `ui.BuildRows` becomes the single row-building API.
- Exit handling: minimal change, no explicit exit-code result type.

## Tradeoffs / Risks

- Fixed concurrency is simpler, but users with very large or very slow workspaces cannot tune it without a code change.
- Post-exit warnings avoid terminal corruption, but users will not see verbose failures live during the TUI run.
- Context cancellation adds plumbing across packages, but it is the right foundation for clean Bubble Tea shutdown.
- Making `BuildRows` canonical may require small test rewrites where tests currently inspect `RowsForRepo` behavior directly.

## Execution Guidance

If implementation deviates from this plan, update this saved plan to match the newly approved direction and surface the deviation before continuing.
