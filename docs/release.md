# Releases

Releases are SemVer Git tags on `main`.

## Release

Required repository secret:

- `HOMEBREW_TAP_TOKEN`: fine-grained token with contents write and pull request write access to `flexdinesh/homebrew-tap`.

1. Merge release-ready code to `main`.
2. Run the **Release** workflow. It requires no inputs.
3. The workflow selects the next patch version, verifies the repository, then publishes the tag and GitHub Release with GoReleaser.
4. It generates `Formula/gitsy.rb` and opens or updates a pull request against `flexdinesh/homebrew-tap`.
5. Merge the tap pull request after its Homebrew checks pass.

Each release publishes `checksums.txt` and four archives, where `<version>` omits the leading `v`:

- `gitsy_<version>_darwin_amd64.tar.gz`
- `gitsy_<version>_darwin_arm64.tar.gz`
- `gitsy_<version>_linux_amd64.tar.gz`
- `gitsy_<version>_linux_arm64.tar.gz`

Each archive contains the native `gitsy` binary and README.

The tap branch is deterministic per version, such as `gitsy-v0.1.2`. Rerunning a release whose tag still points to current `main` reuses the published artifacts and updates the same branch and pull request. Published artifacts are not rebuilt or replaced. The workflow publishes the GitHub Release before updating the tap, so rerunning it can repair a failed tap update.

The tap repository owns Homebrew style, strict audit, install, and formula test checks before merge.

## Version series

`.release-version` contains the active `major.minor` release series. For example, `0.1` selects `v0.1.2` when `v0.1.1` is the latest release, then `v0.1.3`, and so on. A rerun from the same commit reuses its existing tag and release.

To begin a new minor or major series, change `.release-version` in the repo. Changing it to `0.2` makes the next release `v0.2.0`; changing it to `1.0` makes the next release `v1.0.0`. Later releases continue incrementing that series' patch number.

## Installing

```bash
# Latest release.
go install github.com/flexdinesh/gitsy/cmd/gitsy@latest

# Specific release.
go install github.com/flexdinesh/gitsy/cmd/gitsy@v0.1.0

# Development release.
go install github.com/flexdinesh/gitsy/cmd/gitsy@dev
```

## Version Output

Local builds print a development version. Release builds get the version from
the release tag through GoReleaser linker flags.

```bash
gitsy --version
```

Do not create a moving `latest` tag. Go already resolves `@latest` to the newest
SemVer tag.
