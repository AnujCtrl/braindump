package printer

import (
	"bytes"
	"testing"
)

func TestCmdInit(t *testing.T) {
	expected := []byte{0x1b, 0x40}
	if !bytes.Equal(CmdInit, expected) {
		t.Errorf("CmdInit = %v, want %v", CmdInit, expected)
	}
}

func TestCmdBoldOn(t *testing.T) {
	expected := []byte{0x1b, 0x45, 0x01}
	if !bytes.Equal(CmdBoldOn, expected) {
		t.Errorf("CmdBoldOn = %v, want %v", CmdBoldOn, expected)
	}
}

func TestCmdBoldOff(t *testing.T) {
	expected := []byte{0x1b, 0x45, 0x00}
	if !bytes.Equal(CmdBoldOff, expected) {
		t.Errorf("CmdBoldOff = %v, want %v", CmdBoldOff, expected)
	}
}

func TestCmdCenter(t *testing.T) {
	expected := []byte{0x1b, 0x61, 0x01}
	if !bytes.Equal(CmdCenter, expected) {
		t.Errorf("CmdCenter = %v, want %v", CmdCenter, expected)
	}
}

func TestCmdLeft(t *testing.T) {
	expected := []byte{0x1b, 0x61, 0x00}
	if !bytes.Equal(CmdLeft, expected) {
		t.Errorf("CmdLeft = %v, want %v", CmdLeft, expected)
	}
}

func TestCmdFeed(t *testing.T) {
	expected := []byte{0x0a}
	if !bytes.Equal(CmdFeed, expected) {
		t.Errorf("CmdFeed = %v, want %v", CmdFeed, expected)
	}
}

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
