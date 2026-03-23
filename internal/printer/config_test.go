package printer

import (
	"os"
	"path/filepath"
	"testing"
)

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

func TestLoadConfig_MissingFile_ReturnsDefault(t *testing.T) {
	cfg, err := LoadConfig("/nonexistent/path/printer.yaml")
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	def := DefaultConfig()
	if cfg.Enabled != def.Enabled {
		t.Errorf("Enabled = %v, want %v (default)", cfg.Enabled, def.Enabled)
	}
	if cfg.DevicePath != def.DevicePath {
		t.Errorf("DevicePath = %q, want %q (default)", cfg.DevicePath, def.DevicePath)
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
	if cfg.Enabled != def.Enabled || cfg.DevicePath != def.DevicePath {
		t.Errorf("empty file should return defaults, got %+v", cfg)
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

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.Enabled {
		t.Error("DefaultConfig.Enabled should be true")
	}
	if cfg.DevicePath != "/dev/usb/lp0" {
		t.Errorf("DefaultConfig.DevicePath = %q, want /dev/usb/lp0", cfg.DevicePath)
	}
}
