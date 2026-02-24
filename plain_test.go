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
			MyReview: MyNone, OthReview: OthApproved, Activity: ActMine,
			RepoName: "api",
			Number:   42,
			Title:    "Add endpoint",
			Author:   "alice",
		},
		{
			MyReview: MyStale, OthReview: OthNone, Activity: ActNone,
			RepoName: "web",
			Number:   7,
			Title:    "Fix layout",
			Author:   "bob",
		},
	}

	var buf bytes.Buffer
	renderPlain(&buf, items, SortPriority)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), buf.String())
	}

	// Columns are padded: repo to 6 (api#42), author to 5 (alice), age 4 chars right-aligned
	expected0 := "· ✓ ● api#42  alice     -  Add endpoint"
	if lines[0] != expected0 {
		t.Fatalf("line 0:\ngot:  %q\nwant: %q", lines[0], expected0)
	}

	expected1 := "~ · · web#7   bob       -  Fix layout"
	if lines[1] != expected1 {
		t.Fatalf("line 1:\ngot:  %q\nwant: %q", lines[1], expected1)
	}
}

func TestRenderPlain_Empty(t *testing.T) {
	var buf bytes.Buffer
	renderPlain(&buf, nil, SortPriority)
	if buf.Len() != 0 {
		t.Fatalf("expected empty output, got %q", buf.String())
	}
}

func TestRenderPlain_AllIndicators(t *testing.T) {
	items := []ClassifiedPR{
		{MyReview: MyNone, OthReview: OthNone, Activity: ActNone, RepoName: "r", Number: 1, Title: "t", Author: "a"},
		{MyReview: MyApproved, OthReview: OthApproved, Activity: ActOthers, RepoName: "r", Number: 2, Title: "t", Author: "a"},
		{MyReview: MyChanges, OthReview: OthChanges, Activity: ActMine, RepoName: "r", Number: 3, Title: "t", Author: "a"},
		{MyReview: MyStale, OthReview: OthMixed, Activity: ActNone, RepoName: "r", Number: 4, Title: "t", Author: "a"},
	}

	var buf bytes.Buffer
	renderPlain(&buf, items, SortPriority)

	output := buf.String()
	// Should NOT contain old tags
	if strings.Contains(output, "[NEW]") || strings.Contains(output, "[STL]") {
		t.Error("plain output should not contain old tags")
	}
	// Should contain indicator characters
	for _, ch := range []string{"·", "✓", "✗", "~", "±", "○", "●"} {
		if !strings.Contains(output, ch) {
			t.Errorf("expected output to contain %s", ch)
		}
	}
}

func TestFormatAge(t *testing.T) {
	cases := []struct {
		dur  time.Duration
		want string
	}{
		{30 * time.Minute, "now"},
		{2 * time.Hour, "2h"},
		{9 * time.Hour, "9h"},
		{10 * time.Hour, "1d"},
		{3 * 24 * time.Hour, "3d"},
		{10 * 24 * time.Hour, "1w"},
		{22 * 24 * time.Hour, "3w"},
		{45 * 24 * time.Hour, "1m"},
		{180 * 24 * time.Hour, "6m"},
		{400 * 24 * time.Hour, "1y"},
	}
	for _, tc := range cases {
		got := formatAge(time.Now().Add(-tc.dur))
		if got != tc.want {
			t.Errorf("formatAge(-%v) = %q, want %q", tc.dur, got, tc.want)
		}
	}

	// Zero time
	if got := formatAge(time.Time{}); got != " -" {
		t.Errorf("formatAge(zero) = %q, want %q", got, " -")
	}
}

func TestRenderPlain_NewFormat(t *testing.T) {
	items := []ClassifiedPR{
		{
			MyReview: MyNone, OthReview: OthApproved, Activity: ActMine,
			RepoName: "repo", Number: 42, Author: "alice", Title: "Add feature",
		},
		{
			MyReview: MyStale, OthReview: OthNone, Activity: ActNone,
			RepoName: "repo", Number: 43, Author: "bob", Title: "Fix bug",
		},
	}
	var buf bytes.Buffer
	renderPlain(&buf, items, SortPriority)
	output := buf.String()

	if strings.Contains(output, "[NEW]") || strings.Contains(output, "[STL]") {
		t.Error("plain output should not contain old tags")
	}
	if !strings.Contains(output, "alice") {
		t.Error("expected alice in output")
	}
	if !strings.Contains(output, "bob") {
		t.Error("expected bob in output")
	}
}
