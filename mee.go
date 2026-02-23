package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/halqme/mee/pkg/core"
	"github.com/halqme/mee/pkg/platform"
	"github.com/halqme/mee/pkg/plugin"
	"github.com/halqme/mee/pkg/tui"
)

func main() {
	config := core.Load()
	ranker := core.NewRanker()

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

func execAction(action, payload string) {
	switch action {
	case "open", "launch":
		platform.Open(payload)
	}
}
