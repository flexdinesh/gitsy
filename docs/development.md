# Development

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

## Skipping Actions

`[skip ci]` can be used as a temporary escape hatch when a commit should skip
GitHub Actions, such as a docs-only change that should not run release
automation.

```bash
git commit -m "docs: update readme [skip ci]"
```

## Trying the TUI locally

```bash
# Regenerate the demo workspace (26 repos, lands in /tmp/gitsy-demo).
./scripts/setup-demo.sh

# Run the CLI against it.
go run ./cmd/gitsy --dir /tmp/gitsy-demo
```

Re-run the script anytime to reset fixture state (e.g. after `--sync`).
