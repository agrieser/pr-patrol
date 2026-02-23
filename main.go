package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	pflag "github.com/spf13/pflag"
)

func main() {
	org := pflag.String("org", "", "GitHub organization (or set GITHUB_ORG)")
	plain := pflag.Bool("plain", false, "Plain text output (no TUI)")
	self := pflag.Bool("authored", false, "Include PRs you authored")
	mine := pflag.Bool("assigned", false, "Only show PRs assigned to you for review")
	author := pflag.Bool("author", false, "Show your own PRs and their review status")
	limit := pflag.Int("limit", 500, "Maximum number of PRs to fetch")
	dismissRepos := pflag.StringSlice("dismiss-repos", nil, "Repos to hide (comma-separated)")
	demo := pflag.Bool("demo", false, "Show demo data (for screenshots)")
	pflag.Parse()

	if *org == "" {
		*org = os.Getenv("GITHUB_ORG")
	}
	if *org == "" {
		fmt.Fprintln(os.Stderr, "error: --org flag or GITHUB_ORG env var is required")
		pflag.Usage()
		os.Exit(1)
	}

	if *demo {
		renderPlain(os.Stdout, demoData())
		return
	}

	if _, err := ghToken(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	dismissedRepoSet := make(map[string]bool)
	for _, r := range *dismissRepos {
		r = strings.TrimSpace(r)
		if r != "" {
			dismissedRepoSet[r] = true
		}
	}

	if *plain {
		fmt.Fprintf(os.Stderr, "Fetching PRs for %s...\n", *org)

		me, err := fetchCurrentUser()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		prs, err := fetchOpenPRs(*org, *limit)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		if *author {
			classified := classifyAllAuthor(prs, me, SortPriority)
			classified = filterDismissedRepos(classified, dismissedRepoSet)
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
		classified := classifyAll(prs, me, *self, filter, SortPriority)
		classified = filterDismissedRepos(classified, dismissedRepoSet)
		if len(classified) == 0 {
			fmt.Fprintln(os.Stderr, "No PRs pending your review.")
			return
		}
		renderPlain(os.Stdout, classified)
		return
	}

	// TUI path: start with loading spinner, fetch data async
	p := tea.NewProgram(newModel(modelConfig{
		loading:        true,
		org:            *org,
		limit:          *limit,
		showAuthored:   *self,
		showAssigned:   *mine,
		showAuthor:     *author,
		dismissedRepos: dismissedRepoSet,
	}), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
