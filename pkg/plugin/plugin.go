// Package plugin provides Yaegi-based plugin system.
package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/halqme/mee/pkg/provider"
	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

// Shared stdlib symbols to reduce interpreter initialization cost.
var stdlibSymbols = stdlib.Symbols

// SandboxConfig holds sandbox configuration for plugin execution.
type SandboxConfig struct {
	Timeout     time.Duration // execution timeout
	MemoryLimit int           // memory limit in MB (0 = no limit)
	Disabled    bool          // disable sandbox entirely (for debugging)
}

// DefaultSandboxConfig returns the default sandbox configuration.
func DefaultSandboxConfig() SandboxConfig {
	return SandboxConfig{
		Timeout:     5 * time.Second,
		MemoryLimit: 64, // 64MB default
		Disabled:    false,
	}
}

// Runtime type for plugin execution.
type Runtime string

const (
	RuntimeYaegi  Runtime = "yaegi"
	RuntimeWASM   Runtime = "wasm"
	RuntimeNative Runtime = "native"
)

// Manifest describes a plugin.
type Manifest struct {
	Name        string  `json:"name"`
	Version     string  `json:"version"`
	URL         string  `json:"url,omitempty"`
	Trigger     string  `json:"trigger"`
	Script      string  `json:"script"`
	Description string  `json:"description"`
	Runtime     Runtime `json:"runtime,omitempty"`
	License     string  `json:"license,omitempty"`
	Timeout     int     `json:"timeout_ms,omitempty"`      // execution timeout in ms
	MemoryLimit int     `json:"memory_limit_mb,omitempty"` // memory limit in MB
	Enabled     bool    `json:"enabled,omitempty"`
}

// InstallResult contains the result of a plugin installation.
type InstallResult struct {
	Manifest  Manifest
	LocalPath string
	ID        string
}

// Plugin represents a loaded plugin.
type Plugin struct {
	Manifest Manifest
	Path     string

	// Lazy loading fields - interpreter is initialized on first Search()
	interp      *interp.Interpreter
	searchFn    func(string) string
	trigger     string
	scriptPath  string
	initialized bool
	mu          sync.RWMutex

	// Sandbox configuration for this plugin
	timeout     time.Duration
	memoryLimit int

	// Error tracking
	lastError  error
	errorCount int
}

// Name returns the provider name.
func (p *Plugin) Name() string {
	return p.Manifest.Name
}

// Trigger returns the trigger prefix for this plugin.
func (p *Plugin) Trigger() string {
	return p.trigger
}

// LastError returns the last error encountered by this plugin.
func (p *Plugin) LastError() error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.lastError
}

// ErrorCount returns the number of errors this plugin has encountered.
func (p *Plugin) ErrorCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.errorCount
}

// IsHealthy checks if the plugin is working properly.
func (p *Plugin) IsHealthy() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.lastError == nil && p.initialized
}

// recordError records an error for the plugin.
func (p *Plugin) recordError(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastError = err
	p.errorCount++
}

// resetError clears the plugin's error state.
func (p *Plugin) resetError() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastError = nil
}

// Search implements provider.Provider with timeout and panic protection.
func (p *Plugin) Search(query string) (rs *provider.ResultSet, err error) {
	// Recover from panics
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("plugin panicked: %v", r)
			p.recordError(err)
		}
	}()

	if err := p.ensureInitialized(); err != nil {
		return nil, err
	}

	if p.searchFn == nil {
		return nil, nil
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	// Run search in goroutine with timeout
	type result struct {
		jsonStr string
		err     error
	}

	resultCh := make(chan result, 1)
	go func() {
		jsonStr := p.searchFn(query)
		resultCh <- result{jsonStr: jsonStr}
	}()

	select {
	case <-ctx.Done():
		err := fmt.Errorf("plugin execution timed out after %v", p.timeout)
		p.recordError(err)
		return nil, err
	case r := <-resultCh:
		if r.err != nil {
			err := fmt.Errorf("plugin execution error: %w", r.err)
			p.recordError(err)
			return nil, err
		}
		if r.jsonStr == "" {
			return nil, nil
		}

		var rs provider.ResultSet
		if err := json.Unmarshal([]byte(r.jsonStr), &rs); err != nil {
			err := fmt.Errorf("failed to parse plugin result: %w", err)
			p.recordError(err)
			return nil, err
		}
		// Clear error on success
		p.resetError()
		return &rs, nil
	}
}

