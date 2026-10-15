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

Adaptive slate neutrals support sea glass, amber, and coral. Light/dark token pairs map to Lip Gloss CompleteAdaptiveColor values in `internal/tui/theme.go`: the frontmatter hex values drive truecolor terminals, and each token pins explicit ANSI256 and ANSI16 fallbacks instead of nearest-color degradation, which merged tokens and lost contrast. The terminal background selects the pair; inside tmux or screen, gitsy queries the background (OSC 11) itself before the TUI starts, because termenv skips that query for multiplexer TERM values. tmux answers with the pane or client background; an unanswered query returns immediately through termenv's cursor-position sentinel and falls back to `COLORFGBG`, then dark.

| Token | Light 256 | Light 16 | Dark 256 | Dark 16 |
| --- | --- | --- | --- | --- |
| text-hi | 236 | default | 254 | default |
| text-med | 240 | default | 250 | 7 |
| text-lo | 242 | 8 | 247 | 8 |
| brand | 23 | 6 | 73 | 14 |
| success | 22 | 2 | 108 | 10 |
| warning | 94 | 3 | 179 | 11 |
| danger | 124 | 1 | 210 | 9 |
| info | 25 | 4 | 110 | 12 |
| violet | 96 | 5 | 140 | 13 |
| border-subtle | 250 | 8 | 238 | 8 |
| selection | 255 | none | 236 | none |

ANSI256 tokens never share an index within a background, and every text token measures at least 4.5:1 on its selection wash (lowest: light violet 4.51:1, light text-lo 4.53:1, dark violet 4.57:1). ANSI16 neutrals use the default foreground or ANSI 7/8; semantic tokens use standard color names. Border-subtle stays near 2:1 so rules remain hairlines.

- Primary: brand colors identify the title, selected marker, and loading state.
- Semantic accents: warning marks stale results, short stale or skipped reasons, and behind/stale/skipped summary counts; danger marks failures, failure reasons, and conflicts. Success marks completed sync, synced counts, and staged/added detail; info marks changed branch summaries and changed/ahead counts, so header and row "changed" match; violet marks renamed detail. Untracked detail uses text-med rather than danger.
- Neutrals: text-hi identifies repositories and primary content; text-med identifies context sections; text-lo carries metadata and subdued clean/file rows. Border-subtle draws rules. Selection supplies the current repository's wash.

Unselected clean summaries and ordinary file detail use text-lo even when their semantic tone differs. Selected rows restore semantic tones.

**The Visible Selection Rule.** Selection retains its marker and emphasis when terminal colors are unavailable. ANSI16 selection uses the marker and bold text without a background. Without any styling (`NO_COLOR` or no color support), selected rows also end with a “ ‹” cap, and the rail or detail strip names the selection.

## Typography

The user's terminal controls family, size, line height, and glyph rendering. The application applies normal or bold emphasis only. Bold distinguishes the title, table headings, directory paths, context labels, and selected repository; explicit low-contrast colors replace terminal faint styling. Directory counts use subdued metadata text.

Width measurement uses display cells, not bytes. Truncation and wrapping respect Unicode display width. Prose wraps at word boundaries; paths wrap after `/`. Only a token wider than the line breaks mid-token.

## Layout

All dimensions use terminal cells. Header, ledger, and footer share two-cell horizontal gutters. Columns and panels use two-cell gaps; context has one-cell horizontal insets.

The regular view reserves three header lines (title, summary, tabs), plus a second summary line only when the widest summary the workspace could produce would wrap at the current width, so changing counts never shift the layout. It also reserves two ledger heading/rule lines, one footer line, and, when the context rail is hidden, one detail-strip line. Inline action lines reduce the row viewport, which retains at least one row. Each `--dir` adds an always-expanded group in argument order; the current directory supplies the default group. All groups share columns. Default density is one row per repository appearance. f expands all file rows and adds a blank separator between repositories within each group.

Directory headers remain in the scrolling ledger, with sticky directory context when more than one viewport row is available. Empty groups retain their header and explanation, including trailing groups in short terminals. Overlapping repositories appear in every matching group; inspection results and Git actions are shared per unique repository.

Repositories is the default tab and includes linked worktrees. Worktrees filters those same groups to linked worktrees, including paths outside the scan directories; main checkouts remain in Repositories. Tab switches tabs (Shift+Tab and left/right also work), restoring each tab's selection and scroll position. f toggles file detail.

Context appears at 104 columns: at least 72 for the ledger, two for the gap, and 30 for context. Context takes one quarter of terminal width, clamped to 30–38 columns. Repository names cap at 28 cells; remaining width favors status.

Below 32 columns or nine lines plus the inline action height, a compact view shows tabs, directory context, the selected repository or worktree, and available detail, truncated to the terminal bounds. The footer stays on the last terminal line. Group-only views show directory context and its count or empty explanation. During confirmation or removal, action text takes priority over the ledger. Confirmation requires the entire wrapped target path to fit; otherwise show a word-wrapped enlargement instruction and retain cancellation. Unspecified dimensions default to 80 × 24.

## Elevation & Depth

Flat terminal output uses a tinted selection strip and text hierarchy. No shadows or layered panels. The terminal's existing background remains visible outside selection.

## Shapes

