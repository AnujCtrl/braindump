package printer

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds receipt printer configuration.
type Config struct {
	Enabled       bool   `yaml:"enabled"`
	DevicePath    string `yaml:"device_path"`
	Mode          string `yaml:"mode"`           // "text" or "escpos"
	EncoderScript string `yaml:"encoder_script"` // path to Node.js ESC/POS encoder
	Width         int    `yaml:"width"`           // characters per line (default: 32)
}

// DefaultConfig returns the default printer configuration.
func DefaultConfig() Config {
	return Config{
		Enabled:       true,
		DevicePath:    "/dev/usb/lp0",
		Mode:          "text",
		EncoderScript: "scripts/receipt-encoder/encode.js",
		Width:         receiptWidth,
	}
}

// LoadConfig reads printer configuration from a YAML file.
// Returns DefaultConfig if file does not exist or is empty.
func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return Config{}, err
	}
	if len(data) == 0 {
		return DefaultConfig(), nil
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	// Backfill defaults for any fields absent in the YAML.
	def := DefaultConfig()
	if cfg.DevicePath == "" {
		cfg.DevicePath = def.DevicePath
	}
	if cfg.Mode == "" {
		cfg.Mode = def.Mode
	}
	if cfg.EncoderScript == "" {
		cfg.EncoderScript = def.EncoderScript
	}
	if cfg.Width == 0 {
		cfg.Width = def.Width
	}
	return cfg, nil
}
