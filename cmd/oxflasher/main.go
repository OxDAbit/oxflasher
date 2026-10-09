// Comando oxflasher: backup, borrado y flasheo de dispositivos ESP
// (ESP8266/ESP32) con firmwares como HAA o Tasmota, usando esptool.
//
// Por convención, los ejecutables de un proyecto Go viven en cmd/<nombre>/ y
// la lógica reutilizable en internal/ (que Go impide importar desde fuera
// de este módulo).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/OxDAbit/oxflasher/internal/app"
	"github.com/OxDAbit/oxflasher/internal/esptool"
	"github.com/OxDAbit/oxflasher/internal/serialport"
	"github.com/OxDAbit/oxflasher/internal/tui"
	"github.com/OxDAbit/oxflasher/internal/ui"
	"github.com/OxDAbit/oxflasher/internal/workspace"
)

// Estas variables se rellenan al compilar con:
//
//	go build -ldflags "-X main.version=v0.2.0 -X main.commit=abc123"
//
// Es la forma estándar de "grabar" la versión en el binario (lo usaremos al
// publicar releases en GitHub).
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

const usage = `oxflasher — backup, erase and flash ESP devices (HAA, Tasmota...) using esptool

Usage:
  oxflasher                        interactive menu
  oxflasher ports  [--all]         list serial ports
  oxflasher info   [flags]         show chip, MAC and flash size
  oxflasher backup [flags] [-o FILE|DIR]
  oxflasher erase  [flags]
  oxflasher flash  [flags] FILE.bin
  oxflasher full   [flags] FILE.bin   backup → erase → flash
  oxflasher version

Flags (for info, backup, erase, flash, full and the menu):
  -p, --port PORT        serial port (auto-detected if omitted)
  -b, --baud N           baud rate (default 460800; use 115200 if unstable)
  -y, --yes              do not ask for confirmation
      --workspace DIR    workspace folder (default $OXFLASHER_HOME or ~/oxflasher)
      --esptool CMD      esptool command (default: auto-detect)
      --flash-mode MODE  dout, dio, qout, qio, keep (default dout)
      --flash-size SIZE  detect, keep, 1MB, 4MB... (default detect)

FILE.bin may be an absolute path or a path relative to the workspace,
e.g. "tasmota/tasmota.bin".
`

// options guarda los valores de todas las flags.
type options struct {
	port, workspace, esptool string
	baud                     int
	yes, all                 bool
	output                   string
	write                    esptool.WriteOptions
}

func main() {
	// os.Exit no ejecuta los `defer` pendientes, por eso toda la lógica está
	// en run() y main solo traduce su resultado a código de salida.
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	// NotifyContext devuelve un contexto que se cancela al pulsar Ctrl+C.
	// Ese contexto llega hasta exec.CommandContext, que detiene esptool.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	// defer ejecuta stop() justo antes de que run() termine, pase lo que pase.
	defer stop()

	cmd := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd, args = args[0], args[1:]
	}

	var err error
	switch cmd {
	case "", "menu":
		err = cmdMenu(ctx, args)
	case "ports":
		err = cmdPorts(args)
	case "info", "backup", "erase", "flash", "full":
		err = cmdDevice(ctx, cmd, args)
	case "version", "--version":
		cmdVersion(ctx)
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", cmd, usage)
		return 2
	}

	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		if app.Report(err) {
			return 130 // código habitual para "interrumpido por el usuario"
		}
		return 1
	}
	return 0
}

// newFlagSet registra las flags comunes. Cada flag se define dos veces
// (corta y larga) apuntando a la MISMA variable, porque el paquete estándar
// `flag` no tiene alias.
func newFlagSet(name string, o *options) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard) // mostramos nuestra propia ayuda
	fs.Usage = func() { fmt.Print(usage) }

	fs.StringVar(&o.port, "p", "", "")
	fs.StringVar(&o.port, "port", "", "")
	fs.IntVar(&o.baud, "b", app.DefaultBaud, "")
	fs.IntVar(&o.baud, "baud", app.DefaultBaud, "")
	fs.BoolVar(&o.yes, "y", false, "")
	fs.BoolVar(&o.yes, "yes", false, "")
	fs.StringVar(&o.workspace, "workspace", "", "")
	fs.StringVar(&o.esptool, "esptool", "", "")
	fs.StringVar(&o.write.FlashMode, "flash-mode", esptool.DefaultWriteOptions.FlashMode, "")
	fs.StringVar(&o.write.FlashSize, "flash-size", esptool.DefaultWriteOptions.FlashSize, "")
	fs.StringVar(&o.output, "o", "", "")
	fs.StringVar(&o.output, "output", "", "")
	return fs
}

