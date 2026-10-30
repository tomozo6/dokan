package ui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func selection(t *testing.T, options ...string) selectionModel {
	t.Helper()
	m, err := newSelection("Choose:", options)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func update(m selectionModel, msg tea.Msg) (selectionModel, tea.Cmd) {
	next, cmd := m.Update(msg)
	return next.(selectionModel), cmd
}

func key(code rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: code} }

func TestSelectionFiltersAndReturnsOriginalName(t *testing.T) {
	m := selection(t, "db-Prod", "EC2-PROD", "staging")
	m, _ = update(m, tea.KeyPressMsg{Text: "PROD"})
	if strings.Join(m.matches, ",") != "db-Prod,EC2-PROD" {
		t.Fatalf("unexpected matches: %v", m.matches)
	}
	m, _ = update(m, key(tea.KeyTab))
	m, cmd := update(m, key(tea.KeyEnter))
	value, err := m.value()
	if cmd == nil || err != nil || value == nil || *value != "EC2-PROD" {
		t.Fatalf("selection = %v, %v; quit = %v", value, err, cmd != nil)
	}
	if ansi.Strip(m.View().Content) != "✓ Choose: EC2-PROD\n" || m.View().AltScreen {
		t.Fatalf("unexpected final view: %+v", m.View())
	}
}

func TestSelectionNoMatchesCanRecover(t *testing.T) {
	m := selection(t, "東京", "大阪")
	m, _ = update(m, tea.PasteMsg{Content: "東\n京X"})
	m, cmd := update(m, key(tea.KeyEnter))
	if cmd != nil || m.selected || !strings.Contains(m.View().Content, "No matching candidates") {
		t.Fatal("Enter must not select an empty result")
	}
	m, _ = update(m, key(tea.KeyBackspace))
	if m.query != "東京" || len(m.matches) != 1 || m.matches[0] != "東京" {
		t.Fatalf("cannot recover after backspace: %+v", m)
	}
	m, _ = update(m, key(tea.KeyBackspace))
	if m.query != "東" {
		t.Fatalf("backspace split a Unicode character: %q", m.query)
	}
}

func TestSelectionNavigationScrollsAndWraps(t *testing.T) {
	m := selection(t, "0", "1", "2", "3", "4", "5", "6", "7", "8")
	for range 8 {
		m, _ = update(m, key(tea.KeyDown))
	}
	if m.cursor != 8 || m.top != 2 || strings.Contains(ansi.Strip(m.View().Content), "\n    0\n") || !strings.Contains(ansi.Strip(m.View().Content), "▸ 8") {
		t.Fatalf("selected item not visible: %+v\n%s", m, m.View().Content)
	}
	m, _ = update(m, key(tea.KeyTab))
	if m.cursor != 0 || m.top != 0 {
		t.Fatalf("down did not wrap: %+v", m)
	}
	m, _ = update(m, tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	if m.cursor != 8 || m.top != 2 {
		t.Fatalf("shift+tab did not wrap: %+v", m)
	}
	m, _ = update(m, tea.WindowSizeMsg{Width: 20, Height: 7})
	if m.height != 2 || m.top != 7 {
		t.Fatalf("resize hid selection: %+v", m)
	}
	m, _ = update(m, key(tea.KeyUp))
	if m.cursor != 7 {
		t.Fatalf("up did not move: %+v", m)
	}
}

func TestSelectionLettersAreSearchNotCommands(t *testing.T) {
	m := selection(t, "qjk", "other")
	for _, r := range "qjk" {
		m, _ = update(m, tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	if m.canceled || m.query != "qjk" || len(m.matches) != 1 {
		t.Fatalf("letters should filter: %+v", m)
	}
	m, _ = update(m, tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	if m.query != "" || len(m.matches) != 2 || m.cursor != 0 {
		t.Fatalf("clear did not restore candidates: %+v", m)
	}
}

func TestSelectionCancelDoesNotReturnAValue(t *testing.T) {
	for _, msg := range []tea.KeyPressMsg{key(tea.KeyEscape), {Code: 'c', Mod: tea.ModCtrl}} {
		m := selection(t, "a")
		m, cmd := update(m, msg)
		value, err := m.value()
		if cmd == nil || value != nil || !errors.Is(err, errSelectionCanceled) || m.View().Content != "" {
			t.Fatalf("unexpected cancellation: value=%v, err=%v, model=%+v", value, err, m)
		}
	}
}

func TestAskRejectsEmptyCandidates(t *testing.T) {
	for _, ask := range []func([]string) (*string, error){AskEC2, AskDB} {
		value, err := ask(nil)
		if value != nil || err == nil || !strings.Contains(err.Error(), "no candidates available") {
			t.Fatalf("empty candidates: value=%v, err=%v", value, err)
		}
	}
}
