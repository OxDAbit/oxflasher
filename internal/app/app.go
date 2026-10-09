// Package app contiene la lógica de las operaciones (info, backup, erase,
// flash y full). La usan tanto los subcomandos del CLI como el menú
// interactivo, así no se duplica código (en el script Python, backup y flash
// eran casi copias la una de la otra).
package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/OxDAbit/oxflasher/internal/esptool"
	"github.com/OxDAbit/oxflasher/internal/serialport"
	"github.com/OxDAbit/oxflasher/internal/ui"
	"github.com/OxDAbit/oxflasher/internal/workspace"
)

// DefaultBaud es la velocidad por defecto. Con 460800 un backup de 1MB tarda
// unos 25 s, frente a ~1,5 min a 115200 (la que usaba el script original).
const DefaultBaud = 460800

// ErrCancelled se devuelve cuando el usuario responde "No" a una confirmación.
var ErrCancelled = errors.New("operation cancelled")

// App reúne todo lo que necesitan las operaciones. En lugar de variables
// globales (como default_path en Python), el estado viaja dentro de este
// struct: es más fácil de seguir y de testear.
type App struct {
	Tool      *esptool.Tool
	WS        *workspace.Workspace
	Baud      int
	Write     esptool.WriteOptions
	AssumeYes bool // --yes: no preguntar (para scripts)

	// Confirm es un CAMPO de tipo función. Por defecto pregunta con ui.Confirm,
	// pero un test podría sustituirlo por una función que responda siempre "sí".
	Confirm func(ctx context.Context, title, description string) (bool, error)
}

// Device es un ESP ya conectado e identificado.
type Device struct {
	Conn esptool.Connection
	Info esptool.ChipInfo
}

// confirm respeta --yes y convierte un "No" en ErrCancelled.
func (a *App) confirm(ctx context.Context, title, description string) error {
	if a.AssumeYes {
		return nil
	}
	ask := a.Confirm
	if ask == nil {
		if !ui.IsInteractive() {
			return errors.New("confirmation required: run in a terminal or pass --yes")
		}
		ask = ui.Confirm
	}
	ok, err := ask(ctx, title, description)
	if err != nil {
		return err
	}
	if !ok {
		return ErrCancelled
	}
	return nil
}

// ResolvePort decide qué puerto usar: el indicado con -p, el único detectado,
// o el que elija el usuario si hay varios.
func (a *App) ResolvePort(ctx context.Context, explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	ports, err := serialport.List(false)
	if err != nil {
		return "", err
	}
	switch len(ports) {
	case 0:
		return "", errors.New("no USB-serial adapter detected: plug in the device (run `oxflasher ports --all` to see every port)")
	case 1:
		ui.Dim("Using the only USB-serial port found: %s", ports[0])
		return ports[0].Path, nil
	}
	if !ui.IsInteractive() {
		return "", fmt.Errorf("%d ports detected, choose one with -p", len(ports))
	}
	opts := make([]ui.Option[string], len(ports))
	for i, p := range ports {
		opts[i] = ui.Option[string]{Label: p.String(), Value: p.Path}
	}
	return ui.Select(ctx, "Several ports detected. Which one?", opts)
}

// Connect identifica el chip conectado al puerto y muestra su información.
func (a *App) Connect(ctx context.Context, port string) (*Device, error) {
	conn := esptool.Connection{Port: port, Baud: a.Baud}
	ui.Step("Connecting to %s ...", port)
	info, err := a.Tool.Info(ctx, conn)
	if err != nil {
		return nil, err
	}
	ui.Box("Device",
		ui.KV{Key: "Port", Value: port},
		ui.KV{Key: "Chip", Value: info.Chip},
		ui.KV{Key: "MAC", Value: info.MAC},
		ui.KV{Key: "Flash size", Value: info.FlashSizeLabel()},
	)
	return &Device{Conn: conn, Info: info}, nil
}

// DefaultBackupPath es la ruta que se propone para un backup nuevo.
func (a *App) DefaultBackupPath(dev *Device) string {
	name := workspace.BackupName(dev.Info.Chip, dev.Info.MAC, time.Now())
	return workspace.UniquePath(filepath.Join(a.WS.BackupPath(), name))
}

// BackupTarget convierte lo que escribe el usuario en una ruta de destino:
//   - vacío            → nombre automático en <workspace>/backup/
//   - una carpeta      → nombre automático dentro de esa carpeta
//   - un nombre suelto → <workspace>/backup/<nombre>.bin
//   - una ruta         → esa ruta (se añade .bin si falta)
func (a *App) BackupTarget(dev *Device, input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return a.DefaultBackupPath(dev), nil
	}
	if !strings.ContainsRune(input, '/') && !strings.HasPrefix(input, "~") {
		input = filepath.Join(a.WS.BackupPath(), input)
	}
	path, err := workspace.ExpandPath(input)
	if err != nil {
		return "", err
	}
	if fi, err := os.Stat(path); err == nil && fi.IsDir() {
		name := workspace.BackupName(dev.Info.Chip, dev.Info.MAC, time.Now())
		return workspace.UniquePath(filepath.Join(path, name)), nil
	}
	if !strings.EqualFold(filepath.Ext(path), ".bin") {
		path += ".bin"
	}
	return path, nil
}

