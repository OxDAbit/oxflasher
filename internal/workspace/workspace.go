// Package workspace gestiona el árbol de carpetas de trabajo de oxflasher:
//
//	<workspace>/
//	├── backup/    backups leídos de los dispositivos
//	├── haa/       binarios de HAA
//	└── tasmota/   binarios de Tasmota
//
// En el script Python las rutas dependían del directorio desde el que se
// lanzara. Ahora el workspace tiene una ubicación fija y predecible.
package workspace

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// EnvVar permite fijar el workspace sin pasar --workspace cada vez.
const EnvVar = "OXFLASHER_HOME"

// Carpetas que se crean automáticamente.
const BackupDir = "backup"

var FirmwareDirs = []string{"haa", "tasmota"}

// Workspace es la carpeta raíz de trabajo.
type Workspace struct {
	Root string
}

// Resolve decide qué carpeta usar, por orden de prioridad:
// flag --workspace > variable OXFLASHER_HOME > ~/oxflasher.
func Resolve(flagValue string) (string, error) {
	switch {
	case flagValue != "":
		return ExpandPath(flagValue)
	case os.Getenv(EnvVar) != "":
		return ExpandPath(os.Getenv(EnvVar))
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot find home directory: %w", err)
	}
	return filepath.Join(home, "oxflasher"), nil
}

// Open prepara el workspace creando las carpetas que falten.
func Open(root string) (*Workspace, error) {
	dirs := append([]string{BackupDir}, FirmwareDirs...)
	for _, d := range dirs {
		// MkdirAll es como `mkdir -p`: no falla si ya existe. 0o755 son los
		// permisos en octal (rwxr-xr-x).
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			// %w "envuelve" el error original: quien llame podrá inspeccionarlo
			// con errors.Is / errors.As, y el mensaje queda encadenado.
			return nil, fmt.Errorf("creating workspace folder: %w", err)
		}
	}
	return &Workspace{Root: root}, nil
}

// BackupPath devuelve la ruta absoluta de la carpeta de backups.
func (w *Workspace) BackupPath() string {
	return filepath.Join(w.Root, BackupDir)
}

// Firmware es un fichero .bin encontrado en el workspace.
type Firmware struct {
	Path  string // ruta absoluta
	Rel   string // ruta relativa al workspace, p. ej. "tasmota/tasmota.bin"
	Group string // carpeta de primer nivel: "haa", "tasmota", "backup"...
	Size  int64
}

// Firmwares recorre el workspace y devuelve todos los .bin, agrupados por
// carpeta. Incluye los backups: flashear un backup es la forma de restaurar.
func (w *Workspace) Firmwares() ([]Firmware, error) {
	var list []Firmware

	// filepath.WalkDir recorre el árbol llamando a esta función anónima
	// (un "closure": puede leer y modificar `list`, declarada fuera).
	err := filepath.WalkDir(w.Root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != w.Root && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir // ignoramos carpetas ocultas (.git...)
			}
			return nil
		}
		if !strings.EqualFold(filepath.Ext(path), ".bin") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(w.Root, path)
		group := "."
		if i := strings.IndexRune(rel, filepath.Separator); i >= 0 {
			group = rel[:i]
		}
		list = append(list, Firmware{Path: path, Rel: rel, Group: group, Size: info.Size()})
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Los backups al final; dentro de cada grupo, orden alfabético.
	sort.Slice(list, func(i, j int) bool {
		bi, bj := list[i].Group == BackupDir, list[j].Group == BackupDir
		if bi != bj {
			return bj
		}
		return list[i].Rel < list[j].Rel
	})
	return list, nil
}

var unsafeChars = regexp.MustCompile(`[^a-z0-9-]+`)

// BackupName genera un nombre único y descriptivo para un backup:
//
//	esp8266ex_84f3eb123456_20261009-153012.bin
//
// Así no se sobrescriben backups (el script original siempre usaba
// "backup.bin") y se sabe de qué dispositivo es cada uno.
func BackupName(chip, mac string, t time.Time) string {
	chip = unsafeChars.ReplaceAllString(strings.ToLower(chip), "")
	if chip == "" {
		chip = "esp"
	}
	mac = strings.ReplaceAll(strings.ToLower(mac), ":", "")
	parts := []string{chip}
	if mac != "" {
		parts = append(parts, mac)
	}
	// Go formatea fechas con una fecha de referencia fija:
	// "Mon Jan 2 15:04:05 MST 2006" (1-2-3-4-5-6). Es peculiar, pero se aprende rápido.
	parts = append(parts, t.Format("20060102-150405"))
	return strings.Join(parts, "_") + ".bin"
}

// UniquePath devuelve path si no existe; si existe, añade -1, -2... antes de
// la extensión para no sobrescribir nunca un fichero.
func UniquePath(path string) string {
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		return path
	}
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s-%d%s", base, i, ext)
		if _, err := os.Stat(candidate); errors.Is(err, fs.ErrNotExist) {
			return candidate
		}
	}
}

// ExpandPath expande "~" y convierte la ruta en absoluta.
func ExpandPath(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		p = filepath.Join(home, strings.TrimPrefix(p, "~"))
	}
	return filepath.Abs(p)
}

// CheckFirmware valida que path sea un fichero .bin existente y no vacío.
// El script original solo comprobaba que existiera la CARPETA.
func CheckFirmware(path string) (fs.FileInfo, error) {
	info, err := os.Stat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, fmt.Errorf("firmware file not found: %s", path)
	case err != nil:
		return nil, err
	case info.IsDir():
		return nil, fmt.Errorf("%s is a directory, not a firmware file", path)
	case info.Size() == 0:
		return nil, fmt.Errorf("firmware file is empty: %s", path)
	case !strings.EqualFold(filepath.Ext(path), ".bin"):
		return nil, fmt.Errorf("firmware file must have .bin extension: %s", path)
	}
	return info, nil
}