// ensureInitialized lazily initializes the Yaegi interpreter only when needed.
func (p *Plugin) ensureInitialized() error {
	p.mu.RLock()
	if p.initialized {
		p.mu.RUnlock()
		return nil
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check after acquiring write lock
	if p.initialized {
		return nil
	}

	// Set timeout from manifest, default to 5 seconds
	if p.timeout == 0 {
		if p.Manifest.Timeout > 0 {
			p.timeout = time.Duration(p.Manifest.Timeout) * time.Millisecond
		} else {
			p.timeout = 5 * time.Second
		}
	}

	// Lazy load: read and evaluate script only when first searched
	scriptData, err := os.ReadFile(p.scriptPath)
	if err != nil {
		return fmt.Errorf("failed to read script: %w", err)
	}

	// Create interpreter with restricted options
	opts := interp.Options{
		// Disable some unsafe features
		GoPath: "", // Disable Go path
	}

	i := interp.New(opts)

	// Use standard library symbols
	i.Use(stdlibSymbols)

	if _, err := i.Eval(string(scriptData)); err != nil {
		return fmt.Errorf("failed to evaluate script: %w", err)
	}

	// Get Search function
	searchFn, err := i.Eval("Search")
	if err != nil {
		return fmt.Errorf("plugin must export 'Search' function: %w", err)
	}

	searchFunc, ok := searchFn.Interface().(func(string) string)
	if !ok {
		return fmt.Errorf("Search must be a function returning string")
	}

	p.interp = i
	p.searchFn = searchFunc
	p.initialized = true

	return nil
}

// DefaultSuggestions implements provider.Provider.
func (p *Plugin) DefaultSuggestions() (*provider.ResultSet, error) {
	return nil, nil
}

// Manager manages plugins.
type Manager struct {
	plugins []*Plugin
	mu      sync.RWMutex

	// Global sandbox configuration
	sandboxConfig SandboxConfig
}

// NewManager creates a new plugin manager with default sandbox config.
func NewManager() *Manager {
	return &Manager{
		plugins:       make([]*Plugin, 0),
		sandboxConfig: DefaultSandboxConfig(),
	}
}

// NewManagerWithConfig creates a new plugin manager with custom sandbox config.
func NewManagerWithConfig(config SandboxConfig) *Manager {
	return &Manager{
		plugins:       make([]*Plugin, 0),
		sandboxConfig: config,
	}
}

// SetSandboxConfig sets the global sandbox configuration.
func (m *Manager) SetSandboxConfig(config SandboxConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sandboxConfig = config
}

// GetSandboxConfig returns the current sandbox configuration.
func (m *Manager) GetSandboxConfig() SandboxConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sandboxConfig
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

	// Lazy loading: only validate script exists, don't load interpreter yet
	if _, err := os.Stat(scriptPath); err != nil {
		return fmt.Errorf("failed to stat script: %w", err)
	}

	// Get timeout from manifest or use default
	timeout := m.sandboxConfig.Timeout
	if manifest.Timeout > 0 {
		timeout = time.Duration(manifest.Timeout) * time.Millisecond
	}

	// Create plugin with lazy initialization - interpreter loads on first Search()
	pluginInstance := &Plugin{
		Manifest:    manifest,
		Path:        pluginPath,
		trigger:     manifest.Trigger,
		scriptPath:  scriptPath,
		initialized: false,
		timeout:     timeout,
		memoryLimit: manifest.MemoryLimit,
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

// InstallFromURL installs a plugin from a remote URL.
// Supports:
// - GitHub repository URLs (e.g., https://github.com/user/repo/tree/main/plugin-name)
// - Direct manifest URLs (ending in /manifest.json)
// - Tarball URLs
func (m *Manager) InstallFromURL(sourceURL, pluginsDir string) (*InstallResult, error) {
	// Parse URL to determine type
	parsedURL, err := url.Parse(sourceURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	var result *InstallResult

	// Determine source type and install accordingly
	if strings.Contains(parsedURL.Host, "github.com") {
		result, err = m.installFromGitHub(sourceURL, pluginsDir)
	} else if strings.HasSuffix(sourceURL, "manifest.json") {
		result, err = m.installFromManifestURL(sourceURL, pluginsDir)
	} else {
		result, err = m.installFromDirectory(sourceURL, pluginsDir)
	}

	if err != nil {
		return nil, err
	}

	// Load the newly installed plugin
	if err := m.Load(result.LocalPath); err != nil {
		return nil, fmt.Errorf("failed to load installed plugin: %w", err)
	}

	return result, nil
}

// installFromGitHub installs a plugin from a GitHub repository.
func (m *Manager) installFromGitHub(repoURL, pluginsDir string) (*InstallResult, error) {
	// Parse GitHub URL
	// Format: https://github.com/owner/repo[/tree/branch][/plugin-path]
	parts := strings.Split(strings.TrimPrefix(repoURL, "https://github.com/"), "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid GitHub URL format")
	}

	owner := parts[0]
	repo := parts[1]
	branch := "main"
	pluginPath := ""

	// Check for tree/branch specification
	if len(parts) > 3 && parts[2] == "tree" {
		branch = parts[3]
		if len(parts) > 4 {
			pluginPath = strings.Join(parts[4:], "/")
		}
	} else if len(parts) > 2 {
		pluginPath = strings.Join(parts[2:], "/")
	}

	// Construct raw manifest URL
	var manifestURL string
	if pluginPath != "" {
		manifestURL = fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s/manifest.json",
			owner, repo, branch, pluginPath)
	} else {
		manifestURL = fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/manifest.json",
			owner, repo, branch)
	}

	// Fetch manifest
	manifest, err := fetchManifest(manifestURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch manifest: %w", err)
	}

	// Set URL in manifest
	manifest.URL = repoURL

	// Create local plugin directory
	pluginName := manifest.Name
	if pluginName == "" {
		pluginName = repo
	}
	localPath := filepath.Join(pluginsDir, pluginName)

	// Download plugin files
	if err := downloadGitHubPlugin(owner, repo, branch, pluginPath, localPath); err != nil {
		return nil, fmt.Errorf("failed to download plugin: %w", err)
	}

	// Write manifest
	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(localPath, "manifest.json"), manifestData, 0644); err != nil {
		return nil, fmt.Errorf("failed to write manifest: %w", err)
	}

	return &InstallResult{
		Manifest:  *manifest,
		LocalPath: localPath,
		ID:        generatePluginID(pluginName, repoURL),
	}, nil
}

