// Package setup provides Mee initialization and setup functionality.
package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/halqme/mee/pkg/core"
	"github.com/halqme/mee/pkg/database"
	"github.com/halqme/mee/pkg/platform"
	"github.com/halqme/mee/pkg/plugin"
)

// Options holds setup options.
type Options struct {
	Force       bool   // overwrite existing config
	WithPlugins bool   // install default plugins
	PluginDir   string // source plugin directory to copy from
}

// Run performs the initial setup.
func Run(opts Options) error {
	fmt.Println("Setting up mee...")

	// Create directories
	if err := createDirectories(); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	// Create default config
	if err := createConfig(opts.Force); err != nil {
		return fmt.Errorf("failed to create config: %w", err)
	}

	// Initialize database
	db, err := initDatabase()
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer db.Close()

	// Install default plugins if requested
	if opts.WithPlugins && opts.PluginDir != "" {
		if err := installDefaultPlugins(db, opts.PluginDir); err != nil {
			return fmt.Errorf("failed to install plugins: %w", err)
		}
	}

	fmt.Println("\nSetup complete!")
	fmt.Println("\nDirectories created:")
	fmt.Printf("  Config:  %s\n", platform.ConfigDir())
	fmt.Printf("  Data:    %s\n", platform.DataDir())
	fmt.Printf("  Cache:   %s\n", platform.CacheDir())
	fmt.Printf("  Plugins: %s\n", platform.PluginDir())

	fmt.Println("\nNext steps:")
	fmt.Println("  mee                    # Launch TUI")
	fmt.Println("  mee 'query'            # Direct search")
	fmt.Println("  mee plugin add <url>   # Install a plugin")
	fmt.Println("  mee -h                 # Show help")

	return nil
}

func createDirectories() error {
	dirs := []string{
		platform.ConfigDir(),
		platform.DataDir(),
		platform.CacheDir(),
		platform.PluginDir(),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return nil
}

func createConfig(force bool) error {
	configPath := platform.ConfigDir() + "/config.yaml"

	// Check if config exists
	if !force {
		if _, err := os.Stat(configPath); err == nil {
			fmt.Printf("Config already exists at %s (skipping)\n", configPath)
			return nil
		}
	}

	// Create default config
	cfg := core.Config{
		App: core.AppConfig{
			Mode:     "interactive",
			Startup:  false,
			LogLevel: "info",
		},
		Display: core.DisplayConfig{
			Theme:      "auto",
			MaxResults: 50,
			ListHeight: 15,
		},
		Plugins: core.PluginsConfig{
			Dirs: []string{
				platform.ConfigDir() + "/plugins",
			},
			RuntimeDefault: "yaegi",
		},
		Search: core.SearchConfig{
			FuzzyThreshold: 0.7,
			HistoryBoost:   true,
		},
		Storage: core.StorageConfig{
			DBPath: platform.DataDir() + "/mee.db",
		},
		Registry: core.RegistryConfig{
			CacheDir: platform.CacheDir() + "/plugins",
		},
	}

	if err := cfg.Save(); err != nil {
		return err
	}

	fmt.Printf("Created config: %s\n", configPath)
	return nil
}

func initDatabase() (*database.DB, error) {
	dbPath := platform.DataDir() + "/mee.db"

	db, err := database.Open(dbPath)
	if err != nil {
		return nil, err
	}

	if err := db.Migrate(); err != nil {
		db.Close()
		return nil, err
	}

	fmt.Printf("Initialized database: %s\n", dbPath)
	return db, nil
}

func installDefaultPlugins(db *database.DB, sourceDir string) error {
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return fmt.Errorf("failed to read plugin source: %w", err)
	}

	pluginStore := database.NewPluginStore(db)
	installed := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		srcPath := filepath.Join(sourceDir, entry.Name())
		destPath := filepath.Join(platform.PluginDir(), entry.Name())

		// Check if plugin already exists
		if _, err := os.Stat(destPath); err == nil {
			continue
		}

		// Copy plugin
		if err := copyDir(srcPath, destPath); err != nil {
			fmt.Printf("Warning: failed to copy %s: %v\n", entry.Name(), err)
			continue
		}

		// Read manifest and register in database
		manifestPath := filepath.Join(destPath, "manifest.json")
		manifestData, err := os.ReadFile(manifestPath)
		if err != nil {
			fmt.Printf("Warning: failed to read manifest for %s: %v\n", entry.Name(), err)
			continue
		}

		var m plugin.Manifest
		if err := json.Unmarshal(manifestData, &m); err != nil {
			fmt.Printf("Warning: failed to parse manifest for %s: %v\n", entry.Name(), err)
			continue
		}

		// Register in database
		pluginID := m.Name
		if m.URL != "" {
			pluginID = fmt.Sprintf("%s@%s", m.Name, m.URL)
		}

		pluginInfo := database.PluginInfo{
			ID:          pluginID,
			Name:        m.Name,
			Version:     m.Version,
			URL:         m.URL,
			LocalPath:   destPath,
			Runtime:     string(m.Runtime),
			Trigger:     m.Trigger,
			Description: m.Description,
			Enabled:     true,
		}

		if err := pluginStore.Insert(pluginInfo); err != nil {
			fmt.Printf("Warning: failed to register %s: %v\n", entry.Name(), err)
			continue
		}

		fmt.Printf("Installed plugin: %s\n", entry.Name())
		installed++
	}

	if installed > 0 {
		fmt.Printf("Installed %d plugin(s)\n", installed)
	}

	return nil
}

func copyDir(src, dst string) error {
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

// Check checks if mee is properly set up.
func Check() error {
	issues := []string{}

	// Check config directory
	configDir := platform.ConfigDir()
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		issues = append(issues, "Config directory not found")
	}

	// Check config file
	configPath := configDir + "/config.yaml"
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		issues = append(issues, "Config file not found")
	}

	// Check database
	dbPath := platform.DataDir() + "/mee.db"
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		issues = append(issues, "Database not found")
	}

	// Check plugins directory
	pluginsDir := platform.PluginDir()
	if _, err := os.Stat(pluginsDir); os.IsNotExist(err) {
		issues = append(issues, "Plugins directory not found")
	}

	if len(issues) > 0 {
		fmt.Println("Mee setup check found issues:")
		for _, issue := range issues {
			fmt.Printf("  - %s\n", issue)
		}
		fmt.Println("\nRun 'mee setup' to initialize mee")
		return fmt.Errorf("setup incomplete")
	}

	// Check database connection
	db, err := database.Open(dbPath)
	if err != nil {
		issues = append(issues, fmt.Sprintf("Cannot open database: %v", err))
	} else {
		db.Close()
	}

	if len(issues) > 0 {
		fmt.Println("Mee has issues:")
		for _, issue := range issues {
			fmt.Printf("  - %s\n", issue)
		}
		return fmt.Errorf("setup has issues")
	}

	fmt.Println("Mee is properly set up!")
	return nil
}
