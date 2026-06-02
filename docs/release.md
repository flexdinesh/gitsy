# Releases

Releases are SemVer Git tags on `main`.

## Current Policy

Stable releases are created manually from the latest code on `main` by running
the GitHub Actions release workflow. Each dispatch creates the next `v0.0.x`
release.

Examples:

```bash
v0.0.1
v0.0.2
v0.0.3
```

The GitHub Actions release workflow creates the tag, runs GoReleaser, and
publishes macOS and Linux archives plus checksums.

Pushes to `dev` still run automatic GoReleaser snapshot releases through CI.

## Installing

```bash
# Latest release.
go install github.com/flexdinesh/gitsy/cmd/gitsy@latest

# Specific release.
go install github.com/flexdinesh/gitsy/cmd/gitsy@v0.0.1

# Development release.
go install github.com/flexdinesh/gitsy/cmd/gitsy@dev
```

## Version Output

Local builds print a development version. Release builds get the version from
the release tag through GoReleaser linker flags.

```bash
gitsy --version
```

## Switching Minor Versions

Switch manually when `0.0.x` no longer feels right, for example when a release
is the first meaningful preview rather than just the next small change.

To switch, update `.github/workflows/release.yml` so the tag selector uses the
new minor line, such as `v0.1.*`, and starts at `v0.1.0`.

After that, releases should continue as:

```bash
v0.1.0
v0.1.1
v0.1.2
```

Do not create a moving `latest` tag. Go already resolves `@latest` to the newest
SemVer tag.
