package database

import (
	"database/sql"
	"time"
)

// HistoryItem represents a search history entry.
type HistoryItem struct {
	ID             int
	PluginID       string
	Query          string
	SelectedItem   string
	SelectedAt     time.Time
	SelectionCount int
}

// HistoryStore handles history operations.
type HistoryStore struct {
	db *DB
}

// NewHistoryStore creates a new history store.
func NewHistoryStore(db *DB) *HistoryStore {
	return &HistoryStore{db: db}
}

// Record records a selection in history.
// If the same query was selected before, increments the count.
func (s *HistoryStore) Record(pluginID, query, selectedItem string) error {
	// Check if this query exists for this plugin
	var existingID int
	var existingCount int
	err := s.db.QueryRow(
		`SELECT id, selection_count FROM history WHERE plugin_id = ? AND query = ?`,
		pluginID, query).Scan(&existingID, &existingCount)

	if err == sql.ErrNoRows {
		// New entry
		_, err = s.db.Exec(`
INSERT INTO history (plugin_id, query, selected_item, selected_at, selection_count)
VALUES (?, ?, ?, ?, 1)`, pluginID, query, selectedItem, time.Now())
		return err
	}

	if err != nil {
		return err
	}

	// Update existing entry
	_, err = s.db.Exec(`
UPDATE history SET selection_count = ?, selected_item = ?, selected_at = ?
WHERE id = ?`,
		existingCount+1, selectedItem, time.Now(), existingID)
	return err
}

// GetFrequency returns the selection count for a query.
func (s *HistoryStore) GetFrequency(query string) (int, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COALESCE(SUM(selection_count), 0) FROM history WHERE query = ?`,
		query).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// GetRecent returns recent history items.
func (s *HistoryStore) GetRecent(limit int) ([]HistoryItem, error) {
	rows, err := s.db.Query(`
SELECT id, plugin_id, query, selected_item, selected_at, selection_count
FROM history
ORDER BY selected_at DESC
LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []HistoryItem
	for rows.Next() {
		var item HistoryItem
		var pluginID, selectedItem sql.NullString
		if err := rows.Scan(
			&item.ID, &pluginID, &item.Query, &selectedItem,
			&item.SelectedAt, &item.SelectionCount); err != nil {
			return nil, err
		}
		if pluginID.Valid {
			item.PluginID = pluginID.String
		}
		if selectedItem.Valid {
			item.SelectedItem = selectedItem.String
		}
		items = append(items, item)
	}
	return items, nil
}

// GetByPlugin returns history items for a specific plugin.
func (s *HistoryStore) GetByPlugin(pluginID string, limit int) ([]HistoryItem, error) {
	rows, err := s.db.Query(`
SELECT id, plugin_id, query, selected_item, selected_at, selection_count
FROM history
WHERE plugin_id = ?
ORDER BY selection_count DESC, selected_at DESC
LIMIT ?`, pluginID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []HistoryItem
	for rows.Next() {
		var item HistoryItem
		var selectedItem sql.NullString
		if err := rows.Scan(
			&item.ID, &item.PluginID, &item.Query, &selectedItem,
			&item.SelectedAt, &item.SelectionCount); err != nil {
			return nil, err
		}
		if selectedItem.Valid {
			item.SelectedItem = selectedItem.String
		}
		items = append(items, item)
	}
	return items, nil
}

// GetPopular returns the most frequently selected items.
func (s *HistoryStore) GetPopular(limit int) ([]HistoryItem, error) {
	rows, err := s.db.Query(`
SELECT id, plugin_id, query, selected_item, selected_at, selection_count
FROM history
ORDER BY selection_count DESC
LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []HistoryItem
	for rows.Next() {
		var item HistoryItem
		var pluginID, selectedItem sql.NullString
		if err := rows.Scan(
			&item.ID, &pluginID, &item.Query, &selectedItem,
			&item.SelectedAt, &item.SelectionCount); err != nil {
			return nil, err
		}
		if pluginID.Valid {
			item.PluginID = pluginID.String
		}
		if selectedItem.Valid {
			item.SelectedItem = selectedItem.String
		}
		items = append(items, item)
	}
	return items, nil
}

// Clear clears all history.
func (s *HistoryStore) Clear() error {
	_, err := s.db.Exec(`DELETE FROM history`)
	return err
}

// Prune removes old history entries.
func (s *HistoryStore) Prune(olderThan time.Duration) error {
	threshold := time.Now().Add(-olderThan)
	_, err := s.db.Exec(`DELETE FROM history WHERE selected_at < ?`, threshold)
	return err
}

// GetScore returns a boost score for a query based on history.
// Returns a score from 0-50 to add to the base score.
func (s *HistoryStore) GetScore(query string) (uint8, error) {
	freq, err := s.GetFrequency(query)
	if err != nil {
		return 0, err
	}

	// Cap at 50 points max boost
	if freq >= 10 {
		return 50, nil
	}
	// Linear scaling: 5 points per selection
	return uint8(freq * 5), nil
}

// GetRecencyBoost returns a boost score based on how recently an item was selected.
// More recent selections get higher boost (max 30 points).
func (s *HistoryStore) GetRecencyBoost(query string) (uint8, error) {
	var selectedAt sql.NullTime
	err := s.db.QueryRow(
		`SELECT MAX(selected_at) FROM history WHERE query = ?`,
		query).Scan(&selectedAt)
	if err != nil {
		return 0, err
	}

	if !selectedAt.Valid {
		return 0, nil
	}

	// Calculate recency boost (30 points for today, diminishing)
	hoursAgo := time.Since(selectedAt.Time).Hours()
	switch {
	case hoursAgo < 24: // Last 24 hours
		return 30, nil
	case hoursAgo < 72: // Last 3 days
		return 20, nil
	case hoursAgo < 168: // Last week
		return 10, nil
	default:
		return 0, nil
	}
}
