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

## Install the dev version

For the latest tested changes on `main`:

```bash
go install github.com/flexdinesh/gitsy/cmd/gitsy@dev
```

## Skipping Actions

Include `[skip ci]` in the commit message pushed to `main` (including the merge
or squash commit) to skip the entire **CI** workflow, including tests and the
`dev` update. Manual stable releases are unaffected.

```bash
git commit -m "docs: update readme [skip ci]"
```

On pull requests, skipped required checks remain pending and can block merging.
See [GitHub's skip instructions](https://docs.github.com/en/actions/how-tos/manage-workflow-runs/skip-workflow-runs).

## Trying the TUI locally

```bash
# Regenerate the demo workspace (27 repos, lands in /tmp/gitsy-demo).
./scripts/setup-demo.sh

# Run the CLI against it.
go run ./cmd/gitsy --dir /tmp/gitsy-demo
```

Re-run the script to reset fixture state (e.g. after `--sync`). It only resets
directories carrying its `.gitsy-demo` ownership marker. Existing unmarked
directories and symlink targets are rejected; choose a new path for older demos.

Pass a separate target for each concurrent run:

```bash
./scripts/setup-demo.sh /tmp/gitsy-demo-my-run
go run ./cmd/gitsy --dir /tmp/gitsy-demo-my-run
```
