package serialport

import (
	"path/filepath"
	"strings"
)

// Este fichero solo se compila en macOS porque su nombre termina en
// _darwin.go ("darwin" es el nombre interno de macOS en Go: GOOS=darwin).
//
// En macOS cada puerto aparece dos veces: /dev/tty.* y /dev/cu.*.
// Para INICIAR una conexión (lo que hace esptool) lo correcto es /dev/cu.*:
// /dev/tty.* espera a la señal DCD y puede quedarse bloqueado.
func listCandidates() ([]Port, error) {
	paths, err := filepath.Glob("/dev/cu.*")
	if err != nil {
		return nil, err
	}

	ports := make([]Port, 0, len(paths))
	for _, path := range paths {
		desc, usb := describe(path)
		if !usb {
			desc = guessNonUSB(path)
		}
		ports = append(ports, Port{Path: path, Description: desc, USB: usb})
	}
	return ports, nil
}

// guessNonUSB etiqueta los puertos que no nos interesan, para que en el modo
// --all se entienda por qué se descartan.
func guessNonUSB(path string) string {
	name := strings.ToLower(filepath.Base(path))
	switch {
	case strings.Contains(name, "bluetooth"), strings.Contains(name, "blth"):
		return "Bluetooth"
	case strings.Contains(name, "debug-console"):
		return "macOS debug console"
	default:
		return "not a USB-serial adapter?"
	}
}
