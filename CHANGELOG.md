# Changelog

All notable changes to this project are documented here. The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project follows [Semantic Versioning](https://semver.org/).

## [Unreleased] — v0.2.0

Complete rewrite in Go.

### Fixed
- **Flash Device ran `read_flash` instead of `write_flash`**, overwriting the local firmware file with the device contents instead of flashing the device.
- Operations always reported "SUCCESSFULLY" because the esptool exit code was ignored. Failures are now detected and reported with troubleshooting hints.
- Paths with spaces or shell characters broke commands (they were built as shell strings). esptool now runs with an argument list.
- Backups read a fixed 1MB, so 4MB modules were backed up incompletely. The flash size is now detected, and the backup size is verified.
- The port index was not validated (`-1` silently picked the last port).
- No ports were detected on Linux (`/dev/tty.*` does not match `/dev/ttyUSB0`).
- The flash step only checked that the folder existed, not the firmware file.
- Did not run on Python ≥ 3.10 (PyInquirer is unmaintained).

### Added
- CLI sub-commands: `ports`, `info`, `backup`, `erase`, `flash`, `full`, `version`, with `--yes` for scripting.
- **Full workflow**: backup → erase → flash with a single confirmation.
- Chip information (chip, MAC, flash size) shown on connect.
- Automatic, unique backup names: `<chip>_<mac>_<date>.bin`.
- Firmware picker listing every `.bin` in the workspace. Restoring a backup means flashing it.
- Fixed workspace (`~/oxflasher`, `OXFLASHER_HOME` or `--workspace`) created automatically with `backup/`, `haa/` and `tasmota/`.
- USB-serial filtering (hides Bluetooth and debug ports), uses `/dev/cu.*` on macOS, and auto-selects the port when there is only one.
- Configurable baud rate (default 460800), flash mode (default `dout`) and flash size.
- Support for esptool v4 (`esptool.py`, `read_flash`) and v5 (`esptool`, `read-flash`).
- Firmware validation before connecting (exists, `.bin`, not empty, fits in the flash).
- Unit tests with a fake esptool, CI on macOS and Linux, and a Makefile.

### Changed
- Destructive confirmations now default to **No**.
- Ctrl+C / Esc cancel cleanly and stop esptool.

### Removed
- Python script `src/oxflasher.py` (still available at tag `v0.1.1`).

## [v0.1.1] — 2022

- Python version with PyInquirer menu: backup, erase and flash.
