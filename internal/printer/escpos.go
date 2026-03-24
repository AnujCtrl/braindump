package printer

import (
	"fmt"
	"os"
)

// Printer is the interface for printing raw bytes to a receipt printer.
type Printer interface {
	Print(data []byte) error
	Available() bool
}

// NullPrinter is a no-op printer used when no physical printer is available.
type NullPrinter struct{}

func (n *NullPrinter) Print(data []byte) error { return nil }
func (n *NullPrinter) Available() bool          { return false }

// BufferPrinter captures printed bytes in memory (for testing).
type BufferPrinter struct {
	Data []byte
}

func (b *BufferPrinter) Print(data []byte) error {
	b.Data = append(b.Data, data...)
	return nil
}

func (b *BufferPrinter) Available() bool { return true }

// ESCPOSPrinter sends raw bytes to a thermal receipt printer via a device file.
type ESCPOSPrinter struct {
	DevicePath string
}

// Available checks if the device file exists and is writable.
func (p *ESCPOSPrinter) Available() bool {
	info, err := os.Stat(p.DevicePath)
	if err != nil {
		return false
	}
	// Check it's not a directory and attempt open for write
	if info.IsDir() {
		return false
	}
	f, err := os.OpenFile(p.DevicePath, os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

// Print writes raw ESC/POS bytes to the device file.
// Uses a write loop to handle partial writes from USB bulk transfer buffers.
func (p *ESCPOSPrinter) Print(data []byte) error {
	f, err := os.OpenFile(p.DevicePath, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("open printer device: %w", err)
	}
	defer f.Close()
	// Force blocking I/O — Go's runtime wraps fds with its netpoller
	// which can interfere with USB character device writes.
	// Fd() removes the fd from the poller and restores blocking mode.
	f.Fd()
	for len(data) > 0 {
		n, err := f.Write(data)
		if err != nil {
			return fmt.Errorf("write to printer: %w", err)
		}
		data = data[n:]
	}
	return nil
}
