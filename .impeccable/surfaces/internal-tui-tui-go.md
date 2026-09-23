---
version: 1
slug: "internal-tui-tui-go"
primary_target: "internal/tui/tui.go"
related_targets: ["internal/tui/theme.go"]
---

# Terminal workspace

Mode: Operate. Priorities: identify changed/behind/failed repositories quickly and fit many repositories. User delegated aesthetics; support light and dark terminals. Preserve Git behavior, ordering, keyboard and mouse navigation. Use terminal-native code; no raster assets.

## Direction contract

THESIS: A dispatch board for workspace exceptions, with dense repository strips and optional file detail.

OWN-WORLD: Slate neutrals, restrained sea-glass selection, amber warnings, coral failures. Open gutters, a thin heading rule, one tinted current row. Terminal font and background remain user-owned.

STORY: Scan the workspace summary, move through repositories, inspect selected metadata, toggle file rows when needed. Clean states recede; exceptions stay explicit.

FIRST VIEWPORT: A two-line summary above a full-height ledger. One row per repository by default. Wide terminals add selected-repository context behind a single vertical rule; narrow terminals devote width to the ledger. Footer carries position, navigation, tab to toggle file details, and quit. Signature interaction: the tinted current strip drives the context rail immediately.

FORM: Flight-progress dispatch strips, candidate 3; seed 4e574b15. Other grounded candidates: editor gutter, departure board, laboratory log, mixing desk, map index, build trace. Catalog challengers declined for weaker developer identification and status clarity; retain consistent fields, alignment, structural compression, whitespace, color-independent selection, and a tonal hierarchy. User priorities tighten strips to one line. No unresolved questions.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance
