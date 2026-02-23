package main

import (
	"fmt"
	"io"
)

type colWidths struct {
	repo   int
	author int
}

func computeColumns(items []ClassifiedPR) colWidths {
	var w colWidths
	for _, pr := range items {
		repoCol := fmt.Sprintf("%s#%d", pr.RepoName, pr.Number)
		if len(repoCol) > w.repo {
			w.repo = len(repoCol)
		}
		if len(pr.Author) > w.author {
			w.author = len(pr.Author)
		}
	}
	return w
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

	return col1 + " " + col2 + " " + col3
}

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
