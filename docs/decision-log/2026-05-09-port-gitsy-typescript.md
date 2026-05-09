---
title: Port gitsy shell script to native TypeScript Ink CLI
description: Durable implementation decisions for the gitsy TypeScript CLI port
date: 2026-05-09
slug: port-gitsy-typescript
status: implemented
tags:
  - typescript
  - cli
  - ink
  - git
related_paths:
  - package.json
  - src/index.ts
  - src/discover.ts
  - src/status.ts
  - src/ui/App.ts
---

## Why

`gitsy` was ported from a Bash script into a maintainable TypeScript CLI with richer terminal output, linked worktree support, and testable repo/status parsing.

## What

- Use pnpm and Node 24 native TypeScript execution.
- Keep the CLI entrypoint as `src/index.ts`; expose it through both `bin.gitsy` and package `main`.
- Use Ink for all terminal output.
- Render pretty parsed Git status instead of raw `git status --short --branch` output.
- Hide clean repositories by default; show them with `--all`.
- Support `--max-depth`, defaulting to `3`, where depth means repository directory nesting depth below cwd.
- Support `--fullscreen` for an alternate-screen, scrollable UI.
- Support `--verbose` for warnings that are otherwise silent.
- Support `--help` and `--version`; version is read from `package.json`.
- Discover `.git` directories and `.git` files, and also include linked worktrees from `git worktree list --porcelain`.

## How

- Run directly from anywhere with `node /Users/dineshpandiyan/workspace/gitsy` or `node /Users/dineshpandiyan/workspace/gitsy/src/index.ts`.
- For a global local command, run `pnpm link --global` from the repo and then use `gitsy` from any directory.
- Avoid JSX/TSX so native TypeScript can run without a transpile/build step.
- Hardcode common generated/cache directories to skip while scanning.
- Deduplicate repos/worktrees by realpath.
- Display paths inside cwd as relative paths and linked worktrees outside cwd as absolute paths.

## Tradeoffs and gotchas

- Ink plus native TypeScript means UI code uses `React.createElement` instead of JSX.
- Linked worktrees may appear outside the scanned directory by design.
- Unicode icons make output prettier but may render differently across terminals.
- Fullscreen mode requires an interactive TTY and exits with an error otherwise.
