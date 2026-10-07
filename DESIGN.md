---
name: gitsy
description: Dense terminal dispatch board for workspace Git state.
colors:
  border-subtle-light: "#C4CDD6"
  border-subtle-dark: "#354555"
  text-hi-light: "#243446"
  text-hi-dark: "#D8E2ED"
  text-med-light: "#465A6E"
  text-med-dark: "#ADBDCD"
  text-lo-light: "#566B7F"
  text-lo-dark: "#8CA2B8"
  brand-light: "#176C61"
  brand-dark: "#63C5B5"
  success-light: "#306B49"
  success-dark: "#9AC79D"
  warning-light: "#895B10"
  warning-dark: "#E5B567"
  danger-light: "#AC3439"
  danger-dark: "#EE9298"
  info-light: "#325F91"
  info-dark: "#8BB4DF"
  violet-light: "#785297"
  violet-dark: "#BDA6DB"
  selection-light: "#E2EFEB"
  selection-dark: "#203B3B"
typography:
  emphasis:
    fontWeight: 700
  body:
    fontWeight: 400
spacing:
  inset: "1 cell"
  gutter: "2 cells"
components:
  selected-row-light:
    backgroundColor: "{colors.selection-light}"
    textColor: "{colors.text-hi-light}"
    typography: "{typography.emphasis}"
  selected-row-dark:
    backgroundColor: "{colors.selection-dark}"
    textColor: "{colors.text-hi-dark}"
    typography: "{typography.emphasis}"
  selection-marker-light:
    textColor: "{colors.brand-light}"
    typography: "{typography.emphasis}"
  selection-marker-dark:
    textColor: "{colors.brand-dark}"
    typography: "{typography.emphasis}"
---

# Design System: gitsy

## Overview

**Creative North Star: "The Workspace Dispatch Board"**

A terminal dispatch board: aligned repository strips make workspace exceptions easy to scan while preserving density. Open gutters and restrained sea-glass selection organize the view; the terminal owns its font and background.

**Key Characteristics:**

- Always-expanded directory groups; one repository appearance per row by default.
- Explicit status text alongside semantic color.
- Adaptive light and dark colors; no app-wide background fill.

## Colors

Adaptive slate neutrals support sea glass, amber, and coral. Light/dark token pairs map directly to Lip Gloss AdaptiveColor values in `internal/tui/theme.go`; the terminal determines the active pair.

- Primary: brand colors identify the title, selected marker, and loading state.
- Semantic accents: warning marks stale results and summary counts for changed/behind repositories; danger marks failures and conflicts. Success marks completed sync and staged/added detail; info marks changed branch summaries; violet marks renamed detail.
- Neutrals: text-hi identifies repositories and primary content; text-med identifies context sections; text-lo carries metadata and subdued clean/file rows. Border-subtle draws rules. Selection supplies the current repository's wash.

Unselected clean summaries and ordinary file detail use text-lo even when their semantic tone differs. Selected rows restore semantic tones.

**The Visible Selection Rule.** Selection retains its marker and emphasis when terminal colors are unavailable.

## Typography

The user's terminal controls family, size, line height, and glyph rendering. The application applies normal or bold emphasis only. Bold distinguishes the title, table headings, directory paths, context labels, and selected repository; explicit low-contrast colors replace terminal faint styling. Directory counts use subdued metadata text.

Width measurement uses display cells, not bytes. Truncation and path wrapping respect Unicode display width.

## Layout

All dimensions use terminal cells. Header, ledger, and footer share two-cell horizontal gutters. Columns and panels use two-cell gaps; context has one-cell horizontal insets.

The regular view reserves four header lines, including the tabs, two ledger heading/rule lines, and one footer line. Inline action lines reduce the row viewport, which retains at least one row. Each `--dir` adds an always-expanded group in argument order; the current directory supplies the default group. All groups share columns. Default density is one row per repository appearance. f expands all file rows and adds a blank separator between repositories within each group.

Directory headers remain in the scrolling ledger, with sticky directory context when more than one viewport row is available. Empty groups retain their header and explanation, including trailing groups in short terminals. Overlapping repositories appear in every matching group; inspection results and Git actions are shared per unique repository.

Repositories is the default tab and includes linked worktrees. Worktrees filters those same groups to linked worktrees, including paths outside the scan directories; main checkouts remain in Repositories. Tab switches tabs (Shift+Tab and left/right also work), restoring each tab's selection and scroll position. f toggles file detail.

Context appears at 104 columns: at least 72 for the ledger, two for the gap, and 30 for context. Context takes one quarter of terminal width, clamped to 30–38 columns. Repository names cap at 28 cells; remaining width favors status.

