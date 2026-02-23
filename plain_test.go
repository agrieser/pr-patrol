package main

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestRenderPlain(t *testing.T) {
	items := []ClassifiedPR{
		{
			State:    StateNew,
			RepoName: "api",
			Number:   42,
			Title:    "Add endpoint",
			Author:   "alice",
		},
		{
			State:    StateStale,
			RepoName: "web",
			Number:   7,
			Title:    "Fix layout",
			Author:   "bob",
		},
	}

	var buf bytes.Buffer
	renderPlain(&buf, items)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), buf.String())
	}

	// Columns are padded: repo to 6 (api#42), author to 5 (alice)
	expected0 := "[NEW] api#42  alice  Add endpoint"
	if lines[0] != expected0 {
		t.Fatalf("line 0:\ngot:  %q\nwant: %q", lines[0], expected0)
	}

	expected1 := "[STL] web#7   bob    Fix layout"
	if lines[1] != expected1 {
		t.Fatalf("line 1:\ngot:  %q\nwant: %q", lines[1], expected1)
	}
}

func TestRenderPlain_Empty(t *testing.T) {
	var buf bytes.Buffer
	renderPlain(&buf, nil)
	if buf.Len() != 0 {
		t.Fatalf("expected empty output, got %q", buf.String())
	}
}

func TestRenderPlain_AllStates(t *testing.T) {
	items := []ClassifiedPR{
		{State: StateNew, RepoName: "r", Number: 1, Title: "t", Author: "a", CreatedAt: time.Now()},
		{State: StateCommented, RepoName: "r", Number: 2, Title: "t", Author: "a", CreatedAt: time.Now()},
		{State: StateDismissed, RepoName: "r", Number: 3, Title: "t", Author: "a", CreatedAt: time.Now()},
		{State: StateStale, RepoName: "r", Number: 4, Title: "t", Author: "a", CreatedAt: time.Now()},
	}

	var buf bytes.Buffer
	renderPlain(&buf, items)

	output := buf.String()
	for _, tag := range []string{"[NEW]", "[CMT]", "[DIS]", "[STL]"} {
		if !strings.Contains(output, tag) {
			t.Errorf("expected output to contain %s", tag)
		}
	}
}
