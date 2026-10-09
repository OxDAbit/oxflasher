//go:build !darwin && !linux

package serialport

import "errors"

// La línea `//go:build` de arriba es una "build constraint": este fichero solo
// se compila cuando el sistema NO es macOS ni Linux (p. ej. Windows).
// Así el programa compila en cualquier plataforma; en las no soportadas la
// detección automática falla con un error claro y el usuario puede indicar el
// puerto a mano con -p COM3.
func listCandidates() ([]Port, error) {
	return nil, errors.New("automatic port detection is not supported on this OS; pass the port with -p")
}
