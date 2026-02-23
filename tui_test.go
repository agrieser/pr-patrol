package main

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func withURL(url string) func(*PRNode) {
	return func(pr *PRNode) {
		pr.URL = url
	}
}

// testModelConfig returns a config with 3 raw PRs (alice=untouched, bob=commented, carol=stale)
// plus a self-authored PR (hidden by default).
func testModelConfig() modelConfig {
	reviewTime := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	commitTime := time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC)

	return modelConfig{
		rawPRs: []PRNode{
			makePR(withAuthor("alice"), withURL("https://github.com/org/repo/pull/1")),
			makePR(withAuthor("bob"), withComment("me"), withURL("https://github.com/org/repo/pull/2")),
			makePR(withAuthor("carol"), withReview("me", "APPROVED", reviewTime), withLastCommit(commitTime), withURL("https://github.com/org/repo/pull/3")),
			makePR(withAuthor("me"), withURL("https://github.com/org/repo/pull/4")),
		},
		me:       "me",
		myTeams:  make(map[string]bool),
		showSelf: false,
		showMine: false,
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
	m := newModel(testModelConfig())
	if m.cursor != 0 {
		t.Fatalf("expected cursor at 0, got %d", m.cursor)
	}
	if len(m.dismissed) != 0 {
		t.Fatalf("expected empty dismissed map")
	}
	// Should have 3 items (self-authored excluded)
	if len(m.items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(m.items))
	}
}

func TestModel_NavigateDown(t *testing.T) {
	m := sendKey(newModel(testModelConfig()), 'j')
	if m.cursor != 1 {
		t.Fatalf("expected cursor at 1 after j, got %d", m.cursor)
	}
}

func TestModel_NavigateUp(t *testing.T) {
	m := sendKey(newModel(testModelConfig()), 'j')
	m = sendKey(m, 'k')
	if m.cursor != 0 {
		t.Fatalf("expected cursor at 0 after j then k, got %d", m.cursor)
	}
}

func TestModel_NavigateUpAtTop(t *testing.T) {
	m := sendKey(newModel(testModelConfig()), 'k')
	if m.cursor != 0 {
		t.Fatalf("expected cursor to stay at 0, got %d", m.cursor)
	}
}

func TestModel_NavigateDownAtBottom(t *testing.T) {
	var m tea.Model = newModel(testModelConfig())
	for i := 0; i < 10; i++ {
		m = sendKey(m, 'j')
	}
	if m.(model).cursor != 2 {
		t.Fatalf("expected cursor clamped at 2, got %d", m.(model).cursor)
	}
}

func TestModel_Dismiss(t *testing.T) {
	m := newModel(testModelConfig())
	firstURL := m.items[0].URL
	m = sendKey(m, 'd')
	if !m.dismissed[firstURL] {
		t.Fatal("expected first item to be dismissed by URL")
	}
	vis := m.visibleItems()
	if len(vis) != 2 {
		t.Fatalf("expected 2 visible items, got %d", len(vis))
	}
}

func TestModel_DismissAdjustsCursor(t *testing.T) {
	cfg := testModelConfig()
	cfg.rawPRs = cfg.rawPRs[:2] // 2 non-self PRs
	m := newModel(cfg)
	m = sendKey(m, 'j') // move to second item
	m = sendKey(m, 'd') // dismiss it
	if m.cursor != 0 {
		t.Fatalf("expected cursor adjusted to 0, got %d", m.cursor)
	}
}

func TestModel_ViewContainsItems(t *testing.T) {
	m := sendMsg(newModel(testModelConfig()), tea.WindowSizeMsg{Width: 120, Height: 20})
	view := m.View()

	for _, want := range []string{"alice", "bob", "carol"} {
		if !strings.Contains(view, want) {
			t.Errorf("expected view to contain %q", want)
		}
	}
}

func TestModel_ViewContainsHelpBar(t *testing.T) {
	m := sendMsg(newModel(testModelConfig()), tea.WindowSizeMsg{Width: 120, Height: 20})
	view := m.View()

	if !strings.Contains(view, "j/k: navigate") {
		t.Error("expected help bar in view")
	}
}

func TestModel_ViewContainsIndicators(t *testing.T) {
	m := sendMsg(newModel(testModelConfig()), tea.WindowSizeMsg{Width: 120, Height: 20})
	view := m.View()
	for _, want := range []string{"alice", "bob", "carol"} {
		if !strings.Contains(view, want) {
			t.Errorf("expected view to contain %q", want)
		}
	}
	if strings.Contains(view, "[NEW]") {
		t.Error("view should not contain old [NEW] tags")
	}
}

func TestModel_Quit(t *testing.T) {
	m := newModel(testModelConfig())
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}

