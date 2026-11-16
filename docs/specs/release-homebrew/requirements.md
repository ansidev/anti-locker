# Requirements: Release and Homebrew Distribution

## Project Description

Mac users need reliable release downloads and a documented Homebrew installation path for anti-locker. The project already builds macOS releases and maintains a Homebrew formula; this feature preserves those flows and hardens release asset upload so tag values are handled safely.

## Scope

This feature covers published macOS release binaries, Homebrew formula updates, installation guidance, and safe handling of release tags in workflow shell commands. It does not add other operating systems, package managers, code signing, notarization, or runtime behavior.

## Requirements

### Requirement 1: macOS release binaries

1.1 When a release is published, the release workflow shall build and publish a macOS Intel (`darwin/amd64`) binary.

1.2 When a release is published, the release workflow shall build and publish an Apple Silicon (`darwin/arm64`) binary.

### Requirement 2: Homebrew formula version and checksum

2.1 When a release is published, the formula updater shall set the formula source URL to the archive for that release tag.

2.2 When a release is published, the formula updater shall set the formula SHA-256 checksum to match that release archive.

### Requirement 3: Formula update commits and failures

3.1 When the rendered formula is unchanged, the updater shall not create a commit.

3.2 If a build, checksum, or publish operation fails, the workflow shall report failure.

### Requirement 4: Installation guidance

4.1 The README shall document the Homebrew tap, formula trust, and install commands.

### Requirement 5: Safe release tag handling

5.1 When a release tag is passed to a shell command, the workflow shall treat the tag as data rather than interpolating it into shell source.

5.2 When uploading an asset, the workflow shall target the release identified by the supplied tag.
