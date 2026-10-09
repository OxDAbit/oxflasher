[![CI](https://github.com/OxDAbit/oxflasher/actions/workflows/ci.yml/badge.svg)](https://github.com/OxDAbit/oxflasher/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/tag/OxDAbit/oxflasher?color=red&label=release)](https://github.com/OxDAbit/oxflasher/releases/latest)
[![License: GPL v3](https://img.shields.io/badge/license-GPLv3-blue.svg)](LICENSE)

# oxflasher

A fast terminal tool to **back up, erase and flash ESP devices** (ESP8266 / ESP32) with firmwares such as [HAA](https://github.com/RavenSystem/esp-homekit-devices) or [Tasmota](https://tasmota.github.io/). It wraps Espressif's [`esptool`](https://github.com/espressif/esptool) so you can use a menu or one-line commands.

```
$ oxflasher full tasmota/tasmota.bin
Using the only USB-serial port found: /dev/cu.usbserial-14310  (USB-serial (CH340 / CP210x / FTDI))
▸ Connecting to /dev/cu.usbserial-14310 ...
╭───────────────────────────────╮
│ Device                        │
│ Port        /dev/cu.usbse...  │
│ Chip        ESP8266EX         │
│ MAC         84:f3:eb:12:34:56 │
│ Flash size  1MB               │
╰───────────────────────────────╯
? Run full workflow (backup → erase → flash)?  Yes / No
▸ Reading 1MB of flash ... into ~/oxflasher/backup/esp8266ex_84f3eb123456_20261009-153012.bin
✔ Backup saved (1MB)
▸ Erasing flash ...
✔ Flash erased
▸ Flashing ~/oxflasher/tasmota/tasmota.bin
✔ Device flashed with tasmota.bin
✔ Full workflow completed
```

## Features

- **Detects USB-serial adapters automatically** (CH340, CP210x, FTDI, native USB). Bluetooth and other non-USB ports are hidden, and if only one adapter is connected it is used without asking.
- **Shows chip info** (chip, MAC and flash size) as soon as it connects.
- **Full backups.** The flash size is detected, so 4MB modules are not cut at 1MB. Backups get unique names (`<chip>_<mac>_<date>.bin`) and are never overwritten silently. Each backup is checked for size.
- **Pick a firmware from a list** of every `.bin` in your workspace. Restoring a backup is just flashing it.
- **Full workflow:** backup → erase → flash with a single confirmation. It stops if the backup fails.
- **Safe defaults.** Destructive actions default to **No**, the firmware is validated before connecting (exists, `.bin`, fits in the flash), and esptool errors are reported with hints.
- **Interactive menu or scriptable CLI** (`--yes` for non-interactive use).
- **Single static binary** written in Go. No Python packages to manage for oxflasher itself.

## Requirements

| What | Why |
|---|---|
| **esptool** ≥ 3.x | Talks to the chip. Install with `pipx install esptool` (or `pip install esptool`, or `brew install esptool`). v4 and v5 are both supported. |
| **USB-serial driver** | Recent macOS includes CH34x/CP210x drivers. On older systems, install the driver for your adapter. |
| **Linux only:** serial permissions | `sudo usermod -aG dialout $USER`, then log out and back in. |
| **Go ≥ 1.24** | Only needed to build from source. |

Supported OS: **macOS** and **Linux**. Windows is untested: auto-detection is disabled there, but `-p COM3` may work.

## Installation

**With Go:**

```bash
go install github.com/OxDAbit/oxflasher/cmd/oxflasher@latest
```

The binary goes to `$(go env GOPATH)/bin` (usually `~/go/bin`); make sure it is in your `PATH`.

**From source:**

```bash
git clone https://github.com/OxDAbit/oxflasher
cd oxflasher
make build          # → ./bin/oxflasher
make install        # or install it into ~/go/bin
```

Pre-built binaries will be published on the [Releases](https://github.com/OxDAbit/oxflasher/releases) page.

Check the installation:

```bash
oxflasher version
```

## Workspace

On first run oxflasher creates its working folder:

```
~/oxflasher/
├── backup/     backups read from devices
├── haa/        HAA binaries (e.g. fullhaaboot.bin)
└── tasmota/    Tasmota binaries (e.g. tasmota.bin)
```

Copy your firmware files into `haa/` or `tasmota/` (any sub-folder works, every `.bin` is listed). To use a different location, set `OXFLASHER_HOME=/path` or pass `--workspace /path`.

## Putting the ESP into flash mode

The chip only accepts commands while in **bootloader (flash) mode**:

1. Connect **TX→RX, RX→TX, GND→GND and 3.3V→3.3V**. ⚠️ **Never use 5V** on the ESP pins.
2. Hold **GPIO0 to GND** (on Sonoff devices: hold the button) while powering the device on.
3. Release after ~2 seconds. Dev boards with auto-reset (NodeMCU, ESP32 DevKit) do this automatically.

Repeat this step before each operation if your board has no auto-reset. Each esptool command resets the chip at the end.

> **Disconnect the device from mains power** before opening it and wiring the adapter.

## Usage

### Interactive menu

```bash
oxflasher
```

1. Select the port (↑/↓ + Enter). Use **Refresh** after plugging in the device, or **Show all serial ports** if yours is not listed.
2. oxflasher connects and shows the chip, MAC and flash size.
3. Choose an action: **Create backup**, **Erase device**, **Flash firmware**, **Full workflow**, **Show device info**, **Change port** or **Exit**.

Press **Esc** or **Ctrl+C** at any prompt to cancel.

### Commands

```bash
oxflasher ports [--all]                 # list serial ports
oxflasher info                          # chip, MAC, flash size
oxflasher backup                        # → ~/oxflasher/backup/<chip>_<mac>_<date>.bin
oxflasher backup -o sonoff_basic        # → ~/oxflasher/backup/sonoff_basic.bin
oxflasher backup -o ~/Desktop/          # auto-named file inside a folder
oxflasher erase
oxflasher flash haa/fullhaaboot.bin
oxflasher full tasmota/tasmota.bin      # backup → erase → flash
```

| Flag | Default | Description |
|---|---|---|
| `-p`, `--port` | auto | Serial port, e.g. `/dev/cu.usbserial-14310` or `/dev/ttyUSB0` |
| `-b`, `--baud` | `460800` | Baud rate. Use `115200` if the connection is unstable |
| `-y`, `--yes` | off | Skip confirmations (for scripts) |
| `--workspace` | `~/oxflasher` | Workspace folder (or `OXFLASHER_HOME`) |
| `--esptool` | auto | esptool command, e.g. `"python3 -m esptool"` |
| `--flash-mode` | `dout` | `dout`, `dio`, `qout`, `qio` or `keep` |
| `--flash-size` | `detect` | `detect`, `keep`, `1MB`, `4MB`... |

Firmware paths can be absolute or relative to the workspace. Exit codes: `0` OK, `1` error, `2` bad usage, `130` cancelled.

## Typical workflows

**Install HAA on a Sonoff (ESP8266, 1MB):**

```bash
oxflasher full haa/fullhaaboot.bin --flash-size 1MB
```

**Install Tasmota:**

```bash
oxflasher full tasmota/tasmota.bin
```

**Restore the original firmware from a backup:**

```bash
oxflasher flash backup/esp8266ex_84f3eb123456_20261009-153012.bin --flash-mode keep
```

> `dout` is the safest flash mode for ESP8266 modules and is what HAA and Tasmota recommend. When restoring a full backup, `--flash-mode keep` keeps the header exactly as it was read.

## Troubleshooting

| Message | Fix |
|---|---|
| `No USB-serial adapter detected` | Check the cable (some are charge-only), the driver, and run `oxflasher ports --all`. |
| `Failed to connect` / `Wrong boot mode` | The chip is not in flash mode: hold GPIO0 to GND while powering it on. Check TX/RX are crossed. |
| `Resource busy` / `Could not open port` | Close any serial monitor (Arduino IDE, `screen`…) using the port, or replug the adapter. |
| `Permission denied` (Linux) | Add your user to the `dialout` group. |
| `Timed out` / `Invalid head of packet` | Use `-b 115200`, a shorter cable, or a better 3.3V supply (the ESP needs ~300mA peaks). |
| `esptool not found` | `pipx install esptool`, or point to it with `--esptool`. |
| `could not detect the flash size` | Some clones report an unknown flash ID; flash with an explicit `--flash-size`. |

## Development

```
cmd/oxflasher/        entry point and CLI sub-commands
internal/app/         operations shared by CLI and menu (backup, erase, flash, full)
internal/esptool/     locating and running esptool, parsing its output
internal/serialport/  USB-serial port detection (macOS / Linux, no cgo)
internal/workspace/   workspace folders, firmware listing, backup naming
internal/ui/          styles (lipgloss) and prompts (huh)
internal/tui/         interactive menu
```

```bash
make test     # unit tests (a fake esptool exercises every workflow, no hardware needed)
make vet      # go vet + gofmt check
make run      # start the menu from source
```

Code comments are in Spanish and explain Go concepts along the way. This project is also a Go learning exercise.

### Migrating from v0.1.x (Python)

The Python script (`src/oxflasher.py`) has been replaced by the Go binary. The last Python version is still available at tag [`v0.1.1`](https://github.com/OxDAbit/oxflasher/tree/v0.1.1). Notable changes:

- **Fixed:** "Flash Device" ran `read_flash` and overwrote your local `.bin` instead of flashing the device.
- Backups now cover the full detected flash size (not a fixed 1MB) and are never silently overwritten.
- Files live in a fixed workspace (`~/oxflasher`) instead of the current directory.

See [CHANGELOG.md](CHANGELOG.md) for details.

## License

[GNU GPL v3](LICENSE). Flashing firmware can brick devices or void warranties. Use at your own risk.

## Contact

- GitHub: [**OxDAbit**](https://github.com/OxDAbit)
- X / Twitter: [**@0xDA_bit**](https://x.com/0xDA_bit)
- Mail: **prevenir.amistad9k@icloud.com**
