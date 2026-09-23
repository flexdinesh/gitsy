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

- One repository per row by default.
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

The user's terminal controls family, size, line height, and glyph rendering. The application applies normal or bold emphasis only. Bold distinguishes the title, table headings, context labels, and selected repository; explicit low-contrast colors replace terminal faint styling.

Width measurement uses display cells, not bytes. Truncation and path wrapping respect Unicode display width.

## Layout

All dimensions use terminal cells. Header, ledger, and footer share two-cell horizontal gutters. Columns and panels use two-cell gaps; context has one-cell horizontal insets.

The regular view reserves three header lines, two ledger heading/rule lines, and one footer line. Remaining height is the row viewport, at least one row. Default density is one row per repository. Tab expands all file rows and adds a blank separator between repositories.

Context appears at 104 columns: at least 72 for the ledger, two for the gap, and 30 for context. Context takes one quarter of terminal width, clamped to 30–38 columns. Repository names cap at 28 cells; remaining width favors status.

Below 32 columns or seven lines, a compact view shows the selected repository and available detail, truncated to the terminal bounds. Unspecified dimensions default to 80 × 24.

## Elevation & Depth

Flat terminal output uses a tinted selection strip and text hierarchy. No shadows or layered panels. The terminal's existing background remains visible outside selection.

## Shapes

Open rectangular rows, a horizontal heading rule, and one vertical context divider. No enclosing table box, rounded corners, or per-repository outlines.

## Components

- Workspace header: sea-glass title; right-aligned fetch/local/sync mode and pending/done text. Up to two summary lines show repository count and exceptions. Failures precede other exception counts, preserving their priority at limited widths.
- Repository strip: number, repository, branch/status. The current repository gains a sea-glass › marker, bold identity, and selection wash across its rows. Narrow status columns switch to actionable compact wording before truncation.
- Status detail: branch, ahead/behind arrows, clean state, and deterministic file-category counts. Failed status replaces unavailable detail; failed sync and stale fetch remain explicit. Loading uses the Bubbles dot spinner and status text.
- Context rail: selected name, wrapped path, branch, upstream, status, and skipped-sync reason when present. It follows selection immediately and shares the ledger's height.
- Footer: selected position and width-adaptive key hints. Arrow keys or j/k navigate repositories; Tab toggles files; page and half-page keys scroll; Home/End or g/G select endpoints; q/Q/Ctrl+C quit. Mouse wheel scrolling remains available.
- Empty state: “No child git repositories found.” appears in the ledger without fabricated rows.

## Do's and Don'ts

- Do preserve terminal-cell alignment and deterministic repository order.
- Do keep selection visible through the › marker and bold text.
- Do expose failure, stale, conflict, and ahead/behind state in text.
- Do use Tab to reveal file detail without changing the default density.

- Don't force a terminal font, font size, or full-screen background.
- Don't use color alone to identify selection or status.
- Don't add borders around every repository or consume rows with decoration.

Source of truth: `internal/tui/theme.go`, `internal/tui/tui.go`, and `internal/ui/ui.go`. Sidecar previews translate native patterns into HTML for inspection only; they are not shipping UI.
