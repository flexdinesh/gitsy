# GoReleaser Release Automation Plan

## Summary

Automate `gitsy` releases from `main` with GoReleaser and GitHub Actions while
keeping the first release line on automatically created `v0.0.x` SemVer tags.
Use the `dev` branch for development validation, including GoReleaser snapshot
checks, without publishing development releases.

## Implemented Changes

- Add `.goreleaser.yaml` for macOS and Linux release artifacts.
- Add GitHub Actions CI for pushes to `dev` and `main`, and PRs into `main`.
- Run GoReleaser snapshot validation on pushes to `dev`.
- Add a GitHub Actions release workflow for pushes to `main`.
- Have the release workflow create the next `v0.0.x` tag automatically.
- Start at `v0.0.1` when no release tags exist.
- Update version handling so release builds use GoReleaser linker flags and
  local builds report a development version.
- Document release behavior in `docs/release.md`.

## Release Workflow

- Push development changes to `dev`.
- The CI workflow runs tests, builds the CLI, and validates GoReleaser snapshot
  packaging on `dev`.
- Push or merge to `main`.
- The release workflow runs tests.
- If the current commit already has a `v0.0.x` tag, the workflow reuses it.
- Otherwise, it creates the next patch tag:

  ```bash
  v0.0.1
  v0.0.2
  v0.0.3
  ```

- GoReleaser publishes the GitHub Release.

## Switching Minor Versions

Minor version changes are manual for now. To switch from `v0.0.x` to `v0.1.x`,
update `.github/workflows/release.yml` so the tag selector uses `v0.1.*` and
starts at `v0.1.0`.

## Decisions

- `dev` is used for development validation and GoReleaser snapshot checks.
- Development builds are not published as GitHub Releases.
- No Homebrew tap.
- No Arch package or AUR package.
- Windows is not targeted.
- No moving `latest` tag; Go resolves `@latest` from immutable SemVer tags.
