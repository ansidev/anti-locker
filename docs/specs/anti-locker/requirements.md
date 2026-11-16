# Requirements: anti-locker

## Project Description

Mac users who work across trusted and untrusted Wi-Fi networks need control over idle sleep without losing manual control of their devices. anti-locker checks the connected Wi-Fi network and runs `caffeinate` only while the network matches the user's configuration; it should provide an interactive first-run setup and shut down cleanly.

## Scope

The feature is a foreground macOS CLI with YAML configuration, Wi-Fi SSID detection, conditional idle-sleep prevention, and clean signal handling. It does not inhibit manual sleep, screen lock, or display-off; support other operating systems; run as a daemon; or add Wi-Fi event monitoring.

## Requirements

### Requirement 1: Configuration loading

1.1 When no config path is supplied, anti-locker shall use `~/.config/anti-locker.yaml`.

1.2 When a config path is supplied with `--config`, anti-locker shall load and save configuration at that path, expanding a leading `~/` to the user's home directory.

1.3 When a config file omits `interval`, anti-locker shall use 3600 seconds.

1.4 When a config file omits `caffeinate_arg`, anti-locker shall use `-i`.

1.5 When a config file is malformed, unreadable, has no `networks` key, contains a non-positive interval, contains an empty network name, or contains an empty `caffeinate_arg`, anti-locker shall report a startup error and exit with status 1.

1.6 When a config contains duplicate network names, anti-locker shall remove duplicates while preserving the first occurrence and emit a startup warning.

1.7 When a config explicitly contains an empty network list, anti-locker shall accept it and never treat any SSID as trusted.

1.8 When loading network names, anti-locker shall trim surrounding whitespace. SSID matching shall be exact and case-sensitive.

### Requirement 2: First-run setup

2.1 When the effective config file does not exist and standard input is interactive, anti-locker shall prompt for a check interval and one or more trusted Wi-Fi network names.

2.2 When the interval prompt is left empty, anti-locker shall use 3600 seconds; when the entered value is invalid or non-positive, anti-locker shall prompt again.

2.3 When the user tries to finish setup without entering a network, anti-locker shall prompt for at least one network.

2.4 When setup succeeds, anti-locker shall create parent directories as needed, persist interval and networks to the effective config path, report that path, and continue into normal operation without restarting.

2.5 When the config is missing and standard input is not interactive, anti-locker shall report that setup must be run interactively and exit with status 1 without writing a config file.

2.6 When SIGINT or SIGTERM cancels first-run setup, anti-locker shall report cancellation, avoid writing a partial config, and exit with status 1.

### Requirement 3: Wi-Fi detection and matching

3.1 While running, anti-locker shall determine the current Wi-Fi SSID and compare it with the configured network names.

3.2 When Wi-Fi is disconnected or no usable SSID is available, anti-locker shall treat the current SSID as unmatched.

3.3 When SSID lookup encounters a transient error, anti-locker shall log a warning and continue checking on the next interval.

### Requirement 4: Conditional idle-sleep prevention

4.1 At startup and then every configured interval, anti-locker shall check the current SSID.

4.2 When the SSID matches a configured network and the keep-awake process is not running, anti-locker shall start `caffeinate` with the configured argument.

4.3 When the SSID does not match a configured network and the keep-awake process is running, anti-locker shall stop that process.

4.4 When the SSID match and process state require no change, anti-locker shall leave the process state unchanged.

4.5 When starting `caffeinate` fails, anti-locker shall log a warning and continue checking on subsequent intervals.

4.6 `caffeinate` shall prevent idle system sleep only; users shall retain manual control of screen lock, display-off, and manual sleep.

### Requirement 5: Process lifecycle and shutdown

5.1 Before starting the loop, anti-locker shall verify that `caffeinate` is available and report a fatal startup error if it is missing.

5.2 When anti-locker receives SIGINT or SIGTERM during normal operation, it shall stop `caffeinate` and exit cleanly.

5.3 When anti-locker exits for any reason after starting `caffeinate`, it shall ensure the child process does not outlive it, escalating from SIGTERM to SIGKILL after a short grace period if needed.

### Requirement 6: CLI and runtime reporting

6.1 The `anti-locker` command shall provide help, version output, and a `--config` path option.

6.2 At startup, anti-locker shall report the effective config path, check interval, and number of configured networks.

6.3 On network or keep-awake state transitions, anti-locker shall log the transition; unchanged iterations shall not cause redundant transition logs.
