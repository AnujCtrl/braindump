package printer

// ESC/POS command constants for thermal receipt printers.
var (
	CmdInit    = []byte{0x1b, 0x40}
	CmdBoldOn  = []byte{0x1b, 0x45, 0x01}
	CmdBoldOff = []byte{0x1b, 0x45, 0x00}
	CmdCenter  = []byte{0x1b, 0x61, 0x01}
	CmdLeft    = []byte{0x1b, 0x61, 0x00}
	CmdFeed    = []byte{0x0a}
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
