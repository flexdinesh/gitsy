---
version: 1
slug: "internal-tui-tui-go"
primary_target: "internal/tui/tui.go"
related_targets: ["internal/tui/theme.go"]
---

# Terminal workspace

Mode: Operate. Priorities: identify changed/behind/failed repositories quickly and fit many repositories. User delegated aesthetics; support light and dark terminals. Preserve Git behavior, ordering, keyboard and mouse navigation. Use terminal-native code; no raster assets.

## Direction contract

THESIS: A dispatch board grouped by scan directory, with dense repository strips and optional file detail.

OWN-WORLD: Slate neutrals, restrained sea-glass selection, amber warnings, coral failures. Open gutters, a thin heading rule, one tinted current row. Terminal font and background remain user-owned.

STORY: Scan unique-repository totals, identify each directory's repositories, move across groups, inspect selected metadata, toggle file rows. Overlapping repositories appear in every matching group while Git operations run once per unique repository.

FIRST VIEWPORT: A two-line summary above one scrolling ledger with shared columns. Each --dir produces an always-expanded section in argument order; without --dir, current directory is the group. Bold path and subdued count introduce each section. Empty directories retain a header and explanation. One row per repository appearance by default. Headers are unselectable; keyboard movement crosses them. Scrolling pins directory context when space allows; compact mode identifies the selected directory. Wide terminals retain the context rail. Footer counts appearances. Signature interaction: one selected appearance drives context while shared inspection results update every appearance.

SCROLLING: Page keys and mouse wheel reach group-only rows, including trailing empty groups at eight lines. With no repository visible, suppress selection marker, footer position, and repository metadata; retain directory context and count or explanation. Compact summaries label overlapping totals “unique repos.”

FORM: Extension of flight-progress dispatch strips, candidate 3; seed 4e574b15. Preserve slate/sea-glass identity, shared alignment and one-line density; add directory sections without enclosing boxes or collapse controls. External linked worktrees inherit every discovering scan group. No unresolved questions.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

VERIFIED: Follow-up finish review scored trailing-empty-group reachability, compact unique-count labels, and resize selection recovery resolved; disposition ship at that scope. DESIGN.md records the extension; tokens unchanged. Full tests and local checks pass. No raster assets ship.