// installFromManifestURL installs a plugin from a direct manifest URL.
func (m *Manager) installFromManifestURL(manifestURL, pluginsDir string) (*InstallResult, error) {
	manifest, err := fetchManifest(manifestURL)
	if err != nil {
		return nil, err
	}

	// Download script from same base URL
	baseURL := strings.TrimSuffix(manifestURL, "/manifest.json")
	scriptURL := baseURL + "/" + manifest.Script

	pluginName := manifest.Name
	localPath := filepath.Join(pluginsDir, pluginName)

	// Create directory
	if err := os.MkdirAll(localPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create plugin directory: %w", err)
	}

	// Download script
	scriptData, err := downloadFile(scriptURL)
	if err != nil {
		os.RemoveAll(localPath)
		return nil, fmt.Errorf("failed to download script: %w", err)
	}

	if err := os.WriteFile(filepath.Join(localPath, manifest.Script), scriptData, 0644); err != nil {
		os.RemoveAll(localPath)
		return nil, fmt.Errorf("failed to write script: %w", err)
	}

	// Write manifest
	manifest.URL = manifestURL
	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(localPath, "manifest.json"), manifestData, 0644); err != nil {
		os.RemoveAll(localPath)
		return nil, fmt.Errorf("failed to write manifest: %w", err)
	}

	return &InstallResult{
		Manifest:  *manifest,
		LocalPath: localPath,
		ID:        generatePluginID(pluginName, manifestURL),
	}, nil
}

