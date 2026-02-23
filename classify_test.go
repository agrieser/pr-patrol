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

func withReviewRequest(login, teamSlug string, asCodeOwner bool) func(*PRNode) {
	return func(pr *PRNode) {
		rr := ReviewRequestNode{AsCodeOwner: asCodeOwner}
		rr.RequestedReviewer.Login = login
		rr.RequestedReviewer.Slug = teamSlug
		pr.ReviewRequests.Nodes = append(pr.ReviewRequests.Nodes, rr)
	}
}

func withCommentAt(login string, at time.Time) func(*PRNode) {
	return func(pr *PRNode) {
		c := CommentNode{CreatedAt: at}
		c.Author.Login = login
		pr.Comments.Nodes = append(pr.Comments.Nodes, c)
	}
}

// --- isRequestedReviewer tests ---

func TestIsRequestedReviewer_DirectUser(t *testing.T) {
	pr := makePR(withReviewRequest("me", "", false))
	if !isRequestedReviewer(pr, "me", nil) {
		t.Fatal("expected true for direct user match")
	}
}

func TestIsRequestedReviewer_TeamMember(t *testing.T) {
	pr := makePR(withReviewRequest("", "backend-team", true))
	myTeams := map[string]bool{"backend-team": true}
	if !isRequestedReviewer(pr, "me", myTeams) {
		t.Fatal("expected true for team membership match")
	}
}

func TestIsRequestedReviewer_TeamNonMember(t *testing.T) {
	pr := makePR(withReviewRequest("", "frontend-team", true))
	myTeams := map[string]bool{"backend-team": true}
	if isRequestedReviewer(pr, "me", myTeams) {
		t.Fatal("expected false for non-member team")
	}
}

func TestIsRequestedReviewer_NoRequests(t *testing.T) {
	pr := makePR()
	if isRequestedReviewer(pr, "me", nil) {
		t.Fatal("expected false for no review requests")
	}
}

func TestIsRequestedReviewer_OtherUser(t *testing.T) {
	pr := makePR(withReviewRequest("someone-else", "", false))
	if isRequestedReviewer(pr, "me", nil) {
		t.Fatal("expected false for different user")
	}
}

// --- computeMyReview tests ---

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
	if got := computeMyReview(pr, "me"); got != MyNone {
		t.Fatalf("expected MyNone (COMMENTED is not a substantive review), got %s", got)
	}
}

// --- computeOthReview tests ---

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

// --- computeActivity tests ---

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

func TestComputeActivity_ReviewCountsAsActivity(t *testing.T) {
	pr := makePR(withReview("me", "APPROVED", time.Now()))
	if got := computeActivity(pr, "me"); got != ActMine {
		t.Fatalf("expected ActMine (review counts as activity), got %s", got)
	}
}

func TestComputeActivity_OthersReviewCountsAsActivity(t *testing.T) {
	pr := makePR(withReview("alice", "COMMENTED", time.Now()))
	if got := computeActivity(pr, "me"); got != ActOthers {
		t.Fatalf("expected ActOthers (others' review counts as activity), got %s", got)
	}
}

func TestComputeActivity_ReviewNoComments(t *testing.T) {
	// PR with only a review (no issue comments) should still show activity
	pr := makePR(withReview("alice", "CHANGES_REQUESTED", time.Now()))
	if got := computeActivity(pr, "me"); got != ActOthers {
		t.Fatalf("expected ActOthers, got %s", got)
	}
}

// --- computeAuthorActivity tests ---

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

// --- Indicator types ---

