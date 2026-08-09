# Anti Locker

`antilocker` keeps your MacBook awake by preventing idle sleep — but only when you're on a Wi-Fi network you've explicitly allowed. On any other network (or if Wi-Fi is off) it stays silent and lets your Mac sleep normally.

**macOS only.** Manual screen lock (`Ctrl+Cmd+Q`) and display-off always work — `antilocker` only blocks *idle* sleep, never your explicit actions.

## Install

    go build -o antilocker .
    # optionally: install -m 0755 antilocker /usr/local/bin/antilocker

## Usage

    antilocker                                # use ~/.config/antilocker.yaml
    antilocker --config /path/to/config.yaml  # custom config path

On first run (no config file), `antilocker` will interactively ask for:

1. **Check interval** in seconds (default `3600`)
2. **Wi-Fi networks** to keep the Mac awake on (at least one, one at a time, empty line to finish)

It saves your answers to the config path and starts running immediately.

## Configuration

```yaml
interval: 3600            # optional, seconds; defaults to 3600
networks:                 # required, may be empty list
  - "Home Network 1"
  - "Home Network 2"
```

- SSID matching is exact and case-sensitive.
- `interval: 0` or negative → startup error.
- `networks: null` or a missing `networks:` key → startup error.
- Duplicates are de-duplicated with a startup warning.

## How it works

1. Every `interval` seconds, `antilocker` checks the current Wi-Fi SSID via `ipconfig getsummary en0` (falling back to `system_profiler SPAirPortDataType`).
2. If the SSID matches a configured network, it spawns `caffeinate -i` to block idle sleep.
3. If the SSID stops matching (or Wi-Fi drops), it stops `caffeinate`.
4. `Ctrl-C` cleanly shuts down the app and terminates `caffeinate`.

## Development

    go build ./...
    go test ./...
    go test -short ./...
    go vet ./...
    gofmt -s -l .
    GOOS=darwin go build -o antilocker .

## License

See [LICENSE](LICENSE).