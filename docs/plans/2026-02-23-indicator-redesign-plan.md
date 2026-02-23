# Indicator Redesign + Author Mode Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the single `[TAG]` system with three indicator columns (👤👥💬), add author mode toggle, add draft awareness, add refresh-in-place, and show a spinner on startup instead of blocking stderr.

**Architecture:** The `ClassifiedPR` struct loses its single `State` field and gains three typed indicator fields plus `IsDraft`. A new `computeIndicators()` function replaces `classify()`. The TUI renders three colored characters instead of a `[TAG]`. Author mode reuses the same struct with different indicator semantics. Async fetch via BubbleTea `Cmd` enables spinner on startup and refresh.

**Tech Stack:** Go, BubbleTea (TUI), Lipgloss (styling), `gh` CLI (GitHub API)

---

## Phase 1: New Indicator Types and Data Model

### Task 1: Add indicator types and ClassifiedPR fields

**Files:**
- Modify: `classify.go:1-26`
- Test: `classify_test.go`

**Step 1: Write the failing test**

Add to `classify_test.go`:

```go
func TestIndicatorTypes_Exist(t *testing.T) {
	// Verify the indicator types and their values compile and are correct
	var me MyReviewIndicator = MyNone
	if me != MyReviewIndicator("none") {
		t.Fatalf("expected 'none', got %s", me)
	}
	var oth OthReviewIndicator = OthApproved
	if oth != OthReviewIndicator("approved") {
		t.Fatalf("expected 'approved', got %s", oth)
	}
	var act ActivityIndicator = ActMine
	if act != ActivityIndicator("mine") {
		t.Fatalf("expected 'mine', got %s", act)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./... -run TestIndicatorTypes_Exist -v`
Expected: FAIL — `MyReviewIndicator` undefined

**Step 3: Write minimal implementation**