func TestIndicatorTypes_Exist(t *testing.T) {
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

func TestClassifiedPR_HasIndicatorFields(t *testing.T) {
	pr := ClassifiedPR{
		MyReview:  MyApproved,
		OthReview: OthMixed,
		Activity:  ActMine,
		IsDraft:   true,
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

// --- classifyAll tests ---

func TestClassifyAll_BasicIndicators(t *testing.T) {
	prs := []PRNode{
		makePR(withAuthor("alice"), withURL("https://github.com/org/repo/pull/1")),
		makePR(
			withAuthor("bob"),
			withReview("someone", "APPROVED", time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)),
			withComment("me"),
			withURL("https://github.com/org/repo/pull/2"),
		),
	}
	result := classifyAll(prs, "me", false, nil, SortPriority)
	if len(result) != 2 {
		t.Fatalf("expected 2 PRs, got %d", len(result))
	}

	if result[0].MyReview != MyNone {
		t.Errorf("PR1: expected MyNone, got %s", result[0].MyReview)
	}
	if result[0].OthReview != OthNone {
		t.Errorf("PR1: expected OthNone, got %s", result[0].OthReview)
	}
	if result[0].Activity != ActNone {
		t.Errorf("PR1: expected ActNone, got %s", result[0].Activity)
	}

	if result[1].OthReview != OthApproved {
		t.Errorf("PR2: expected OthApproved, got %s", result[1].OthReview)
	}
	if result[1].Activity != ActMine {
		t.Errorf("PR2: expected ActMine, got %s", result[1].Activity)
	}
}

func TestClassifyAll_ExcludesSelf(t *testing.T) {
	prs := []PRNode{
		makePR(withAuthor("me")),
		makePR(withAuthor("other")),
	}
	result := classifyAll(prs, "me", false, nil, SortPriority)
	if len(result) != 1 {
		t.Fatalf("expected 1 PR, got %d", len(result))
	}
}

func TestClassifyAll_IncludesSelf(t *testing.T) {
	prs := []PRNode{
		makePR(withAuthor("me")),
		makePR(withAuthor("other")),
	}
	result := classifyAll(prs, "me", true, nil, SortPriority)
	if len(result) != 2 {
		t.Fatalf("expected 2 PRs, got %d", len(result))
	}
}

func TestClassifyAll_WithFilter(t *testing.T) {
	prs := []PRNode{
		makePR(withReviewRequest("me", "", false)),
		makePR(),
	}
	filter := func(pr PRNode) bool {
		return isRequestedReviewer(pr, "me", nil)
	}
	result := classifyAll(prs, "me", false, filter, SortPriority)
	if len(result) != 1 {
		t.Fatalf("expected 1 PR, got %d", len(result))
	}
}

func TestClassifyAll_SortOrder(t *testing.T) {
	reviewTime := time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC)
	commitAfter := time.Date(2025, 1, 4, 0, 0, 0, 0, time.UTC)
	commitBefore := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)

	prs := []PRNode{
		makePR(
			withAuthor("alice"),
			withReview("me", "APPROVED", reviewTime),
			withLastCommit(commitBefore),
			withURL("https://github.com/org/repo/pull/1"),
		),
		makePR(
			withAuthor("bob"),
			withURL("https://github.com/org/repo/pull/2"),
		),
		makePR(
			withAuthor("carol"),
			withReview("me", "APPROVED", reviewTime),
			withLastCommit(commitAfter),
			withURL("https://github.com/org/repo/pull/3"),
		),
	}
	result := classifyAll(prs, "me", false, nil, SortPriority)
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

func TestClassifyAll_DraftField(t *testing.T) {
	pr := makePR(withAuthor("alice"))
	pr.IsDraft = true
	result := classifyAll([]PRNode{pr}, "me", false, nil, SortPriority)
	if len(result) != 1 {
		t.Fatalf("expected 1 PR, got %d", len(result))
	}
	if !result[0].IsDraft {
		t.Fatal("expected IsDraft to be true")
	}
}

func TestClassifyAll_IncludesApproved(t *testing.T) {
	reviewTime := time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC)
	commitBefore := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	prs := []PRNode{
		makePR(
			withAuthor("alice"),
			withReview("me", "APPROVED", reviewTime),
			withLastCommit(commitBefore),
		),
	}
	result := classifyAll(prs, "me", false, nil, SortPriority)
	if len(result) != 1 {
		t.Fatalf("expected 1 PR (approved PRs no longer hidden), got %d", len(result))
	}
}

// --- classifyAllAuthor tests ---

func TestClassifyAllAuthor_OnlyOwnPRs(t *testing.T) {
	prs := []PRNode{
		makePR(withAuthor("me")),
		makePR(withAuthor("other")),
	}
	result := classifyAllAuthor(prs, "me", SortPriority)
	if len(result) != 1 {
		t.Fatalf("expected 1 PR (only mine), got %d", len(result))
	}
}

func TestClassifyAllAuthor_DraftIndicator(t *testing.T) {
	pr := makePR(withAuthor("me"))
	pr.IsDraft = true
	result := classifyAllAuthor([]PRNode{pr}, "me", SortPriority)
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
	result := classifyAllAuthor([]PRNode{pr}, "me", SortPriority)
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
	result := classifyAllAuthor([]PRNode{pr}, "me", SortPriority)
	if result[0].Activity != ActMine {
		t.Fatalf("expected ActMine (new comments since push), got %s", result[0].Activity)
	}
}

func TestClassifyAllAuthor_SortOrder(t *testing.T) {
	prs := []PRNode{
		makePR(withAuthor("me"), withURL("https://github.com/org/repo/pull/1")),
		makePR(
			withAuthor("me"),
			withReview("alice", "APPROVED", time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)),
			withURL("https://github.com/org/repo/pull/2"),
		),
		makePR(
			withAuthor("me"),
			withReview("bob", "CHANGES_REQUESTED", time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)),
			withURL("https://github.com/org/repo/pull/3"),
		),
	}
	result := classifyAllAuthor(prs, "me", SortPriority)
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

func TestClassifyAll_SortByDate(t *testing.T) {
	old := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	mid := time.Date(2025, 1, 5, 0, 0, 0, 0, time.UTC)
	recent := time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC)

	prs := []PRNode{
		makePR(withAuthor("alice"), withLastCommit(old), withURL("https://github.com/org/repo/pull/1")),
		makePR(withAuthor("bob"), withLastCommit(recent), withURL("https://github.com/org/repo/pull/2")),
		makePR(withAuthor("carol"), withLastCommit(mid), withURL("https://github.com/org/repo/pull/3")),
	}
	result := classifyAll(prs, "me", false, nil, SortDate)
	if len(result) != 3 {
		t.Fatalf("expected 3 PRs, got %d", len(result))
	}
	// Most recently active first
	if result[0].Author != "bob" {
		t.Errorf("expected bob first (most recent), got %s", result[0].Author)
	}
	if result[1].Author != "carol" {
		t.Errorf("expected carol second, got %s", result[1].Author)
	}
	if result[2].Author != "alice" {
		t.Errorf("expected alice last (oldest), got %s", result[2].Author)
	}
}

func TestComputeLastActivity(t *testing.T) {
	created := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	commitTime := time.Date(2025, 1, 5, 0, 0, 0, 0, time.UTC)
	reviewTime := time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC)

	pr := makePR(
		withLastCommit(commitTime),
		withReview("alice", "APPROVED", reviewTime),
	)
	pr.CreatedAt = created
	got := computeLastActivity(pr)
	if !got.Equal(reviewTime) {
		t.Fatalf("expected %v, got %v", reviewTime, got)
	}
}
