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
	pflag.Parse()

	if *org == "" {
		*org = os.Getenv("GITHUB_ORG")
	}
	if *org == "" {
		fmt.Fprintln(os.Stderr, "error: --org flag or GITHUB_ORG env var is required")
		pflag.Usage()
		os.Exit(1)
	}

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

	classified := classifyAll(prs, me, *self)

	if len(classified) == 0 {
		fmt.Fprintln(os.Stderr, "No PRs pending your review.")
		return
	}

	if *plain {
		renderPlain(os.Stdout, classified)
		return
	}

	p := tea.NewProgram(newModel(classified), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
