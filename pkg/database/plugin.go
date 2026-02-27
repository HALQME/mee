package database

import (
	"database/sql"
	"fmt"
	"time"
)

// PluginInfo represents stored plugin metadata.
type PluginInfo struct {
	ID          string
	Name        string
	Version     string
	URL         string
	LocalPath   string
	Runtime     string
	Trigger     string
	Description string
	Enabled     bool
	InstalledAt time.Time
	UpdatedAt   time.Time
}

// PluginStore handles plugin registry operations.
type PluginStore struct {
	db *DB
}

// NewPluginStore creates a new plugin store.
func NewPluginStore(db *DB) *PluginStore {
	return &PluginStore{db: db}
}

// Insert registers a new plugin in the database.
func (s *PluginStore) Insert(p PluginInfo) error {
	_, err := s.db.Exec(`
INSERT OR REPLACE INTO plugins (id, name, version, url, local_path, runtime, trigger, description, enabled)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.Version, p.URL, p.LocalPath, p.Runtime, p.Trigger, p.Description, p.Enabled)
	return err
}

// Get retrieves a plugin by ID.
func (s *PluginStore) Get(id string) (*PluginInfo, error) {
	var p PluginInfo
	var enabled int
	err := s.db.QueryRow(`
SELECT id, name, version, url, local_path, runtime, trigger, description, enabled, installed_at, updated_at
FROM plugins WHERE id = ?`, id).Scan(
		&p.ID, &p.Name, &p.Version, &p.URL, &p.LocalPath, &p.Runtime, &p.Trigger, &p.Description,
		&enabled, &p.InstalledAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.Enabled = enabled == 1
	return &p, nil
}

// GetByName retrieves a plugin by name.
func (s *PluginStore) GetByName(name string) (*PluginInfo, error) {
	var p PluginInfo
	var enabled int
	err := s.db.QueryRow(`
SELECT id, name, version, url, local_path, runtime, trigger, description, enabled, installed_at, updated_at
FROM plugins WHERE name = ?`, name).Scan(
		&p.ID, &p.Name, &p.Version, &p.URL, &p.LocalPath, &p.Runtime, &p.Trigger, &p.Description,
		&enabled, &p.InstalledAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.Enabled = enabled == 1
	return &p, nil
}

// List returns all registered plugins.
func (s *PluginStore) List() ([]PluginInfo, error) {
	rows, err := s.db.Query(`
SELECT id, name, version, url, local_path, runtime, trigger, description, enabled, installed_at, updated_at
FROM plugins ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var plugins []PluginInfo
	for rows.Next() {
		var p PluginInfo
		var enabled int
		if err := rows.Scan(
			&p.ID, &p.Name, &p.Version, &p.URL, &p.LocalPath, &p.Runtime, &p.Trigger, &p.Description,
			&enabled, &p.InstalledAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		p.Enabled = enabled == 1
		plugins = append(plugins, p)
	}
	return plugins, nil
}

// ListEnabled returns all enabled plugins.
func (s *PluginStore) ListEnabled() ([]PluginInfo, error) {
	rows, err := s.db.Query(`
SELECT id, name, version, url, local_path, runtime, trigger, description, enabled, installed_at, updated_at
FROM plugins WHERE enabled = 1 ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var plugins []PluginInfo
	for rows.Next() {
		var p PluginInfo
		var enabled int
		if err := rows.Scan(
			&p.ID, &p.Name, &p.Version, &p.URL, &p.LocalPath, &p.Runtime, &p.Trigger, &p.Description,
			&enabled, &p.InstalledAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		p.Enabled = true
		plugins = append(plugins, p)
	}
	return plugins, nil
}

// Delete removes a plugin from the registry.
func (s *PluginStore) Delete(id string) error {
	_, err := s.db.Exec(`DELETE FROM plugins WHERE id = ?`, id)
	return err
}

// Enable enables a plugin.
func (s *PluginStore) Enable(id string) error {
	_, err := s.db.Exec(`UPDATE plugins SET enabled = 1, updated_at = ? WHERE id = ?`, time.Now(), id)
	return err
}

// Disable disables a plugin.
func (s *PluginStore) Disable(id string) error {
	_, err := s.db.Exec(`UPDATE plugins SET enabled = 0, updated_at = ? WHERE id = ?`, time.Now(), id)
	return err
}

// UpdateVersion updates the version of a plugin.
func (s *PluginStore) UpdateVersion(id, version string) error {
	_, err := s.db.Exec(`UPDATE plugins SET version = ?, updated_at = ? WHERE id = ?`, version, time.Now(), id)
	return err
}

// Count returns the total number of plugins.
func (s *PluginStore) Count() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM plugins`).Scan(&count)
	return count, err
}

// GeneratePluginID generates a unique ID for a plugin.
func GeneratePluginID(name, url string) string {
	if url != "" {
		// Use URL hash for remote plugins
		return fmt.Sprintf("%s@%s", name, hashURL(url))
	}
	// Use name for local plugins
	return name
}

func hashURL(url string) string {
	// Simple hash for ID generation
	if len(url) > 16 {
		return url[len(url)-16:]
	}
	return url
}
