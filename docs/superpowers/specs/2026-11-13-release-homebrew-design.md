# macOS Release and Homebrew Distribution Design

**Date:** 2026-11-13  
**Status:** Approved for implementation under ALK-1  
**Scope:** Harden the existing release workflow while preserving the current distribution design.

## Intent

Mac users should be able to download release binaries for both supported Mac architectures and install anti-locker through Homebrew. Release tags must be handled as data when passed into shell commands.

## Goals

- Build and upload `darwin/amd64` and `darwin/arm64` binaries for published releases.
- Keep the existing Homebrew formula source build and automated version/checksum update.
- Document tap trust and installation in the README.
- Avoid unsafe interpolation of a release tag into shell source.

## Non-goals

- Linux or Windows artifacts, code signing, notarization, new package managers, or runtime changes.
- Replacing the existing release and formula workflows.

## Constraints

- macOS-only product, Go implementation, existing GitHub Actions workflows.
- Release credentials remain in GitHub Actions secrets.
- No new project dependencies.

## Approaches

1. **Retain and harden the current workflows (chosen).** The release workflow already builds both architectures; the formula updater already computes the tagged archive checksum and commits only when the formula changes. Pass the release tag using an environment variable so its value cannot alter shell syntax. This is the smallest change and keeps current release behavior.
2. **Replace the release pipeline with a packaging action.** This could produce compressed archives and checksums in one action, but would change asset names and add migration work without a stated requirement.
3. **Move Homebrew metadata into a separate tap repository.** This is a valid distribution layout if separately requested, but requires repository setup, credential changes, and formula relocation outside the current issue scope.

## Architecture and interfaces

`release_artifacts.yaml` remains triggered by a published GitHub release. Its matrix cross-builds macOS Intel and Apple Silicon binaries and uploads each to the triggering release with `gh`. The release tag is read from an environment variable (`TAG`) and passed as a quoted argument. Version embedding continues to use the runner-provided `GITHUB_REF_NAME`.

`update_homebrew_formula.yaml` and `.github/scripts/update_homebrew_formula.sh` continue to set the formula URL and SHA-256 from the published tag, then commit only if the formula changed. `Formula/anti-locker.rb` remains a source-built macOS formula. README installation and trust instructions remain the user entry point.

## Error handling and security

Build and upload failures must remain non-zero workflow failures. Release tag values must not be expanded into shell source; values used in shell commands are quoted. Release credentials remain sourced from the configured GitHub secret.

## Validation strategy

Review the workflow diff for expression interpolation in `run` blocks and shell quoting. Run repository build, vet, and formatting checks as applicable; do not run Go tests unless requested.
