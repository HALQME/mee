// Package historycmd provides history management CLI commands.
package historycmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/halqme/mee/pkg/database"
)

// Command represents a history management command.
type Command struct {
	db *database.DB
}

// New creates a new history command.
func New(db *database.DB) *Command {
	return &Command{db: db}
}

// Run executes the given subcommand.
func (c *Command) Run(args []string) error {
	if len(args) == 0 {
		return c.List()
	}

	switch args[0] {
	case "list", "ls":
		return c.List()
	case "search":
		if len(args) < 2 {
			return fmt.Errorf("usage: history search <query>")
		}
		return c.Search(args[1])
	case "clear", "clean":
		return c.Clear()
	case "prune":
		var days int
		if len(args) >= 2 {
			fmt.Sscanf(args[1], "%d", &days)
		}
		if days == 0 {
			days = 30 // default 30 days
		}
		return c.Prune(days)
	case "export":
		var file string
		if len(args) >= 2 {
			file = args[1]
		}
		return c.Export(file)
	case "import":
		if len(args) < 2 {
			return fmt.Errorf("usage: history import <file>")
		}
		return c.Import(args[1])
	case "top", "popular":
		return c.Popular()
	default:
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

// List lists recent history.
func (c *Command) List() error {
	store := database.NewHistoryStore(c.db)
	items, err := store.GetRecent(20)
	if err != nil {
		return fmt.Errorf("failed to get history: %w", err)
	}

	if len(items) == 0 {
		fmt.Println("No history")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "QUERY\tCOUNT\tLAST USED\n")
	for _, item := range items {
		fmt.Fprintf(w, "%s\t%d\t%s\n",
			item.Query,
			item.SelectionCount,
			item.SelectedAt.Format("2006-01-02 15:04"))
	}
	w.Flush()
	return nil
}

// Search searches history for a query.
func (c *Command) Search(query string) error {
	store := database.NewHistoryStore(c.db)

	// Get recent items and filter
	items, err := store.GetRecent(100)
	if err != nil {
		return fmt.Errorf("failed to get history: %w", err)
	}

	fmt.Printf("Searching history for: %s\n\n", query)

	var found int
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "QUERY\tCOUNT\tLAST USED\n")
	for _, item := range items {
		if containsIgnoreCase(item.Query, query) {
			fmt.Fprintf(w, "%s\t%d\t%s\n",
				item.Query,
				item.SelectionCount,
				item.SelectedAt.Format("2006-01-02 15:04"))
			found++
		}
	}
	w.Flush()

	if found == 0 {
		fmt.Println("No matching history found")
	}
	return nil
}

// Popular shows most frequently used queries.
func (c *Command) Popular() error {
	store := database.NewHistoryStore(c.db)
	items, err := store.GetPopular(20)
	if err != nil {
		return fmt.Errorf("failed to get history: %w", err)
	}

	if len(items) == 0 {
		fmt.Println("No history")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "QUERY\tCOUNT\tLAST USED\n")
	for _, item := range items {
		fmt.Fprintf(w, "%s\t%d\t%s\n",
			item.Query,
			item.SelectionCount,
			item.SelectedAt.Format("2006-01-02 15:04"))
	}
	w.Flush()
	return nil
}

// Clear clears all history.
func (c *Command) Clear() error {
	fmt.Print("Clear all history? This cannot be undone. (y/N): ")
	var confirm string
	fmt.Scanln(&confirm)
	if confirm != "y" && confirm != "Y" {
		fmt.Println("Cancelled")
		return nil
	}

	store := database.NewHistoryStore(c.db)
	if err := store.Clear(); err != nil {
		return fmt.Errorf("failed to clear history: %w", err)
	}

	fmt.Println("History cleared")
	return nil
}

// Prune removes history older than specified days.
func (c *Command) Prune(days int) error {
	store := database.NewHistoryStore(c.db)
	olderThan := time.Duration(days) * 24 * time.Hour

	fmt.Printf("Removing history older than %d days...\n", days)
	if err := store.Prune(olderThan); err != nil {
		return fmt.Errorf("failed to prune history: %w", err)
	}

	fmt.Println("Done")
	return nil
}

// Export exports history to a JSON file.
func (c *Command) Export(filename string) error {
	store := database.NewHistoryStore(c.db)
	items, err := store.GetRecent(1000)
	if err != nil {
		return fmt.Errorf("failed to get history: %w", err)
	}

	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal history: %w", err)
	}

	if filename == "" {
		// Print to stdout
		fmt.Println(string(data))
		return nil
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Printf("Exported %d entries to %s\n", len(items), filename)
	return nil
}

// Import imports history from a JSON file.
func (c *Command) Import(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	var items []database.HistoryItem
	if err := json.Unmarshal(data, &items); err != nil {
		return fmt.Errorf("failed to parse file: %w", err)
	}

	store := database.NewHistoryStore(c.db)
	imported := 0
	for _, item := range items {
		if err := store.Record(item.PluginID, item.Query, item.SelectedItem); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to import %s: %v\n", item.Query, err)
			continue
		}
		imported++
	}

	fmt.Printf("Imported %d entries\n", imported)
	return nil
}

func containsIgnoreCase(s, substr string) bool {
	s = strings.ToLower(s)
	substr = strings.ToLower(substr)
	return strings.Contains(s, substr)
}
