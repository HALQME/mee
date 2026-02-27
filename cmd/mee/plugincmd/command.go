// Package plugincmd provides plugin management CLI commands.
package plugincmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/halqme/mee/pkg/database"
	"github.com/halqme/mee/pkg/platform"
	"github.com/halqme/mee/pkg/plugin"
)

// Command represents a plugin management command.
type Command struct {
	db *database.DB
	pm *plugin.Manager
}

// New creates a new plugin command.
func New(db *database.DB, pm *plugin.Manager) *Command {
	return &Command{db: db, pm: pm}
}

// Run executes the given subcommand.
func (c *Command) Run(args []string) error {
	if len(args) == 0 {
		return c.List()
	}

	switch args[0] {
	case "add", "install":
		if len(args) < 2 {
			return fmt.Errorf("usage: plugin add <url|path>")
		}
		return c.Add(args[1])
	case "list", "ls":
		return c.List()
	case "remove", "rm", "uninstall":
		if len(args) < 2 {
			return fmt.Errorf("usage: plugin remove <id|name>")
		}
		return c.Remove(args[1])
	case "enable":
		if len(args) < 2 {
			return fmt.Errorf("usage: plugin enable <id|name>")
		}
		return c.Enable(args[1])
	case "disable":
		if len(args) < 2 {
			return fmt.Errorf("usage: plugin disable <id|name>")
		}
		return c.Disable(args[1])
	case "info":
		if len(args) < 2 {
			return fmt.Errorf("usage: plugin info <id|name>")
		}
		return c.Info(args[1])
	case "update", "upgrade":
		var name string
		if len(args) >= 2 {
			name = args[1]
		}
		return c.Update(name)
	case "outdated":
		return c.Outdated()
	default:
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

// Add installs a plugin from URL or local path.
func (c *Command) Add(source string) error {
	pluginsDir := platform.PluginDir()
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		return fmt.Errorf("failed to create plugins directory: %w", err)
	}

	result, err := c.pm.InstallFromURL(source, pluginsDir)
	if err != nil {
		return fmt.Errorf("failed to install plugin: %w", err)
	}

	// Register in database
	store := database.NewPluginStore(c.db)
	pluginInfo := database.PluginInfo{
		ID:          result.ID,
		Name:        result.Manifest.Name,
		Version:     result.Manifest.Version,
		URL:         result.Manifest.URL,
		LocalPath:   result.LocalPath,
		Runtime:     string(result.Manifest.Runtime),
		Trigger:     result.Manifest.Trigger,
		Description: result.Manifest.Description,
		Enabled:     true,
	}

	if err := store.Insert(pluginInfo); err != nil {
		return fmt.Errorf("failed to register plugin: %w", err)
	}

	fmt.Printf("Installed: %s (%s)\n", result.Manifest.Name, result.ID)
	fmt.Printf("Path: %s\n", result.LocalPath)
	return nil
}

// List lists all installed plugins.
func (c *Command) List() error {
	store := database.NewPluginStore(c.db)
	plugins, err := store.List()
	if err != nil {
		return fmt.Errorf("failed to list plugins: %w", err)
	}

	if len(plugins) == 0 {
		fmt.Println("No plugins installed")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "NAME\tVERSION\tENABLED\tID\n")
	for _, p := range plugins {
		enabled := "yes"
		if !p.Enabled {
			enabled = "no"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", p.Name, p.Version, enabled, p.ID)
	}
	w.Flush()
	return nil
}

// Remove removes a plugin.
func (c *Command) Remove(idOrName string) error {
	store := database.NewPluginStore(c.db)

	// Get plugin info first
	p, err := store.Get(idOrName)
	if err != nil {
		return err
	}
	if p == nil {
		// Try by name
		p, err = store.GetByName(idOrName)
		if err != nil {
			return err
		}
		if p == nil {
			return fmt.Errorf("plugin not found: %s", idOrName)
		}
	}

	// Remove from filesystem
	if err := os.RemoveAll(p.LocalPath); err != nil {
		return fmt.Errorf("failed to remove plugin files: %w", err)
	}

	// Remove from database
	if err := store.Delete(p.ID); err != nil {
		return fmt.Errorf("failed to remove from registry: %w", err)
	}

	// Remove from manager
	if err := c.pm.Remove(p.ID); err != nil {
		// Non-fatal, plugin may not be loaded
	}

	fmt.Printf("Removed: %s\n", p.Name)
	return nil
}

// Enable enables a plugin.
func (c *Command) Enable(idOrName string) error {
	store := database.NewPluginStore(c.db)
	p, err := store.Get(idOrName)
	if err != nil {
		return err
	}
	if p == nil {
		p, err = store.GetByName(idOrName)
		if err != nil {
			return err
		}
	}
	if p == nil {
		return fmt.Errorf("plugin not found: %s", idOrName)
	}

	if err := store.Enable(p.ID); err != nil {
		return err
	}

	fmt.Printf("Enabled: %s\n", p.Name)
	return nil
}

// Disable disables a plugin.
func (c *Command) Disable(idOrName string) error {
	store := database.NewPluginStore(c.db)
	p, err := store.Get(idOrName)
	if err != nil {
		return err
	}
	if p == nil {
		p, err = store.GetByName(idOrName)
		if err != nil {
			return err
		}
	}
	if p == nil {
		return fmt.Errorf("plugin not found: %s", idOrName)
	}

	if err := store.Disable(p.ID); err != nil {
		return err
	}

	fmt.Printf("Disabled: %s\n", p.Name)
	return nil
}

// Info shows plugin details.
func (c *Command) Info(idOrName string) error {
	store := database.NewPluginStore(c.db)
	p, err := store.Get(idOrName)
	if err != nil {
		return err
	}
	if p == nil {
		p, err = store.GetByName(idOrName)
		if err != nil {
			return err
		}
	}
	if p == nil {
		return fmt.Errorf("plugin not found: %s", idOrName)
	}

	fmt.Printf("Name:        %s\n", p.Name)
	fmt.Printf("Version:     %s\n", p.Version)
	fmt.Printf("ID:         %s\n", p.ID)
	fmt.Printf("Trigger:    %s\n", p.Trigger)
	fmt.Printf("Runtime:    %s\n", p.Runtime)
	fmt.Printf("URL:        %s\n", p.URL)
	fmt.Printf("Path:       %s\n", p.LocalPath)
	fmt.Printf("Enabled:    %t\n", p.Enabled)
	fmt.Printf("Installed:  %s\n", p.InstalledAt.Format("2006-01-02 15:04"))

	// Show runtime status if loaded
	if loadedPlugin := c.pm.GetByID(p.ID); loadedPlugin != nil {
		fmt.Printf("Status:     ")
		if loadedPlugin.IsHealthy() {
			fmt.Printf("healthy\n")
		} else {
			fmt.Printf("error (%d errors)\n", loadedPlugin.ErrorCount())
			if err := loadedPlugin.LastError(); err != nil {
				fmt.Printf("Last Error: %s\n", err)
			}
		}
	}

	return nil
}

// Update updates a plugin or all plugins.
func (c *Command) Update(name string) error {
	if name == "" {
		return c.UpdateAll()
	}

	store := database.NewPluginStore(c.db)
	p, err := store.Get(name)
	if err != nil {
		return err
	}
	if p == nil {
		p, err = store.GetByName(name)
		if err != nil {
			return err
		}
	}
	if p == nil {
		return fmt.Errorf("plugin not found: %s", name)
	}

	if p.URL == "" {
		return fmt.Errorf("plugin %s has no remote URL (local plugin)", p.Name)
	}

	fmt.Printf("Updating %s...\n", p.Name)
	// TODO: Implement actual update logic
	fmt.Println("Update not yet implemented")
	return nil
}

// UpdateAll updates all outdated plugins.
func (c *Command) UpdateAll() error {
	fmt.Println("Checking for updates...")
	// TODO: Implement
	fmt.Println("No updates available")
	return nil
}

// Outdated lists outdated plugins.
func (c *Command) Outdated() error {
	fmt.Println("Checking for outdated plugins...")
	// TODO: Implement
	fmt.Println("No outdated plugins")
	return nil
}
