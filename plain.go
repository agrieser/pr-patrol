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

func renderPlain(w io.Writer, items []ClassifiedPR) {
	cols := computeColumns(items)
	for _, pr := range items {
		repoCol := fmt.Sprintf("%s#%d", pr.RepoName, pr.Number)
		fmt.Fprintf(w, "[%s] %-*s  %-*s  %s\n",
			pr.State,
			cols.repo, repoCol,
			cols.author, pr.Author,
			pr.Title)
	}
}