func TestModel_ToggleSelf(t *testing.T) {
	m := newModel(testModelConfig())
	initialCount := len(m.visibleItems())
	if initialCount != 3 {
		t.Fatalf("expected 3 items initially, got %d", initialCount)
	}

	// Toggle self on — should now include self-authored PR
	m = sendKey(m, 's')
	if !m.showSelf {
		t.Fatal("expected showSelf to be true after toggle")
	}
	if len(m.visibleItems()) != 4 {
		t.Fatalf("expected 4 items with self included, got %d", len(m.visibleItems()))
	}

	// Toggle self off — back to original
	m = sendKey(m, 's')
	if m.showSelf {
		t.Fatal("expected showSelf to be false after second toggle")
	}
	if len(m.visibleItems()) != 3 {
		t.Fatalf("expected 3 items after toggling self off, got %d", len(m.visibleItems()))
	}
}

func TestModel_ToggleMine(t *testing.T) {
	cfg := testModelConfig()
	// Add a review request to the first PR for "me"
	cfg.rawPRs[0].ReviewRequests.Nodes = []ReviewRequestNode{
		{RequestedReviewer: struct {
			Login string `json:"login"`
			Slug  string `json:"slug"`
		}{Login: "me"}},
	}
	m := newModel(cfg)

	// Toggle mine on — only the PR with review request for "me"
	m = sendKey(m, 'm')
	if !m.showMine {
		t.Fatal("expected showMine to be true")
	}
	if len(m.visibleItems()) != 1 {
		t.Fatalf("expected 1 item with mine filter, got %d", len(m.visibleItems()))
	}

	// Toggle mine off — all non-self PRs again
	m = sendKey(m, 'm')
	if len(m.visibleItems()) != 3 {
		t.Fatalf("expected 3 items after turning mine off, got %d", len(m.visibleItems()))
	}
}

func TestModel_HelpBarShowsToggleState(t *testing.T) {
	cfg := testModelConfig()
	cfg.rawPRs[0].ReviewRequests.Nodes = []ReviewRequestNode{
		{RequestedReviewer: struct {
			Login string `json:"login"`
			Slug  string `json:"slug"`
		}{Login: "me"}},
	}
	m := newModel(cfg)
	m = sendMsg(m, tea.WindowSizeMsg{Width: 120, Height: 20})

	view := m.View()
	if !strings.Contains(view, "self:off") {
		t.Error("expected help bar to show self:off")
	}
	if !strings.Contains(view, "mine:off") {
		t.Error("expected help bar to show mine:off")
	}

	m = sendKey(m, 's')
	view = m.View()
	if !strings.Contains(view, "self:on") {
		t.Error("expected help bar to show self:on after toggle")
	}

	m = sendKey(m, 'm')
	view = m.View()
	if !strings.Contains(view, "mine:on") {
		t.Error("expected help bar to show mine:on after toggle")
	}
}

func TestModel_EmptyFilterShowsMessage(t *testing.T) {
	cfg := testModelConfig()
	cfg.showMine = true // no review requests, so everything filtered out
	m := newModel(cfg)
	m = sendMsg(m, tea.WindowSizeMsg{Width: 120, Height: 20})

	view := m.View()
	if !strings.Contains(view, "s/m") {
		t.Error("expected empty view to hint about toggle keys")
	}
}

func TestModel_DismissPersistsAcrossToggle(t *testing.T) {
	m := newModel(testModelConfig())
	dismissedURL := m.items[0].URL
	initialCount := len(m.visibleItems())

	// Dismiss the first item
	m = sendKey(m, 'd')
	if !m.dismissed[dismissedURL] {
		t.Fatal("expected PR to be dismissed")
	}
	if len(m.visibleItems()) != initialCount-1 {
		t.Fatal("expected one fewer visible item after dismiss")
	}

	// Toggle self on and off — dismissal should persist
	m = sendKey(m, 's')
	m = sendKey(m, 's')
	if !m.dismissed[dismissedURL] {
		t.Fatal("expected dismissal to persist across toggle")
	}
	if len(m.visibleItems()) != initialCount-1 {
		t.Fatal("expected dismissed item to stay hidden after toggle round-trip")
	}
}

func TestModel_ToggleResetsCursor(t *testing.T) {
	m := newModel(testModelConfig())
	m = sendKey(m, 'j') // move cursor down
	if m.cursor != 1 {
		t.Fatalf("expected cursor at 1, got %d", m.cursor)
	}
	m = sendKey(m, 's') // toggle
	if m.cursor != 0 {
		t.Fatalf("expected cursor reset to 0 after toggle, got %d", m.cursor)
	}
}