Open rectangular rows, a horizontal heading rule, and one vertical context divider. No enclosing table box, rounded corners, or per-repository outlines.

## Components

- Workspace header: sea-glass title and directory count; right-aligned fetch/local/sync mode and completed/total progress with “in progress” or “done.” Progress counts unique repositories in the active tab, including failed inspections as completed, and excludes deleted worktrees. Narrow headers omit the state text to retain the counter; compact headers place the counter beside the summary and drop whole segments to fit: the title first, then counts from least to most urgent; the total stays. One or two summary lines (see Layout) show unique repository totals and counts, separated by “ · ”, in priority order: failed, behind, ahead, stale, changed, then under `--sync` skipped (sync skipped after a failed fetch) and synced. Narrow headers drop counts from the end. Counts use singular nouns for one (“1 repo,” “1 dir”). Overlapping groups label the total “unique repos,” including in compact mode.
- Tabs: Repositories and Worktrees sit beneath the summary with a two-cell gap and Tab hint. The active label uses brackets, sea-glass emphasis, and bold text; the inactive label uses subdued metadata styling. Worktrees summaries and directory counts use “worktree(s),” with “unique worktree(s)” for overlapping groups.
- Directory header: bold path and subdued completed/total repository count span the shared ledger width, including sticky and compact directory context. Each matching group counts shared inspection results; empty groups show 0/0. Narrow directories omit the noun before sacrificing the counter. Home paths use `~`; long paths retain their trailing portion. Headers are unselectable and groups remain expanded.
- Repository strip: number, repository, branch/status. Linked worktrees append a subdued “ · worktree” tag to the repository name; the name keeps priority and the tag truncates independently, keeping at least four cells. The current repository gains a sea-glass › marker, bold identity, and selection wash across its rows. Narrow status columns switch to actionable compact wording before truncation.
- Identity: display names come from the origin remote's repository name. Without a usable remote, a linked worktree takes its main checkout's directory name and other repositories their own directory name; remote-helper URLs such as `ext::` never supply names. Linked worktrees sort after the main checkout that shares their name.
- Worktree strip: number, Repository, Worktree, and Branch / status, with the same selection and one-line density. Repository uses the same display name as the Repositories tab; Worktree uses the escaped path basename. Each identity column caps at 28 cells; smaller ledgers allocate up to one quarter to repository and one third to worktree, preserving status space. Compact selection shows “repository / worktree,” truncating each identity independently. Locked worktrees carry explicit “locked” status text.
- Status detail: branch, ahead/behind arrows, clean state, and deterministic file-category counts. Parts join with “ · ”. Completed sync appends “synced N commit(s)” without an arrow. Failed status replaces unavailable detail; failed sync and stale fetch remain explicit. Inspection derives a short reason without `--verbose`: “fetch timed out after 30s,” “fetch failed: auth,” “fetch failed: repo not found,” “remote unreachable,” “status failed: <first line>,” or “sync failed: <first line>.” Status failures outrank sync failures, which outrank fetch failures. Loading uses the Bubbles dot spinner and “fetching…” when inspection fetches (default or `--sync`) or “checking status…” under `--no-fetch`, in regular and compact wording alike.
- Context rail: selected name, wrapped `~`-abbreviated path, branch, upstream, status, the short reason, and skipped-sync reason when present, shown once. It follows the selected appearance immediately and shares the ledger's height. Worktrees uses “Selected worktree” and adds the repository display name and owning main checkout path under “Repository,” plus a lock reason when present. Group-only and empty views replace selected metadata with “Nothing selected.”; the ledger keeps the empty explanation.
- Detail strip: below 104 columns in the regular view, one line above the footer shows the selected `~`-abbreviated path, then any short reason in its semantic tone, then the upstream. The upstream drops first, then the path shortens from the left; the reason stays. The line stays reserved but blank while nothing is selected, and confirmation or removal replaces it. Compact views list the short reason under the status line.
- Footer: selected appearance position and width-adaptive key hints. Up/down or j/k navigate appearances across headers; Tab switches views; f toggles files; page and half-page keys scroll; Home/End or g/G select endpoints; q/Q/Ctrl+C quit. Runes coalesced into one key event by fast repeat or paste replay in order; a burst containing q/Q quits, and bursts never trigger x/X or y/Y. Worktrees adds “x delete” while a worktree is selectable; X stays documented in `--help`, the README, and here. Page keys and mouse wheel can inspect group-only rows; selection marker, position, and selected context disappear while no repository or worktree is visible.
- Worktree deletion: x starts asynchronous removal without a prompt after fresh Git protection checks, so clean worktrees delete immediately. X starts force removal without a prompt, repeating the same protection checks. Navigation stays fixed during confirmation and removal. Initial inspection must finish first. Main/current-directory worktrees, locks, and failed validation prevent removal. Fresh changed/untracked/ignored files or submodules found by x trigger a coral warning with the wrapped target path: “Force delete worktree? Discards changed, untracked, ignored, and submodule files. Branch kept.” y/Y starts force removal after repeating all protection checks; Esc or n/N cancels. The full warning and target must fit before confirmation is enabled. A failure retains the row, shows a wrapped notice of at most three lines, and permits retry after external cleanup. Success reports “Deleted <worktree>. Branch kept.” with the worktree basename capped at 28 cells and removes every appearance from both tabs.
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
