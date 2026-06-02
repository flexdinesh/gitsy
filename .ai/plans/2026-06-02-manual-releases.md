# Manual Stable Releases

## Summary

Change stable releases from automatic releases on every push to `main` to manual
workflow dispatches. A manual dispatch releases the latest code from `main`,
automatically creates the next `v0.0.x` tag, and runs GoReleaser.

Development snapshot releases from `dev` remain automatic and unchanged.

## Key Implementation Changes

- Update `.github/workflows/release.yml`.
  - Remove the `push` trigger for `main`.
  - Keep `workflow_dispatch`.
  - Force checkout of `main` with full tag history.
  - Keep the existing next `v0.0.x` tag selection logic.
  - Keep automated tag creation before GoReleaser runs.
- Update `docs/release.md`.
  - Document manual stable releases from `main`.
  - Clarify that the workflow creates SemVer tags automatically.
  - Document that pushes to `dev` still run automatic snapshot releases.
  - Keep minor-line switching as future manual YAML work.
- Add `docs/development.md`.
  - Move the README development commands there.
  - Briefly document `[skip ci]` as a temporary escape hatch for intentionally
    skipping GitHub Actions, especially while the old push-triggered release
    policy exists or during docs-only changes.
- Update `README.md`.
  - Replace the inline Development section with a link to `docs/development.md`.
  - Keep the release docs link.

## Verification

- Run `go test ./...`.
- Review workflow YAML for expected trigger behavior.

## Decisions

- Manual stable releases only release latest code from `main`.
- Stable releases continue the `v0.0.x` line for now.
- Minor release strategy is future work.
- Tags remain automated during stable release workflow execution.
- Pushes to `dev` continue automatic snapshot releases.
- `[skip ci]` is documented in development docs as a temporary escape hatch.

## Tradeoffs And Risks

- Prevents accidental stable releases from README or docs pushes.
- Stable releases now require an explicit human action in GitHub Actions.
- The release workflow still mutates repository state by creating and pushing a
  tag, but only during manual dispatch.
- Because checkout is forced to `main`, selecting another branch in the workflow
  UI will not release that branch.

## Execution Guidance

If execution deviates from this plan, update this file to reflect the latest
approved plan and surface the deviation to the user.
