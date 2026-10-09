// Package ui agrupa todo lo visual: colores, banner, mensajes y preguntas.
//
// Usa lipgloss (estilos de terminal) y huh (formularios interactivos), ambos
// de Charm. Sustituyen a los códigos ANSI escritos a mano y a PyInquirer.
package ui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// Paleta. lipgloss.AdaptiveColor elige un color distinto según el fondo del
// terminal sea claro u oscuro.
var (
	blue   = lipgloss.AdaptiveColor{Light: "#1F5FBF", Dark: "#5FA8FF"}
	yellow = lipgloss.AdaptiveColor{Light: "#946200", Dark: "#F5C542"}
	green  = lipgloss.AdaptiveColor{Light: "#1E7B34", Dark: "#5FD068"}
	red    = lipgloss.AdaptiveColor{Light: "#B3261E", Dark: "#FF6B6B"}
	orange = lipgloss.AdaptiveColor{Light: "#A04A00", Dark: "#FF9F43"}
	grey   = lipgloss.AdaptiveColor{Light: "#666666", Dark: "#9A9A9A"}

	bannerStyle = lipgloss.NewStyle().Foreground(blue).Bold(true)
	authorStyle = lipgloss.NewStyle().Foreground(yellow)
	stepStyle   = lipgloss.NewStyle().Foreground(blue).Bold(true)
	okStyle     = lipgloss.NewStyle().Foreground(green).Bold(true)
	errStyle    = lipgloss.NewStyle().Foreground(red).Bold(true)
	warnStyle   = lipgloss.NewStyle().Foreground(orange)
	dimStyle    = lipgloss.NewStyle().Foreground(grey)
	keyStyle    = lipgloss.NewStyle().Foreground(grey).Width(12)
	valueStyle  = lipgloss.NewStyle().Foreground(yellow)
	boxStyle    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(blue).Padding(0, 1)
)

// Out es donde se escriben los mensajes. Es una variable de paquete para que
// los tests puedan redirigirla a un buffer.
var Out io.Writer = os.Stdout

const banner = `▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄
█▀▄▄▀█░█░█░▄▄█░██░▄▄▀█░▄▄█░████░▄▄█░▄▄▀
█░██░█▀▄▀█░▄██░██░▀▀░█▄▄▀█░▄▄░█░▄▄█░▀▀▄
██▄▄██▄█▄█▄███▄▄█▄██▄█▄▄▄█▄██▄█▄▄▄█▄█▄▄
▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀`

// Banner imprime la cabecera del menú interactivo.
func Banner(version string) {
	fmt.Fprintln(Out)
	fmt.Fprintln(Out, bannerStyle.Render(banner))
	fmt.Fprintf(Out, "%s  %s\n\n", authorStyle.Render("by 0XDΛ_BIƬ"), dimStyle.Render(version))
}

// ClearScreen limpia la pantalla con una secuencia ANSI (sin lanzar `clear`).
func ClearScreen() {
	fmt.Fprint(Out, "\033[H\033[2J")
}

// Las funciones con `args ...any` son "variádicas": aceptan cualquier número
// de argumentos, que llegan como un slice. `any` es un alias de interface{},
// el tipo que admite cualquier valor. Luego se reenvían a Sprintf con args...

// Step anuncia el comienzo de una operación.
func Step(format string, args ...any) {
	fmt.Fprintln(Out, stepStyle.Render("▸ "+fmt.Sprintf(format, args...)))
}

// Success informa de que algo terminó bien.
func Success(format string, args ...any) {
	fmt.Fprintln(Out, okStyle.Render("✔ "+fmt.Sprintf(format, args...)))
}

// Warn muestra un aviso.
func Warn(format string, args ...any) {
	fmt.Fprintln(Out, warnStyle.Render("! "+fmt.Sprintf(format, args...)))
}

// Error muestra un error y, si se conoce, una pista para resolverlo.
func Error(err error, hint string) {
	fmt.Fprintln(Out, errStyle.Render("✘ "+err.Error()))
	if hint != "" {
		fmt.Fprintln(Out, warnStyle.Render("  hint: "+hint))
	}
}

// Dim imprime texto secundario en gris.
func Dim(format string, args ...any) {
	fmt.Fprintln(Out, dimStyle.Render(fmt.Sprintf(format, args...)))
}

// KV es un par clave/valor para mostrar en un recuadro.
type KV struct{ Key, Value string }

// Box muestra una lista de pares clave/valor dentro de un recuadro.
func Box(title string, rows ...KV) {
	var b strings.Builder // strings.Builder concatena strings de forma eficiente
	b.WriteString(stepStyle.Render(title))
	for _, r := range rows {
		b.WriteString("\n" + keyStyle.Render(r.Key) + valueStyle.Render(r.Value))
	}
	fmt.Fprintln(Out, boxStyle.Render(b.String()))
}

// ErrAborted indica que el usuario canceló (Esc o Ctrl+C en un formulario).
var ErrAborted = errors.New("aborted by user")

// theme es el tema de huh para todos los formularios.
var theme = huh.ThemeBase16()

// run ejecuta un formulario de un solo campo y traduce el error de huh al
// nuestro, para que el resto del programa no dependa de huh.
func run(ctx context.Context, field huh.Field) error {
	err := huh.NewForm(huh.NewGroup(field)).WithTheme(theme).RunWithContext(ctx)
	if errors.Is(err, huh.ErrUserAborted) || errors.Is(err, context.Canceled) {
		return ErrAborted
	}
	return err
}

// Confirm pregunta sí/no. La respuesta por defecto es NO: en acciones
// destructivas (borrar, flashear) es lo seguro. El script original tenía "Sí".
func Confirm(ctx context.Context, title, description string) (bool, error) {
	var ok bool // el "valor cero" de un bool es false → opción por defecto: No
	err := run(ctx, huh.NewConfirm().
		Title(title).
		Description(description).
		Affirmative("Yes").
		Negative("No").
		Value(&ok)) // pasamos un PUNTERO para que huh pueda escribir la respuesta
	return ok, err
}

// Option es una opción de un menú de selección. Es genérico: T puede ser
// cualquier tipo comparable (string, int, un struct...). Los genéricos
// llegaron en Go 1.18.
type Option[T comparable] struct {
	Label string
	Value T
}

// Select muestra una lista para elegir con las flechas.
// Sustituye a "escribe el índice del puerto" del script original.
func Select[T comparable](ctx context.Context, title string, options []Option[T]) (T, error) {
	// huh coloca el cursor en la opción cuyo valor coincide con `selected`.
	// Si lo dejáramos con su valor cero, el cursor podría empezar en mitad de
	// la lista; partimos siempre de la primera opción.
	var selected T
	if len(options) > 0 {
		selected = options[0].Value
	}
	opts := make([]huh.Option[T], len(options))
	for i, o := range options {
		opts[i] = huh.NewOption(o.Label, o.Value)
	}
	err := run(ctx, huh.NewSelect[T]().Title(title).Options(opts...).Value(&selected))
	return selected, err
}

// Input pide un texto, con valor inicial y una función de validación opcional.
// validate es un parámetro de tipo función: en Go las funciones son valores.
func Input(ctx context.Context, title, initial string, validate func(string) error) (string, error) {
	value := initial
	field := huh.NewInput().Title(title).Value(&value)
	if validate != nil {
		field = field.Validate(validate)
	}
	err := run(ctx, field)
	return strings.TrimSpace(value), err
}

// IsInteractive indica si la entrada estándar es un terminal (y no, por
// ejemplo, una tubería en un script). Sin terminal no podemos preguntar nada.
func IsInteractive() bool {
	fi, err := os.Stdin.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}