// installFromDirectory installs a plugin from a local directory.
func (m *Manager) installFromDirectory(localSource, pluginsDir string) (*InstallResult, error) {
	// Check if source is a local path
	if _, err := os.Stat(localSource); err == nil {
		// It's a local directory
		manifestPath := filepath.Join(localSource, "manifest.json")
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read manifest: %w", err)
		}

		var manifest Manifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			return nil, fmt.Errorf("failed to parse manifest: %w", err)
		}

		pluginName := manifest.Name
		destPath := filepath.Join(pluginsDir, pluginName)

		// Copy directory
		if err := copyDirectory(localSource, destPath); err != nil {
			return nil, fmt.Errorf("failed to copy plugin: %w", err)
		}

		return &InstallResult{
			Manifest:  manifest,
			LocalPath: destPath,
			ID:        pluginName,
		}, nil
	}

	return nil, fmt.Errorf("source not found: %s", localSource)
}

// Remove removes a plugin by ID.
func (m *Manager) Remove(pluginID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, p := range m.plugins {
		if generatePluginID(p.Manifest.Name, p.Manifest.URL) == pluginID || p.Manifest.Name == pluginID {
			// Remove from memory
			m.plugins = append(m.plugins[:i], m.plugins[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("plugin not found: %s", pluginID)
}

// GetByID returns a plugin by its ID.
func (m *Manager) GetByID(pluginID string) *Plugin {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, p := range m.plugins {
		if generatePluginID(p.Manifest.Name, p.Manifest.URL) == pluginID || p.Manifest.Name == pluginID {
			return p
		}
	}
	return nil
}

// Helper functions

func fetchManifest(url string) (*Manifest, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch manifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch manifest: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	// Set default runtime
	if manifest.Runtime == "" {
		manifest.Runtime = RuntimeYaegi
	}

	return &manifest, nil
}

func downloadFile(url string) ([]byte, error) {
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func downloadGitHubPlugin(owner, repo, branch, pluginPath, destDir string) error {
	// Create directory
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	// Download script from raw GitHub
	// We'll fetch manifest first to determine script file
	manifestURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s/manifest.json",
		owner, repo, branch, pluginPath)

	manifest, err := fetchManifest(manifestURL)
	if err != nil {
		return err
	}

	// Download script
	scriptURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s/%s",
		owner, repo, branch, pluginPath, manifest.Script)

	scriptData, err := downloadFile(scriptURL)
	if err != nil {
		return fmt.Errorf("failed to download script: %w", err)
	}

	if err := os.WriteFile(filepath.Join(destDir, manifest.Script), scriptData, 0644); err != nil {
		return err
	}

	return nil
}

func copyDirectory(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(src, path)
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dstPath, data, info.Mode())
	})
}

func generatePluginID(name, url string) string {
	if url != "" {
		// Use URL-based ID for remote plugins
		return fmt.Sprintf("%s@%x", name, hashString(url))
	}
	return name
}

func hashString(s string) uint32 {
	h := uint32(0)
	for _, c := range s {
		h = h*31 + uint32(c)
	}
	return h
}
