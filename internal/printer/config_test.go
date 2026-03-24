package printer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.Enabled {
		t.Error("DefaultConfig.Enabled should be true")
	}
	if cfg.DevicePath != "/dev/usb/lp0" {
		t.Errorf("DefaultConfig.DevicePath = %q, want /dev/usb/lp0", cfg.DevicePath)
	}
	if cfg.Mode != "text" {
		t.Errorf("DefaultConfig.Mode = %q, want \"text\"", cfg.Mode)
	}
	if cfg.EncoderScript != "scripts/receipt-encoder/encode.js" {
		t.Errorf("DefaultConfig.EncoderScript = %q, want \"scripts/receipt-encoder/encode.js\"", cfg.EncoderScript)
	}
	if cfg.Width != 32 {
		t.Errorf("DefaultConfig.Width = %d, want 32", cfg.Width)
	}
}

func TestLoadConfig_MissingFile_ReturnsDefault(t *testing.T) {
	cfg, err := LoadConfig("/nonexistent/path/printer.yaml")
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	def := DefaultConfig()
	if cfg != def {
		t.Errorf("missing file should return full default config\ngot:  %+v\nwant: %+v", cfg, def)
	}
}

func TestLoadConfig_EmptyFile_ReturnsDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "printer.yaml")
	os.WriteFile(path, []byte(""), 0644)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	def := DefaultConfig()
	if cfg != def {
		t.Errorf("empty file should return full default config\ngot:  %+v\nwant: %+v", cfg, def)
	}
}

func TestLoadConfig_InvalidYAML_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "printer.yaml")
	os.WriteFile(path, []byte("{{{{not valid yaml"), 0644)

	_, err := LoadConfig(path)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestLoadConfig_ValidEnabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "printer.yaml")
	os.WriteFile(path, []byte("enabled: true\ndevice_path: /dev/usb/lp1\n"), 0644)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if !cfg.Enabled {
		t.Error("expected Enabled=true")
	}
	if cfg.DevicePath != "/dev/usb/lp1" {
		t.Errorf("DevicePath = %q, want /dev/usb/lp1", cfg.DevicePath)
	}
}

func TestLoadConfig_ValidDisabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "printer.yaml")
	os.WriteFile(path, []byte("enabled: false\n"), 0644)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if cfg.Enabled {
		t.Error("expected Enabled=false")
	}
}

func TestLoadConfig_CustomDevicePath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "printer.yaml")
	os.WriteFile(path, []byte("enabled: true\ndevice_path: /dev/custom/printer\n"), 0644)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if cfg.DevicePath != "/dev/custom/printer" {
		t.Errorf("DevicePath = %q, want /dev/custom/printer", cfg.DevicePath)
	}
}

// TestLoadConfig_PartialYAML_BackfillsDefaults is the critical regression test
// for the bug where a printer.yaml with only enabled + device_path left
// EncoderScript as "", causing FormatReceipt to run "node """  which reads
// stdin as JavaScript. This test would have caught that bug.
func TestLoadConfig_PartialYAML_BackfillsDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "printer.yaml")
	// Real-world config: user sets only enabled and device_path.
	// Mode and EncoderScript are absent.
	os.WriteFile(path, []byte("enabled: true\ndevice_path: /dev/usb/lp0\n"), 0644)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	def := DefaultConfig()

	if cfg.Mode == "" {
		t.Fatal("Mode was not backfilled -- it is still empty string")
	}
	if cfg.Mode != def.Mode {
		t.Errorf("Mode = %q, want default %q", cfg.Mode, def.Mode)
	}

	if cfg.EncoderScript == "" {
		t.Fatal("EncoderScript was not backfilled -- it is still empty string (this was the bug)")
	}
	if cfg.EncoderScript != def.EncoderScript {
		t.Errorf("EncoderScript = %q, want default %q", cfg.EncoderScript, def.EncoderScript)
	}

	if cfg.Width == 0 {
		t.Fatal("Width was not backfilled -- it is still zero")
	}
	if cfg.Width != def.Width {
		t.Errorf("Width = %d, want default %d", cfg.Width, def.Width)
	}
}

// TestLoadConfig_ExplicitEncoderScript_NotOverwritten verifies that when a user
// provides an explicit encoder_script in YAML, the backfill logic does NOT
// replace it with the default. Without this test, a naive "always set defaults"
// implementation could silently discard user config.
func TestLoadConfig_ExplicitEncoderScript_NotOverwritten(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "printer.yaml")
	yaml := `enabled: true
device_path: /dev/usb/lp0
mode: escpos
encoder_script: custom/path.js
`
	os.WriteFile(path, []byte(yaml), 0644)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	if cfg.EncoderScript != "custom/path.js" {
		t.Errorf("EncoderScript = %q, want \"custom/path.js\" -- backfill overwrote explicit value", cfg.EncoderScript)
	}
	if cfg.Mode != "escpos" {
		t.Errorf("Mode = %q, want \"escpos\" -- backfill overwrote explicit value", cfg.Mode)
	}
	if cfg.DevicePath != "/dev/usb/lp0" {
		t.Errorf("DevicePath = %q, want \"/dev/usb/lp0\"", cfg.DevicePath)
	}
}

// TestLoadConfig_AllFieldsAbsent_FullBackfill covers the case where the YAML
// file has content (so it doesn't hit the empty-file path) but no recognized
// fields. All fields should be backfilled to defaults.
func TestLoadConfig_AllFieldsAbsent_FullBackfill(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "printer.yaml")
	// Valid YAML but no recognized keys -- all fields stay zero-value after unmarshal.
	os.WriteFile(path, []byte("some_other_key: hello\n"), 0644)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	def := DefaultConfig()
	// Enabled is bool so zero-value (false) is a legitimate user choice -- backfill
	// does NOT touch it. But DevicePath, Mode, EncoderScript, Width must be backfilled.
	if cfg.DevicePath != def.DevicePath {
		t.Errorf("DevicePath = %q, want default %q", cfg.DevicePath, def.DevicePath)
	}
	if cfg.Mode != def.Mode {
		t.Errorf("Mode = %q, want default %q", cfg.Mode, def.Mode)
	}
	if cfg.EncoderScript != def.EncoderScript {
		t.Errorf("EncoderScript = %q, want default %q", cfg.EncoderScript, def.EncoderScript)
	}
	if cfg.Width != def.Width {
		t.Errorf("Width = %d, want default %d", cfg.Width, def.Width)
	}
}

func TestLoadConfig_PartialYAML_BackfillsWidth(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "printer.yaml")
	os.WriteFile(path, []byte("enabled: true\ndevice_path: /dev/usb/lp0\n"), 0644)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	if cfg.Width == 0 {
		t.Fatal("Width was not backfilled -- it is still zero")
	}
	if cfg.Width != 32 {
		t.Errorf("Width = %d, want default 32", cfg.Width)
	}
}

func TestLoadConfig_ExplicitWidth_NotOverwritten(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "printer.yaml")
	os.WriteFile(path, []byte("enabled: true\nwidth: 28\n"), 0644)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	if cfg.Width != 28 {
		t.Errorf("Width = %d, want 28 -- backfill overwrote explicit value", cfg.Width)
	}
}