// parse admite flags antes y después de los argumentos posicionales
// (`flash fw.bin -y` y `flash -y fw.bin`). El paquete flag se detiene en el
// primer argumento que no es una flag, así que vamos parseando por tramos.
func parse(fs *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	for {
		if err := fs.Parse(args); err != nil {
			if !errors.Is(err, flag.ErrHelp) {
				fmt.Fprintf(os.Stderr, "%v\n\n%s", err, usage)
			}
			return nil, err
		}
		if fs.NArg() == 0 {
			return positional, nil
		}
		positional = append(positional, fs.Arg(0))
		args = fs.Args()[1:]
	}
}

// newApp construye la App a partir de las opciones.
func newApp(ctx context.Context, o *options) (*app.App, error) {
	if err := esptool.ValidateFlashMode(o.write.FlashMode); err != nil {
		return nil, err
	}
	if err := esptool.ValidateFlashSize(o.write.FlashSize); err != nil {
		return nil, err
	}
	root, err := workspace.Resolve(o.workspace)
	if err != nil {
		return nil, err
	}
	ws, err := workspace.Open(root)
	if err != nil {
		return nil, err
	}
	tool, err := esptool.Find(ctx, o.esptool)
	if err != nil {
		return nil, err
	}
	return &app.App{Tool: tool, WS: ws, Baud: o.baud, Write: o.write, AssumeYes: o.yes}, nil
}

func cmdMenu(ctx context.Context, args []string) error {
	var o options
	if _, err := parse(newFlagSet("menu", &o), args); err != nil {
		return err
	}
	if !ui.IsInteractive() {
		return errors.New("the interactive menu needs a terminal; use a subcommand instead (see `oxflasher help`)")
	}
	a, err := newApp(ctx, &o)
	if err != nil {
		return err
	}
	err = tui.Run(ctx, a, version)
	if errors.Is(err, ui.ErrAborted) {
		return nil // salir con Esc en el menú principal no es un error
	}
	return err
}

func cmdPorts(args []string) error {
	var o options
	fs := newFlagSet("ports", &o)
	fs.BoolVar(&o.all, "all", false, "")
	fs.BoolVar(&o.all, "a", false, "")
	if _, err := parse(fs, args); err != nil {
		return err
	}
	ports, err := serialport.List(o.all)
	if err != nil {
		return err
	}
	if len(ports) == 0 {
		ui.Warn("No USB-serial adapter detected (try --all)")
		return nil
	}
	for _, p := range ports {
		fmt.Println(p) // Println usa el método String() de Port
	}
	return nil
}

func cmdDevice(ctx context.Context, cmd string, args []string) error {
	var o options
	positional, err := parse(newFlagSet(cmd, &o), args)
	if err != nil {
		return err
	}

	needsFile := cmd == "flash" || cmd == "full"
	if needsFile && len(positional) != 1 {
		return fmt.Errorf("%s needs exactly one firmware file, e.g. `oxflasher %s tasmota/tasmota.bin`", cmd, cmd)
	}
	if !needsFile && len(positional) > 0 {
		return fmt.Errorf("unexpected argument %q", positional[0])
	}

	a, err := newApp(ctx, &o)
	if err != nil {
		return err
	}

	// Validamos el firmware ANTES de conectar: falla rápido si hay una errata.
	var file string
	if needsFile {
		if file, err = firmwarePath(a.WS, positional[0]); err != nil {
			return err
		}
	}

	port, err := a.ResolvePort(ctx, o.port)
	if err != nil {
		return err
	}
	dev, err := a.Connect(ctx, port)
	if err != nil {
		return err
	}

	switch cmd {
	case "backup":
		dest, err := a.BackupTarget(dev, o.output)
		if err != nil {
			return err
		}
		_, err = a.Backup(ctx, dev, dest)
		return err
	case "erase":
		return a.Erase(ctx, dev)
	case "flash":
		return a.Flash(ctx, dev, file)
	case "full":
		return a.Full(ctx, dev, file)
	}
	return nil // "info": Connect ya mostró la información
}

// firmwarePath busca el fichero tal cual y, si no existe, relativo al
// workspace (así funciona `oxflasher flash tasmota/tasmota.bin` desde
// cualquier carpeta).
func firmwarePath(ws *workspace.Workspace, arg string) (string, error) {
	p, err := workspace.ExpandPath(arg)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(p); err != nil && !filepath.IsAbs(arg) && !strings.HasPrefix(arg, "~") {
		if candidate := filepath.Join(ws.Root, arg); fileExists(candidate) {
			p = candidate
		}
	}
	if _, err := workspace.CheckFirmware(p); err != nil {
		return "", err
	}
	return p, nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func cmdVersion(ctx context.Context) {
	fmt.Printf("oxflasher %s (commit %s, built %s)\n", version, commit, date)
	if tool, err := esptool.Find(ctx, ""); err == nil {
		fmt.Printf("using %s\n", tool)
	} else {
		fmt.Println(err)
	}
}
