# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/en/1.0.0/) and this project adheres to [Semantic Versioning](http://semver.org).

## [v1.0.3](https://github.com/ansidev/anti-locker/compare/v1.0.2...v1.0.3) (2026-08-11)

### Features

- allow overriding caffeinate arguments

Full Changelog: [v1.0.2...v1.0.3](https://github.com/ansidev/anti-locker/compare/v1.0.2...v1.0.3)

## [v1.0.2](https://github.com/ansidev/anti-locker/compare/v1.0.1...v1.0.2) (2026-08-09)

### Bug Fixes

- use macwifi library to get the current SSID

- **action:** add branch to run tests when code changes are pushed to

- **test:** correct test cases

Full Changelog: [v1.0.1...v1.0.2](https://github.com/ansidev/anti-locker/compare/v1.0.1...v1.0.2)

## [v1.0.1](https://github.com/ansidev/anti-locker/compare/v1.0.0...v1.0.1) (2026-08-09)

### Bug Fixes

- rename module

### Features

- add workflow to test Go code

Full Changelog: [v1.0.0...v1.0.1](https://github.com/ansidev/anti-locker/compare/v1.0.0...v1.0.1)

## v1.0.0 (2026-08-09)

### Bug Fixes

- update CI workflows

- **test:** fix stdout race and child cleanup in integration test

### Features

- **cli:** add main.go with urfave/cli v3 and integration tests

- **config:** add interactive first-run setup prompts

- **config:** add validation rules and Contains()

- **config:** add Config struct and Load() with yaml.v3

- **hardening:** fix keepawake race, add wifi integration test, finalize README

- **keepawake:** add ExecManager with Start/Stop/IsRunning

- **keepawake:** add Manager interface and VerifyCaffeinate

- **loop:** add periodic state machine with context shutdown

- **wifi:** add ExecProvider with ipconfig/system_profiler fallback

Full Changelog: [v1.0.0](https://github.com/ansidev/anti-locker/commits/v1.0.0)
