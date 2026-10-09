package esptool

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ChipInfo es la información básica del ESP conectado.
type ChipInfo struct {
	Chip      string // p. ej. "ESP8266EX" o "ESP32-D0WD-V3"
	MAC       string // p. ej. "84:f3:eb:12:34:56"
	FlashSize int64  // en bytes; 0 si no se pudo detectar
}

// FlashSizeLabel devuelve el tamaño en formato legible (1MB, 512KB...).
func (c ChipInfo) FlashSizeLabel() string {
	return FormatSize(c.FlashSize)
}

// FormatSize convierte bytes en "4MB", "512KB" o "?".
func FormatSize(n int64) string {
	switch {
	case n <= 0:
		return "?"
	case n%(1<<20) == 0: // 1<<20 es 1 MiB (desplazamiento de bits: 2^20)
		return fmt.Sprintf("%dMB", n>>20)
	case n%(1<<10) == 0:
		return fmt.Sprintf("%dKB", n>>10)
	default:
		return fmt.Sprintf("%dB", n)
	}
}

// Expresiones regulares compatibles con la salida de esptool v4 y v5:
//
//	v4: "Chip is ESP8266EX"            v5: "Chip type:          ESP8266EX"
//	    "MAC: 84:f3:eb:12:34:56"           "MAC:                84:f3:eb:12:34:56"
//	    "Detected flash size: 1MB"         "Detected flash size: 1MB"
//
// (?m) activa el modo multilínea: ^ y $ casan con inicio y fin de cada línea.
var (
	chipRe  = regexp.MustCompile(`(?m)^(?:Chip is|Chip type:)\s*(.+?)\s*(?:\(|$)`)
	macRe   = regexp.MustCompile(`(?m)^MAC:\s*([0-9a-fA-F]{2}(?::[0-9a-fA-F]{2}){5})`)
	flashRe = regexp.MustCompile(`(?m)Detected flash size:\s*(\d+)\s*(KB|MB)`)
)

// ParseInfo extrae ChipInfo de la salida de `esptool flash_id`.
func ParseInfo(out string) (ChipInfo, error) {
	var info ChipInfo

	if m := chipRe.FindStringSubmatch(out); m != nil {
		info.Chip = strings.TrimSpace(m[1])
	}
	if m := macRe.FindStringSubmatch(out); m != nil {
		info.MAC = strings.ToLower(m[1])
	}
	if m := flashRe.FindStringSubmatch(out); m != nil {
		n, _ := strconv.ParseInt(m[1], 10, 64) // la regex garantiza que son dígitos
		if m[2] == "MB" {
			info.FlashSize = n << 20
		} else {
			info.FlashSize = n << 10
		}
	}

	if info.Chip == "" {
		return info, fmt.Errorf("could not identify the chip in esptool output")
	}
	return info, nil
}

// Info conecta con el ESP y devuelve chip, MAC y tamaño de la flash.
func (t *Tool) Info(ctx context.Context, conn Connection) (ChipInfo, error) {
	out, err := t.run(ctx, conn, "flash_id", false)
	if err != nil {
		return ChipInfo{}, err
	}
	return ParseInfo(out)
}

// ReadFlash hace un backup de `size` bytes desde la dirección 0 al fichero dest.
//
// El script original leía siempre 1MB fijo (0x100000), lo que dejaba
// incompletos los backups de módulos de 4MB. Ahora el tamaño lo decide quien
// llama, normalmente a partir de ChipInfo.FlashSize.
func (t *Tool) ReadFlash(ctx context.Context, conn Connection, size int64, dest string) error {
	if size <= 0 {
		return fmt.Errorf("invalid flash size %d", size)
	}
	_, err := t.run(ctx, conn, "read_flash", true, "0", fmt.Sprintf("0x%x", size), dest)
	return err
}

// Erase borra toda la flash.
func (t *Tool) Erase(ctx context.Context, conn Connection) error {
	_, err := t.run(ctx, conn, "erase_flash", true)
	return err
}

// WriteOptions son los parámetros de escritura.
type WriteOptions struct {
	FlashMode string // qio, qout, dio, dout o keep. HAA y Tasmota (ESP8266) recomiendan dout
	FlashSize string // detect, keep, 1MB, 4MB...
}

// DefaultWriteOptions son valores seguros para ESP8266 con HAA o Tasmota.
var DefaultWriteOptions = WriteOptions{FlashMode: "dout", FlashSize: "detect"}

// WriteFlash escribe el binario file en la dirección 0x0.
//
// ¡Este era el fallo grave del script Python! Usaba read_flash en vez de
// write_flash, así que en lugar de flashear el dispositivo SOBRESCRIBÍA el
// .bin local con el contenido del chip.
func (t *Tool) WriteFlash(ctx context.Context, conn Connection, file string, opts WriteOptions) error {
	if err := ValidateFlashMode(opts.FlashMode); err != nil {
		return err
	}
	if err := ValidateFlashSize(opts.FlashSize); err != nil {
		return err
	}
	_, err := t.run(ctx, conn, "write_flash", true, "-fm", opts.FlashMode, "-fs", opts.FlashSize, "0x0", file)
	return err
}

// FlashModes son los modos aceptados por esptool.
var FlashModes = []string{"dout", "dio", "qout", "qio", "keep"}

// ValidateFlashMode comprueba el modo antes de lanzar esptool, para dar un
// error claro y temprano.
func ValidateFlashMode(mode string) error {
	for _, m := range FlashModes {
		if mode == m {
			return nil
		}
	}
	return fmt.Errorf("invalid flash mode %q (valid: %s)", mode, strings.Join(FlashModes, ", "))
}

var flashSizeRe = regexp.MustCompile(`^(detect|keep|256KB|512KB|1MB|2MB|4MB|8MB|16MB|32MB|64MB|128MB)$`)

// ValidateFlashSize comprueba el tamaño de flash indicado.
func ValidateFlashSize(size string) error {
	if !flashSizeRe.MatchString(size) {
		return fmt.Errorf("invalid flash size %q (valid: detect, keep, 512KB, 1MB, 2MB, 4MB, 8MB, 16MB...)", size)
	}
	return nil
}
