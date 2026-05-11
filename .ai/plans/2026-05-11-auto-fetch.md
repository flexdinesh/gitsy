# Plan: Auto-fetch upstreams before rendering status table

## Summary
Make `gitsy` automatically run `git fetch --all` in every discovered repo before showing the status table. While fetches are in flight, the table renders `⏳ fetching…` rows. Once a fetch completes, the row flips to the real branch/status info. If fetch fails (timeout, no remotes, offline), the row still shows local status but adds a `⚠ stale` hint that the behind-count may be old. Clean repos disappear after their status is known, just like today. A `--no-fetch` flag skips the network call and falls back to the current local-only behavior.

## Key implementation changes

1. **`src/git.ts`** — Add async `fetchAll(repoPath, timeoutMs)` using `node:child_process spawn` wrapped in a Promise with a 30s `setTimeout` that `kill()`s the child process. Return the same `GitResult` shape so callers stay consistent.

2. **`src/args.ts`** — Add `noFetch: boolean` to `CliOptions`. Parse `--no-fetch` flag. Update `USAGE`. Update tests in `test/args.test.ts`.

3. **`src/index.ts`** — Stop pre-building `RepoStatus[]`. After `discoverRepos`, pass the raw `DiscoveredRepo[]` list (plus `noFetch`) into the `App` component. Remove the synchronous `for…of getShortStatus` loop entirely.

4. **`src/ui/App.ts`** — Restructure props:
   - Replace `repos: readonly RepoStatus[]` with `repos: readonly DiscoveredRepo[]`
   - Add `noFetch: boolean`
   - Keep `fullscreen`, `message`, `showAll`, `totalDiscovered`
   
   Add internal React state:
   - `Map<string, {kind: 'fetching' | 'done' | 'failed'; status?: ParsedStatus}>` keyed by `repo.realPath`
   
   `useEffect` flow:
   - Initialise map with every repo in `fetching` state
   - If `noFetch` is true, skip fetch and run status immediately for all repos
   - Otherwise, launch `fetchAll` for every repo in parallel (30s timeout each)
   - On fetch success or failure, run `getShortStatus` + `parseStatus` and update the map entry to `done` or `failed`
   - In static mode, track a completion counter; when every repo has reached `done`/`failed`, call `exit()` after a short `setTimeout` so Ink flushes the final frame
   - In fullscreen mode, never auto-exit

5. **`src/ui/table.ts`** — Introduce a new function `buildVisualRowsFromFetchState` that takes the state map and `showAll` flag. For each repo:
   - `fetching` → single row: `repo.displayName | ⏳ fetching…` (yellow, dim)
   - `done` with clean status + `!showAll` → skip entirely
   - `done` with changes → normal branch summary + file rows
   - `failed` → branch summary from local status + append `⚠ stale` to the summary text, then file rows
   
   Keep `formatBranchSummary` for the local-status formatting; the stale append happens at the row-builder level.

6. **Tests**
   - `test/args.test.ts` — add a test that `--no-fetch` sets `noFetch: true`.
   - `test/integration.test.ts` — add a test that invokes the CLI with `--no-fetch` on a temp repo and asserts the output contains the branch name (verifies the no-fetch path still works end-to-end).

## Decisions made by the user
- Fetch is automatic by default; `--no-fetch` is the offline escape hatch.
- `⏳ fetching…` rows shown while fetches are in flight (Option B).
- Clean repos disappear after status is known (same as today).
- Fetch failures get a `⚠ stale` badge on the status row.
- No title-bar progress indicator.
- Fetch failures are only warned with `--verbose`.
- 30s timeout per repo.
- Parallel fetching is acceptable.

## Tradeoffs and risks
- Coupling data fetching into the `App` component slightly mixes presentation with I/O. A custom hook (`useFetchRepos`) can keep it tidy.
- Static mode now blocks until all fetches complete, which may feel slow on first run with many repos. This is unavoidable if we want accurate counts before printing.
- Dynamic row removal as clean repos finish could shift the viewport in fullscreen mode while the user is scrolling. Acceptable because the user explicitly chose the "disappear like today" behavior.

## Open questions
- None.

## Execution guidance
If any deviation from this plan is required during implementation, surface it to the user immediately and update the plan file before proceeding.
