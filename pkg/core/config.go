// Package core provides minimal configuration.
package core

import (
	"os"

	"github.com/halqme/mee/pkg/platform"
	"gopkg.in/yaml.v3"
)

// Config holds application settings.
type Config struct {
	App      AppConfig      `yaml:"app"`
	Display  DisplayConfig  `yaml:"display"`
	Plugins  PluginsConfig  `yaml:"plugins"`
	Search   SearchConfig   `yaml:"search"`
	Storage  StorageConfig  `yaml:"storage"`
	Registry RegistryConfig `yaml:"registry"`
	Colors   *ColorConfig   `yaml:"colors,omitempty"`
	// Deprecated: Use Plugins.Dirs instead
	PluginDirs []string `yaml:"-"`
}

// AppConfig holds application-level settings.
type AppConfig struct {
	Mode     string `yaml:"mode"`      // "daemon" or "interactive"
	Startup  bool   `yaml:"startup"`   // start on system startup
	LogLevel string `yaml:"log_level"` // "debug", "info", "warn", "error"
}

// DisplayConfig holds display settings.
type DisplayConfig struct {
	Theme      string `yaml:"theme"`       // "auto", "light", "dark"
	MaxResults int    `yaml:"max_results"` // max results to show
	ListHeight int    `yaml:"list_height"` // list height in lines
}

// PluginsConfig holds plugin settings.
type PluginsConfig struct {
	Dirs           []string `yaml:"dirs"`
	RuntimeDefault string   `yaml:"runtime_default"` // "yaegi", "wasm", "native"
}

// SearchConfig holds search settings.
type SearchConfig struct {
	FuzzyThreshold float64 `yaml:"fuzzy_threshold"` // 0.0-1.0
	HistoryBoost   bool    `yaml:"history_boost"`   // boost results by history
}

// StorageConfig holds storage settings.
type StorageConfig struct {
	DBPath string `yaml:"db_path"`
}

// RegistryConfig holds registry/cache settings.
type RegistryConfig struct {
	CacheDir string `yaml:"cache_dir"`
}

// ColorConfig holds custom color settings.
type ColorConfig struct {
	Title string `yaml:"title,omitempty"`
	Input string `yaml:"input,omitempty"`
	Mark  string `yaml:"mark,omitempty"`
	Item  string `yaml:"item,omitempty"`
	Sub   string `yaml:"sub,omitempty"`
	Help  string `yaml:"help,omitempty"`
}

// Load loads configuration from YAML file.
func Load() Config {
	path := platform.ConfigDir() + "/config.yaml"
	data, err := os.ReadFile(path)
	if err != nil {
		return defaults()
	}

	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return defaults()
	}

	// Apply defaults for zero values
	if c.Display.MaxResults == 0 {
		c.Display.MaxResults = 50
	}
	if c.Display.ListHeight == 0 {
		c.Display.ListHeight = 15
	}
	if c.Display.Theme == "" {
		c.Display.Theme = "auto"
	}
	if len(c.Plugins.Dirs) == 0 {
		c.Plugins.Dirs = []string{
			platform.ConfigDir() + "/plugins",
		}
	}
	if c.Plugins.RuntimeDefault == "" {
		c.Plugins.RuntimeDefault = "yaegi"
	}
	if c.Search.FuzzyThreshold == 0 {
		c.Search.FuzzyThreshold = 0.7
	}
	if c.Storage.DBPath == "" {
		c.Storage.DBPath = platform.DataDir() + "/mee.db"
	}
	if c.Registry.CacheDir == "" {
		c.Registry.CacheDir = platform.CacheDir() + "/plugins"
	}

	// For backward compatibility with old code
	c.PluginDirs = c.Plugins.Dirs

	return c
}

func defaults() Config {
	return Config{
		App: AppConfig{
			Mode:     "interactive",
			Startup:  false,
			LogLevel: "info",
		},
		Display: DisplayConfig{
			Theme:      "auto",
			MaxResults: 50,
			ListHeight: 15,
		},
		Plugins: PluginsConfig{
			Dirs: []string{
				platform.ConfigDir() + "/plugins",
				"/usr/local/share/mee/plugins",
			},
			RuntimeDefault: "yaegi",
		},
		Search: SearchConfig{
			FuzzyThreshold: 0.7,
			HistoryBoost:   true,
		},
		Storage: StorageConfig{
			DBPath: platform.DataDir() + "/mee.db",
		},
		Registry: RegistryConfig{
			CacheDir: platform.CacheDir() + "/plugins",
		},
		// For backward compatibility
		PluginDirs: []string{
			platform.ConfigDir() + "/plugins",
			"/usr/local/share/mee/plugins",
		},
	}
}

// Save saves configuration to YAML file.
func (c *Config) Save() error {
	path := platform.ConfigDir() + "/config.yaml"
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
