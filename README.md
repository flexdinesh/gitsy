# gitsy

gitsy is a small Go CLI that scans a directory for Git repositories and linked worktrees, fetches fresh upstream metadata by default, and shows a compact status table so you can see what needs attention.

## Install

Make sure you have [go](https://go.dev) installed in your machine. If you're on MacOS, [brew](https://formulae.brew.sh/formula/go) is the easiest way.

`@latest` resolves to the newest stable SemVer tag, such as `v0.1.0`. There is no moving `latest` Git tag.

```bash
# Install the latest stable release.
go install github.com/flexdinesh/gitsy/cmd/gitsy@latest

# Install a specific stable release.
go install github.com/flexdinesh/gitsy/cmd/gitsy@v0.1.0
```

## Usage

```bash
# Show all discovered repositories under the current directory.
gitsy

# Fast-forward repositories that can safely update without conflicts.
gitsy --sync

# Scan repository directories up to a specific nested depth.
gitsy --max-depth 5

# Start scanning from a specific directory instead of the current directory.
gitsy --dir ~/workspace

# Skip fetching upstream changes and use local status only.
gitsy --no-fetch

# Print warnings for skipped repos and failed git commands.
gitsy --verbose

# Show help.
gitsy --help

# Show the installed version.
gitsy --version
```

## Development

See [docs/development.md](docs/development.md).

## Releases

Releases are created from `main`. See [docs/release.md](docs/release.md).
