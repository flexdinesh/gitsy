# gitsy

gitsy is a small Go CLI that scans a directory for Git repositories and linked worktrees, fetches fresh upstream metadata by default, and shows a compact status table so you can see what needs attention.

## install

`@latest` resolves to the newest stable SemVer tag, such as `v0.0.1` or
`v0.0.2`. There is no moving `latest` Git tag.

```bash
# Install the latest stable release.
go install github.com/flexdinesh/gitsy/cmd/gitsy@latest

# Install a specific stable release.
go install github.com/flexdinesh/gitsy/cmd/gitsy@v0.0.1
```

## Usage

```bash
# Show all discovered repositories under the current directory.
gitsy

# Scan repository directories up to a specific nested depth.
gitsy --max-depth 5

# Start scanning from a specific directory instead of the current directory.
gitsy --dir ~/workspace

# Print warnings for skipped repos and failed git commands.
gitsy --verbose

# Skip fetching upstream changes and use local status only.
gitsy --no-fetch

# Fast-forward repositories that can safely update without conflicts.
gitsy --sync

# Show help.
gitsy --help

# Show the installed version.
gitsy --version
```

## Development

```bash
# Check out the repo.
git clone git@github.com:flexdinesh/gitsy.git && cd gitsy

# Run tests.
go test ./...

# Build the binary.
go build -o bin/gitsy ./cmd/gitsy

# Install the local build as a binary.
go install ./cmd/gitsy
```

## Releases

Releases are created from `main`. See `docs/release.md`.
