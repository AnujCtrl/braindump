package printer

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds receipt printer configuration.
type Config struct {
	Enabled    bool   `yaml:"enabled"`
	DevicePath string `yaml:"device_path"`
}

// DefaultConfig returns the default printer configuration.
func DefaultConfig() Config {
	return Config{
		Enabled:    true,
		DevicePath: "/dev/usb/lp0",
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
	return cfg, nil
}
