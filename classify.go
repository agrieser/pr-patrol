package main

import (
	"sort"
	"time"
)

type MyReviewIndicator string

const (
	MyNone     MyReviewIndicator = "none"
	MyApproved MyReviewIndicator = "approved"
	MyChanges  MyReviewIndicator = "changes"
	MyStale    MyReviewIndicator = "stale"
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
	ActNone   ActivityIndicator = "none"
	ActOthers ActivityIndicator = "others"
	ActMine   ActivityIndicator = "mine"
)

type ClassifiedPR struct {
	MyReview     MyReviewIndicator
	OthReview    OthReviewIndicator
	Activity     ActivityIndicator
	IsDraft      bool
	RepoName     string
	RepoFullName string
	Number       int
	Title        string
	Author       string
	URL          string
	CreatedAt    time.Time
}

func computeMyReview(pr PRNode, me string) MyReviewIndicator {
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

	if len(pr.Commits.Nodes) > 0 {
		lastCommit := pr.Commits.Nodes[0].Commit.CommittedDate
		if lastCommit.After(lastReview.SubmittedAt) {
			return MyStale
		}
	}

	switch lastReview.State {
	case "APPROVED":
		return MyApproved
	case "CHANGES_REQUESTED":
		return MyChanges
	}
	return MyNone
}

func computeOthReview(pr PRNode, me string) OthReviewIndicator {
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
	// Reviews (approve, request changes, comment) also count as activity
	for _, r := range pr.Reviews.Nodes {
		if r.Author.Login == me {
			hasMine = true
		} else if r.Author.Login != "" {
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

func computeAuthorActivity(pr PRNode) ActivityIndicator {
	if len(pr.Comments.Nodes) == 0 && len(pr.Reviews.Nodes) == 0 {
		return ActNone
	}

	var lastCommit time.Time
	if len(pr.Commits.Nodes) > 0 {
		lastCommit = pr.Commits.Nodes[0].Commit.CommittedDate
	}

	for _, c := range pr.Comments.Nodes {
		if lastCommit.IsZero() || c.CreatedAt.After(lastCommit) {
			return ActMine
		}
	}
	for _, r := range pr.Reviews.Nodes {
		if lastCommit.IsZero() || r.SubmittedAt.After(lastCommit) {
			return ActMine
		}
	}
	return ActOthers
}

func isRequestedReviewer(pr PRNode, me string, myTeams map[string]bool) bool {
	for _, rr := range pr.ReviewRequests.Nodes {
		if rr.RequestedReviewer.Login == me {
			return true
		}
		if rr.RequestedReviewer.Slug != "" && myTeams[rr.RequestedReviewer.Slug] {
			return true
		}
	}
	return false
}

func sortPriority(pr ClassifiedPR) int {
	switch pr.MyReview {
	case MyStale:
		return 0
	case MyNone:
		if pr.OthReview == OthNone {
			return 1
		}
		return 2
	case MyChanges:
		return 3
	case MyApproved:
		return 4
	}
	return 5
}

func classifyAll(prs []PRNode, me string, includeSelf bool, filter func(PRNode) bool) []ClassifiedPR {
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
			MyReview:     MyNone,
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
