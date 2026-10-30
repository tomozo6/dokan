package ui

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/term"
)

var errSelectionCanceled = errors.New("selection canceled")

type selectionModel struct {
	prompt   string
	options  []string
	matches  []string
	query    string
	cursor   int
	top      int
	height   int
	width    int
	compact  bool
	selected bool
	canceled bool
}

func newSelection(prompt string, options []string) (selectionModel, error) {
	if len(options) == 0 {
		return selectionModel{}, fmt.Errorf("%s no candidates available", prompt)
	}
	return selectionModel{prompt: prompt, options: options, matches: options, height: 7}, nil
}

func askSelection(prompt string, options []string) (*string, error) {
	model, err := newSelection(prompt, options)
	if err != nil {
		return nil, err
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return nil, errors.New("selection requires an interactive terminal")
	}
	result, err := tea.NewProgram(model).Run()
	if err != nil {
		return nil, fmt.Errorf("%s %w", prompt, err)
	}
	return result.(selectionModel).value()
}

func (m selectionModel) value() (*string, error) {
	if m.canceled || !m.selected {
		return nil, errSelectionCanceled
	}
	value := m.matches[m.cursor]
	return &value, nil
}

func (m selectionModel) Init() tea.Cmd { return nil }

func (m selectionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.compact = msg.Height < 12
		overhead := 9
		if m.compact {
			overhead = 5
		}
		m.height = max(1, min(7, msg.Height-overhead))
	case tea.PasteMsg:
		m.filter(m.query + printable(msg.Content))
	case tea.KeyPressMsg:
		switch msg.Keystroke() {
		case "ctrl+c", "esc":
			m.canceled = true
			return m, tea.Quit
		case "enter":
			if len(m.matches) > 0 {
				m.selected = true
				return m, tea.Quit
			}
		case "up", "shift+tab":
			if len(m.matches) > 0 {
				m.cursor = (m.cursor + len(m.matches) - 1) % len(m.matches)
			}
		case "down", "tab":
			if len(m.matches) > 0 {
				m.cursor = (m.cursor + 1) % len(m.matches)
			}
		case "backspace":
			runes := []rune(m.query)
			if len(runes) > 0 {
				m.filter(string(runes[:len(runes)-1]))
			}
		case "ctrl+u", "ctrl+w":
			m.filter("")
		default:
			if msg.Text != "" {
				m.filter(m.query + printable(msg.Text))
			}
		}
	}
	if m.cursor < m.top {
		m.top = m.cursor
	} else if m.cursor >= m.top+m.height {
		m.top = m.cursor - m.height + 1
	}
	return m, nil
}

func printable(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsPrint(r) {
			return r
		}
		return -1
	}, s)
}

func (m *selectionModel) filter(query string) {
	m.query = query
	m.matches = nil
	for _, option := range m.options {
		if strings.Contains(strings.ToLower(option), strings.ToLower(query)) {
			m.matches = append(m.matches, option)
		}
	}
	m.cursor, m.top = 0, 0
}

var (
	selectionBadge = lipgloss.NewStyle().Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).Background(lipgloss.Color("#7C3AED"))
	selectionTitle  = lipgloss.NewStyle().Bold(true)
	selectionAccent = lipgloss.NewStyle().Foreground(lipgloss.Cyan)
	selectionMuted  = lipgloss.NewStyle().Foreground(lipgloss.BrightBlack)
	selectionActive = lipgloss.NewStyle().Bold(true).
			Foreground(lipgloss.Color("#E0F2FE")).Background(lipgloss.Color("#164E63"))
	selectionSuccess = lipgloss.NewStyle().Foreground(lipgloss.Green).Bold(true)
)

func (m selectionModel) View() tea.View {
	if m.canceled {
		return tea.NewView("")
	}
	if m.selected {
		return tea.NewView(m.line(selectionSuccess.Render("✓ ")+m.prompt+" "+
			selectionTitle.Render(m.matches[m.cursor])) + "\n")
	}
	var lines []string
	if !m.compact {
		lines = append(lines, "  "+selectionBadge.Render(" DOKAN ")+selectionMuted.Render(" / CONNECT"))
	}
	lines = append(lines, "  "+selectionTitle.Render(m.prompt))
	if !m.compact {
		lines = append(lines, "")
	}
	query := selectionAccent.Render(m.query)
	if m.query == "" {
		query = selectionMuted.Render("Type to filter…")
	}
	lines = append(lines, "  "+selectionAccent.Render("› ")+query)
	if !m.compact {
		lines = append(lines, "")
	}
	rowWidth := 64
	if m.width > 0 {
		rowWidth = max(1, min(rowWidth, m.width-2))
	}
	if len(m.matches) == 0 {
		lines = append(lines, "  "+selectionMuted.Render("No matching candidates · edit your search"))
	}
	for i := m.top; i < min(len(m.matches), m.top+m.height); i++ {
		row := "  " + m.matches[i]
		if i == m.cursor {
			row = selectionActive.Width(rowWidth).Render(ansi.Truncate("▸ "+m.matches[i], rowWidth, "…"))
		}
		lines = append(lines, "  "+row)
	}
	position := 0
	if len(m.matches) > 0 {
		position = m.cursor + 1
	}
	count := fmt.Sprintf("%d / %d", position, len(m.matches))
	if m.query != "" {
		count += fmt.Sprintf(" · %d total", len(m.options))
	}
	if m.top > 0 {
		count += " · ↑ more"
	}
	if m.top+m.height < len(m.matches) {
		count += " · ↓ more"
	}
	if !m.compact {
		lines = append(lines, "")
	}
	lines = append(lines, "  "+selectionMuted.Render(count))
	if !m.compact {
		key := func(s string) string { return selectionAccent.Render(s) }
		lines = append(lines, "  "+key("↑↓/tab")+selectionMuted.Render(" move   ")+
			key("enter")+selectionMuted.Render(" select   ")+key("esc/ctrl+c")+selectionMuted.Render(" cancel"))
	}
	for i, line := range lines {
		lines[i] = m.line(line)
	}
	return tea.NewView(strings.Join(lines, "\n"))
}

func (m selectionModel) line(s string) string {
	if m.width > 0 {
		return ansi.Truncate(s, m.width, "…")
	}
	return s
}
