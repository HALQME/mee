// Package core provides minimal configuration.
package core

import (
	"encoding/json"
	"os"

	"github.com/halqme/mee/pkg/platform"
)

// Config holds application settings.
type Config struct {
	Display    Display  `json:"display"`
	PluginDirs []string `json:"plugin_dirs"`
}

// Display holds display settings.
type Display struct {
	MaxResults int `json:"max_results"`
	ListHeight int `json:"list_height"`
}

// Load loads configuration.
func Load() Config {
	path := platform.ConfigDir() + "/config.json"
	data, err := os.ReadFile(path)
	if err != nil {
		return defaults()
	}

	var c Config
	json.Unmarshal(data, &c)
	if c.Display.MaxResults == 0 {
		c.Display.MaxResults = 50
	}
	if c.Display.ListHeight == 0 {
		c.Display.ListHeight = 15
	}
	if len(c.PluginDirs) == 0 {
		c.PluginDirs = []string{
			platform.ConfigDir() + "/plugins",
		}
	}
	return c
}

func defaults() Config {
	return Config{
		Display: Display{MaxResults: 50, ListHeight: 15},
		PluginDirs: []string{
			platform.ConfigDir() + "/plugins",
			"/usr/local/share/mee/plugins",
		},
	}
}
