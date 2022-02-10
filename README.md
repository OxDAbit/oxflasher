# oxflasher

Manage ESP devices to backup, erase or flash with firmwares like Tasmota, HAA,... using `esptool.py`

# Application

## Installation

1. Download github repository or just a [**oxflasher.py**](https://github.com/OxDAbit/oxflasher/blob/main/src/oxflasher.py) script

	```
	git clone https://github.com/OxDAbit/oxflasher
	```

2. Python packages required for `oxflasher.py`:

	```
	pip install PyInquirer
	```

---
## Script execution

First of all you need to locate your backup path and binary location.

```
├── LICENSE
├── README.md
├── backup
│   └── sonoff_basic.bin
├── haa
│   └── fullhaaboot.bin
├── src
│   └── oxflasher.py
└── tasmota
	└── tasmota.bin
```

This is the structure that I've used where:
- backup -> Content backups from different devices
- tasmota -> Content tasmota binaries
- haa -> Content HAA binaries

---

## Run `oxflasher.py`
```
$ python3 oxflasher.py
```

At the beginning, the script will display the USB ports detected.

![first_menu](https://github.com/OxDAbit/oxflasher/tree/main/images/01-first_menu.png)

Conenct your device and press `Refresh Ports` option

![refresh_ports](https://github.com/OxDAbit/oxflasher/tree/main/images/02-refresh_ports.png)

Your device should appears in list **Ports detected**
Press the option `Connect to Port` and then add the index of the USB port where your device is connected (Ex: Port nº 2 -> /dev/tty.usbserial-14310)

![connect_port](https://github.com/OxDAbit/oxflasher/tree/main/images/03-connect_port.png)

At this point you can create a backup of your device, erase your device or flash it with the desired binary.

![create_backup](https://github.com/OxDAbit/oxflasher/tree/main/images/04-create_backup.png)

### Create backup

Script will ask you for:
- Path where to save the backup file
- Name of your backup file

By default the backup path will be the path where script `oxflasher.py` are executed and the default name will be **backup.bin**

You can leave fields empty and the default parameters will be selected.

After that, the script will show you the backup information before ask you if you want continue.

![backup_menu](https://github.com/OxDAbit/oxflasher/tree/main/images/05-backup_menu.png)

### Erase device

Script will show you the USB port selected for erase process before ask you if you want continue.

![erase_menu](https://github.com/OxDAbit/oxflasher/tree/main/images/06-erase_menu.png)

### Flash device

Script will ask you for:
- Path where to load the binary file
- Name of binary file

By default the backup path will be the path where script `oxflasher.py` are executed and the default name will be **fullhaaboot.bin** (HAA binary)

You can leave fields empty and the default parameters will be selected.

After that, the script will show you the backup information before ask you if you want continue.

![flash_menu](https://github.com/OxDAbit/oxflasher/tree/main/images/07-flash_menu.png)

Contact
=======
- Twitter. [**@0xDA_bit**](https://twitter.com/0xDA_bit)
- Github. [**OxDAbit**](https://github.com/OxDAbit)
- Mail. **oxdabit@protonmail.com**