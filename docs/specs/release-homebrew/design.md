# Design: macOS Release and Homebrew Distribution

**Date:** 2026-11-13
**Status:** Approved for implementation under ALK-1
**Scope:** Harden the existing release workflow while preserving the current distribution design.

## Intent

Mac users can download release binaries for both supported Mac architectures and install anti-locker through Homebrew. Release tags are treated as data when passed into shell commands.

## Goals

- Build and upload `darwin/amd64` and `darwin/arm64` binaries for published releases.
- Keep the existing Homebrew formula source build and automated version/checksum update.
- Document tap trust and installation in the README.
- Avoid unsafe interpolation of a release tag into shell source.

## Non-goals

- Linux or Windows artifacts, code signing, notarization, new package managers, or runtime changes.
- Replacing the existing release and formula workflows.

## Chosen approach

Retain and harden the current workflows. The release workflow already builds both architectures, and the formula updater computes the tagged archive checksum and commits only when the formula changes. Pass the release tag through an environment variable and quote it in the `gh release upload` argument. This keeps existing release behavior and limits the change to the unsafe shell interpolation.

Replacing the release pipeline with a packaging action could change asset names and add migration work without a requirement. Moving Homebrew metadata to a separate tap repository would require repository setup, credential changes, and formula relocation outside this feature's scope.

## Architecture and interfaces

`release_artifacts.yaml` remains triggered by a published GitHub release. Its matrix cross-builds macOS Intel and Apple Silicon binaries and uploads each to the triggering release with `gh`. The release tag is exposed to the upload step through the `TAG` environment variable and passed as the quoted `"$TAG"` argument. Version embedding continues to use the runner-provided `GITHUB_REF_NAME`.

`update_homebrew_formula.yaml` and `.github/scripts/update_homebrew_formula.sh` continue to set the formula URL and SHA-256 from the published tag, then commit only if the formula changed. `Formula/anti-locker.rb` remains a source-built macOS formula. README installation and trust instructions remain the user entry point.

## Error handling and security

Build and upload failures remain non-zero workflow failures. Release tag values are not expanded into shell source; shell arguments containing values are quoted. Release credentials remain sourced from the configured GitHub secret.

## Validation strategy

Review the workflow diff for expression interpolation in `run` blocks and shell quoting. Run repository build, vet, and formatting checks as applicable; do not run Go tests unless requested.
