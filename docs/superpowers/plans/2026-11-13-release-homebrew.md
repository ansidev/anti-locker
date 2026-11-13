# ALK-1 Release and Homebrew Implementation Plan

## Requirements

- R1: Published releases contain macOS Intel and Apple Silicon binaries.
- R2: Homebrew formula URL and SHA-256 track the release tag.
- R3: Formula automation skips empty commits and fails on build, checksum, or publish errors.
- R4: README documents tap trust and install commands.
- R5: Release tag data cannot inject shell syntax into workflow commands.

## Design

Preserve the existing release-artifact matrix, formula updater, source-built macOS formula, and README instructions. In the binary upload workflow, pass the release tag through the `TAG` environment variable and use the quoted variable as the `gh release upload` tag argument. See [the design document](../specs/2026-11-13-release-homebrew-design.md).

## Tasks

- [x] 1. Inspect current release, formula, and README flows against ALK-1.
- [x] 2. Record Phase 1 requirements on Kanpal and in this plan.
- [x] 3. Write the bounded design for the required workflow hardening.
- [x] 4. Remove direct release-tag interpolation from the binary upload shell command.
- [x] 5. Review changed workflow syntax and run permitted repository validation.
- [x] 6. Stage only the issue-related files, commit in the ALK-1 worktree, and move the issue to `in-review`.
