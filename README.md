# Anti Locker

macOS-only CLI that prevents idle sleep conditionally based on the connected Wi-Fi network.

**Status: in active development.** See [docs/superpowers/specs/2026-08-01-antilocker-design.md](docs/superpowers/specs/2026-08-01-antilocker-design.md) for the design specification.

## Build

    go build -o antilocker .

## Usage (planned)

    antilocker                                # start with default config
    antilocker --config /path/to/config.yaml  # start with custom config
