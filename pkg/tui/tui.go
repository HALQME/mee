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

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED")).Padding(0, 1)
	inputStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Padding(0, 1)
	selectStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#7C3AED")).Bold(true)
	itemStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	subStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
	helpStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
)

// Model is the TUI model.
type Model struct {
	input    textinput.Model
	viewport viewport.Model
	ranker   *core.Ranker
	results  []provider.ResultItem
	selected int
	config   core.Config
	quitting bool
}

// New creates a TUI model.
func New(config core.Config, ranker *core.Ranker) Model {
	ti := textinput.New()
	ti.Placeholder = " Search..."
	ti.Focus()
	ti.Width = 50

	return Model{
		input:    ti,
		viewport: viewport.New(60, config.Display.ListHeight),
		ranker:   ranker,
		config:   config,
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
			b.WriteString(selectStyle.Render("▶ "+item.Title) + "\n")
		} else {
			b.WriteString(itemStyle.Render("  "+item.Title) + "\n")
		}
		b.WriteString(subStyle.Render("    "+item.Subtitle) + "\n\n")
	}
	m.viewport.SetContent(b.String())
}

// View implements tea.Model.
func (m Model) View() string {
	if m.quitting {
		return ""
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render("mee") + "\n\n")
	b.WriteString(inputStyle.Render(m.input.View()) + "\n\n")
	if len(m.results) > 0 {
		b.WriteString(m.viewport.View())
	} else {
		b.WriteString(itemStyle.Render("  No results..."))
	}
	b.WriteString("\n\n" + helpStyle.Render("↑↓ Nav  Tab Complete  ⏎ Select  ⎋ Quit"))
	return b.String()
}

// Selected returns the selected item.
func (m Model) Selected() *provider.ResultItem {
	if m.selected < len(m.results) {
		return &m.results[m.selected]
	}
	return nil
}
