package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/halqme/mee/cmd/history"
	"github.com/halqme/mee/cmd/plugin"
	"github.com/halqme/mee/pkg/core"
	"github.com/halqme/mee/pkg/database"
	"github.com/halqme/mee/pkg/platform"
	"github.com/halqme/mee/pkg/plugin"
	"github.com/halqme/mee/pkg/setup"
	"github.com/halqme/mee/pkg/tui"
)

func main() {
	// Parse flags before checking subcommands
	setupCmd := flag.NewFlagSet("setup", flag.ContinueOnError)
	forceFlag := setupCmd.Bool("force", false, "overwrite existing config")
	withPluginsFlag := setupCmd.Bool("with-plugins", false, "install default plugins")
	pluginDirFlag := setupCmd.String("plugin-dir", "", "source plugin directory")

	// Check for subcommands
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "setup":
			setupCmd.Parse(os.Args[2:])
			opts := setup.Options{
				Force:       *forceFlag,
				WithPlugins: *withPluginsFlag,
				PluginDir:   *pluginDirFlag,
			}
			if err := setup.Run(opts); err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}
			return
		case "check":
			if err := setup.Check(); err != nil {
				os.Exit(1)
			}
			return
		case "plugin", "plugins":
			runPluginCommand()
			return
		case "history":
			runHistoryCommand()
			return
		case "--help", "-h":
			printHelp()
			return
		case "--version", "-v":
			printVersion()
			return
		}
	}

	// Run main TUI or direct mode
	runMain()
}

func runMain() {
	config := core.Load()
	ranker := core.NewRanker()

	// Open database for history recording
	db, err := database.Open(platform.DataDir() + "/mee.db")
	if err == nil {
		defer db.Close()
		db.Migrate() // ignore error, DB might already exist
		ranker = core.NewRankerWithHistory(db)
	}

	pm := plugin.NewManager()
	for _, dir := range config.PluginDirs {
		pm.LoadFromDirectory(dir)
	}
	for _, p := range pm.GetProviders() {
		ranker.AddProvider(p)
	}
	for _, t := range pm.GetTriggers() {
		ranker.AddTrigger(t)
	}

	stat, _ := os.Stdin.Stat()
	piped := (stat.Mode() & os.ModeCharDevice) == 0

	if piped || len(os.Args) > 1 {
		var q string
		if piped {
			if s := bufio.NewScanner(os.Stdin); s.Scan() {
				q = strings.TrimSpace(s.Text())
			}
		}
		if len(os.Args) > 1 {
			q = strings.TrimSpace(os.Args[1])
		}
		if q != "" {
			if r := ranker.Search(q); len(r) > 0 {
				fmt.Println(r[0].Payload)
				execAction(r[0].Action, r[0].Payload)
				// Record in history
				if db != nil {
					ranker.RecordSelection("", q, r[0].Payload)
				}
			}
		}
		return
	}

	m := tui.New(config, ranker)
	p := tea.NewProgram(m, tea.WithAltScreen())
	f, err := p.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if m, ok := f.(tui.Model); ok {
		if item := m.Selected(); item != nil {
			fmt.Println(item.Payload)
			execAction(item.Action, item.Payload)
		}
	}
}

func runPluginCommand() {
	db, err := database.Open(platform.DataDir() + "/mee.db")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		fmt.Fprintf(os.Stderr, "Migration error: %v\n", err)
		os.Exit(1)
	}

	pm := plugin.NewManager()
	// Load existing plugins from config dirs
	config := core.Load()
	for _, dir := range config.PluginDirs {
		pm.LoadFromDirectory(dir)
	}

	cmd := plugincmd.New(db, pm)
	if err := cmd.Run(os.Args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runHistoryCommand() {
	db, err := database.Open(platform.DataDir() + "/mee.db")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		fmt.Fprintf(os.Stderr, "Migration error: %v\n", err)
		os.Exit(1)
	}

	cmd := historycmd.New(db)
	if err := cmd.Run(os.Args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func execAction(action, payload string) {
	switch action {
	case "open", "launch":
		platform.Open(payload)
	}
}

func printHelp() {
	fmt.Println(`mee - Fast, extensible launcher

Usage:
  mee [query]           Launch TUI or search directly
  mee setup [options]   Initialize mee
  mee check             Check setup status
  mee plugin <command>  Manage plugins
  mee history <command> Manage search history

Setup Options:
  --force              Overwrite existing config
  --with-plugins       Install default plugins
  --plugin-dir <dir>   Source plugin directory

Plugin Commands:
  add <url|path>       Install a plugin from URL or local path
  list                 List installed plugins
  remove <id|name>     Remove a plugin
  enable <id|name>     Enable a plugin
  disable <id|name>    Disable a plugin
  info <id|name>       Show plugin details
  update [id|name]     Update plugin(s)

History Commands:
  list                 Show recent history
  search <query>       Search history
  top, popular         Show most used queries
  clear                Clear all history
  prune [days]         Remove old entries
  export [file]        Export to JSON
  import <file>        Import from JSON

Examples:
  mee setup --with-plugins              # Initial setup with plugins
  mee check                              # Check setup status
  mee "calc"                            # Direct search
  mee plugin add https://github.com/... # Install plugin
  mee history top                       # Show popular queries

Options:
  -h, --help     Show this help
  -v, --version  Show version`)
}

func printVersion() {
	fmt.Println("mee version 0.2.0")
}
