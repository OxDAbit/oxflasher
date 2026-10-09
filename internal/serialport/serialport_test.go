package serialport

import "testing"

// Los tests en Go viven en ficheros *_test.go del mismo paquete y se ejecutan
// con `go test ./...`. Cada función TestXxx(t *testing.T) es un test.
//
// El patrón más habitual es el "table-driven test": una tabla de casos y un
// bucle que los recorre con t.Run, que crea un subtest por caso.
func TestDescribe(t *testing.T) {
	tests := []struct {
		path    string
		wantUSB bool
	}{
		{"/dev/cu.usbserial-14310", true},
		{"/dev/cu.wchusbserial1410", true},
		{"/dev/cu.SLAB_USBtoUART", true},
		{"/dev/cu.usbmodem101", true},
		{"/dev/ttyUSB0", true},
		{"/dev/ttyACM0", true},
		{"/dev/cu.Bluetooth-Incoming-Port", false},
		{"/dev/cu.debug-console", false},
		{"/dev/cu.BLTH", false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			_, usb := describe(tt.path)
			if usb != tt.wantUSB {
				t.Errorf("describe(%q) usb = %v, want %v", tt.path, usb, tt.wantUSB)
			}
		})
	}
}

func TestFilter(t *testing.T) {
	in := func() []Port {
		return []Port{
			{Path: "/dev/cu.usbserial-2", USB: true},
			{Path: "/dev/cu.Bluetooth-Incoming-Port"},
			{Path: "/dev/cu.usbserial-1", USB: true},
		}
	}

	got := filter(in(), false)
	if len(got) != 2 || got[0].Path != "/dev/cu.usbserial-1" {
		t.Fatalf("filter(all=false) = %v, want the two USB ports sorted", got)
	}
	if got := filter(in(), true); len(got) != 3 {
		t.Fatalf("filter(all=true) returned %d ports, want 3", len(got))
	}
}
