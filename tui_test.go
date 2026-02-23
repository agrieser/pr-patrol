package main

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func testItems() []ClassifiedPR {
	return []ClassifiedPR{
		{State: StateNew, RepoName: "api", Number: 1, Title: "First PR", Author: "alice", RepoFullName: "org/api", CreatedAt: time.Now()},
		{State: StateCommented, RepoName: "web", Number: 2, Title: "Second PR", Author: "bob", RepoFullName: "org/web", CreatedAt: time.Now()},
		{State: StateStale, RepoName: "cli", Number: 3, Title: "Third PR", Author: "carol", RepoFullName: "org/cli", CreatedAt: time.Now()},
	}
}

func sendKey(m tea.Model, key rune) model {
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{key}})
	return updated.(model)
}

func sendMsg(m tea.Model, msg tea.Msg) model {
	updated, _ := m.Update(msg)
	return updated.(model)
}

func TestModel_Init(t *testing.T) {
	m := newModel(testItems())
	if m.cursor != 0 {
		t.Fatalf("expected cursor at 0, got %d", m.cursor)
	}
	if len(m.dismissed) != 0 {
		t.Fatalf("expected empty dismissed map")
	}
}

func TestModel_NavigateDown(t *testing.T) {
	m := sendKey(newModel(testItems()), 'j')
	if m.cursor != 1 {
		t.Fatalf("expected cursor at 1 after j, got %d", m.cursor)
	}
}

func TestModel_NavigateUp(t *testing.T) {
	m := sendKey(newModel(testItems()), 'j')
	m = sendKey(m, 'k')
	if m.cursor != 0 {
		t.Fatalf("expected cursor at 0 after j then k, got %d", m.cursor)
	}
}

func TestModel_NavigateUpAtTop(t *testing.T) {
	m := sendKey(newModel(testItems()), 'k')
	if m.cursor != 0 {
		t.Fatalf("expected cursor to stay at 0, got %d", m.cursor)
	}
}

func TestModel_NavigateDownAtBottom(t *testing.T) {
	var m tea.Model = newModel(testItems())
	for i := 0; i < 10; i++ {
		m = sendKey(m, 'j')
	}
	if m.(model).cursor != 2 {
		t.Fatalf("expected cursor clamped at 2, got %d", m.(model).cursor)
	}
}

func TestModel_Dismiss(t *testing.T) {
	m := sendKey(newModel(testItems()), 'd')
	if !m.dismissed[0] {
		t.Fatal("expected item 0 to be dismissed")
	}
	vis := m.visibleItems()
	if len(vis) != 2 {
		t.Fatalf("expected 2 visible items, got %d", len(vis))
	}
}

func TestModel_DismissAdjustsCursor(t *testing.T) {
	items := testItems()[:2] // 2 items
	m := sendKey(newModel(items), 'j')
	m = sendKey(m, 'd')
	if m.cursor != 0 {
		t.Fatalf("expected cursor adjusted to 0, got %d", m.cursor)
	}
}

func TestModel_ViewContainsItems(t *testing.T) {
	m := sendMsg(newModel(testItems()), tea.WindowSizeMsg{Width: 120, Height: 20})
	view := m.View()

	for _, want := range []string{"api#1", "web#2", "cli#3", "alice", "bob", "carol"} {
		if !strings.Contains(view, want) {
			t.Errorf("expected view to contain %q", want)
		}
	}
}

func TestModel_ViewContainsHelpBar(t *testing.T) {
	m := sendMsg(newModel(testItems()), tea.WindowSizeMsg{Width: 120, Height: 20})
	view := m.View()

	if !strings.Contains(view, "j/k: navigate") {
		t.Error("expected help bar in view")
	}
}

func TestModel_Quit(t *testing.T) {
	m := newModel(testItems())
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}
