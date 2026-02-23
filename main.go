package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	pflag "github.com/spf13/pflag"
)

func main() {
	org := pflag.String("org", "", "GitHub organization (or set GITHUB_ORG)")
	plain := pflag.Bool("plain", false, "Plain text output (no TUI)")
	self := pflag.Bool("self", false, "Include self-authored PRs")
	mine := pflag.Bool("mine", false, "Only show PRs where you are a requested reviewer")
	author := pflag.Bool("author", false, "Show your own PRs and their review status")
	pflag.Parse()

	if *org == "" {
		*org = os.Getenv("GITHUB_ORG")
	}
	if *org == "" {
		fmt.Fprintln(os.Stderr, "error: --org flag or GITHUB_ORG env var is required")
		pflag.Usage()
		os.Exit(1)
	}

	if _, err := ghToken(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if *plain {
		fmt.Fprintf(os.Stderr, "Fetching PRs for %s...\n", *org)

		me, err := fetchCurrentUser()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		prs, err := fetchOpenPRs(*org)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

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
		classified := classifyAll(prs, me, *self, filter)
		if len(classified) == 0 {
			fmt.Fprintln(os.Stderr, "No PRs pending your review.")
			return
		}
		renderPlain(os.Stdout, classified)
		return
	}

	// TUI path: start with loading spinner, fetch data async
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
}