Add to `classify.go` (keep existing `ReviewState` and constants for now — we'll remove them later when nothing references them):

```go
type MyReviewIndicator string

const (
	MyNone    MyReviewIndicator = "none"
	MyApproved MyReviewIndicator = "approved"
	MyChanges  MyReviewIndicator = "changes"
	MyStale    MyReviewIndicator = "stale"  // approved or changes_requested, but new commits since
)

type OthReviewIndicator string

const (
	OthNone     OthReviewIndicator = "none"
	OthApproved OthReviewIndicator = "approved"
	OthChanges  OthReviewIndicator = "changes"
	OthMixed    OthReviewIndicator = "mixed"
)

type ActivityIndicator string

const (
	ActNone    ActivityIndicator = "none"
	ActOthers  ActivityIndicator = "others"
	ActMine    ActivityIndicator = "mine"
)
```

**Step 4: Run test to verify it passes**

Run: `go test ./... -run TestIndicatorTypes_Exist -v`
Expected: PASS

**Step 5: Commit**

```bash
git add classify.go classify_test.go
git commit -m "feat: add indicator types for three-column display"
```

---

### Task 2: Add indicator fields to ClassifiedPR

**Files:**
- Modify: `classify.go:17-26` (ClassifiedPR struct)
- Test: `classify_test.go`

**Step 1: Write the failing test**

```go
func TestClassifiedPR_HasIndicatorFields(t *testing.T) {
	pr := ClassifiedPR{
		MyReview: MyApproved,
		OthReview: OthMixed,
		Activity: ActMine,
		IsDraft: true,
	}
	if pr.MyReview != MyApproved {
		t.Fatal("MyReview field not set")
	}
	if pr.OthReview != OthMixed {
		t.Fatal("OthReview field not set")
	}
	if pr.Activity != ActMine {
		t.Fatal("Activity field not set")
	}
	if !pr.IsDraft {
		t.Fatal("IsDraft field not set")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./... -run TestClassifiedPR_HasIndicatorFields -v`
Expected: FAIL — `pr.MyReview` undefined

**Step 3: Write minimal implementation**

Add fields to `ClassifiedPR` struct:

```go
type ClassifiedPR struct {
	// Indicators (new)
	MyReview  MyReviewIndicator
	OthReview OthReviewIndicator
	Activity  ActivityIndicator
	IsDraft   bool

	// Existing fields
	State        ReviewState  // kept temporarily for backwards compat during migration
	RepoName     string
	RepoFullName string
	Number       int
	Title        string
	Author       string
	URL          string
	CreatedAt    time.Time
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./... -run TestClassifiedPR_HasIndicatorFields -v`
Expected: PASS

**Step 5: Run all tests to verify nothing broke**

Run: `go test ./... -v`
Expected: all PASS (existing code still sets `State`, new fields just default to zero values)

**Step 6: Commit**

```bash
git add classify.go classify_test.go
git commit -m "feat: add indicator fields to ClassifiedPR struct"
```

---

### Task 3: Add `isDraft` to GraphQL query and PRNode

**Files:**
- Modify: `github.go:11-35` (PRNode struct)
- Modify: `github.go:80-123` (GraphQL query)
- Modify: `github_test.go` (update sample response)
- Test: `github_test.go`

**Step 1: Write the failing test**

Add to `github_test.go` inside `TestParseSearchResult`, after the existing assertions on the first node:

```go
func TestParseSearchResult_DraftField(t *testing.T) {
	draftJSON := `{
	  "data": {
	    "search": {
	      "pageInfo": { "hasNextPage": false, "endCursor": "x" },
	      "nodes": [
	        {
	          "title": "WIP feature",
	          "url": "https://github.com/org/repo/pull/99",
	          "number": 99,
	          "isDraft": true,
	          "createdAt": "2025-01-15T10:00:00Z",
	          "author": { "login": "alice" },
	          "repository": { "name": "repo", "nameWithOwner": "org/repo" },
	          "reviews": { "nodes": [] },
	          "comments": { "nodes": [] },
	          "commits": { "nodes": [{ "commit": { "committedDate": "2025-01-15T09:00:00Z" } }] },
	          "reviewRequests": { "nodes": [] }
	        }
	      ]
	    }
	  }
	}`
	var result searchResult
	if err := json.Unmarshal([]byte(draftJSON), &result); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	pr := result.Data.Search.Nodes[0]
	if !pr.IsDraft {
		t.Fatal("expected isDraft to be true")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./... -run TestParseSearchResult_DraftField -v`
Expected: FAIL — `pr.IsDraft` undefined

**Step 3: Write minimal implementation**

Add to `PRNode` struct in `github.go`:

```go
IsDraft bool `json:"isDraft"`
```

Add `isDraft` to the GraphQL query string, after the `createdAt` line:

```
isDraft
```

**Step 4: Run test to verify it passes**

Run: `go test ./... -run TestParseSearchResult_DraftField -v`
Expected: PASS

**Step 5: Run all tests**

Run: `go test ./... -v`
Expected: all PASS

**Step 6: Commit**

```bash
git add github.go github_test.go
git commit -m "feat: add isDraft to GraphQL query and PRNode"
```

---

### Task 4: Add `createdAt` to CommentNode

Needed for author mode "new comments since last push" logic.

**Files:**
- Modify: `github.go:45-49` (CommentNode struct)
- Modify: `github.go:100-105` (GraphQL query comments section)
- Test: `github_test.go`

**Step 1: Write the failing test**

```go
func TestParseSearchResult_CommentCreatedAt(t *testing.T) {
	commentJSON := `{
	  "data": {
	    "search": {
	      "pageInfo": { "hasNextPage": false, "endCursor": "x" },
	      "nodes": [
	        {
	          "title": "Test",
	          "url": "https://github.com/org/repo/pull/50",
	          "number": 50,
	          "isDraft": false,
	          "createdAt": "2025-01-15T10:00:00Z",
	          "author": { "login": "alice" },
	          "repository": { "name": "repo", "nameWithOwner": "org/repo" },
	          "reviews": { "nodes": [] },
	          "comments": { "nodes": [
	            { "author": { "login": "bob" }, "createdAt": "2025-01-16T10:00:00Z" }
	          ] },
	          "commits": { "nodes": [{ "commit": { "committedDate": "2025-01-15T09:00:00Z" } }] },
	          "reviewRequests": { "nodes": [] }
	        }
	      ]
	    }
	  }
	}`
	var result searchResult
	if err := json.Unmarshal([]byte(commentJSON), &result); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	comment := result.Data.Search.Nodes[0].Comments.Nodes[0]
	expected := time.Date(2025, 1, 16, 10, 0, 0, 0, time.UTC)
	if !comment.CreatedAt.Equal(expected) {
		t.Fatalf("expected comment createdAt %v, got %v", expected, comment.CreatedAt)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./... -run TestParseSearchResult_CommentCreatedAt -v`
Expected: FAIL — `comment.CreatedAt` undefined

**Step 3: Write minimal implementation**

Add to `CommentNode` struct:

```go
type CommentNode struct {
	Author struct {
		Login string `json:"login"`
	} `json:"author"`
	CreatedAt time.Time `json:"createdAt"`
}
```

Update GraphQL query comments section:

```
comments(last: 100) {
  nodes {
    author { login }
    createdAt
  }
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./... -run TestParseSearchResult_CommentCreatedAt -v`
Expected: PASS

**Step 5: Commit**

```bash
git add github.go github_test.go
git commit -m "feat: add createdAt to comment nodes in GraphQL query"
```

---

## Phase 2: Indicator Computation Functions

### Task 5: Implement `computeMyReview()`

**Files:**
- Modify: `classify.go`
- Test: `classify_test.go`

**Step 1: Write failing tests**

```go
func TestComputeMyReview_None(t *testing.T) {
	pr := makePR()
	if got := computeMyReview(pr, "me"); got != MyNone {
		t.Fatalf("expected MyNone, got %s", got)
	}
}

func TestComputeMyReview_Approved(t *testing.T) {
	reviewTime := time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC)
	commitTime := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	pr := makePR(
		withReview("me", "APPROVED", reviewTime),
		withLastCommit(commitTime),
	)
	if got := computeMyReview(pr, "me"); got != MyApproved {
		t.Fatalf("expected MyApproved, got %s", got)
	}
}

func TestComputeMyReview_Changes(t *testing.T) {
	reviewTime := time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC)
	commitTime := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	pr := makePR(
		withReview("me", "CHANGES_REQUESTED", reviewTime),
		withLastCommit(commitTime),
	)
	if got := computeMyReview(pr, "me"); got != MyChanges {
		t.Fatalf("expected MyChanges, got %s", got)
	}
}

func TestComputeMyReview_StaleApproval(t *testing.T) {
	reviewTime := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	commitTime := time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC)
	pr := makePR(
		withReview("me", "APPROVED", reviewTime),
		withLastCommit(commitTime),
	)
	if got := computeMyReview(pr, "me"); got != MyStale {
		t.Fatalf("expected MyStale, got %s", got)
	}
}

func TestComputeMyReview_StaleChanges(t *testing.T) {
	reviewTime := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	commitTime := time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC)
	pr := makePR(
		withReview("me", "CHANGES_REQUESTED", reviewTime),
		withLastCommit(commitTime),
	)
	if got := computeMyReview(pr, "me"); got != MyStale {
		t.Fatalf("expected MyStale, got %s", got)
	}
}

func TestComputeMyReview_SkipsPending(t *testing.T) {
	pr := makePR(
		withReview("me", "PENDING", time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)),
	)
	if got := computeMyReview(pr, "me"); got != MyNone {
		t.Fatalf("expected MyNone (pending skipped), got %s", got)
	}
}

func TestComputeMyReview_SkipsDismissed(t *testing.T) {
	pr := makePR(
		withReview("me", "DISMISSED", time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)),
	)
	if got := computeMyReview(pr, "me"); got != MyNone {
		t.Fatalf("expected MyNone (dismissed review treated as none), got %s", got)
	}
}

func TestComputeMyReview_LatestWins(t *testing.T) {
	commitTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	pr := makePR(
		withReview("me", "CHANGES_REQUESTED", time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)),
		withReview("me", "APPROVED", time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC)),
		withLastCommit(commitTime),
	)
	if got := computeMyReview(pr, "me"); got != MyApproved {
		t.Fatalf("expected MyApproved (latest review wins), got %s", got)
	}
}

func TestComputeMyReview_CommentedOnly(t *testing.T) {
	pr := makePR(
		withReview("me", "COMMENTED", time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)),
	)
	// COMMENTED review without APPROVED/CHANGES_REQUESTED = no substantive review
	if got := computeMyReview(pr, "me"); got != MyNone {
		t.Fatalf("expected MyNone (COMMENTED is not a substantive review), got %s", got)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./... -run TestComputeMyReview -v`
Expected: FAIL — `computeMyReview` undefined

**Step 3: Write minimal implementation**

```go
func computeMyReview(pr PRNode, me string) MyReviewIndicator {
	// Find my latest substantive review (APPROVED or CHANGES_REQUESTED)
	var lastReview *ReviewNode
	for i := range pr.Reviews.Nodes {
		r := &pr.Reviews.Nodes[i]
		if r.Author.Login != me {
			continue
		}
		switch r.State {
		case "APPROVED", "CHANGES_REQUESTED":
			lastReview = r
		}
	}

	if lastReview == nil {
		return MyNone
	}

	// Check staleness: latest commit newer than my review?
	stale := false
	if len(pr.Commits.Nodes) > 0 {
		lastCommit := pr.Commits.Nodes[0].Commit.CommittedDate
		if lastCommit.After(lastReview.SubmittedAt) {
			stale = true
		}
	}

	if stale {
		return MyStale
	}

	switch lastReview.State {
	case "APPROVED":
		return MyApproved
	case "CHANGES_REQUESTED":
		return MyChanges
	}
	return MyNone
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./... -run TestComputeMyReview -v`
Expected: all PASS

**Step 5: Commit**

```bash
git add classify.go classify_test.go
git commit -m "feat: implement computeMyReview() indicator function"
```

---

### Task 6: Implement `computeOthReview()`

**Files:**
- Modify: `classify.go`
- Test: `classify_test.go`

**Step 1: Write failing tests**

```go
func TestComputeOthReview_None(t *testing.T) {
	pr := makePR()
	if got := computeOthReview(pr, "me"); got != OthNone {
		t.Fatalf("expected OthNone, got %s", got)
	}
}

func TestComputeOthReview_AllApproved(t *testing.T) {
	pr := makePR(
		withReview("alice", "APPROVED", time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)),
		withReview("bob", "APPROVED", time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC)),
	)
	if got := computeOthReview(pr, "me"); got != OthApproved {
		t.Fatalf("expected OthApproved, got %s", got)
	}
}

func TestComputeOthReview_ChangesRequested(t *testing.T) {
	pr := makePR(
		withReview("alice", "APPROVED", time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)),
		withReview("bob", "CHANGES_REQUESTED", time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC)),
	)
	if got := computeOthReview(pr, "me"); got != OthChanges {
		t.Fatalf("expected OthChanges, got %s", got)
	}
}

func TestComputeOthReview_Mixed(t *testing.T) {
	pr := makePR(
		withReview("alice", "APPROVED", time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)),
		withReview("bob", "COMMENTED", time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC)),
	)
	if got := computeOthReview(pr, "me"); got != OthMixed {
		t.Fatalf("expected OthMixed, got %s", got)
	}
}

func TestComputeOthReview_ExcludesMe(t *testing.T) {
	pr := makePR(
		withReview("me", "APPROVED", time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)),
	)
	if got := computeOthReview(pr, "me"); got != OthNone {
		t.Fatalf("expected OthNone (my review excluded), got %s", got)
	}
}

func TestComputeOthReview_LatestPerReviewer(t *testing.T) {
	pr := makePR(
		withReview("alice", "CHANGES_REQUESTED", time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)),
		withReview("alice", "APPROVED", time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC)),
	)
	if got := computeOthReview(pr, "me"); got != OthApproved {
		t.Fatalf("expected OthApproved (latest review from alice wins), got %s", got)
	}
}

func TestComputeOthReview_SkipsPendingAndDismissed(t *testing.T) {
	pr := makePR(
		withReview("alice", "PENDING", time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)),
		withReview("bob", "DISMISSED", time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC)),
	)
	if got := computeOthReview(pr, "me"); got != OthNone {
		t.Fatalf("expected OthNone (PENDING and DISMISSED skipped), got %s", got)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./... -run TestComputeOthReview -v`
Expected: FAIL — `computeOthReview` undefined

**Step 3: Write minimal implementation**

```go
func computeOthReview(pr PRNode, me string) OthReviewIndicator {
	// Deduplicate to latest review per reviewer, skip PENDING/DISMISSED and self
	latest := make(map[string]ReviewNode)
	for _, r := range pr.Reviews.Nodes {
		if r.Author.Login == "" || r.Author.Login == me {
			continue
		}
		if r.State == "PENDING" || r.State == "DISMISSED" {
			continue
		}
		if existing, ok := latest[r.Author.Login]; ok {
			if r.SubmittedAt.After(existing.SubmittedAt) {
				latest[r.Author.Login] = r
			}
		} else {
			latest[r.Author.Login] = r
		}
	}

	if len(latest) == 0 {
		return OthNone
	}

	allApproved := true
	for _, r := range latest {
		if r.State == "CHANGES_REQUESTED" {
			return OthChanges
		}
		if r.State != "APPROVED" {
			allApproved = false
		}
	}

	if allApproved {
		return OthApproved
	}
	return OthMixed
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./... -run TestComputeOthReview -v`
Expected: all PASS

**Step 5: Commit**

```bash
git add classify.go classify_test.go
git commit -m "feat: implement computeOthReview() indicator function"
```

---

### Task 7: Implement `computeActivity()`

**Files:**
- Modify: `classify.go`
- Test: `classify_test.go`

**Step 1: Write failing tests**

```go
func TestComputeActivity_None(t *testing.T) {
	pr := makePR()
	if got := computeActivity(pr, "me"); got != ActNone {
		t.Fatalf("expected ActNone, got %s", got)
	}
}

func TestComputeActivity_Others(t *testing.T) {
	pr := makePR(withComment("alice"))
	if got := computeActivity(pr, "me"); got != ActOthers {
		t.Fatalf("expected ActOthers, got %s", got)
	}
}

func TestComputeActivity_Mine(t *testing.T) {
	pr := makePR(withComment("me"))
	if got := computeActivity(pr, "me"); got != ActMine {
		t.Fatalf("expected ActMine, got %s", got)
	}
}

func TestComputeActivity_BothMineAndOthers(t *testing.T) {
	pr := makePR(withComment("alice"), withComment("me"))
	if got := computeActivity(pr, "me"); got != ActMine {
		t.Fatalf("expected ActMine (mine takes precedence), got %s", got)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./... -run TestComputeActivity -v`
Expected: FAIL — `computeActivity` undefined

**Step 3: Write minimal implementation**

```go
func computeActivity(pr PRNode, me string) ActivityIndicator {
	hasMine := false
	hasOthers := false
	for _, c := range pr.Comments.Nodes {
		if c.Author.Login == me {
			hasMine = true
		} else if c.Author.Login != "" {
			hasOthers = true
		}
	}
	if hasMine {
		return ActMine
	}
	if hasOthers {
		return ActOthers
	}
	return ActNone
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./... -run TestComputeActivity -v`
Expected: all PASS

**Step 5: Commit**

```bash
git add classify.go classify_test.go
git commit -m "feat: implement computeActivity() indicator function"
```

---

### Task 8: Implement author-mode activity (`computeAuthorActivity()`)

**Files:**
- Modify: `classify.go`
- Test: `classify_test.go`

This function checks if there are new comments since the author's last push.

**Step 1: Add `withCommentAt` test helper**

Add to `classify_test.go`:

```go
func withCommentAt(login string, at time.Time) func(*PRNode) {
	return func(pr *PRNode) {
		c := CommentNode{CreatedAt: at}
		c.Author.Login = login
		pr.Comments.Nodes = append(pr.Comments.Nodes, c)
	}
}
```

**Step 2: Write failing tests**

```go
func TestComputeAuthorActivity_None(t *testing.T) {
	pr := makePR(withAuthor("me"))
	if got := computeAuthorActivity(pr); got != ActNone {
		t.Fatalf("expected ActNone, got %s", got)
	}
}

func TestComputeAuthorActivity_OldComments(t *testing.T) {
	commitTime := time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC)
	pr := makePR(
		withAuthor("me"),
		withLastCommit(commitTime),
		withCommentAt("alice", time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)),
	)
	if got := computeAuthorActivity(pr); got != ActOthers {
		t.Fatalf("expected ActOthers (comment before push), got %s", got)
	}
}

func TestComputeAuthorActivity_NewComments(t *testing.T) {
	commitTime := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	pr := makePR(
		withAuthor("me"),
		withLastCommit(commitTime),
		withCommentAt("alice", time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC)),
	)
	if got := computeAuthorActivity(pr); got != ActMine {
		t.Fatalf("expected ActMine (new comments since push), got %s", got)
	}
}

func TestComputeAuthorActivity_NoCommits(t *testing.T) {
	pr := makePR(withAuthor("me"))
	pr.Commits.Nodes = nil
	pr.Comments.Nodes = []CommentNode{{CreatedAt: time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)}}
	pr.Comments.Nodes[0].Author.Login = "alice"
	if got := computeAuthorActivity(pr); got != ActMine {
		t.Fatalf("expected ActMine (no commits to compare = treat as new), got %s", got)
	}
}
```

Note: We reuse `ActMine` to mean "new comments since push" and `ActOthers` to mean "comments exist but all before push". This avoids introducing new constants — the rendering layer interprets them differently in author mode via label text, not the constant name.

**Step 3: Run tests to verify they fail**

Run: `go test ./... -run TestComputeAuthorActivity -v`
Expected: FAIL — `computeAuthorActivity` undefined

**Step 4: Write minimal implementation**

```go
func computeAuthorActivity(pr PRNode) ActivityIndicator {
	if len(pr.Comments.Nodes) == 0 {
		return ActNone
	}

	// Get last commit time
	var lastCommit time.Time
	if len(pr.Commits.Nodes) > 0 {
		lastCommit = pr.Commits.Nodes[0].Commit.CommittedDate
	}

	// Check if any comment is after last commit
	for _, c := range pr.Comments.Nodes {
		if lastCommit.IsZero() || c.CreatedAt.After(lastCommit) {
			return ActMine // "new comments since push"
		}
	}
	return ActOthers // "comments exist, but all before last push"
}
```

**Step 5: Run tests to verify they pass**

Run: `go test ./... -run TestComputeAuthorActivity -v`
Expected: all PASS

**Step 6: Commit**

```bash
git add classify.go classify_test.go
git commit -m "feat: implement computeAuthorActivity() for author mode"
```

---

## Phase 3: New Classification Pipeline

### Task 9: Implement `classifyAllNew()` (reviewer mode)

This replaces the current `classifyAll()`. It uses the new indicator functions and the new sort order.

**Files:**
- Modify: `classify.go`
- Test: `classify_test.go`

**Step 1: Write failing tests**

```go
func TestClassifyAllNew_BasicIndicators(t *testing.T) {
	prs := []PRNode{
		makePR(withAuthor("alice"), withURL("https://github.com/org/repo/pull/1")),
		makePR(
			withAuthor("bob"),
			withReview("someone", "APPROVED", time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)),
			withComment("me"),
			withURL("https://github.com/org/repo/pull/2"),
		),
	}
	result := classifyAllNew(prs, "me", false, nil)
	if len(result) != 2 {
		t.Fatalf("expected 2 PRs, got %d", len(result))
	}

	// First PR: untouched by anyone
	if result[0].MyReview != MyNone {
		t.Errorf("PR1: expected MyNone, got %s", result[0].MyReview)
	}
	if result[0].OthReview != OthNone {
		t.Errorf("PR1: expected OthNone, got %s", result[0].OthReview)
	}
	if result[0].Activity != ActNone {
		t.Errorf("PR1: expected ActNone, got %s", result[0].Activity)
	}

	// Second PR: someone approved, I commented
	if result[1].OthReview != OthApproved {
		t.Errorf("PR2: expected OthApproved, got %s", result[1].OthReview)
	}
	if result[1].Activity != ActMine {
		t.Errorf("PR2: expected ActMine, got %s", result[1].Activity)
	}
}

func TestClassifyAllNew_ExcludesSelf(t *testing.T) {
	prs := []PRNode{
		makePR(withAuthor("me")),
		makePR(withAuthor("other")),
	}
	result := classifyAllNew(prs, "me", false, nil)
	if len(result) != 1 {
		t.Fatalf("expected 1 PR, got %d", len(result))
	}
}

func TestClassifyAllNew_IncludesSelf(t *testing.T) {
	prs := []PRNode{
		makePR(withAuthor("me")),
		makePR(withAuthor("other")),
	}
	result := classifyAllNew(prs, "me", true, nil)
	if len(result) != 2 {
		t.Fatalf("expected 2 PRs, got %d", len(result))
	}
}

func TestClassifyAllNew_WithFilter(t *testing.T) {
	prs := []PRNode{
		makePR(withReviewRequest("me", "", false)),
		makePR(),
	}
	filter := func(pr PRNode) bool {
		return isRequestedReviewer(pr, "me", nil)
	}
	result := classifyAllNew(prs, "me", false, filter)
	if len(result) != 1 {
		t.Fatalf("expected 1 PR, got %d", len(result))
	}
}

func TestClassifyAllNew_SortOrder(t *testing.T) {
	reviewTime := time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC)
	commitAfter := time.Date(2025, 1, 4, 0, 0, 0, 0, time.UTC)
	commitBefore := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)

	prs := []PRNode{
		// Approved current — should be last
		makePR(
			withAuthor("alice"),
			withReview("me", "APPROVED", reviewTime),
			withLastCommit(commitBefore),
			withURL("https://github.com/org/repo/pull/1"),
		),
		// Untouched — should be middle
		makePR(
			withAuthor("bob"),
			withURL("https://github.com/org/repo/pull/2"),
		),
		// Stale — should be first
		makePR(
			withAuthor("carol"),
			withReview("me", "APPROVED", reviewTime),
			withLastCommit(commitAfter),
			withURL("https://github.com/org/repo/pull/3"),
		),
	}
	result := classifyAllNew(prs, "me", false, nil)
	if len(result) != 3 {
		t.Fatalf("expected 3 PRs, got %d", len(result))
	}
	if result[0].MyReview != MyStale {
		t.Errorf("expected stale first, got MyReview=%s", result[0].MyReview)
	}
	if result[1].MyReview != MyNone {
		t.Errorf("expected untouched second, got MyReview=%s", result[1].MyReview)
	}
	if result[2].MyReview != MyApproved {
		t.Errorf("expected approved last, got MyReview=%s", result[2].MyReview)
	}
}

func TestClassifyAllNew_DraftField(t *testing.T) {
	pr := makePR(withAuthor("alice"))
	pr.IsDraft = true
	result := classifyAllNew([]PRNode{pr}, "me", false, nil)
	if len(result) != 1 {
		t.Fatalf("expected 1 PR, got %d", len(result))
	}
	if !result[0].IsDraft {
		t.Fatal("expected IsDraft to be true")
	}
}

func TestClassifyAllNew_IncludesApproved(t *testing.T) {
	// Unlike old classifyAll, approved-and-current PRs are NOT filtered out
	reviewTime := time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC)
	commitBefore := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	prs := []PRNode{
		makePR(
			withAuthor("alice"),
			withReview("me", "APPROVED", reviewTime),
			withLastCommit(commitBefore),
		),
	}
	result := classifyAllNew(prs, "me", false, nil)
	if len(result) != 1 {
		t.Fatalf("expected 1 PR (approved PRs no longer hidden), got %d", len(result))
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./... -run TestClassifyAllNew -v`
Expected: FAIL — `classifyAllNew` undefined

**Step 3: Write minimal implementation**

```go
// sortPriority returns a numeric priority for sorting in reviewer mode.
// Lower = higher priority (shown first).
func sortPriority(pr ClassifiedPR) int {
	switch pr.MyReview {
	case MyStale:
		return 0
	case MyNone:
		if pr.OthReview == OthNone {
			return 1 // untouched, no other reviews
		}
		return 2 // untouched, others have looked
	case MyChanges:
		return 3
	case MyApproved:
		return 4
	}
	return 5
}

func classifyAllNew(prs []PRNode, me string, includeSelf bool, filter func(PRNode) bool) []ClassifiedPR {
	var result []ClassifiedPR
	for _, pr := range prs {
		if !includeSelf && pr.Author.Login == me {
			continue
		}
		if filter != nil && !filter(pr) {
			continue
		}
		result = append(result, ClassifiedPR{
			MyReview:     computeMyReview(pr, me),
			OthReview:    computeOthReview(pr, me),
			Activity:     computeActivity(pr, me),
			IsDraft:      pr.IsDraft,
			RepoName:     pr.Repository.Name,
			RepoFullName: pr.Repository.NameWithOwner,
			Number:       pr.Number,
			Title:        pr.Title,
			Author:       pr.Author.Login,
			URL:          pr.URL,
			CreatedAt:    pr.CreatedAt,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		pi, pj := sortPriority(result[i]), sortPriority(result[j])
		if pi != pj {
			return pi < pj
		}
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./... -run TestClassifyAllNew -v`
Expected: all PASS

**Step 5: Commit**

```bash
git add classify.go classify_test.go
git commit -m "feat: implement classifyAllNew() with indicator computation and new sort"
```

---

### Task 10: Implement `classifyAllAuthor()`

**Files:**
- Modify: `classify.go`
- Test: `classify_test.go`

**Step 1: Write failing tests**

```go
func TestClassifyAllAuthor_OnlyOwnPRs(t *testing.T) {
	prs := []PRNode{
		makePR(withAuthor("me")),
		makePR(withAuthor("other")),
	}
	result := classifyAllAuthor(prs, "me")
	if len(result) != 1 {
		t.Fatalf("expected 1 PR (only mine), got %d", len(result))
	}
}

func TestClassifyAllAuthor_DraftIndicator(t *testing.T) {
	pr := makePR(withAuthor("me"))
	pr.IsDraft = true
	result := classifyAllAuthor([]PRNode{pr}, "me")
	if !result[0].IsDraft {
		t.Fatal("expected IsDraft to be true")
	}
}

func TestClassifyAllAuthor_ReviewIndicators(t *testing.T) {
	pr := makePR(
		withAuthor("me"),
		withReview("alice", "APPROVED", time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)),
		withReview("bob", "CHANGES_REQUESTED", time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC)),
	)
	result := classifyAllAuthor([]PRNode{pr}, "me")
	if result[0].OthReview != OthChanges {
		t.Fatalf("expected OthChanges, got %s", result[0].OthReview)
	}
}

func TestClassifyAllAuthor_ActivityIndicator(t *testing.T) {
	commitTime := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	pr := makePR(
		withAuthor("me"),
		withLastCommit(commitTime),
		withCommentAt("alice", time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC)),
	)
	result := classifyAllAuthor([]PRNode{pr}, "me")
	if result[0].Activity != ActMine {
		t.Fatalf("expected ActMine (new comments since push), got %s", result[0].Activity)
	}
}

func TestClassifyAllAuthor_SortOrder(t *testing.T) {
	prs := []PRNode{
		// No reviews = middle
		makePR(withAuthor("me"), withURL("https://github.com/org/repo/pull/1")),
		// All approved = last
		makePR(
			withAuthor("me"),
			withReview("alice", "APPROVED", time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)),
			withURL("https://github.com/org/repo/pull/2"),
		),
		// Changes requested = first
		makePR(
			withAuthor("me"),
			withReview("bob", "CHANGES_REQUESTED", time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)),
			withURL("https://github.com/org/repo/pull/3"),
		),
	}
	result := classifyAllAuthor(prs, "me")
	if len(result) != 3 {
		t.Fatalf("expected 3 PRs, got %d", len(result))
	}
	if result[0].OthReview != OthChanges {
		t.Errorf("expected changes first, got %s", result[0].OthReview)
	}
	if result[1].OthReview != OthNone {
		t.Errorf("expected no reviews second, got %s", result[1].OthReview)
	}
	if result[2].OthReview != OthApproved {
		t.Errorf("expected approved last, got %s", result[2].OthReview)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./... -run TestClassifyAllAuthor -v`
Expected: FAIL — `classifyAllAuthor` undefined

**Step 3: Write minimal implementation**

```go
func authorSortPriority(pr ClassifiedPR) int {
	switch pr.OthReview {
	case OthChanges:
		return 0
	case OthMixed:
		return 1
	case OthNone:
		return 2
	case OthApproved:
		return 3
	}
	return 4
}

func classifyAllAuthor(prs []PRNode, me string) []ClassifiedPR {
	var result []ClassifiedPR
	for _, pr := range prs {
		if pr.Author.Login != me {
			continue
		}
		result = append(result, ClassifiedPR{
			MyReview:     MyNone, // not meaningful in author mode
			OthReview:    computeOthReview(pr, me),
			Activity:     computeAuthorActivity(pr),
			IsDraft:      pr.IsDraft,
			RepoName:     pr.Repository.Name,
			RepoFullName: pr.Repository.NameWithOwner,
			Number:       pr.Number,
			Title:        pr.Title,
			Author:       pr.Author.Login,
			URL:          pr.URL,
			CreatedAt:    pr.CreatedAt,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		pi, pj := authorSortPriority(result[i]), authorSortPriority(result[j])
		if pi != pj {
			return pi < pj
		}
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./... -run TestClassifyAllAuthor -v`
Expected: all PASS

**Step 5: Commit**

```bash
git add classify.go classify_test.go
git commit -m "feat: implement classifyAllAuthor() for author mode classification"
```

---

## Phase 4: TUI Rendering

### Task 11: Implement `formatIndicators()` and update TUI rendering

Replace `formatTag()` with `formatIndicators()` that renders the three indicator columns.

**Files:**
- Modify: `tui.go`
- Test: `tui_test.go`

**Step 1: Write failing tests**

Add to `tui_test.go`:

```go
func TestFormatIndicators_ReviewerMode(t *testing.T) {
	pr := ClassifiedPR{MyReview: MyNone, OthReview: OthApproved, Activity: ActMine}
	result := formatIndicators(pr, false)
	// Should contain the indicator characters (checking raw runes, not ANSI)
	if result == "" {
		t.Fatal("expected non-empty indicator string")
	}
}

func TestFormatIndicators_AuthorMode(t *testing.T) {
	pr := ClassifiedPR{IsDraft: true, OthReview: OthNone, Activity: ActNone}
	result := formatIndicators(pr, true)
	if result == "" {
		t.Fatal("expected non-empty indicator string")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./... -run TestFormatIndicators -v`
Expected: FAIL — `formatIndicators` undefined

**Step 3: Write minimal implementation**

In `tui.go`, replace the tag styles with indicator styles:

```go
var (
	// Indicator styles
	styleGreen  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	styleRed    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	styleYellow = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	styleCyan   = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	styleDim    = lipgloss.NewStyle().Faint(true)
	styleWhite  = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))

	selLine   = lipgloss.NewStyle().Bold(true).Reverse(true)
	helpStyle = lipgloss.NewStyle().Faint(true)
)

func formatIndicators(pr ClassifiedPR, authorMode bool) string {
	var col1, col2, col3 string

	if authorMode {
		// Column 1: draft status
		if pr.IsDraft {
			col1 = styleDim.Render("○")
		} else {
			col1 = styleWhite.Render("●")
		}
	} else {
		// Column 1: my review
		switch pr.MyReview {
		case MyNone:
			col1 = styleDim.Render("·")
		case MyApproved:
			col1 = styleGreen.Render("✓")
		case MyChanges:
			col1 = styleRed.Render("✗")
		case MyStale:
			col1 = styleDim.Render("~")
		default:
			col1 = styleDim.Render("·")
		}
	}

	// Column 2: others' reviews (same in both modes)
	switch pr.OthReview {
	case OthNone:
		col2 = styleDim.Render("·")
	case OthApproved:
		col2 = styleGreen.Render("✓")
	case OthChanges:
		col2 = styleRed.Render("✗")
	case OthMixed:
		col2 = styleYellow.Render("±")
	default:
		col2 = styleDim.Render("·")
	}

	// Column 3: activity
	switch pr.Activity {
	case ActNone:
		col3 = styleDim.Render("·")
	case ActOthers:
		col3 = styleWhite.Render("○")
	case ActMine:
		col3 = styleCyan.Render("●")
	default:
		col3 = styleDim.Render("·")
	}

	return col1 + col2 + col3
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./... -run TestFormatIndicators -v`
Expected: PASS

**Step 5: Commit**

```bash
git add tui.go tui_test.go
git commit -m "feat: implement formatIndicators() for three-column display"
```

---

### Task 12: Switch TUI model to use new classification + rendering

This is the big switchover. Replace `reclassify()` to use `classifyAllNew()`/`classifyAllAuthor()`, update `View()` to use `formatIndicators()`, add `showAuthor` field, add `a` key handler, update help bar.

**Files:**
- Modify: `tui.go`
- Modify: `tui_test.go`

**Step 1: Update `testModelConfig()` and write tests for new TUI behavior**

First, update `tui_test.go` `testModelConfig()` comment to reflect new behavior, then add author mode tests:

```go
func TestModel_AuthorToggle(t *testing.T) {
	cfg := testModelConfig()
	m := newModel(cfg)

	// Default: reviewer mode, 3 non-self items (now all included, even approved)
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

func TestModel_ViewContainsIndicators(t *testing.T) {
	m := sendMsg(newModel(testModelConfig()), tea.WindowSizeMsg{Width: 120, Height: 20})
	view := m.View()
	// View should contain author names (indicators are ANSI-styled so hard to check directly)
	for _, want := range []string{"alice", "bob", "carol"} {
		if !strings.Contains(view, want) {
			t.Errorf("expected view to contain %q", want)
		}
	}
	// Should NOT contain old-style [NEW] tags
	if strings.Contains(view, "[NEW]") {
		t.Error("view should not contain old [NEW] tags")
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
```

**Step 2: Run tests to verify they fail**

Run: `go test ./... -run "TestModel_Author|TestModel_ViewContainsIndicators|TestModel_HelpBarShowsAuthorToggle" -v`
Expected: FAIL — `showAuthor` field undefined, old `[NEW]` tags still in view

**Step 3: Write the implementation**

Update `model` and `modelConfig` structs:

```go
type model struct {
	items     []ClassifiedPR
	cursor    int
	dismissed map[string]bool
	cols      colWidths
	width     int
	height    int

	rawPRs  []PRNode
	me      string
	myTeams map[string]bool

	showSelf   bool
	showMine   bool
	showAuthor bool
}

type modelConfig struct {
	rawPRs     []PRNode
	me         string
	myTeams    map[string]bool
	showSelf   bool
	showMine   bool
	showAuthor bool
}
```

Update `newModel()`:

```go
func newModel(cfg modelConfig) model {
	m := model{
		dismissed:  make(map[string]bool),
		rawPRs:     cfg.rawPRs,
		me:         cfg.me,
		myTeams:    cfg.myTeams,
		showSelf:   cfg.showSelf,
		showMine:   cfg.showMine,
		showAuthor: cfg.showAuthor,
	}
	m.reclassify()
	return m
}
```

Update `reclassify()`:

```go
func (m *model) reclassify() {
	if m.showAuthor {
		m.items = classifyAllAuthor(m.rawPRs, m.me)
	} else {
		var filter func(PRNode) bool
		if m.showMine {
			me, teams := m.me, m.myTeams
			filter = func(pr PRNode) bool {
				return isRequestedReviewer(pr, me, teams)
			}
		}
		m.items = classifyAllNew(m.rawPRs, m.me, m.showSelf, filter)
	}
	m.cols = computeColumns(m.items)
}
```

Add `a` key handler and guard `s`/`m` in `Update()`:

```go
case "a":
	m.showAuthor = !m.showAuthor
	m.reclassify()
	m.cursor = 0
case "s":
	if !m.showAuthor {
		m.showSelf = !m.showSelf
		m.reclassify()
		m.cursor = 0
	}
case "m":
	if !m.showAuthor {
		m.showMine = !m.showMine
		m.reclassify()
		m.cursor = 0
	}
```

Update `View()` to use `formatIndicators()` instead of `formatTag()`:

```go
for i := start; i < end; i++ {
	pr := vis[i]
	indicators := formatIndicators(pr, m.showAuthor)
	repoCol := fmt.Sprintf("%s#%d", pr.RepoName, pr.Number)
	line := fmt.Sprintf("%s %-*s  %-*s  %s", indicators, m.cols.repo, repoCol, m.cols.author, pr.Author, pr.Title)
	// ... rest unchanged
}
```

Update help bar:

```go
authorLabel := "author:off"
if m.showAuthor {
	authorLabel = "author:on"
}

var help string
if m.showAuthor {
	help = helpStyle.Render(fmt.Sprintf(
		"j/k: navigate  enter: open  d: dismiss  a: %s  r: refresh  q: quit",
		authorLabel,
	))
} else {
	selfLabel := "self:off"
	if m.showSelf {
		selfLabel = "self:on"
	}
	mineLabel := "mine:off"
	if m.showMine {
		mineLabel = "mine:on"
	}
	help = helpStyle.Render(fmt.Sprintf(
		"j/k: navigate  enter: open  d: dismiss  s: %s  m: %s  a: %s  r: refresh  q: quit",
		selfLabel, mineLabel, authorLabel,
	))
}
```

Update empty state:

```go
if len(vis) == 0 {
	if m.showAuthor {
		return "No open PRs authored by you. Press a to switch to reviewer mode.\n"
	}
	return "No PRs match current filters. Press s/m to adjust, or a for author mode.\n"
}
```

Add the header line with emoji columns above the PR list:

```go
// Header
header := "👤👥💬"
if m.showAuthor {
	header = "📝👥💬" // different first column label in author mode
}
b.WriteString(helpStyle.Render(header))
b.WriteString("\n")
```

**Step 4: Run new tests to verify they pass**

Run: `go test ./... -run "TestModel_Author|TestModel_ViewContainsIndicators|TestModel_HelpBarShowsAuthorToggle" -v`
Expected: PASS

**Step 5: Fix any broken existing tests**

Run: `go test ./... -v`

Expected breakages and fixes:
- `TestModel_Init`: item count may change since approved PRs are now included. The carol PR (approved+stale) was included before, and still is. So count should remain 3. Verify.
- `TestModel_ViewContainsHelpBar`: check `j/k: navigate` still present — should be.
- `TestModel_HelpBarShowsToggleState`: checks for `self:off`, `mine:off` — should still work.
- `TestModel_EmptyFilterShowsMessage`: checks for `s/m` — the new message says `s/m to adjust, or a for author mode` — still contains `s/m`.

The key change: `testModelConfig()` currently has a carol PR that was APPROVED+STALE (stale review). Under old classify, this showed as STL. Under new classifyAllNew, this shows with `MyReview=MyStale`. The carol PR is still included (not filtered). So item count stays 3. Good.

However, `TestModel_Init` checks `len(m.items) != 3`. Under the new system, the previously-hidden "current approval" PRs would now be included. But `testModelConfig()` only has one PR with my review (carol's — which is stale), so count is still 3. Good.

**Step 6: Commit**

```bash
git add tui.go tui_test.go
git commit -m "feat: switch TUI to three-column indicator display with author mode"
```

---

### Task 13: Update `renderPlain()` for new indicators

**Files:**
- Modify: `plain.go`
- Test: Add a plain test or update existing code

**Step 1: Write failing test**

Create a test that verifies plain output uses indicators:

Add to a new section at the bottom of `classify_test.go` (or a new `plain_test.go`):

```go
// In plain_test.go or classify_test.go
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
	var buf strings.Builder
	renderPlain(&buf, items)
	output := buf.String()

	// Should NOT contain old [TAG] format
	if strings.Contains(output, "[NEW]") || strings.Contains(output, "[STL]") {
		t.Error("plain output should not contain old tags")
	}

	// Should contain author names and PR info
	if !strings.Contains(output, "alice") {
		t.Error("expected alice in output")
	}
	if !strings.Contains(output, "bob") {
		t.Error("expected bob in output")
	}

	// Should contain indicator characters
	if !strings.Contains(output, "·") && !strings.Contains(output, "✓") {
		t.Error("expected indicator characters in output")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./... -run TestRenderPlain_NewFormat -v`
Expected: FAIL — still uses old `[%s]` format with `pr.State`

**Step 3: Write minimal implementation**

Update `renderPlain()` in `plain.go`:

```go
func renderPlain(w io.Writer, items []ClassifiedPR) {
	cols := computeColumns(items)
	for _, pr := range items {
		repoCol := fmt.Sprintf("%s#%d", pr.RepoName, pr.Number)
		indicators := plainIndicators(pr)
		fmt.Fprintf(w, "%s %-*s  %-*s  %s\n",
			indicators,
			cols.repo, repoCol,
			cols.author, pr.Author,
			pr.Title)
	}
}

func plainIndicators(pr ClassifiedPR) string {
	var col1, col2, col3 string

	switch pr.MyReview {
	case MyNone:
		col1 = "·"
	case MyApproved:
		col1 = "✓"
	case MyChanges:
		col1 = "✗"
	case MyStale:
		col1 = "~"
	default:
		col1 = "·"
	}

	switch pr.OthReview {
	case OthNone:
		col2 = "·"
	case OthApproved:
		col2 = "✓"
	case OthChanges:
		col2 = "✗"
	case OthMixed:
		col2 = "±"
	default:
		col2 = "·"
	}

	switch pr.Activity {
	case ActNone:
		col3 = "·"
	case ActOthers:
		col3 = "○"
	case ActMine:
		col3 = "●"
	default:
		col3 = "·"
	}

	return col1 + col2 + col3
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./... -run TestRenderPlain_NewFormat -v`
Expected: PASS

**Step 5: Commit**

```bash
git add plain.go classify_test.go
git commit -m "feat: update plain text output for three-column indicators"
```

---

### Task 14: Update `main.go` with `--author` flag and new classification

**Files:**
- Modify: `main.go`

**Step 1: No new test needed — this is wiring**

The classification functions are already tested. This task wires them into main.

**Step 2: Implement**

Add flag:

```go
author := pflag.Bool("author", false, "Show your own PRs and their review status")
```

Update plain path:

```go
if *plain {
	if *author {
		classified := classifyAllAuthor(prs, me)
		if len(classified) == 0 {
			fmt.Fprintln(os.Stderr, "No open PRs authored by you.")
			return
		}
		renderPlain(os.Stdout, classified)
		return
	}
	var filter func(PRNode) bool
	if *mine {
		myTeams, err := fetchUserTeams(*org)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not fetch team memberships: %v\n", err)
			myTeams = make(map[string]bool)
		}
		filter = func(pr PRNode) bool {
			return isRequestedReviewer(pr, me, myTeams)
		}
	}
	classified := classifyAllNew(prs, me, *self, filter)
	if len(classified) == 0 {
		fmt.Fprintln(os.Stderr, "No PRs pending your review.")
		return
	}
	renderPlain(os.Stdout, classified)
	return
}
```

Update TUI modelConfig:

```go
p := tea.NewProgram(newModel(modelConfig{
	rawPRs:     prs,
	me:         me,
	myTeams:    myTeams,
	showSelf:   *self,
	showMine:   *mine,
	showAuthor: *author,
}), tea.WithAltScreen())
```

**Step 3: Run all tests**

Run: `go test ./... -v`
Expected: all PASS

**Step 4: Build**

Run: `go build .`
Expected: builds successfully

**Step 5: Commit**

```bash
git add main.go
git commit -m "feat: add --author flag, wire new classification into main"
```

---

## Phase 5: Async Fetch and Refresh

### Task 15: Add async fetch with spinner

This changes the TUI to start immediately with a loading spinner, then populate when data arrives. Also adds `r` key to refresh.

**Files:**
- Modify: `tui.go`
- Modify: `main.go`
- Test: `tui_test.go`

**Step 1: Write failing tests**

```go
func TestModel_LoadingState(t *testing.T) {
	cfg := testModelConfig()
	cfg.rawPRs = nil // no data yet = loading
	m := newModel(cfg)
	m.loading = true
	m = sendMsg(m, tea.WindowSizeMsg{Width: 120, Height: 20})
	view := m.View()
	if !strings.Contains(view, "Loading") && !strings.Contains(view, "Fetching") {
		t.Error("expected loading message in view")
	}
}

func TestModel_RefreshKey(t *testing.T) {
	m := newModel(testModelConfig())
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	// 'r' should return a non-nil command (the fetch command)
	// But since we can't execute the fetch in tests, we check that the model
	// enters loading state
	if cmd == nil {
		// This is OK if the model needs fetchConfig to produce a command.
		// Check loading state instead.
	}
	// After pressing 'r', model should be in loading state
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m2 := updated.(model)
	if !m2.loading {
		t.Fatal("expected loading=true after pressing r")
	}
}

type dataLoadedMsg struct {
	prs     []PRNode
	myTeams map[string]bool
}

func TestModel_DataLoaded(t *testing.T) {
	cfg := testModelConfig()
	cfg.rawPRs = nil
	m := newModel(cfg)
	m.loading = true

	// Simulate data arriving
	m = sendMsg(m, dataLoadedMsg{
		prs:     testModelConfig().rawPRs,
		myTeams: make(map[string]bool),
	})
	if m.loading {
		t.Fatal("expected loading=false after data loaded")
	}
	if len(m.items) == 0 {
		t.Fatal("expected items to be populated after data loaded")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./... -run "TestModel_LoadingState|TestModel_RefreshKey|TestModel_DataLoaded" -v`
Expected: FAIL — `loading` field, `dataLoadedMsg` type undefined

**Step 3: Write minimal implementation**

Add to model struct:

```go
loading bool

// Fetch config (needed for refresh)
org string
```

Add to modelConfig:

```go
loading bool
org     string
```

Wire in newModel:

```go
loading: cfg.loading,
org:     cfg.org,
```

Define the message types:

```go
type dataLoadedMsg struct {
	prs     []PRNode
	myTeams map[string]bool
}

type fetchErrMsg struct {
	err error
}
```

Define the fetch command:

```go
func fetchDataCmd(org string) tea.Cmd {
	return func() tea.Msg {
		me, err := fetchCurrentUser()
		if err != nil {
			return fetchErrMsg{err}
		}
		prs, err := fetchOpenPRs(org)
		if err != nil {
			return fetchErrMsg{err}
		}
		myTeams, err := fetchUserTeams(org)
		if err != nil {
			// Non-fatal, continue with empty teams
			myTeams = make(map[string]bool)
		}
		return dataLoadedMsg{prs: prs, myTeams: myTeams}
	}
}
```

Note: We can't easily factor out `me` from this because we need it for classification. We'll store it when data arrives. Actually, `me` should be fetched once at startup and stored. Let's include it in the message:

```go
type dataLoadedMsg struct {
	prs     []PRNode
	me      string
	myTeams map[string]bool
}

func fetchDataCmd(org string) tea.Cmd {
	return func() tea.Msg {
		me, err := fetchCurrentUser()
		if err != nil {
			return fetchErrMsg{err}
		}
		prs, err := fetchOpenPRs(org)
		if err != nil {
			return fetchErrMsg{err}
		}
		myTeams, err := fetchUserTeams(org)
		if err != nil {
			myTeams = make(map[string]bool)
		}
		return dataLoadedMsg{prs: prs, me: me, myTeams: myTeams}
	}
}
```

Update `Init()`:

```go
func (m model) Init() tea.Cmd {
	if m.loading {
		return fetchDataCmd(m.org)
	}
	return nil
}
```

Handle messages in `Update()`:

```go
case dataLoadedMsg:
	m.loading = false
	m.rawPRs = msg.prs
	m.me = msg.me
	m.myTeams = msg.myTeams
	m.reclassify()
	m.cursor = 0

case fetchErrMsg:
	m.loading = false
	m.errMsg = msg.err.Error()
```

Add `r` key handler:

```go
case "r":
	m.loading = true
	return m, fetchDataCmd(m.org)
```

Update `View()` for loading state:

```go
func (m model) View() string {
	if m.loading {
		return "Fetching PRs...\n"
	}
	if m.errMsg != "" {
		return fmt.Sprintf("Error: %s\nPress r to retry, q to quit.\n", m.errMsg)
	}
	// ... rest of view
}
```

Add `errMsg` field to model:

```go
errMsg string
```

**Step 4: Run tests to verify they pass**

Run: `go test ./... -run "TestModel_LoadingState|TestModel_RefreshKey|TestModel_DataLoaded" -v`
Expected: PASS

**Step 5: Update `main.go` to use async startup**

Replace the blocking fetch in `main.go` with starting the TUI in loading mode:

```go
// For TUI path, start with loading spinner instead of blocking
if !*plain {
	p := tea.NewProgram(newModel(modelConfig{
		loading:    true,
		org:        *org,
		showSelf:   *self,
		showMine:   *mine,
		showAuthor: *author,
	}), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	return
}

// Plain path: still fetch synchronously (no TUI to show spinner in)
me, err := fetchCurrentUser()
// ... rest of plain path unchanged
```

**Step 6: Run all tests**

Run: `go test ./... -v`
Expected: all PASS

**Step 7: Build**

Run: `go build .`
Expected: builds successfully

**Step 8: Commit**

```bash
git add tui.go main.go tui_test.go
git commit -m "feat: add async fetch with loading spinner and r to refresh"
```

---

## Phase 6: Cleanup

### Task 16: Remove old classification code

Now that nothing references the old system, remove dead code.

**Files:**
- Modify: `classify.go` (remove old constants, `classify()`, old `classifyAll()`, `statePriority`)
- Modify: `classify_test.go` (remove old tests)
- Modify: `tui.go` (remove `formatTag()`, old tag styles)

**Step 1: Run all tests before cleanup**

Run: `go test ./... -v`
Expected: all PASS

**Step 2: Remove dead code**

From `classify.go`, remove:
- `StateNew`, `StateCommented`, `StateDismissed`, `StateStale` constants
- `ReviewState` type (if nothing uses it — check first)
- `statePriority` map
- `classify()` function
- Old `classifyAll()` function
- `ClassifiedPR.State` field

Actually, check if `ReviewState` is still used. The `State` field on `ClassifiedPR` and `formatTag()` reference it. If we've replaced all usages, remove it. If `renderPlain` still uses `pr.State`, update it.

From `tui.go`, remove:
- `tagNew`, `tagCmt`, `tagDis`, `tagStl` styles
- `formatTag()` function

From `classify_test.go`, remove:
- All `TestClassify_*` tests (the old single-state classify function)
- `TestClassifyAll_*` tests that test the old `classifyAll` (keep tests for `classifyAllNew`)

**Step 3: Rename `classifyAllNew` to `classifyAll`**

Now that the old one is gone, rename for clarity. Update all references in `classify.go`, `classify_test.go`, `tui.go`, and `main.go`.

**Step 4: Run all tests**

Run: `go test ./... -v`
Expected: all PASS

**Step 5: Build**

Run: `go build .`
Expected: builds successfully

**Step 6: Commit**

```bash
git add classify.go classify_test.go tui.go main.go plain.go
git commit -m "refactor: remove old single-tag classification system"
```

---

### Task 17: Final verification

**Step 1: Run all tests**

Run: `go test ./... -v`
Expected: all PASS

**Step 2: Build**

Run: `go build .`
Expected: builds successfully

**Step 3: Check test coverage**

Run: `go test ./... -cover`
Expected: reasonable coverage on classify.go and tui.go

**Step 4: Verify no leftover dead code**

Run: `go vet ./...`
Expected: no issues

**Step 5: Commit any final fixes**

Only if needed.

---

## Summary of all changes

| File | Changes |
|------|---------|
| `classify.go` | New indicator types (`MyReviewIndicator`, `OthReviewIndicator`, `ActivityIndicator`); indicator fields on `ClassifiedPR`; `computeMyReview()`, `computeOthReview()`, `computeActivity()`, `computeAuthorActivity()`; `classifyAllNew()` with new sort; `classifyAllAuthor()`; remove old `classify()`, `classifyAll()`, `ReviewState`, tag constants |
| `classify_test.go` | ~30 new tests for indicator computation, classification, sort order; `withCommentAt()` helper; remove old `TestClassify_*` tests |
| `github.go` | Add `IsDraft` to `PRNode`; add `isDraft` to GraphQL query; add `CreatedAt` to `CommentNode`; add `createdAt` to comments query |
| `github_test.go` | Tests for draft field and comment createdAt parsing |
| `tui.go` | Replace tag styles with indicator styles; `formatIndicators()`; add `showAuthor`, `loading`, `org`, `errMsg` to model; update `reclassify()`, `View()`, help bar, empty state; add `a` and `r` key handlers; guard `s`/`m` in author mode; `dataLoadedMsg`, `fetchErrMsg`, `fetchDataCmd()`; remove `formatTag()` |
| `tui_test.go` | Tests for author mode, indicator rendering, loading state, refresh, data loaded |
| `plain.go` | `plainIndicators()`; update `renderPlain()` to use indicators |
| `main.go` | Add `--author` flag; TUI starts in loading mode with async fetch; plain path uses `classifyAllNew()`/`classifyAllAuthor()` |
