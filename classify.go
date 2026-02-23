package main

import (
	"sort"
	"time"
)

type ReviewState string

const (
	StateNew       ReviewState = "NEW"
	StateCommented ReviewState = "CMT"
	StateDismissed ReviewState = "DIS"
	StateStale     ReviewState = "STL"
)

type ClassifiedPR struct {
	State        ReviewState
	RepoName     string
	RepoFullName string
	Number       int
	Title        string
	Author       string
	URL          string
	CreatedAt    time.Time
}

var statePriority = map[ReviewState]int{
	StateNew:       0,
	StateCommented: 1,
	StateDismissed: 2,
	StateStale:     3,
}

func classify(pr PRNode, me string) (ReviewState, bool) {
	// Collect my submitted reviews (skip PENDING)
	var myReviews []ReviewNode
	for _, r := range pr.Reviews.Nodes {
		if r.Author.Login == "" || r.Author.Login != me {
			continue
		}
		if r.State == "PENDING" {
			continue
		}
		myReviews = append(myReviews, r)
	}

	// Check if I've commented (issue-level comments)
	hasComment := false
	for _, c := range pr.Comments.Nodes {
		if c.Author.Login == me {
			hasComment = true
			break
		}
	}

	// No reviews from me
	if len(myReviews) == 0 {
		if hasComment {
			return StateCommented, true
		}
		return StateNew, true
	}

	// Look at my most recent review
	lastReview := myReviews[len(myReviews)-1]

	if lastReview.State == "DISMISSED" {
		return StateDismissed, true
	}

	// Check if stale: latest commit is newer than my last review
	if len(pr.Commits.Nodes) > 0 {
		lastCommitDate := pr.Commits.Nodes[0].Commit.CommittedDate
		if lastCommitDate.After(lastReview.SubmittedAt) {
			return StateStale, true
		}
	}

	// My review is current — skip
	return "", false
}

func classifyAll(prs []PRNode, me string, includeSelf bool) []ClassifiedPR {
	var result []ClassifiedPR
	for _, pr := range prs {
		if !includeSelf && pr.Author.Login == me {
			continue
		}
		state, include := classify(pr, me)
		if !include {
			continue
		}
		result = append(result, ClassifiedPR{
			State:        state,
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
		pi, pj := statePriority[result[i].State], statePriority[result[j].State]
		if pi != pj {
			return pi < pj
		}
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result
}
