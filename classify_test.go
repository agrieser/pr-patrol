package main

import (
	"testing"
	"time"
)

func makePR(opts ...func(*PRNode)) PRNode {
	pr := PRNode{
		Title:  "Test PR",
		URL:    "https://github.com/org/repo/pull/1",
		Number: 1,
	}
	pr.Author.Login = "other"
	pr.Repository.Name = "repo"
	pr.Repository.NameWithOwner = "org/repo"
	pr.CreatedAt = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	// Default: one commit from a week ago
	pr.Commits.Nodes = []CommitNode{{}}
	pr.Commits.Nodes[0].Commit.CommittedDate = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, opt := range opts {
		opt(&pr)
	}
	return pr
}

func withReview(login, state string, at time.Time) func(*PRNode) {
	return func(pr *PRNode) {
		r := ReviewNode{State: state, SubmittedAt: at}
		r.Author.Login = login
		pr.Reviews.Nodes = append(pr.Reviews.Nodes, r)
	}
}

func withComment(login string) func(*PRNode) {
	return func(pr *PRNode) {
		c := CommentNode{}
		c.Author.Login = login
		pr.Comments.Nodes = append(pr.Comments.Nodes, c)
	}
}

func withAuthor(login string) func(*PRNode) {
	return func(pr *PRNode) {
		pr.Author.Login = login
	}
}

func withLastCommit(at time.Time) func(*PRNode) {
	return func(pr *PRNode) {
		pr.Commits.Nodes = []CommitNode{{}}
		pr.Commits.Nodes[0].Commit.CommittedDate = at
	}
}

func TestClassify_New(t *testing.T) {
	pr := makePR()
	state, include := classify(pr, "me")
	if !include {
		t.Fatal("expected PR to be included")
	}
	if state != StateNew {
		t.Fatalf("expected NEW, got %s", state)
	}
}

func TestClassify_NewWithOtherReviews(t *testing.T) {
	pr := makePR(
		withReview("someone-else", "APPROVED", time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)),
	)
	state, include := classify(pr, "me")
	if !include {
		t.Fatal("expected PR to be included")
	}
	if state != StateNew {
		t.Fatalf("expected NEW, got %s", state)
	}
}

func TestClassify_Commented(t *testing.T) {
	pr := makePR(withComment("me"))
	state, include := classify(pr, "me")
	if !include {
		t.Fatal("expected PR to be included")
	}
	if state != StateCommented {
		t.Fatalf("expected CMT, got %s", state)
	}
}

func TestClassify_Dismissed(t *testing.T) {
	pr := makePR(
		withReview("me", "DISMISSED", time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)),
	)
	state, include := classify(pr, "me")
	if !include {
		t.Fatal("expected PR to be included")
	}
	if state != StateDismissed {
		t.Fatalf("expected DIS, got %s", state)
	}
}

func TestClassify_Stale(t *testing.T) {
	reviewTime := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	commitTime := time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC)
	pr := makePR(
		withReview("me", "APPROVED", reviewTime),
		withLastCommit(commitTime),
	)
	state, include := classify(pr, "me")
	if !include {
		t.Fatal("expected PR to be included")
	}
	if state != StateStale {
		t.Fatalf("expected STL, got %s", state)
	}
}

func TestClassify_CurrentReview_Skipped(t *testing.T) {
	reviewTime := time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC)
	commitTime := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	pr := makePR(
		withReview("me", "APPROVED", reviewTime),
		withLastCommit(commitTime),
	)
	_, include := classify(pr, "me")
	if include {
		t.Fatal("expected PR to be skipped (review is current)")
	}
}

func TestClassify_PendingReview_TreatedAsNew(t *testing.T) {
	pr := makePR(
		withReview("me", "PENDING", time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)),
	)
	state, include := classify(pr, "me")
	if !include {
		t.Fatal("expected PR to be included")
	}
	if state != StateNew {
		t.Fatalf("expected NEW (pending review ignored), got %s", state)
	}
}

func TestClassify_DeletedAuthor(t *testing.T) {
	pr := makePR()
	// Review from deleted user (empty login)
	r := ReviewNode{State: "APPROVED", SubmittedAt: time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)}
	r.Author.Login = ""
	pr.Reviews.Nodes = append(pr.Reviews.Nodes, r)

	state, include := classify(pr, "me")
	if !include {
		t.Fatal("expected PR to be included")
	}
	if state != StateNew {
		t.Fatalf("expected NEW (deleted author review ignored), got %s", state)
	}
}

func TestClassifyAll_ExcludesSelf(t *testing.T) {
	prs := []PRNode{
		makePR(withAuthor("me")),
		makePR(withAuthor("other")),
	}
	result := classifyAll(prs, "me", false)
	if len(result) != 1 {
		t.Fatalf("expected 1 PR (self excluded), got %d", len(result))
	}
	if result[0].Author != "other" {
		t.Fatalf("expected 'other', got %s", result[0].Author)
	}
}

func TestClassifyAll_IncludesSelf(t *testing.T) {
	prs := []PRNode{
		makePR(withAuthor("me")),
		makePR(withAuthor("other")),
	}
	result := classifyAll(prs, "me", true)
	if len(result) != 2 {
		t.Fatalf("expected 2 PRs (self included), got %d", len(result))
	}
}

func TestClassifyAll_SortOrder(t *testing.T) {
	reviewTime := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	commitTime := time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC)

	prs := []PRNode{
		makePR(
			withReview("me", "APPROVED", reviewTime),
			withLastCommit(commitTime),
		), // STL
		makePR(), // NEW
		makePR(withComment("me")), // CMT
	}
	result := classifyAll(prs, "me", false)
	if len(result) != 3 {
		t.Fatalf("expected 3, got %d", len(result))
	}
	if result[0].State != StateNew {
		t.Fatalf("expected NEW first, got %s", result[0].State)
	}
	if result[1].State != StateCommented {
		t.Fatalf("expected CMT second, got %s", result[1].State)
	}
	if result[2].State != StateStale {
		t.Fatalf("expected STL third, got %s", result[2].State)
	}
}
