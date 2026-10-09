// Package tui implementa el menú interactivo (lo que se ve al ejecutar
// `oxflasher` sin argumentos). Es una capa fina sobre el paquete app.
package tui

import (
	"context"
	"errors"
	"fmt"

	"github.com/OxDAbit/oxflasher/internal/app"
	"github.com/OxDAbit/oxflasher/internal/esptool"
	"github.com/OxDAbit/oxflasher/internal/serialport"
	"github.com/OxDAbit/oxflasher/internal/ui"
	"github.com/OxDAbit/oxflasher/internal/workspace"
)

// Valores especiales del menú de puertos. Empiezan por el byte nulo (\x00)
// para que nunca puedan coincidir con una ruta real de dispositivo.
const (
	choiceRefresh = "\x00refresh"
	choiceAll     = "\x00all"
	choiceExit    = "\x00exit"
)

// action es un "enum" al estilo Go: un tipo entero y constantes con iota,
// que vale 0, 1, 2... en cada línea del bloque const.
type action int

const (
	actInfo action = iota
	actBackup
	actErase
	actFlash
	actFull
	actChangePort
	actExit
)

// Run lanza el menú interactivo hasta que el usuario sale.
func Run(ctx context.Context, a *app.App, version string) error {
	ui.ClearScreen()
	ui.Banner(version)
	ui.Dim("Workspace: %s   ·   %s", a.WS.Root, a.Tool)
	fmt.Fprintln(ui.Out)

	for {
		port, err := choosePort(ctx)
		if err != nil {
			return err // incluye ui.ErrAborted si el usuario pulsa Esc
		}
		if port == "" {
			return nil // eligió "Exit"
		}

		dev, err := a.Connect(ctx, port)
		if err != nil {
			app.Report(err)
			continue // volvemos a la lista de puertos
		}

		if done, err := deviceMenu(ctx, a, dev); done || err != nil {
			return err
		}
	}
}

// choosePort muestra los puertos detectados con opciones para refrescar.
func choosePort(ctx context.Context) (string, error) {
	showAll := false
	for {
		ports, err := serialport.List(showAll)
		if err != nil {
			return "", err
		}

		title := "Select the port where your ESP is connected"
		if len(ports) == 0 {
			title = "No USB-serial adapter detected. Plug in your device and refresh"
		}

		opts := make([]ui.Option[string], 0, len(ports)+3)
		for _, p := range ports {
			opts = append(opts, ui.Option[string]{Label: p.String(), Value: p.Path})
		}
		opts = append(opts, ui.Option[string]{Label: "↻ Refresh ports", Value: choiceRefresh})
		if !showAll {
			opts = append(opts, ui.Option[string]{Label: "… Show all serial ports", Value: choiceAll})
		}
		opts = append(opts, ui.Option[string]{Label: "✕ Exit", Value: choiceExit})

		choice, err := ui.Select(ctx, title, opts)
		if err != nil {
			return "", err
		}
		switch choice {
		case choiceRefresh:
			continue
		case choiceAll:
			showAll = true
			continue
		case choiceExit:
			return "", nil
		}
		return choice, nil
	}
}

// deviceMenu muestra las acciones disponibles para el dispositivo conectado.
// Devuelve done=true cuando el usuario quiere salir del programa.
func deviceMenu(ctx context.Context, a *app.App, dev *app.Device) (done bool, err error) {
	opts := []ui.Option[action]{
		{Label: "Create backup", Value: actBackup},
		{Label: "Erase device", Value: actErase},
		{Label: "Flash firmware", Value: actFlash},
		{Label: "Full workflow (backup → erase → flash)", Value: actFull},
		{Label: "Show device info", Value: actInfo},
		{Label: "Change port", Value: actChangePort},
		{Label: "Exit", Value: actExit},
	}

	for {
		fmt.Fprintln(ui.Out)
		choice, err := ui.Select(ctx, fmt.Sprintf("What do you want to do with %s?", dev.Info.Chip), opts)
		if errors.Is(err, ui.ErrAborted) {
			return true, nil
		}
		if err != nil {
			return true, err
		}

		switch choice {
		case actExit:
			return true, nil
		case actChangePort:
			return false, nil
		case actInfo:
			// Releer la información también sirve para comprobar que sigue conectado.
			if d, err := a.Connect(ctx, dev.Conn.Port); err != nil {
				app.Report(err)
			} else {
				dev = d
			}
		case actBackup:
			report(doBackup(ctx, a, dev))
		case actErase:
			report(a.Erase(ctx, dev))
		case actFlash:
			report(withFirmware(ctx, a, func(file string) error { return a.Flash(ctx, dev, file) }))
		case actFull:
			report(withFirmware(ctx, a, func(file string) error { return a.Full(ctx, dev, file) }))
		}
	}
}

func report(err error) {
	if err != nil {
		app.Report(err)
	}
}

func doBackup(ctx context.Context, a *app.App, dev *app.Device) error {
	def := a.DefaultBackupPath(dev)
	input, err := ui.Input(ctx, "Backup file (name, folder or full path)", def, nil)
	if err != nil {
		return err
	}
	dest, err := a.BackupTarget(dev, input)
	if err != nil {
		return err
	}
	_, err = a.Backup(ctx, dev, dest)
	return err
}

const manualPath = "\x00manual"

// withFirmware deja elegir un .bin del workspace (o escribir una ruta) y
// llama a fn con él. Recibir una función como parámetro evita repetir este
// código en "Flash" y en "Full workflow".
func withFirmware(ctx context.Context, a *app.App, fn func(file string) error) error {
	list, err := a.WS.Firmwares()
	if err != nil {
		return err
	}

	opts := make([]ui.Option[string], 0, len(list)+1)
	for _, f := range list {
		opts = append(opts, ui.Option[string]{
			Label: fmt.Sprintf("%-40s %8s", f.Rel, esptool.FormatSize(f.Size)),
			Value: f.Path,
		})
	}
	opts = append(opts, ui.Option[string]{Label: "✎ Enter a path manually…", Value: manualPath})

	title := "Choose the firmware to flash"
	if len(list) == 0 {
		title = fmt.Sprintf("No .bin files in %s (put them in haa/ or tasmota/)", a.WS.Root)
	}
	file, err := ui.Select(ctx, title, opts)
	if err != nil {
		return err
	}

	if file == manualPath {
		file, err = ui.Input(ctx, "Path to the .bin file", "", func(s string) error {
			p, err := workspace.ExpandPath(s)
			if err != nil {
				return err
			}
			_, err = workspace.CheckFirmware(p)
			return err
		})
		if err != nil {
			return err
		}
		if file, err = workspace.ExpandPath(file); err != nil {
			return err
		}
	}
	return fn(file)
}
