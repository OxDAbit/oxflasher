package serialport

import (
	"os"
	"path/filepath"
	"strings"
)

// En Linux los adaptadores USB-serie aparecen como /dev/ttyUSB* (CH340, CP210x,
// FTDI) o /dev/ttyACM* (USB nativo de ESP32-S2/S3/C3).
//
// Además, el kernel expone información del dispositivo USB en /sys, así que
// podemos leer el fabricante y el producto sin librerías en C (sin cgo), lo
// que mantiene la compilación cruzada sencilla.
func listCandidates() ([]Port, error) {
	var paths []string
	for _, pattern := range []string{"/dev/ttyUSB*", "/dev/ttyACM*"} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, err
		}
		// El operador ... "expande" un slice como lista de argumentos.
		paths = append(paths, matches...)
	}

	ports := make([]Port, 0, len(paths))
	for _, path := range paths {
		desc, _ := describe(path)
		if product := sysfsProduct(filepath.Base(path)); product != "" {
			desc = product
		}
		ports = append(ports, Port{Path: path, Description: desc, USB: true})
	}
	return ports, nil
}

// sysfsProduct lee /sys/class/tty/<name>/device/../{manufacturer,product}.
// Si algo falla devuelve "" — aquí un error no es grave, solo perdemos la
// descripción bonita, así que lo ignoramos a propósito.
func sysfsProduct(name string) string {
	base := filepath.Join("/sys/class/tty", name, "device")
	// ttyUSB cuelga un nivel más abajo que ttyACM en el árbol de sysfs.
	for _, dir := range []string{filepath.Join(base, ".."), filepath.Join(base, "..", "..")} {
		product := readTrim(filepath.Join(dir, "product"))
		if product == "" {
			continue
		}
		if m := readTrim(filepath.Join(dir, "manufacturer")); m != "" {
			return m + " " + product
		}
		return product
	}
	return ""
}

func readTrim(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}