// Backup lee la flash completa del dispositivo y la guarda en dest.
func (a *App) Backup(ctx context.Context, dev *Device, dest string) (string, error) {
	size := dev.Info.FlashSize
	if size <= 0 {
		return "", errors.New("could not detect the flash size, so a complete backup is not possible")
	}

	// La carpeta de destino debe existir; si no, la creamos.
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", fmt.Errorf("creating backup folder: %w", err)
	}
	if _, err := os.Stat(dest); err == nil {
		if err := a.confirm(ctx, "Overwrite existing file?", dest); err != nil {
			return "", err
		}
	}

	ui.Step("Reading %s of flash from %s into %s", dev.Info.FlashSizeLabel(), dev.Conn.Port, dest)
	if err := a.Tool.ReadFlash(ctx, dev.Conn, size, dest); err != nil {
		return "", err
	}

	// Verificación: el fichero debe tener exactamente el tamaño de la flash.
	fi, err := os.Stat(dest)
	if err != nil {
		return "", err
	}
	if fi.Size() != size {
		return "", fmt.Errorf("backup is incomplete: got %d bytes, expected %d", fi.Size(), size)
	}
	ui.Success("Backup saved: %s (%s)", dest, esptool.FormatSize(size))
	return dest, nil
}

// Erase borra la flash completa, previa confirmación.
func (a *App) Erase(ctx context.Context, dev *Device) error {
	if err := a.confirm(ctx, "Erase the whole flash?",
		fmt.Sprintf("%s on %s. This cannot be undone.", dev.Info.Chip, dev.Conn.Port)); err != nil {
		return err
	}
	return a.erase(ctx, dev)
}

func (a *App) erase(ctx context.Context, dev *Device) error {
	ui.Step("Erasing flash on %s", dev.Conn.Port)
	if err := a.Tool.Erase(ctx, dev.Conn); err != nil {
		return err
	}
	ui.Success("Flash erased")
	return nil
}

// checkFirmware valida el binario y que quepa en la flash del chip.
func checkFirmware(dev *Device, file string) error {
	fi, err := workspace.CheckFirmware(file)
	if err != nil {
		return err
	}
	if dev.Info.FlashSize > 0 && fi.Size() > dev.Info.FlashSize {
		return fmt.Errorf("%s (%s) does not fit in the %s flash",
			filepath.Base(file), esptool.FormatSize(fi.Size()), dev.Info.FlashSizeLabel())
	}
	return nil
}

// Flash escribe el firmware en el dispositivo, previa confirmación.
func (a *App) Flash(ctx context.Context, dev *Device, file string) error {
	if err := checkFirmware(dev, file); err != nil {
		return err
	}
	if err := a.confirm(ctx, "Flash this firmware?", a.flashSummary(dev, file)); err != nil {
		return err
	}
	return a.flash(ctx, dev, file)
}

func (a *App) flash(ctx context.Context, dev *Device, file string) error {
	ui.Step("Flashing %s", file)
	if err := a.Tool.WriteFlash(ctx, dev.Conn, file, a.Write); err != nil {
		return err
	}
	ui.Success("Device flashed with %s", filepath.Base(file))
	return nil
}

func (a *App) flashSummary(dev *Device, file string) string {
	return fmt.Sprintf("%s → %s on %s (mode %s, size %s)",
		filepath.Base(file), dev.Info.Chip, dev.Conn.Port, a.Write.FlashMode, a.Write.FlashSize)
}

// Full ejecuta el flujo habitual para instalar HAA o Tasmota:
// backup → erase → flash, con una única confirmación al principio.
func (a *App) Full(ctx context.Context, dev *Device, file string) error {
	if err := checkFirmware(dev, file); err != nil {
		return err
	}
	backup := a.DefaultBackupPath(dev)
	if err := a.confirm(ctx, "Run full workflow (backup → erase → flash)?",
		fmt.Sprintf("Backup to %s, then %s", backup, a.flashSummary(dev, file))); err != nil {
		return err
	}

	// Si el backup falla NO seguimos: no queremos borrar un dispositivo
	// del que no tenemos copia.
	if _, err := a.Backup(ctx, dev, backup); err != nil {
		return fmt.Errorf("backup step: %w", err)
	}
	if err := a.erase(ctx, dev); err != nil {
		return fmt.Errorf("erase step: %w", err)
	}
	if err := a.flash(ctx, dev, file); err != nil {
		return fmt.Errorf("flash step: %w (your backup is at %s)", err, backup)
	}
	ui.Success("Full workflow completed")
	return nil
}

// Report muestra un error de forma amigable. Devuelve true si era una
// cancelación del usuario (no es un fallo real).
func Report(err error) (cancelled bool) {
	if errors.Is(err, ErrCancelled) || errors.Is(err, ui.ErrAborted) || errors.Is(err, context.Canceled) {
		ui.Warn("Cancelled")
		return true
	}
	// errors.As extrae el *CommandError si está en la cadena de errores,
	// aunque haya sido envuelto con %w varias veces.
	var cmdErr *esptool.CommandError
	hint := ""
	if errors.As(err, &cmdErr) {
		hint = cmdErr.Hint()
	}
	ui.Error(err, hint)
	return false
}
