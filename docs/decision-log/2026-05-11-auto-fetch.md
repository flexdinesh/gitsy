---
title: Auto-fetch upstreams before rendering status
description: Run git fetch --all in every discovered repo before printing the table, with live fetching rows and --no-fetch opt-out.
date: 2026-05-11
slug: auto-fetch-upstream
status: implemented
tags:
  - cli
  - git
  - networking
related_paths:
  - src/git.ts
  - src/args.ts
  - src/ui/App.ts
  - src/ui/table.ts
---

## Why
Users wanted accurate `behind` counts in the status table, but stale metadata only reflects the last fetch. Running fetch before status guarantees fresh data without touching local branches.

## What
- `gitsy` now automatically runs `git fetch --all` per repo before showing status.
- While fetching, each repo shows `⏳ fetching…` in the table.
- Fetch failures show `⚠ stale` on the branch summary.
- `--no-fetch` skips all network calls entirely.
- `--dir <path>` lets the scan start from any directory instead of `cwd`.

## How
- `fetchAll` in `git.ts` spawns `git fetch --all` with a 30s Promise-based timeout (SIGKILL).
- `App.ts` manages async state: initialises every repo as `fetching`, resolves to `done` or `failed` as fetches complete, then runs `getShortStatus`.
- `table.ts` renders rows from the fetch-state map: `fetching` rows stay visible until resolved; clean `done` repos hide when `!showAll`.
- `--verbose` surfaces fetch failure warnings via the optional `warn` prop; failures are silent otherwise.

## Risks / Tradeoffs
- Static mode blocks until all fetches resolve, which can feel slow with many repos.
- Fullscreen rows may shift as clean repos disappear mid-scroll. Acceptable per user choice.
- Fetch failures (offline, no remotes, timeout) silently degrade to stale local data. Only `--verbose` reveals the problem.
