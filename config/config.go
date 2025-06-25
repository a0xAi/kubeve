package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Flags struct {
	DisableLogo bool `yaml:"disableLogo"`
}

type Theme struct {
	BackgroundColor string `yaml:"backgroundColor"`
	TextColor       string `yaml:"textColor"`
}

type Config struct {
	Flags Flags `yaml:"flags"`
	Theme Theme `yaml:"theme"`
}

type fileConfig struct {
	Config Config `yaml:"config"`
}

var Default = Config{
	Flags: Flags{DisableLogo: false},
	Theme: Theme{BackgroundColor: "#000000", TextColor: "#ffffff"},
}

// Path returns the default configuration file location.
func Path() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".kubeve", "config.yaml")
}

// Load reads the configuration from disk or returns Default if the file does not exist or cannot be parsed.
func Load() Config {
	p := Path()
	if p == "" {
		return Default
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return Default
	}
	var fc fileConfig
	if err := yaml.Unmarshal(data, &fc); err != nil {
		return Default
	}
	cfg := fc.Config
	if cfg.Theme.BackgroundColor == "" {
		cfg.Theme.BackgroundColor = Default.Theme.BackgroundColor
	}
	if cfg.Theme.TextColor == "" {
		cfg.Theme.TextColor = Default.Theme.TextColor
	}
	return cfg
}
