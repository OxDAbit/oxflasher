package esptool

import (
	"errors"
	"reflect"
	"testing"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		out       string
		wantVer   string
		wantMajor int
	}{
		{"esptool.py v4.7.0\n4.7.0\n", "4.7.0", 4},
		{"esptool v5.0.2\n5.0.2\n", "5.0.2", 5},
		{"esptool.py v3.0\n3.0\n", "3.0", 3},
	}
	for _, tt := range tests {
		ver, major, ok := parseVersion(tt.out)
		if !ok || ver != tt.wantVer || major != tt.wantMajor {
			t.Errorf("parseVersion(%q) = %q, %d, %v", tt.out, ver, major, ok)
		}
	}
	if _, _, ok := parseVersion("command not found"); ok {
		t.Error("parseVersion should fail on output without a version")
	}
}

func TestArgs(t *testing.T) {
	conn := Connection{Port: "/dev/cu.usbserial 1", Baud: 460800}

	v4 := &Tool{command: []string{"esptool.py"}, major: 4}
	got := v4.args(conn, "write_flash", "-fm", "dout", "0x0", "/tmp/my firmware.bin")
	want := []string{"esptool.py", "-p", "/dev/cu.usbserial 1", "-b", "460800", "write_flash", "-fm", "dout", "0x0", "/tmp/my firmware.bin"}
	// reflect.DeepEqual compara slices elemento a elemento (== no funciona con slices).
	if !reflect.DeepEqual(got, want) {
		t.Errorf("v4 args:\n got %q\nwant %q", got, want)
	}

	v5 := &Tool{command: []string{"python3", "-m", "esptool"}, major: 5}
	got = v5.args(Connection{Port: "/dev/ttyUSB0"}, "erase_flash")
	want = []string{"python3", "-m", "esptool", "-p", "/dev/ttyUSB0", "erase-flash"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("v5 args:\n got %q\nwant %q", got, want)
	}
}

func TestParseInfo(t *testing.T) {
	v4 := `esptool.py v4.7.0
Serial port /dev/cu.usbserial-1410
Connecting....
Detecting chip type... ESP8266
Chip is ESP8266EX
Features: WiFi
Crystal is 26MHz
MAC: 84:F3:EB:12:34:56
Uploading stub...
Manufacturer: 5e
Device: 4014
Detected flash size: 1MB
Hard resetting via RTS pin...`

	v5 := `esptool v5.0.2
Connected to ESP32 on /dev/cu.usbserial-0001:
Chip type:          ESP32-D0WD-V3 (revision v3.1)
Features:           Wi-Fi, BT, Dual Core + LP Core, 240MHz
Crystal frequency:  40MHz
MAC:                24:6f:28:aa:bb:cc

Flash Memory Information:
Detected flash size: 4MB`

	tests := []struct {
		name string
		out  string
		want ChipInfo
	}{
		{"v4", v4, ChipInfo{Chip: "ESP8266EX", MAC: "84:f3:eb:12:34:56", FlashSize: 1 << 20}},
		{"v5", v5, ChipInfo{Chip: "ESP32-D0WD-V3", MAC: "24:6f:28:aa:bb:cc", FlashSize: 4 << 20}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseInfo(tt.out)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want { // los structs con campos comparables sí se comparan con ==
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}

	if _, err := ParseInfo("A fatal error occurred: Failed to connect"); err == nil {
		t.Error("ParseInfo should fail when no chip is reported")
	}
}

func TestFormatSize(t *testing.T) {
	for n, want := range map[int64]string{0: "?", 1 << 20: "1MB", 4 << 20: "4MB", 512 << 10: "512KB", 100: "100B"} {
		if got := FormatSize(n); got != want {
			t.Errorf("FormatSize(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestValidate(t *testing.T) {
	if err := ValidateFlashMode("dout"); err != nil {
		t.Error(err)
	}
	if err := ValidateFlashMode("fast"); err == nil {
		t.Error("expected error for invalid flash mode")
	}
	for _, ok := range []string{"detect", "keep", "1MB", "512KB", "16MB"} {
		if err := ValidateFlashSize(ok); err != nil {
			t.Error(err)
		}
	}
	if err := ValidateFlashSize("3MB"); err == nil {
		t.Error("expected error for invalid flash size")
	}
}

func TestCommandErrorHint(t *testing.T) {
	var err error = &CommandError{Subcommand: "flash_id", Output: "A fatal error occurred: Failed to connect to ESP8266: Timed out waiting for packet header"}

	// errors.As busca en la cadena de errores uno del tipo indicado.
	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatal("errors.As should find *CommandError")
	}
	if cmdErr.Hint() == "" {
		t.Error("expected a hint for a connection failure")
	}
}
