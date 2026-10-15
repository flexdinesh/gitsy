# gitsy

gitsy is a tiny CLI that scans a directory for git repositories and linked worktrees, automatically fetches from upstream, and optionally fast-forwards local branches with `--sync`. It displays status in a nice TUI so you can see what needs attention.

![gitsy demo](assets/gitsy-demo.gif)

## Install

```bash
brew install flexdinesh/tap/gitsy
```

```bash
go install github.com/flexdinesh/gitsy/cmd/gitsy@latest
```

## Usage

```bash
# Show all discovered repositories under the current directory.
gitsy

# Fast-forward repositories that can safely update without conflicts.
gitsy --sync

# Print a status report and exit, leaving it in terminal scrollback.
gitsy --non-interactive

# Fast-forward safely, print the results, and exit.
gitsy --non-interactive --sync

# Scan repository directories up to a specific nested depth.
gitsy --max-depth 5

# Start scanning from a specific directory instead of the current directory.
gitsy --dir ~/workspace

# Scan multiple directories and include their repositories' linked worktrees.
gitsy --dir ~/workspace/org-one --dir ~/workspace/org-two

# Skip fetching upstream changes and use local status only.
gitsy --no-fetch

# Print warnings for skipped repos and failed git commands.
gitsy --verbose

# Show help.
gitsy --help

# Show the installed version.
gitsy --version
```

Use `Tab` to switch **Repositories** / **Worktrees**, `↑/↓` or `j/k` to
select rows, `f` to toggle file details,
`PgUp/PgDn` or the mouse wheel to scroll, and `q` to quit. Wide terminals
show context for the selected repository. Colors adapt to light and dark themes.

By default, each invocation runs `git fetch --all` for fresh upstream state;
nothing is cached between invocations. Within a run, main checkouts and linked
worktrees sharing a Git common directory share one fetch when safe. Separate
clones fetch independently, even with the same remote URL. Repositories with
worktree-specific configuration, conditional includes, submodules, relative
remote URLs, URL rewrites, custom fetch hooks, or custom transport/credential
commands fetch per checkout to preserve their existing behavior. Checkout-relative TLS, cookie,
validation, and bundle paths also disable sharing. Status and sync remain
independent for each checkout.

`--sync` always fetches, even with `--no-fetch`. Only clean branches with an
upstream, commits behind, and no commits ahead can fast-forward. If any remote
fails to fetch, sync is skipped for that checkout, including failures from
unrelated remotes. Available local status is shown as stale with
`sync skipped: fetch failed`.

Use `--non-interactive` for a status report: a brief progress line followed by
grouped checkout paths, branch status, change counts, and sync outcomes. It
includes clean repositories and linked worktrees, prints no filenames or terminal
control sequences, and exits automatically. Existing flags still apply, including
`--no-fetch` and `--verbose`. Dirty, behind, or diverged repositories are normal
results and exit successfully; failed fetch, status, or sync commands return exit
code 1 after printing available results. Repositories skipped by the safe sync
rules are normal results. An empty workspace exits successfully.

With `--dir`, only the supplied directories are scanned. `--max-depth` applies
to each directory; linked worktrees are included wherever they live. Relative
paths resolve from the current directory.

Repositories are grouped by scan directory, in the order `--dir` is supplied.
Without `--dir`, the current directory is the group. Groups stay expanded;
navigation skips directory headers. Repositories found through overlapping
directories appear in each group, while status and sync run once per unique
checkout. Fetches are shared across eligible linked worktrees as described above.
Linked worktrees appear in each group that discovers them.

The **Worktrees** tab lists linked worktrees from the same scan, including those
outside the supplied directories. Main checkouts remain in **Repositories**.
Press `x` on a worktree to delete it. Deletion keeps the branch and
removes the row from both tabs. Inspection must finish first; main, locked, and
current-directory worktrees remain protected. Changed, untracked, or ignored
files trigger a warning; review the path, then press `y` to force deletion and
discard those files, or `Esc`/`n` to cancel. Press `X` to force delete and
discard those files without confirmation. Every attempt checks fresh Git state.
Left/right and Shift+Tab also switch views.

Repository labels use the origin remote's repository name, falling back to the
directory name when unavailable. Full paths appear in the sidebar.

## Development

Project commands run through mise:

```bash
mise trust
mise run setup
mise run check
mise run build
```

See [docs/development.md](docs/development.md).

## Releases

Stable releases are published manually from `main`; development versions install
directly from `main`. See [docs/release.md](docs/release.md).
