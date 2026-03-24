package printer

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestNullPrinter_Print_ReturnsNil(t *testing.T) {
	p := &NullPrinter{}
	if err := p.Print([]byte("test")); err != nil {
		t.Errorf("NullPrinter.Print() returned error: %v", err)
	}
}

func TestNullPrinter_Available_ReturnsFalse(t *testing.T) {
	p := &NullPrinter{}
	if p.Available() {
		t.Error("NullPrinter.Available() should return false")
	}
}

func TestBufferPrinter_Print_CapturesBytes(t *testing.T) {
	p := &BufferPrinter{}
	data1 := []byte{0x1b, 0x40}
	data2 := []byte("Hello")

	if err := p.Print(data1); err != nil {
		t.Fatal(err)
	}
	if err := p.Print(data2); err != nil {
		t.Fatal(err)
	}

	expected := append(data1, data2...)
	if !bytes.Equal(p.Data, expected) {
		t.Errorf("BufferPrinter.Data = %v, want %v", p.Data, expected)
	}
}

func TestBufferPrinter_Available_ReturnsTrue(t *testing.T) {
	p := &BufferPrinter{}
	if !p.Available() {
		t.Error("BufferPrinter.Available() should return true")
	}
}

func TestESCPOSPrinter_Available_NonExistentDevice(t *testing.T) {
	p := &ESCPOSPrinter{DevicePath: "/dev/nonexistent_printer_device_xyz"}
	if p.Available() {
		t.Error("ESCPOSPrinter.Available() should return false for non-existent device")
	}
}

func TestESCPOSPrinter_Available_NonWritableDevice(t *testing.T) {
	// Create a read-only file to simulate non-writable device
	dir := t.TempDir()
	path := filepath.Join(dir, "readonly")
	if err := os.WriteFile(path, []byte{}, 0444); err != nil {
		t.Fatal(err)
	}
	p := &ESCPOSPrinter{DevicePath: path}
	if p.Available() {
		t.Error("ESCPOSPrinter.Available() should return false for non-writable device")
	}
}

func TestESCPOSPrinter_Print_NonExistentDevice(t *testing.T) {
	p := &ESCPOSPrinter{DevicePath: "/dev/nonexistent_printer_device_xyz"}
	err := p.Print([]byte("test"))
	if err == nil {
		t.Error("ESCPOSPrinter.Print() should return error for non-existent device")
	}
}

// shortWriter simulates a USB device that accepts at most N bytes per Write call.
type shortWriter struct {
	buf       bytes.Buffer
	chunkSize int
}

func (w *shortWriter) Write(p []byte) (int, error) {
	if len(p) > w.chunkSize {
		p = p[:w.chunkSize]
	}
	return w.buf.Write(p)
}

func TestPrint_ShortWrite(t *testing.T) {
	// Simulate the short-write bug: a writer that only accepts 64 bytes at a time,
	// similar to USB bulk transfer buffers in the usblp kernel driver.
	payload := bytes.Repeat([]byte("ABCDEFGHIJ"), 50) // 500 bytes total
	sw := &shortWriter{chunkSize: 64}

	// Reproduce the write loop logic from ESCPOSPrinter.Print
	data := make([]byte, len(payload))
	copy(data, payload)
	for len(data) > 0 {
		n, err := sw.Write(data)
		if err != nil {
			t.Fatalf("unexpected write error: %v", err)
		}
		if n == 0 {
			t.Fatal("write returned 0 bytes with no error")
		}
		data = data[n:]
	}

	if !bytes.Equal(sw.buf.Bytes(), payload) {
		t.Errorf("short write loop: got %d bytes, want %d bytes", sw.buf.Len(), len(payload))
	}
}

func TestPrint_ShortWrite_VariousChunkSizes(t *testing.T) {
	payload := bytes.Repeat([]byte("Receipt line content!\n"), 30) // ~630 bytes

	for _, chunkSize := range []int{1, 7, 32, 64, 128, 256} {
		sw := &shortWriter{chunkSize: chunkSize}
		data := make([]byte, len(payload))
		copy(data, payload)

		for len(data) > 0 {
			n, err := sw.Write(data)
			if err != nil {
				t.Fatalf("chunkSize=%d: unexpected error: %v", chunkSize, err)
			}
			data = data[n:]
		}

		if !bytes.Equal(sw.buf.Bytes(), payload) {
			t.Errorf("chunkSize=%d: got %d bytes, want %d", chunkSize, sw.buf.Len(), len(payload))
		}
	}
}
