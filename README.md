# gitsy

A small Go CLI that shows Git status across child repositories and linked worktrees.

It scans the current directory for repos, fetches upstream metadata by default, hides clean repos unless requested, and renders a compact terminal table.

## Install

Install from GitHub:

```bash
go install github.com/flexdinesh/gitsy/cmd/gitsy@latest
```

Or install from a local checkout:

```bash
git clone git@github.com:flexdinesh/gitsy.git
cd gitsy
go install ./cmd/gitsy
```

`go install` puts the `gitsy` binary in your Go bin directory, usually `~/go/bin`. Make sure that directory is on your `PATH` so `gitsy` is available from anywhere:

```bash
export PATH="$(go env GOPATH)/bin:$PATH"
```

Then run `gitsy` from any directory you want to scan:

```bash
gitsy
```

For a local build without installing globally:

```bash
go build -o bin/gitsy ./cmd/gitsy
./bin/gitsy
```

## Usage

```bash
gitsy [options]
```

Options:

```text
--all              Show all discovered repositories, including clean repos
--max-depth <n>    Scan repository directories up to n nested levels; default 3
--dir <path>       Start scanning from <path> instead of the current directory
--verbose          Print warnings for skipped repos and failed git commands
--no-fetch         Skip fetching upstream changes; use local status only
--sync             Fast-forward repos that can safely update without conflicts
--help             Show help
--version          Show version
```

Examples:

```bash
gitsy
gitsy --all
gitsy --max-depth 5
gitsy --dir ~/workspace --no-fetch
gitsy --sync
```

## Behavior

- Repositories are discovered by scanning child directories for `.git` directories or worktree `.git` files.
- Linked worktrees are included via `git worktree list --porcelain`.
- Clean repositories are hidden by default; use `--all` to show them.
- `git fetch --all` runs by default before status is rendered so ahead/behind counts are fresh.
- `--no-fetch` avoids network calls.
- `--sync` fetches first, then runs `git merge --ff-only` only when a repo is clean, strictly behind, and has a valid upstream.
