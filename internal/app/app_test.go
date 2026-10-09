package app

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OxDAbit/oxflasher/internal/esptool"
	"github.com/OxDAbit/oxflasher/internal/ui"
	"github.com/OxDAbit/oxflasher/internal/workspace"
)

// fakeEsptool es un script de shell que imita a esptool v4. Nos permite probar
// todos los flujos sin tener un ESP conectado. Guarda en un log cada llamada
// para que el test pueda comprobar qué se ejecutó.
const fakeEsptool = `#!/bin/sh
echo "$@" >> "$FAKE_LOG"
for last; do :; done   # deja en $last el último argumento
case "$*" in
  version) echo "esptool.py v4.7.0"; echo "4.7.0" ;;
  *flash_id*) printf 'Chip is ESP8266EX\nMAC: 84:f3:eb:12:34:56\nDetected flash size: 1MB\n' ;;
  *read_flash*) head -c 1048576 /dev/zero > "$last" ;;
  *erase_flash*) echo "Chip erase completed successfully" ;;
  *write_flash*) [ -n "$FAKE_FAIL_WRITE" ] && { echo "A fatal error occurred: Failed to connect"; exit 2; }; echo "Hash of data verified." ;;
  *) exit 1 ;;
esac
`

// newTestApp prepara un App con el esptool falso y un workspace temporal.
func newTestApp(t *testing.T) (*App, string) {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "esptool.py")
	if err := os.WriteFile(script, []byte(fakeEsptool), 0o755); err != nil {
		t.Fatal(err)
	}
	logFile := filepath.Join(dir, "calls.log")
	t.Setenv("FAKE_LOG", logFile)

	tool, err := esptool.Find(context.Background(), script)
	if err != nil {
		t.Fatal(err)
	}
	tool.Out = &bytes.Buffer{} // silenciamos la salida de esptool
	ui.Out = &bytes.Buffer{}   // y la de la interfaz

	ws, err := workspace.Open(filepath.Join(dir, "ws"))
	if err != nil {
		t.Fatal(err)
	}
	a := &App{
		Tool:    tool,
		WS:      ws,
		Baud:    DefaultBaud,
		Write:   esptool.DefaultWriteOptions,
		Confirm: func(context.Context, string, string) (bool, error) { return true, nil },
	}
	return a, logFile
}

func readLog(t *testing.T, path string) string {
	t.Helper()
	b, _ := os.ReadFile(path)
	return string(b)
}

func TestFullWorkflow(t *testing.T) {
	a, logFile := newTestApp(t)
	ctx := context.Background()

	fw := filepath.Join(a.WS.Root, "tasmota", "tasmota.bin")
	os.WriteFile(fw, []byte("firmware"), 0o644)

	dev, err := a.Connect(ctx, "/dev/cu.usbserial-1")
	if err != nil {
		t.Fatal(err)
	}
	if dev.Info.FlashSize != 1<<20 {
		t.Fatalf("flash size = %d", dev.Info.FlashSize)
	}

	if err := a.Full(ctx, dev, fw); err != nil {
		t.Fatal(err)
	}

	calls := readLog(t, logFile)
	// Comprobamos el orden: identificar → backup → borrar → flashear.
	order := []string{"flash_id", "read_flash 0 0x100000", "erase_flash", "write_flash -fm dout -fs detect 0x0 " + fw}
	pos := 0
	for _, want := range order {
		i := strings.Index(calls[pos:], want)
		if i < 0 {
			t.Fatalf("expected %q after position %d in calls:\n%s", want, pos, calls)
		}
		pos += i
	}

	backups, _ := filepath.Glob(filepath.Join(a.WS.BackupPath(), "esp8266ex_84f3eb123456_*.bin"))
	if len(backups) != 1 {
		t.Fatalf("expected one backup, found %v", backups)
	}
}

func TestFlashChecksFirmwareBeforeConfirming(t *testing.T) {
	a, _ := newTestApp(t)
	asked := false
	a.Confirm = func(context.Context, string, string) (bool, error) { asked = true; return true, nil }

	dev := &Device{Conn: esptool.Connection{Port: "p"}, Info: esptool.ChipInfo{Chip: "ESP8266EX", FlashSize: 4}}
	big := filepath.Join(a.WS.Root, "haa", "big.bin")
	os.WriteFile(big, []byte("more than four bytes"), 0o644)

	if err := a.Flash(context.Background(), dev, big); err == nil || !strings.Contains(err.Error(), "does not fit") {
		t.Fatalf("expected 'does not fit' error, got %v", err)
	}
	if asked {
		t.Error("should not ask for confirmation when the firmware is invalid")
	}
}

func TestCancelledConfirmation(t *testing.T) {
	a, logFile := newTestApp(t)
	a.Confirm = func(context.Context, string, string) (bool, error) { return false, nil }
	dev := &Device{Conn: esptool.Connection{Port: "p"}, Info: esptool.ChipInfo{Chip: "ESP8266EX", FlashSize: 1 << 20}}

	if err := a.Erase(context.Background(), dev); !errors.Is(err, ErrCancelled) {
		t.Fatalf("expected ErrCancelled, got %v", err)
	}
	if strings.Contains(readLog(t, logFile), "erase_flash") {
		t.Error("erase_flash must not run after the user says no")
	}
}

func TestFlashFailureReturnsCommandErrorWithHint(t *testing.T) {
	a, _ := newTestApp(t)
	t.Setenv("FAKE_FAIL_WRITE", "1")
	fw := filepath.Join(a.WS.Root, "haa", "fullhaaboot.bin")
	os.WriteFile(fw, []byte("fw"), 0o644)
	dev := &Device{Conn: esptool.Connection{Port: "p"}, Info: esptool.ChipInfo{Chip: "ESP8266EX", FlashSize: 1 << 20}}

	err := a.Flash(context.Background(), dev, fw)
	var cmdErr *esptool.CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected *esptool.CommandError, got %v", err)
	}
	if cmdErr.Hint() == "" {
		t.Error("expected a hint for 'Failed to connect'")
	}
}

func TestBackupTarget(t *testing.T) {
	a, _ := newTestApp(t)
	dev := &Device{Info: esptool.ChipInfo{Chip: "ESP8266EX", MAC: "84:f3:eb:12:34:56"}}

	got, _ := a.BackupTarget(dev, "sonoff_basic")
	if want := filepath.Join(a.WS.BackupPath(), "sonoff_basic.bin"); got != want {
		t.Errorf("plain name → %q, want %q", got, want)
	}

	got, _ = a.BackupTarget(dev, "")
	if filepath.Dir(got) != a.WS.BackupPath() || !strings.HasPrefix(filepath.Base(got), "esp8266ex_84f3eb123456_") {
		t.Errorf("empty input → %q", got)
	}

	dir := t.TempDir()
	got, _ = a.BackupTarget(dev, dir)
	if filepath.Dir(got) != dir {
		t.Errorf("directory input → %q, want a file inside %q", got, dir)
	}
}
