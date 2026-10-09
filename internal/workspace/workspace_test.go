package workspace

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBackupName(t *testing.T) {
	ts := time.Date(2026, 10, 9, 15, 30, 12, 0, time.UTC)
	tests := []struct {
		chip, mac, want string
	}{
		{"ESP8266EX", "84:F3:EB:12:34:56", "esp8266ex_84f3eb123456_20261009-153012.bin"},
		{"ESP32-D0WD-V3", "24:6f:28:aa:bb:cc", "esp32-d0wd-v3_246f28aabbcc_20261009-153012.bin"},
		{"", "", "esp_20261009-153012.bin"},
	}
	for _, tt := range tests {
		if got := BackupName(tt.chip, tt.mac, ts); got != tt.want {
			t.Errorf("BackupName(%q, %q) = %q, want %q", tt.chip, tt.mac, got, tt.want)
		}
	}
}

func TestOpenAndFirmwares(t *testing.T) {
	// t.TempDir crea una carpeta temporal que se borra sola al acabar el test.
	root := t.TempDir()
	ws, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"backup", "haa", "tasmota"} {
		if fi, err := os.Stat(filepath.Join(root, d)); err != nil || !fi.IsDir() {
			t.Errorf("folder %s was not created", d)
		}
	}

	write := func(rel string) {
		t.Helper() // si falla, el error apunta a la línea que llamó a write
		if err := os.WriteFile(filepath.Join(root, rel), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("tasmota/tasmota.bin")
	write("haa/fullhaaboot.bin")
	write("backup/esp_1.bin")
	write("tasmota/readme.txt")

	list, err := ws.Firmwares()
	if err != nil {
		t.Fatal(err)
	}
	var rels []string
	for _, f := range list {
		rels = append(rels, f.Rel)
	}
	want := []string{
		filepath.Join("haa", "fullhaaboot.bin"),
		filepath.Join("tasmota", "tasmota.bin"),
		filepath.Join("backup", "esp_1.bin"), // los backups van al final
	}
	if len(rels) != len(want) {
		t.Fatalf("Firmwares() = %v, want %v", rels, want)
	}
	for i := range want {
		if rels[i] != want[i] {
			t.Errorf("Firmwares()[%d] = %q, want %q", i, rels[i], want[i])
		}
	}
}

func TestUniquePath(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "backup.bin")
	if got := UniquePath(p); got != p {
		t.Errorf("UniquePath on missing file = %q, want %q", got, p)
	}
	os.WriteFile(p, nil, 0o644)
	if got, want := UniquePath(p), filepath.Join(dir, "backup-1.bin"); got != want {
		t.Errorf("UniquePath = %q, want %q", got, want)
	}
}

func TestCheckFirmware(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "fw.bin")
	empty := filepath.Join(dir, "empty.bin")
	txt := filepath.Join(dir, "fw.txt")
	os.WriteFile(good, []byte("data"), 0o644)
	os.WriteFile(empty, nil, 0o644)
	os.WriteFile(txt, []byte("data"), 0o644)

	if _, err := CheckFirmware(good); err != nil {
		t.Errorf("valid firmware rejected: %v", err)
	}
	for _, bad := range []string{empty, txt, dir, filepath.Join(dir, "missing.bin")} {
		if _, err := CheckFirmware(bad); err == nil {
			t.Errorf("CheckFirmware(%q) should fail", bad)
		}
	}
}

func TestResolve(t *testing.T) {
	// t.Setenv cambia una variable de entorno solo durante este test.
	t.Setenv(EnvVar, "")
	home, _ := os.UserHomeDir()

	if got, _ := Resolve(""); got != filepath.Join(home, "oxflasher") {
		t.Errorf("default workspace = %q", got)
	}
	t.Setenv(EnvVar, "~/from-env")
	if got, _ := Resolve(""); got != filepath.Join(home, "from-env") {
		t.Errorf("env workspace = %q", got)
	}
	if got, _ := Resolve("~/from-flag"); got != filepath.Join(home, "from-flag") {
		t.Errorf("flag workspace = %q", got)
	}
}