func TestModel_AuthorToggle(t *testing.T) {
	cfg := testModelConfig()
	m := newModel(cfg)
	initialCount := len(m.visibleItems())

	// Toggle author mode
	m = sendKey(m, 'a')
	if !m.showAuthor {
		t.Fatal("expected showAuthor true")
	}
	// Author mode: only "me" PR visible
	if len(m.visibleItems()) != 1 {
		t.Fatalf("expected 1 item in author mode, got %d", len(m.visibleItems()))
	}

	// Toggle back
	m = sendKey(m, 'a')
	if m.showAuthor {
		t.Fatal("expected showAuthor false")
	}
	if len(m.visibleItems()) != initialCount {
		t.Fatalf("expected %d items back in reviewer mode, got %d", initialCount, len(m.visibleItems()))
	}
}

func TestModel_AuthorModeDisablesSM(t *testing.T) {
	m := newModel(testModelConfig())
	m = sendKey(m, 'a') // enter author mode
	m = sendKey(m, 's') // should be ignored
	if m.showSelf {
		t.Fatal("s should be ignored in author mode")
	}
	m = sendKey(m, 'm') // should be ignored
	if m.showMine {
		t.Fatal("m should be ignored in author mode")
	}
}

func TestModel_HelpBarShowsAuthorToggle(t *testing.T) {
	m := sendMsg(newModel(testModelConfig()), tea.WindowSizeMsg{Width: 120, Height: 20})
	view := m.View()
	if !strings.Contains(view, "a:") {
		t.Error("expected help bar to show 'a:' key")
	}
	if !strings.Contains(view, "author:off") {
		t.Error("expected author:off in help bar")
	}

	m = sendKey(m, 'a')
	view = m.View()
	if !strings.Contains(view, "author:on") {
		t.Error("expected author:on in help bar")
	}
}

func TestModel_LoadingState(t *testing.T) {
	cfg := testModelConfig()
	cfg.rawPRs = nil
	cfg.loading = true
	m := newModel(cfg)
	m = sendMsg(m, tea.WindowSizeMsg{Width: 120, Height: 20})
	view := m.View()
	if !strings.Contains(view, "Fetching") {
		t.Error("expected loading message in view")
	}
}

func TestModel_RefreshKey(t *testing.T) {
	cfg := testModelConfig()
	cfg.org = "testorg"
	m := newModel(cfg)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m2 := updated.(model)
	if !m2.loading {
		t.Fatal("expected loading=true after pressing r")
	}
	if cmd == nil {
		t.Fatal("expected non-nil command from r key")
	}
}

func TestModel_DataLoaded(t *testing.T) {
	cfg := testModelConfig()
	cfg.rawPRs = nil
	cfg.loading = true
	m := newModel(cfg)

	m = sendMsg(m, dataLoadedMsg{
		prs:     testModelConfig().rawPRs,
		me:      "me",
		myTeams: make(map[string]bool),
	})
	if m.loading {
		t.Fatal("expected loading=false after data loaded")
	}
	if len(m.items) == 0 {
		t.Fatal("expected items to be populated after data loaded")
	}
}

func TestFormatIndicators_ReviewerMode(t *testing.T) {
	pr := ClassifiedPR{MyReview: MyNone, OthReview: OthApproved, Activity: ActMine}
	result := formatIndicators(pr, false)
	if result == "" {
		t.Fatal("expected non-empty indicator string")
	}
}

func TestModel_LegendToggle(t *testing.T) {
	m := newModel(testModelConfig())
	m = sendMsg(m, tea.WindowSizeMsg{Width: 120, Height: 30})

	// Press ? to show legend
	m = sendKey(m, '?')
	if !m.showHelp {
		t.Fatal("expected showHelp=true after pressing ?")
	}
	view := m.View()
	if !strings.Contains(view, "Your Review") {
		t.Error("expected legend to contain 'Your Review'")
	}
	if !strings.Contains(view, "Others' Reviews") {
		t.Error("expected legend to contain 'Others' Reviews'")
	}
	if !strings.Contains(view, "Comments") {
		t.Error("expected legend to contain 'Comments'")
	}
	if !strings.Contains(view, "Press any key") {
		t.Error("expected legend to contain dismiss hint")
	}

	// Any key dismisses it
	m = sendKey(m, 'j')
	if m.showHelp {
		t.Fatal("expected showHelp=false after pressing any key")
	}
	view = m.View()
	if strings.Contains(view, "Your Review") {
		t.Error("expected legend to be hidden after dismiss")
	}
}

func TestModel_HelpBarShowsLegendKey(t *testing.T) {
	m := sendMsg(newModel(testModelConfig()), tea.WindowSizeMsg{Width: 120, Height: 20})
	view := m.View()
	if !strings.Contains(view, "?: legend") {
		t.Error("expected help bar to show '?: legend'")
	}
}

func TestFormatIndicators_AuthorMode(t *testing.T) {
	pr := ClassifiedPR{IsDraft: true, OthReview: OthNone, Activity: ActNone}
	result := formatIndicators(pr, true)
	if result == "" {
		t.Fatal("expected non-empty indicator string")
	}
}
