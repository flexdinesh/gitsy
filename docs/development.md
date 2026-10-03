# Development

```bash
# Check out the repo.
git clone git@github.com:flexdinesh/gitsy.git && cd gitsy

# Trust project tasks, install tools, download dependencies, and install hooks.
mise trust
mise run setup

# Run the full local checks manually.
mise run check

# Run tests.
mise run test

# Build the binary.
mise run build

# Install the local build as a binary.
mise run install
```

Install [mise](https://mise.jdx.dev/getting-started.html) first. `mise.toml`
pins Go and Lefthook; release tasks also pin GoReleaser. Tasks install missing
tools automatically, without requiring shell activation.

Run `mise tasks` to list commands. Useful tasks:

| Task | Purpose |
| --- | --- |
| `mise run check` | Formatting, module tidiness, vet, race tests, build |
| `mise run ci` | Formatting and build only |
| `mise run fmt` | Format Go source |
| `mise run tidy` | Tidy module files |
| `mise run test:package ./internal/args` | Test a specific package |
| `mise run run -- --no-fetch` | Run the CLI with arguments |
| `mise run demo /tmp/gitsy-demo-my-run` | Create a demo workspace |
| `mise run hooks` | Reinstall Git hooks |
| `mise run release:config` | Validate GoReleaser configuration |
| `mise run release:version` | Print the next release version |

`GITSY_CHECK_ROOT` is reserved for pre-push validation: it redirects task working
directories to the committed snapshot while using the project's task config.

## Push checks

Lefthook reads the tracked `lefthook.yml` on each push. Its version is pinned
in `mise.toml`, separate from the CLI's Go dependencies. It runs the mise
`pre-push` task, which invokes `check` for each committed snapshot.
No global installation or Git config changes are required.

The pre-push hook checks each distinct commit being pushed in a temporary
snapshot, so uncommitted changes cannot mask failures. It checks formatting,
module tidiness, `go vet`, the full test suite with race detection, and builds
all packages. Failed checks block the push. Deletions skip checks; tags pointing
to commits are checked too. Temporary files are removed afterward.

Install hooks once per clone using the command above; later config changes
apply automatically. Go and a C compiler are required for race detection.
Hooks can be bypassed with
`git push --no-verify`.

Routine CI checks formatting and builds all packages; passing pushes to `main`
still update `dev`. The existing **Test** check name is preserved for branch
protection. Manual stable releases retain tests, vet, and build validation.

## Install the dev version

For the latest CI-checked changes on `main`:

```bash
go install github.com/flexdinesh/gitsy/cmd/gitsy@dev
```

## Skipping Actions

Include `[skip ci]` in the commit message pushed to `main` (including the merge
or squash commit) to skip the entire **CI** workflow, including checks and the
`dev` update. Manual stable releases are unaffected.

```bash
git commit -m "docs: update readme [skip ci]"
```

On pull requests, skipped required checks remain pending and can block merging.
See [GitHub's skip instructions](https://docs.github.com/en/actions/how-tos/manage-workflow-runs/skip-workflow-runs).

## Trying the TUI locally

```bash
# Regenerate the demo workspace (27 repos, lands in /tmp/gitsy-demo).
mise run demo

# Run the CLI against it.
mise run run -- --dir /tmp/gitsy-demo
```

Re-run the script to reset fixture state (e.g. after `--sync`). It only resets
directories carrying its `.gitsy-demo` ownership marker. Existing unmarked
directories and symlink targets are rejected; choose a new path for older demos.

Pass a separate target for each concurrent run:

```bash
mise run demo /tmp/gitsy-demo-my-run
mise run run -- --dir /tmp/gitsy-demo-my-run
```
