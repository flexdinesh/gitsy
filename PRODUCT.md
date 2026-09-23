# Product

<!-- impeccable:product-schema 1 -->

## Platform

Terminal CLI with an interactive TUI.

## Users

Developers managing many Git repositories and linked worktrees from one workspace.

## Product Purpose

gitsy scans a workspace, fetches upstream metadata, and presents repository state in one compact terminal view. Success means users can quickly identify changed, behind, stale, or failed repositories and safely fast-forward eligible repositories without inspecting each one manually.

## Positioning

One command combines recursive repository and linked-worktree discovery, fresh upstream status, parsed working-tree detail, and guarded fast-forward sync in a compact interactive TUI.

## Operating Context

gitsy runs in a developer's terminal against the current directory or a chosen workspace. It discovers child repositories, includes linked worktrees, fetches remotes by default, and supports keyboard and mouse navigation. Users can skip network access, inspect warnings, control scan depth, or request safe synchronization.

## Capabilities and Constraints

- Go CLI built with Bubble Tea, Bubbles, and Lip Gloss.
- Discovers repositories and linked worktrees deterministically, deduplicated by real path.
- Fetches before status by default; `--no-fetch` permits local-only inspection.
- `--sync` only fast-forwards repositories that are eligible for conflict-free update.
- Keeps Git execution, discovery, status parsing, display shaping, and TUI state in focused packages.
- Keeps terminal output compact and stable; warnings remain opt-in through `--verbose`.
- Supports Unicode-aware width handling and adaptive light/dark terminal colors.
- Avoids unnecessary dependencies and preserves deterministic ordering.

## Brand Commitments

The product name is `gitsy`. Voice is concise, direct, technical, and useful. Preserve the product's tiny-CLI character and compact status language.

## Evidence on Hand

- Product overview, installation, and usage: `README.md`.
- No testimonials, customer logos, benchmarks, or adoption claims are established; future work must not fabricate them.

## Product Principles

- Reveal workspace-wide Git state at a glance.
- Prefer fresh, accurate upstream information while supporting offline inspection.
- Synchronize only when the operation is safe and predictable.
- Keep interaction fast, compact, and terminal-native.
- Preserve deterministic, testable behavior.

## Accessibility & Inclusion

Maintain keyboard navigation, mouse support, light/dark terminal compatibility, Unicode-aware alignment, and clear text labels alongside color and symbols.
