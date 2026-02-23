# mee

A fast, extensible terminal launcher with a plugin system written in Go.

## Features

- **Terminal UI** - Clean interface powered by Bubble Tea
- **Plugin System** - Extend functionality with Go plugins (Yaegi interpreter)
- **Parallel Search** - All plugins execute concurrently for fast results
- **Trigger-based Filtering** - Plugins with triggers only run when needed
- **Fuzzy Matching** - Built-in scoring for search results
- **Cross-platform** - macOS, Linux, Windows support

## Installation

```bash
go build -o mee ./mee.go
```

## Usage

### Interactive Mode

```bash
./mee
```

### Command Line

```bash
# Direct query
./mee "Safari"

# Pipe input
echo "100+50" | ./mee
```

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `↑` `↓` | Navigate results |
| `Tab` | Complete trigger |
| `Enter` | Select item |
| `Esc` / `Ctrl+C` | Quit |

## Built-in Plugins

| Plugin | Description | Trigger |
|--------|-------------|---------|
| apps | Launch applications | - |
| calc | Calculator | - |
| web | Web search | - |
| emoji | Emoji search | - |
| unixtime | Unix timestamp converter | - |

## Configuration

Config file: `~/.config/mee/config.json`

```json
{
  "display": {
    "max_results": 50,
    "list_height": 15
  },
  "plugin_dirs": [
    "/path/to/mee/plugins",
    "~/.config/mee/plugins"
  ]
}
```

## Creating Plugins

### Plugin Structure

```
plugins/myplugin/
├── manifest.json
└── myplugin.go
```

### manifest.json

```json
{
  "name": "myplugin",
  "version": "1.0.0",
  "trigger": ">",        // Optional: prefix to activate plugin
  "script": "myplugin.go",
  "description": "My custom plugin"
}
```

### myplugin.go

```go
package main

import (
	"encoding/json"
	"strings"
)

type ResultItem struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	Action   string `json:"action"`
	Payload  string `json:"payload"`
	Score    int    `json:"score"`
}

type ResultSet struct {
	Items        []ResultItem `json:"items"`
	ProviderName string       `json:"provider_name"`
}

// Required: Search function
func Search(query string) string {
	// Early return if this plugin doesn't handle the query
	if !shouldHandle(query) {
		return ""
	}

	rs := ResultSet{
		Items: []ResultItem{{
			ID:       "myplugin:result",
			Title:    "Result Title",
			Subtitle: "Subtitle",
			Action:   "open",   // "open", "launch", "copy", "print", "trigger"
			Payload:  "value",
			Score:    100,
		}},
		ProviderName: "myplugin",
	}

	data, _ := json.Marshal(rs)
	return string(data)
}

func shouldHandle(query string) bool {
	// Your logic here
	return strings.HasPrefix(query, ">")
}
```

### Actions

| Action | Description |
|--------|-------------|
| `open` / `launch` | Open URL or launch application |
| `copy` | Copy payload to clipboard |
| `print` | Print to stdout |
| `trigger` | Activate another plugin trigger |

### Tips for Fast Plugins

1. **Early return** - Check if query is relevant before processing
2. **Keep it simple** - Avoid expensive operations
3. **Use triggers** - Set a trigger prefix to avoid unnecessary calls

## Architecture

```
mee.go                 # Entry point
pkg/
├── core/
│   ├── config.go      # Configuration loader
│   └── ranker.go      # Search ranking with parallel execution
├── platform/
│   └── platform.go    # OS-specific utilities
├── plugin/
│   └── plugin.go      # Yaegi-based plugin manager
├── provider/
│   └── provider.go    # Provider interface & fuzzy matching
└── tui/
    └── tui.go         # Terminal UI (Bubble Tea)
plugins/               # Built-in plugins
```

## Performance

- **Parallel Execution**: All plugins search concurrently via goroutines
- **Trigger Filtering**: Plugins with triggers only activate on matching prefixes
- **Shared Symbols**: Interpreter stdlib symbols are shared across plugins

## License

MIT
