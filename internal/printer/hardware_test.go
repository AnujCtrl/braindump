//go:build printer

package printer

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/anujp/braindump/internal/core"
)

// hwDataDir returns the directory for debug log output.
// Reads TEST_DATA_DIR env var, defaulting to /tmp/todo-hw-debug.
func hwDataDir() string {
	if d := os.Getenv("TEST_DATA_DIR"); d != "" {
		return d
	}
	return "/tmp/todo-hw-debug"
}

// hwDebug writes a debug log AFTER printing, including the print result.
// Uses the test name as label so each test gets its own debug file.
func hwDebug(t *testing.T, todoID string, mode string, device string, content string, printErr error) {
	t.Helper()
	if err := LogPrintDebug(hwDataDir(), todoID, t.Name(), mode, device, content, printErr); err != nil {
		t.Logf("warning: failed to write debug log: %v", err)
	} else {
		t.Logf("debug log: %s/.debug/ (print: %v)", hwDataDir(), printErr)
	}
}

// hwWidth returns the receipt width for hardware tests. It checks
// TEST_RECEIPT_WIDTH env var first, falling back to the package default.
func hwWidth() int {
	if s := os.Getenv("TEST_RECEIPT_WIDTH"); s != "" {
		if w, err := strconv.Atoi(s); err == nil && w > 0 {
			return w
		}
	}
	return receiptWidth
}

// requirePrinter checks that a physical printer is available for hardware tests.
// Returns a configured ESCPOSPrinter or skips the test.
func requirePrinter(t *testing.T) *ESCPOSPrinter {
	t.Helper()
	dev := os.Getenv("PRINTER_DEVICE")
	if dev == "" {
		t.Skip("PRINTER_DEVICE not set -- skipping hardware test")
	}
	p := &ESCPOSPrinter{DevicePath: dev}
	if !p.Available() {
		t.Skipf("printer device %s not available", dev)
	}
	return p
}

func hwTodo() *core.Todo {
	return &core.Todo{
		ID:     "hw0001",
		Text:   "Hardware test todo",
		Tags:   []string{"test"},
		Status: "inbox",
	}
}

func TestHW_PlainReceipt(t *testing.T) {
	p := requirePrinter(t)
	todo := hwTodo()
	data := FormatPlainReceipt(todo, 0, hwWidth())
	printErr := p.Print(data)
	hwDebug(t, todo.ID, "text", p.DevicePath, string(data), printErr)
	if printErr != nil {
		t.Fatalf("print plain receipt: %v", printErr)
	}
}

func TestHW_ESCPOSReceipt(t *testing.T) {
	p := requirePrinter(t)
	if !nodeAvailable() {
		t.Skip("node not available -- skipping ESC/POS test")
	}
	script := os.Getenv("ENCODER_SCRIPT")
	if script == "" {
		script = "../../scripts/receipt-encoder/encode.js"
	}
	todo := hwTodo()
	data, cmdScript, err := FormatReceipt(todo, 0, script, hwWidth())
	if err != nil {
		t.Fatalf("format ESC/POS receipt: %v", err)
	}
	t.Logf("ESC/POS binary: %d bytes", len(data))
	printErr := p.Print(data)
	hwDebug(t, todo.ID, "escpos", p.DevicePath, cmdScript, printErr)
	if printErr != nil {
		t.Fatalf("print ESC/POS receipt: %v", printErr)
	}
}

func TestHW_Urgent(t *testing.T) {
	p := requirePrinter(t)
	todo := hwTodo()
	todo.Urgent = true
	data := FormatPlainReceipt(todo, 0, hwWidth())
	printErr := p.Print(data)
	hwDebug(t, todo.ID, "text", p.DevicePath, string(data), printErr)
	if printErr != nil {
		t.Fatalf("print urgent receipt: %v", printErr)
	}
}

func TestHW_Important(t *testing.T) {
	p := requirePrinter(t)
	todo := hwTodo()
	todo.Important = true
	data := FormatPlainReceipt(todo, 0, hwWidth())
	printErr := p.Print(data)
	hwDebug(t, todo.ID, "text", p.DevicePath, string(data), printErr)
	if printErr != nil {
		t.Fatalf("print important receipt: %v", printErr)
	}
}

func TestHW_Streak(t *testing.T) {
	p := requirePrinter(t)
	todo := hwTodo()
	data := FormatPlainReceipt(todo, 7, hwWidth())
	printErr := p.Print(data)
	hwDebug(t, todo.ID, "text", p.DevicePath, string(data), printErr)
	if printErr != nil {
		t.Fatalf("print streak receipt: %v", printErr)
	}
}

func TestHW_Legendary(t *testing.T) {
	p := requirePrinter(t)
	todo := hwTodo()
	todo.Text = "Legendary hardware test"

	var legendaryData []byte
	for i := 0; i < 100; i++ {
		data := FormatPlainReceipt(todo, 0, hwWidth())
		if strings.Contains(string(data), "LEGENDARY") {
			legendaryData = data
			break
		}
	}
	if legendaryData == nil {
		t.Skip("no legendary receipt generated after 100 attempts")
	}
	printErr := p.Print(legendaryData)
	hwDebug(t, todo.ID, "text", p.DevicePath, string(legendaryData), printErr)
	if printErr != nil {
		t.Fatalf("print legendary receipt: %v", printErr)
	}
}

func TestHW_LongTextWrap(t *testing.T) {
	p := requirePrinter(t)
	todo := hwTodo()
	todo.Text = strings.Repeat("This is a long text that should wrap across multiple lines on the thermal printer. ", 3)
	data := FormatPlainReceipt(todo, 0, hwWidth())
	printErr := p.Print(data)
	hwDebug(t, todo.ID, "text", p.DevicePath, string(data), printErr)
	if printErr != nil {
		t.Fatalf("print long text receipt: %v", printErr)
	}
}

func TestHW_EmptyTags(t *testing.T) {
	p := requirePrinter(t)
	todo := hwTodo()
	todo.Tags = nil
	data := FormatPlainReceipt(todo, 0, hwWidth())
	printErr := p.Print(data)
	hwDebug(t, todo.ID, "text", p.DevicePath, string(data), printErr)
	if printErr != nil {
		t.Fatalf("print empty-tags receipt: %v", printErr)
	}
}
