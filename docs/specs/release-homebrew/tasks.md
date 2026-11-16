# Tasks: Release and Homebrew Distribution

- [x] 1. Confirm existing release and Homebrew behavior
  - Inspect the release artifact matrix, formula updater, and README installation instructions.
  - The existing workflows publish both macOS architectures and update the tagged formula checksum.
  - _Requirements: 1.1, 1.2, 2.1, 2.2, 3.1, 4.1_

- [x] 2. Harden release tag handling
  - Pass the release tag to the upload step through the `TAG` environment variable.
  - Quote `"$TAG"` as the release argument to `gh release upload` so the tag remains data.
  - _Requirements: 5.1, 5.2_

- [x] 3. Review and validate the release workflow change
  - Review shell interpolation and quoting in the upload step.
  - Confirm build and upload errors continue to fail the workflow.
  - _Requirements: 3.2, 5.1, 5.2_

- [x] 4. Complete ALK-1 delivery
  - Stage only the issue-related workflow and specification changes, commit them, and move the issue to `in-review`.
  - _Requirements: 1.1, 1.2, 2.1, 2.2, 3.1, 3.2, 4.1, 5.1, 5.2_

## Requirement Mapping

- Requirement 1: 1, 4
- Requirement 2: 1, 4
- Requirement 3: 1, 3, 4
- Requirement 4: 1, 4
- Requirement 5: 2, 3, 4
