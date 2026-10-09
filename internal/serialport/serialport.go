// Package serialport detecta los puertos serie a los que puede estar
// conectado un ESP.
//
// En Go cada carpeta es un "paquete". El nombre que aparece tras `package`
// es el que usarán otros paquetes al importarlo: serialport.List(...).
// Todo lo que empieza por MAYÚSCULA (List, Port) es público ("exportado");
// lo que empieza por minúscula (describe, usbHints) es privado al paquete.
package serialport

import (
	"path/filepath"
	"sort"
	"strings"
)

// Port representa un puerto serie detectado.
//
// Los structs son la forma de agrupar datos en Go (no hay clases).
// Los métodos se definen aparte, asociados al tipo (ver String más abajo).
type Port struct {
	Path        string // Ruta del dispositivo, p. ej. /dev/cu.usbserial-1410
	Description string // Texto legible, p. ej. "CH340 USB-serial"
	USB         bool   // true si parece un adaptador USB-serie (los que nos interesan)
}

// String hace que Port implemente la interfaz fmt.Stringer.
//
// En Go las interfaces se cumplen de forma IMPLÍCITA: no hay "implements".
// Cualquier tipo con un método `String() string` es un fmt.Stringer, y
// fmt.Println lo usará automáticamente para imprimirlo.
func (p Port) String() string {
	if p.Description == "" {
		return p.Path
	}
	return p.Path + "  (" + p.Description + ")"
}

// usbHint asocia un fragmento del nombre del dispositivo con el chip USB-serie
// que suele haber detrás. Es un slice de structs anónimos: útil para pequeñas
// tablas de datos sin necesidad de declarar un tipo con nombre.
var usbHints = []struct {
	fragment    string
	description string
}{
	// macOS
	{"wchusbserial", "CH34x USB-serial"},
	{"slab_usbtouart", "CP210x USB-serial"},
	{"usbserial", "USB-serial (CH340 / CP210x / FTDI)"},
	{"usbmodem", "USB CDC (ESP32-S2/S3/C3 native USB)"},
	// Linux
	{"ttyusb", "USB-serial"},
	{"ttyacm", "USB CDC"},
}

// describe intenta adivinar, solo por el nombre, si un puerto es USB-serie.
// Devuelve dos valores: Go permite múltiples valores de retorno y es muy
// idiomático usarlos (sobre todo el patrón `valor, err`).
func describe(path string) (description string, usb bool) {
	name := strings.ToLower(filepath.Base(path))
	for _, h := range usbHints {
		if strings.Contains(name, h.fragment) {
			return h.description, true
		}
	}
	return "", false
}

// filter aplica el criterio de "solo USB" (salvo que all sea true) y ordena.
func filter(ports []Port, all bool) []Port {
	// Truco idiomático: reutilizar el array subyacente del slice para filtrar
	// sin reservar memoria nueva. ports[:0] es un slice vacío que comparte
	// memoria con ports.
	out := ports[:0]
	for _, p := range ports {
		if all || p.USB {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// List devuelve los puertos serie detectados. Con all=false solo devuelve los
// que parecen adaptadores USB-serie (descarta Bluetooth, debug-console, etc.).
//
// La función listCandidates está implementada en ficheros distintos según el
// sistema operativo (list_darwin.go, list_linux.go, list_other.go). El
// compilador elige el correcto gracias al sufijo _GOOS del nombre de fichero.
func List(all bool) ([]Port, error) {
	ports, err := listCandidates()
	if err != nil {
		return nil, err
	}
	return filter(ports, all), nil
}
