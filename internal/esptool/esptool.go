// Package esptool localiza y ejecuta esptool (la herramienta oficial de
// Espressif) para hablar con el ESP: leer información, hacer backup, borrar y
// flashear.
//
// oxflasher NO reimplementa el protocolo del bootloader: delega en esptool,
// que es el estándar que usan HAA, Tasmota, ESPHome, etc.
package esptool

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// Tool es una instalación concreta de esptool encontrada en el sistema.
type Tool struct {
	command []string // Comando base, p. ej. ["esptool"] o ["python3", "-m", "esptool"]
	Version string   // Versión detectada, p. ej. "4.7.0"
	major   int      // Versión mayor; desde la v5 los subcomandos usan guiones

	// Out recibe la salida de los comandos largos (backup, erase, flash) para
	// que el usuario vea las barras de progreso de esptool en tiempo real.
	// io.Writer es una interfaz con un solo método: Write([]byte) (int, error).
	// os.Stdout la cumple, igual que un bytes.Buffer o un fichero.
	Out io.Writer
}

// Connection agrupa los parámetros de conexión que usan todos los comandos.
type Connection struct {
	Port string
	Baud int
}

// ErrNotFound se devuelve cuando no hay esptool instalado.
//
// Declarar errores como variables ("sentinel errors") permite compararlos
// después con errors.Is(err, esptool.ErrNotFound).
var ErrNotFound = errors.New("esptool not found: install it with `pipx install esptool` (or `pip install esptool`)")

// versionRe busca algo con forma de versión: 4.7.0, 5.0, ...
// regexp.MustCompile entra en pánico si la expresión es inválida; se usa
// en variables globales porque el error se detectaría al arrancar.
var versionRe = regexp.MustCompile(`(\d+)\.(\d+)(?:\.(\d+))?`)

// Find busca esptool. Si override no está vacío se usa ese comando (puede
// llevar argumentos, p. ej. "python3 -m esptool"). Si no, prueba en orden:
// esptool (v5+), esptool.py (v4 y anteriores) y python3 -m esptool.
//
// context.Context se pasa como primer parámetro por convención. Sirve para
// cancelar operaciones (p. ej. al pulsar Ctrl+C) y para poner timeouts.
func Find(ctx context.Context, override string) (*Tool, error) {
	candidates := [][]string{{"esptool"}, {"esptool.py"}, {"python3", "-m", "esptool"}}
	if override != "" {
		candidates = [][]string{strings.Fields(override)}
	}

	for _, cmd := range candidates {
		// exec.LookPath busca el ejecutable en el PATH, como `which`.
		if _, err := exec.LookPath(cmd[0]); err != nil {
			continue
		}
		out, err := exec.CommandContext(ctx, cmd[0], append(cmd[1:], "version")...).CombinedOutput()
		if err != nil {
			continue
		}
		version, major, ok := parseVersion(string(out))
		if !ok {
			continue
		}
		return &Tool{command: cmd, Version: version, major: major, Out: os.Stdout}, nil
	}
	return nil, ErrNotFound
}

// parseVersion extrae la versión de la salida de `esptool version`.
// Se toma la ÚLTIMA coincidencia porque esptool imprime primero un banner y
// en la última línea la versión limpia.
func parseVersion(out string) (version string, major int, ok bool) {
	matches := versionRe.FindAllStringSubmatch(out, -1)
	if len(matches) == 0 {
		return "", 0, false
	}
	m := matches[len(matches)-1]
	major, err := strconv.Atoi(m[1])
	if err != nil {
		return "", 0, false
	}
	return m[0], major, true
}

// String describe la herramienta, p. ej. "esptool.py v4.7.0".
func (t *Tool) String() string {
	return fmt.Sprintf("%s v%s", strings.Join(t.command, " "), t.Version)
}

// subcommand adapta el nombre del subcomando a la versión: esptool v5
// renombró read_flash → read-flash, etc. (los antiguos funcionan pero avisan).
//
// (t *Tool) es el "receptor" del método. Al ser un puntero, el método puede
// modificar el Tool y no se copia el struct en cada llamada.
func (t *Tool) subcommand(name string) string {
	if t.major >= 5 {
		return strings.ReplaceAll(name, "_", "-")
	}
	return name
}

// args construye la línea de comandos completa como una LISTA de argumentos.
// Es la gran diferencia con el script Python original, que montaba un string
// para el shell: aquí una ruta con espacios o caracteres raros no rompe nada
// ni puede inyectar comandos, porque no interviene ningún shell.
func (t *Tool) args(conn Connection, sub string, extra ...string) []string {
	argv := append([]string{}, t.command...) // copia para no modificar t.command
	argv = append(argv, "-p", conn.Port)
	if conn.Baud > 0 {
		argv = append(argv, "-b", strconv.Itoa(conn.Baud))
	}
	argv = append(argv, t.subcommand(sub))
	return append(argv, extra...)
}

// CommandError es un tipo de error propio. Cualquier tipo con un método
// Error() string cumple la interfaz `error`.
//
// Guardamos la salida de esptool para poder dar pistas útiles al usuario.
type CommandError struct {
	Subcommand string
	Output     string
	Err        error
}

func (e *CommandError) Error() string {
	return fmt.Sprintf("esptool %s failed: %v", e.Subcommand, e.Err)
}

// Unwrap permite que errors.Is / errors.As "vean" el error original
// (p. ej. context.Canceled si el usuario pulsó Ctrl+C).
func (e *CommandError) Unwrap() error { return e.Err }

// Hint devuelve una sugerencia según los errores típicos de esptool.
func (e *CommandError) Hint() string {
	out := strings.ToLower(e.Output)
	switch {
	case strings.Contains(out, "failed to connect"), strings.Contains(out, "wrong boot mode"):
		return "the chip is not in flash mode: hold GPIO0 to GND while powering it up, then try again"
	case strings.Contains(out, "resource busy"), strings.Contains(out, "could not open port"):
		return "the port is busy or unavailable: close any serial monitor using it, or replug the adapter"
	case strings.Contains(out, "permission denied"):
		return "no permission to open the port (on Linux add your user to the 'dialout' group)"
	case strings.Contains(out, "invalid head of packet"), strings.Contains(out, "timed out"):
		return "unstable connection: retry with a lower baud rate (-b 115200) or a better USB cable"
	}
	return ""
}

// run construye y ejecuta un subcomando de esptool. Si stream es true, la
// salida se muestra en directo además de capturarse.
func (t *Tool) run(ctx context.Context, conn Connection, sub string, stream bool, extra ...string) (string, error) {
	argv := t.args(conn, sub, extra...)

	var buf bytes.Buffer
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	if stream && t.Out != nil {
		// io.MultiWriter duplica la escritura en varios destinos, como `tee`.
		w := io.MultiWriter(t.Out, &buf)
		cmd.Stdout, cmd.Stderr = w, w
	} else {
		cmd.Stdout, cmd.Stderr = &buf, &buf
	}

	// A diferencia de os.system en Python, aquí SÍ comprobamos el resultado:
	// cmd.Run devuelve un error si el proceso termina con código distinto de 0.
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			err = ctx.Err() // cancelado por el usuario: más claro que "signal: interrupt"
		}
		return buf.String(), &CommandError{Subcommand: t.subcommand(sub), Output: buf.String(), Err: err}
	}
	return buf.String(), nil
}
