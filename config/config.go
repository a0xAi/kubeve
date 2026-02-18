package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Flags struct {
	DisableLogo bool `yaml:"disableLogo"`
}

type Theme struct {
	Name            string `yaml:"name,omitempty"`
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
	Theme: Theme{Name: "midnight", BackgroundColor: "#000000", TextColor: "#ffffff"},
}

var predefinedThemes = []Theme{
	{Name: "midnight", BackgroundColor: "#000000", TextColor: "#ffffff"},
	{Name: "ocean", BackgroundColor: "#021b2f", TextColor: "#d6f0ff"},
	{Name: "forest", BackgroundColor: "#0f1a12", TextColor: "#d7f3d8"},
	{Name: "sunset", BackgroundColor: "#2b1510", TextColor: "#ffe3d6"},
	{Name: "solarized-dark", BackgroundColor: "#002b36", TextColor: "#93a1a1"},
	{Name: "solarized-light", BackgroundColor: "#fdf6e3", TextColor: "#586e75"},
	{Name: "mono-light", BackgroundColor: "#f5f5f5", TextColor: "#1f1f1f"},
	{Name: "terminal-green", BackgroundColor: "#001100", TextColor: "#66ff66"},
	{Name: "cobalt", BackgroundColor: "#0b1f3a", TextColor: "#dbe8ff"},
	{Name: "ember", BackgroundColor: "#1b0f0a", TextColor: "#ffd3b6"},
}

// Themes returns all built-in selectable themes.
func Themes() []Theme {
	themes := make([]Theme, len(predefinedThemes))
	copy(themes, predefinedThemes)
	return themes
}

// ThemeNames returns the built-in theme names.
func ThemeNames() []string {
	names := make([]string, 0, len(predefinedThemes))
	for _, theme := range predefinedThemes {
		names = append(names, theme.Name)
	}
	return names
}

// ThemeByName resolves a built-in theme by name.
func ThemeByName(name string) (Theme, bool) {
	query := strings.ToLower(strings.TrimSpace(name))
	if query == "" {
		return Theme{}, false
	}
	for _, theme := range predefinedThemes {
		if strings.EqualFold(theme.Name, query) {
			return theme, true
		}
	}
	return Theme{}, false
}

func themeNameByColors(backgroundColor string, textColor string) string {
	bg := strings.ToLower(strings.TrimSpace(backgroundColor))
	fg := strings.ToLower(strings.TrimSpace(textColor))
	for _, theme := range predefinedThemes {
		if strings.EqualFold(theme.BackgroundColor, bg) && strings.EqualFold(theme.TextColor, fg) {
			return theme.Name
		}
	}
	return ""
}

// ResolveTheme normalizes a theme and applies defaults.
func ResolveTheme(theme Theme) Theme {
	if preset, ok := ThemeByName(theme.Name); ok {
		return preset
	}
	resolved := theme
	if strings.TrimSpace(resolved.Name) != "" {
		resolved.Name = ""
	}
	if resolved.BackgroundColor == "" {
		resolved.BackgroundColor = Default.Theme.BackgroundColor
	}
	if resolved.TextColor == "" {
		resolved.TextColor = Default.Theme.TextColor
	}
	if resolved.Name == "" {
		if name := themeNameByColors(resolved.BackgroundColor, resolved.TextColor); name != "" {
			resolved.Name = name
		}
	}
	return resolved
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
	cfg.Theme = ResolveTheme(cfg.Theme)
	return cfg
}

// Save writes the configuration to disk.
func Save(cfg Config) error {
	p := Path()
	if p == "" {
		return fmt.Errorf("could not resolve config path")
	}
	cfg.Theme = ResolveTheme(cfg.Theme)
	payload, err := yaml.Marshal(fileConfig{Config: cfg})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, payload, 0o644)
}
