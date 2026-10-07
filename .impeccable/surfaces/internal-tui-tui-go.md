---
version: 1
slug: "internal-tui-tui-go"
primary_target: "internal/tui/tui.go"
related_targets: ["internal/tui/theme.go", "internal/tui/worktrees.go"]
---

# Terminal workspace

Mode: Operate. Priorities: identify changed/behind/failed repositories quickly and fit many repositories. User delegated aesthetics; support light and dark terminals. Preserve Git behavior, ordering, keyboard and mouse navigation. Use terminal-native code; no raster assets.

PROGRESS: Header right shows active-tab unique completed/total inspections with operation mode and in-progress/done text. Failed inspections are completed; deleted worktrees are excluded. Directory right shows completed/total repos or worktrees, shared across overlapping groups and retained in sticky/compact context. Empty directories show 0/0. Narrow layouts preserve the ratio before labels; palette and density unchanged.

## Direction contract

THESIS: A dispatch board grouped by scan directory, with dense repository strips and optional file detail.

OWN-WORLD: Slate neutrals, restrained sea-glass selection, amber warnings, coral failures. Open gutters, a thin heading rule, one tinted current row. Terminal font and background remain user-owned.

STORY: Scan unique-repository totals, identify each directory's repositories, move across groups, inspect selected metadata, toggle file rows. Overlapping repositories appear in every matching group while Git operations run once per unique repository.

WORKTREES: Repositories opens by default and includes linked worktrees. Tab selects Worktrees (Shift+Tab and left/right also work), filtering each scan group to linked worktrees, including external paths; main checkouts remain in Repositories. Each tab retains its selection and scroll offset. f toggles file detail. Worktree basename identifies the row, compact selection, and context; the context adds the owning main checkout path and lock reason. Worktrees uses its own counts, “Worktree” heading, “Selected worktree” context, and “No linked worktrees found.” empty explanation. Overlapping appearances share results and removal.

DELETE: x/X on a selected worktree shows an amber inline confirmation, wrapped target path, and “Branch kept.” y/Y starts asynchronous removal; Esc/n cancels. Initial workspace inspections must finish first; navigation stays fixed during confirmation/removal. Fresh Git validation protects main/current-directory worktrees, locks, and failed checks. Fresh changed/untracked/ignored files trigger a second coral confirmation warning that force deletion discards them; another y/Y forces removal after rechecking protections. Cached failures or dirty/locked state must not veto a retry after external cleanup. Esc/n cancels either confirmation; branch retained. Success removes every appearance from both tabs and shows a notice. Failure retains the row and wraps its notice to at most three lines. If the full confirmation warning and path cannot fit, show the enlargement instruction and disable y/Y until resize.

FIRST VIEWPORT: A two-line summary above one scrolling ledger with shared columns. Each --dir produces an always-expanded section in argument order; without --dir, current directory is the group. Bold path and subdued count introduce each section. Empty directories retain a header and explanation. One row per repository appearance by default. Headers are unselectable; keyboard movement crosses them. Scrolling pins directory context when space allows; compact mode identifies the selected directory. Wide terminals retain the context rail. Footer counts appearances. Signature interaction: one selected appearance drives context while shared inspection results update every appearance.

TAB LAYOUT: Four header lines include the bracketed active tab and Tab hint; two ledger heading/rule lines and one footer line follow. Action lines reduce the viewport. Regular layout needs 32 columns and nine lines plus action height; smaller terminals use compact mode. Compact actions take priority over repository detail. Preserve adaptive slate/sea-glass colors, selection wash, gutters, terminal font, and background.

SCROLLING: Page keys and mouse wheel reach group-only rows, including trailing empty groups at eight lines. With no repository visible, suppress selection marker, footer position, and repository metadata; retain directory context and count or explanation. Compact summaries label overlapping totals “unique repos.”

FORM: Extension of flight-progress dispatch strips, candidate 3; seed 4e574b15. Preserve slate/sea-glass identity, shared alignment and one-line density; add directory sections without enclosing boxes or collapse controls. External linked worktrees inherit every discovering scan group. No unresolved questions.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

VERIFIED: Prior finish review resolved trailing-empty-group reachability, compact unique-count labels, and resize selection recovery. Worktrees evidence: `internal/tui/worktrees_test.go` covers filtering, per-tab selection, exact-target confirmation, fresh-check retry, shared removal, context identity, resize confirmation safety, and terminal bounds across loading/dirty/failed/stale/sync/locked/empty/action states. `.impeccable/review/*worktrees*.png` captures light/dark wide, narrow, compact, loading, empty, confirmation, removal, and error views. DESIGN.md records the feature merge; tokens unchanged. Worktrees verdict pass: ship for scored fixes—fresh-check retry recovery, matching worktree context identity, and feature documentation. No raster assets ship.
