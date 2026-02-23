// Package plugin provides Yaegi-based plugin system.
package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/halqme/mee/pkg/provider"
	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

// Shared stdlib symbols to reduce interpreter initialization cost.
var stdlibSymbols = stdlib.Symbols

// Manifest describes a plugin.
type Manifest struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Trigger     string `json:"trigger"`
	Script      string `json:"script"`
	Description string `json:"description"`
}

// Plugin represents a loaded plugin.
type Plugin struct {
	Manifest Manifest
	Path     string
	interp   *interp.Interpreter
	searchFn func(string) string
	trigger  string
}

// Name returns the provider name.
func (p *Plugin) Name() string {
	return p.Manifest.Name
}

// Trigger returns the trigger prefix for this plugin.
func (p *Plugin) Trigger() string {
	return p.trigger
}

// Search implements provider.Provider.
func (p *Plugin) Search(query string) (*provider.ResultSet, error) {
	if p.searchFn == nil {
		return nil, nil
	}

	jsonStr := p.searchFn(query)
	if jsonStr == "" {
		return nil, nil
	}

	var rs provider.ResultSet
	if err := json.Unmarshal([]byte(jsonStr), &rs); err != nil {
		return nil, err
	}
	return &rs, nil
}

// DefaultSuggestions implements provider.Provider.
func (p *Plugin) DefaultSuggestions() (*provider.ResultSet, error) {
	return nil, nil
}

// Manager manages plugins.
type Manager struct {
	plugins []*Plugin
	mu      sync.RWMutex
}

// NewManager creates a new plugin manager.
func NewManager() *Manager {
	return &Manager{
		plugins: make([]*Plugin, 0),
	}
}

// LoadFromDirectory loads all plugins from a directory.
func (m *Manager) LoadFromDirectory(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pluginPath := filepath.Join(dir, entry.Name())
		if err := m.Load(pluginPath); err != nil {
			fmt.Printf("Warning: Failed to load plugin %s: %v\n", entry.Name(), err)
			continue
		}
	}

	return nil
}

// Load loads a plugin from a directory.
func (m *Manager) Load(pluginPath string) error {
	manifestPath := filepath.Join(pluginPath, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest: %w", err)
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("failed to parse manifest: %w", err)
	}

	if manifest.Name == "" || manifest.Script == "" {
		return fmt.Errorf("invalid manifest: missing name or script")
	}

	scriptPath := filepath.Join(pluginPath, manifest.Script)
	scriptData, err := os.ReadFile(scriptPath)
	if err != nil {
		return fmt.Errorf("failed to read script: %w", err)
	}

	i := interp.New(interp.Options{})
	i.Use(stdlibSymbols)

	if _, err := i.Eval(string(scriptData)); err != nil {
		return fmt.Errorf("failed to evaluate script: %w", err)
	}

	// Get Search function directly
	searchFn, err := i.Eval("Search")
	if err != nil {
		return fmt.Errorf("plugin must export 'Search' function: %w", err)
	}

	searchFunc, ok := searchFn.Interface().(func(string) string)
	if !ok {
		return fmt.Errorf("Search must be a function returning string")
	}

	pluginInstance := &Plugin{
		Manifest: manifest,
		Path:     pluginPath,
		interp:   i,
		searchFn: searchFunc,
		trigger:  manifest.Trigger,
	}

	m.mu.Lock()
	m.plugins = append(m.plugins, pluginInstance)
	m.mu.Unlock()

	return nil
}

// GetProviders returns all plugin providers.
func (m *Manager) GetProviders() []provider.Provider {
	m.mu.RLock()
	defer m.mu.RUnlock()

	providers := make([]provider.Provider, len(m.plugins))
	for i, p := range m.plugins {
		providers[i] = p
	}
	return providers
}

// GetTriggers returns all plugin triggers.
func (m *Manager) GetTriggers() []provider.TriggerInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	triggers := make([]provider.TriggerInfo, len(m.plugins))
	for i, p := range m.plugins {
		triggers[i] = provider.TriggerInfo{
			Prefix:      p.Manifest.Trigger,
			Description: p.Manifest.Description,
		}
	}
	return triggers
}
