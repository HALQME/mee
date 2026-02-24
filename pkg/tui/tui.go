package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/halqme/mee/pkg/core"
	"github.com/halqme/mee/pkg/provider"
)

// styles holds the current styles.
type styles struct {
	title lipgloss.Style
	input lipgloss.Style
	mark  lipgloss.Style
	item  lipgloss.Style
	sub   lipgloss.Style
	help  lipgloss.Style
}

// defaultStyles returns adaptive styles that follow terminal theme.
func defaultStyles() styles {
	return styles{
		title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#A78BFA"}).
			Padding(0, 1),
		input: lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"}).
			Padding(0, 1),
		mark: lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#A78BFA"}).
			Bold(true),
		item: lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#666666", Dark: "#888888"}),
		sub: lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#999999", Dark: "#666666"}),
		help: lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#999999", Dark: "#666666"}),
	}
}

// applyColorConfig applies custom colors from config, preserving adaptive fallback.
func applyColorConfig(s styles, cfg *core.ColorConfig) styles {
	if cfg == nil {
		return s
	}
	if cfg.Title != "" {
		s.title = s.title.Foreground(lipgloss.Color(cfg.Title))
	}
	if cfg.Input != "" {
		s.input = s.input.Foreground(lipgloss.Color(cfg.Input))
	}
	if cfg.Mark != "" {
		s.mark = s.mark.Foreground(lipgloss.Color(cfg.Mark))
	}
	if cfg.Item != "" {
		s.item = s.item.Foreground(lipgloss.Color(cfg.Item))
	}
	if cfg.Sub != "" {
		s.sub = s.sub.Foreground(lipgloss.Color(cfg.Sub))
	}
	if cfg.Help != "" {
		s.help = s.help.Foreground(lipgloss.Color(cfg.Help))
	}
	return s
}

// Model is the TUI model.
type Model struct {
	input    textinput.Model
	viewport viewport.Model
	ranker   *core.Ranker
	results  []provider.ResultItem
	selected int
	config   core.Config
	styles   styles
	quitting bool
}

// New creates a TUI model.
func New(config core.Config, ranker *core.Ranker) Model {
	ti := textinput.New()
	ti.Placeholder = " Search..."
	ti.Focus()
	ti.Width = 50

	s := applyColorConfig(defaultStyles(), config.Colors)

	return Model{
		input:    ti,
		viewport: viewport.New(60, config.Display.ListHeight),
		ranker:   ranker,
		config:   config,
		styles:   s,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return textinput.Blink }

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEscape:
			m.quitting = true
			return m, tea.Quit
		case tea.KeyEnter:
			return m, tea.Quit
		case tea.KeyUp:
			if m.selected > 0 {
				m.selected--
				m.render()
			}
		case tea.KeyDown:
			if m.selected < len(m.results)-1 {
				m.selected++
				m.render()
			}
		case tea.KeyTab:
			if m.selected < len(m.results) && m.results[m.selected].Action == "trigger" {
				m.input.SetValue(m.results[m.selected].Payload)
				m.input.CursorEnd()
				m.search(m.results[m.selected].Payload)
			}
		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			m.search(m.input.Value())
			return m, cmd
		}
	}
	return m, nil
}

func (m *Model) search(q string) {
	m.results = m.ranker.Search(q)
	m.selected = 0
	m.ranker.Limit(m.config.Display.MaxResults)
	m.results = m.ranker.Results()
	m.render()
}

func (m *Model) render() {
	var b strings.Builder
	for i, item := range m.results {
		if i == m.selected {
			b.WriteString(m.styles.mark.Render("▶ "+item.Title) + "\n")
		} else {
			b.WriteString(m.styles.item.Render("  "+item.Title) + "\n")
		}
		b.WriteString(m.styles.sub.Render("    "+item.Subtitle) + "\n\n")
	}
	m.viewport.SetContent(b.String())
}

// View implements tea.Model.
func (m Model) View() string {
	if m.quitting {
		return ""
	}
	var b strings.Builder
	b.WriteString(m.styles.title.Render("mee") + "\n\n")
	b.WriteString(m.styles.input.Render(m.input.View()) + "\n\n")
	if len(m.results) > 0 {
		b.WriteString(m.viewport.View())
	} else {
		b.WriteString(m.styles.item.Render("  No results..."))
	}
	b.WriteString("\n\n" + m.styles.help.Render("↑↓ Nav  Tab Complete  ⏎ Select  ⎋ Quit"))
	return b.String()
}

// Selected returns the selected item.
func (m Model) Selected() *provider.ResultItem {
	if m.selected < len(m.results) {
		return &m.results[m.selected]
	}
	return nil
}