Below 32 columns or nine lines plus the inline action height, a compact view shows tabs, directory context, the selected repository or worktree, and available detail, truncated to the terminal bounds. Group-only views show directory context and its count or empty explanation. During confirmation or removal, action text takes priority over the ledger. Confirmation requires the entire wrapped target path to fit; otherwise show an enlargement instruction and retain cancellation. Unspecified dimensions default to 80 × 24.

## Elevation & Depth

Flat terminal output uses a tinted selection strip and text hierarchy. No shadows or layered panels. The terminal's existing background remains visible outside selection.

## Shapes

Open rectangular rows, a horizontal heading rule, and one vertical context divider. No enclosing table box, rounded corners, or per-repository outlines.

## Components

- Workspace header: sea-glass title and directory count; right-aligned fetch/local/sync mode and completed/total progress with “in progress” or “done.” Progress counts unique repositories in the active tab, including failed inspections as completed, and excludes deleted worktrees. Narrow headers omit the state text to retain the counter; compact headers place the counter beside the summary. Up to two summary lines show unique repository totals and exceptions. Overlapping groups label the total “unique repos,” including in compact mode. Failures precede other exception counts, preserving their priority at limited widths.
- Tabs: Repositories and Worktrees sit beneath the summary with a two-cell gap and Tab hint. The active label uses brackets, sea-glass emphasis, and bold text; the inactive label uses subdued metadata styling. Worktrees summaries and directory counts use “worktrees,” with “unique worktrees” for overlapping groups.
- Directory header: bold path and subdued completed/total repository count span the shared ledger width, including sticky and compact directory context. Each matching group counts shared inspection results; empty groups show 0/0. Narrow directories omit the noun before sacrificing the counter. Home paths use `~`; long paths retain their trailing portion. Headers are unselectable and groups remain expanded.
- Repository strip: number, repository, branch/status. The current repository gains a sea-glass › marker, bold identity, and selection wash across its rows. Narrow status columns switch to actionable compact wording before truncation.
- Worktree strip: the same selection and density under a “Worktree” column heading. Identity uses the path basename in the ledger, compact view, and context. Locked worktrees carry explicit “locked” status text.
- Status detail: branch, ahead/behind arrows, clean state, and deterministic file-category counts. Failed status replaces unavailable detail; failed sync and stale fetch remain explicit. Loading uses the Bubbles dot spinner and status text.
- Context rail: selected name, wrapped path, branch, upstream, status, and skipped-sync reason when present. It follows the selected appearance immediately and shares the ledger's height. Worktrees uses “Selected worktree” and adds the owning main checkout path under “Repository,” plus a lock reason when present. Group-only views replace selected metadata with the active tab's empty explanation.
- Footer: selected appearance position and width-adaptive key hints. Up/down or j/k navigate appearances across headers; Tab switches views; f toggles files; page and half-page keys scroll; Home/End or g/G select endpoints; q/Q/Ctrl+C quit. Worktrees adds “x delete.” Page keys and mouse wheel can inspect group-only rows; selection marker, position, and selected context disappear while no repository or worktree is visible.
- Worktree deletion: x/X opens an amber inline “Delete worktree? Branch kept.” prompt with the wrapped target path. y/Y starts asynchronous removal after fresh Git protection checks; Esc or n cancels. Navigation stays fixed during confirmation and removal. Initial inspection must finish first. Main/current-directory worktrees, locks, and failed validation prevent removal. Fresh changed/untracked/ignored files trigger a second coral warning: “Force delete worktree? Discards changed, untracked, and ignored files. Branch kept.” Another y/Y starts force removal after repeating all protection checks; Esc/n cancels. The full warning and target must fit before confirmation is enabled. A failure retains the row, shows a wrapped notice of at most three lines, and permits retry after external cleanup. Success reports “Deleted worktree. Branch kept.” and removes every appearance from both tabs.
- Empty state: each empty group retains its header and “No child git repositories found.” explanation in Repositories or “No linked worktrees found.” in Worktrees, without fabricated rows.

## Do's and Don'ts

- Do preserve terminal-cell alignment and deterministic repository order.
- Do keep selection visible through the › marker and bold text.
- Do expose failure, stale, conflict, and ahead/behind state in text.
- Do use f to reveal file detail without changing the default density.

- Don't force a terminal font, font size, or full-screen background.
- Don't use color alone to identify selection or status.
- Don't add borders around every repository or consume rows with decoration.

Source of truth: `internal/tui/theme.go`, `internal/tui/tui.go`, `internal/tui/worktrees.go`, and `internal/ui/ui.go`. Sidecar previews translate native patterns into HTML for inspection only; they are not shipping UI.
