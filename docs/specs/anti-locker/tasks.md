# Tasks: anti-locker

The core implementation is present in the current repository. Task 5 records an existing behavior mismatch that remains to be resolved.

- [x] 1. Implement configuration loading and first-run setup
  - Load YAML from the default or overridden config path and validate interval, network entries, duplicates, and `caffeinate_arg`.
  - Run an interactive first-run flow with defaults, validation, persistence, and cancellation handling.
  - An existing empty networks list is accepted; missing or invalid required configuration fails before the loop starts.
  - _Boundary: internal/config_
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.6, 1.7, 1.8, 2.1, 2.2, 2.3, 2.4, 2.5, 2.6_

- [x] 2. Implement Wi-Fi SSID detection and recovery
  - Detect the active SSID through the production provider and return lookup errors for transient handling by the loop.
  - Recover from helper launch failures by clearing the extraction cache, retrying, and falling back to `ipconfig`.
  - _Boundary: internal/wifi_
  - _Requirements: 3.1, 3.2, 3.3_

- [x] 3. Implement the keep-awake process manager
  - Verify `caffeinate` is available before starting normal operation.
  - Start and reap the configured `caffeinate` process, expose its running state, and stop it with TERM-to-KILL escalation.
  - _Boundary: internal/keepawake_
  - _Requirements: 4.2, 4.3, 4.4, 4.5, 5.1, 5.2, 5.3_

- [x] 4. Implement the polling state machine
  - Check the SSID immediately and on each configured interval.
  - Start keep-awake only on trusted networks and stop it after a mismatch.
  - _Boundary: internal/loop_
  - _Requirements: 3.1, 3.2, 3.3, 4.1, 4.2, 4.3, 4.4, 4.5, 6.2_

- [ ] 5. Align quiet-iteration logging with the approved behavior
  - Keep startup and state-transition messages while suppressing repeated status logs when the keep-awake state does not change.
  - _Boundary: internal/loop_
  - _Requirements: 6.3_

- [x] 6. Integrate CLI and graceful shutdown
  - Wire config loading/setup, runtime dependencies, and the loop through the `anti-locker` CLI.
  - Handle `--config`, help/version output, non-interactive first-run failure, and SIGINT/SIGTERM cancellation.
  - Deferred cleanup ensures `caffeinate` cannot outlive the CLI process.
  - _Boundary: main.go_
  - _Requirements: 1.1, 1.2, 2.1, 2.4, 2.5, 2.6, 5.1, 5.2, 5.3, 6.1, 6.2_

- [x] 7. Add package and integration coverage and complete repository docs
  - Cover config validation and setup, SSID parsing/recovery, process lifecycle, and loop transitions with tests.
  - Keep real SSID detection in an integration test that is skipped in short mode.
  - Document install, usage, configuration, runtime behavior, and development commands in README.
  - _Requirements: 1.3, 1.5, 1.6, 1.7, 1.8, 2.2, 2.3, 2.4, 2.5, 2.6, 3.1, 3.2, 3.3, 4.1, 4.2, 4.3, 4.4, 4.5, 4.6, 5.2, 5.3, 6.1, 6.2, 6.3_

## Implementation Notes

- Task 5 is pending: the loop currently logs the untrusted SSID on every interval, contrary to requirement 6.3 and the approved design's quiet no-op behavior.
- The implementation plan's manual live-Wi-Fi walkthrough was not carried forward as a task because it is manual verification rather than implementation work. The opt-in Wi-Fi integration test remains covered by Task 7.

## Requirement Mapping

- Requirement 1: Task 1, Task 6, Task 7
- Requirement 2: Task 1, Task 6, Task 7
- Requirement 3: Task 2, Task 4, Task 7
- Requirement 4: Task 1, Task 3, Task 4, Task 7
- Requirement 5: Task 3, Task 6, Task 7
- Requirement 6: Task 4, Task 5, Task 6, Task 7
