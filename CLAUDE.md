# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
go build .                          # Build binary
go test ./...                       # Run all tests
go test -v ./...                    # Verbose test output
go test -run TestClassify_New       # Run a single test by name
go test -run "TestClassify_"        # Run tests matching a pattern
go run . --org <org>                # Run directly (requires gh CLI auth)
```

## Architecture

Single-package Go CLI (`package main`) that fetches open PRs across a GitHub org and shows which ones need your review.

**Data flow:** `main.go` orchestrates a linear pipeline:

1. **github.go** — Calls GitHub GraphQL API via `gh` CLI subprocess. Fetches current user (`fetchCurrentUser`) and paginated open PRs (`fetchOpenPRs`). Defines `PRNode`, `ReviewNode`, `CommentNode`, `CommitNode` types matching the GraphQL response shape.

2. **classify.go** — `classify()` determines a single PR's `ReviewState` (NEW/CMT/DIS/STL) based on your reviews, comments, and latest commit timestamps. `classifyAll()` filters out current reviews and self-authored PRs, then sorts by state priority then recency. Returns `[]ClassifiedPR`.

3. **tui.go** — BubbleTea interactive terminal UI. The `model` struct holds classified PRs, cursor position, session-local dismissals, and dynamic column widths. Opens PRs in browser via `gh pr view --web`.

4. **plain.go** — `renderPlain()` writes tabular text to an `io.Writer` for scripting/piping.

## External Dependency

All GitHub API access goes through the `gh` CLI as a subprocess (not direct HTTP). The tool requires `gh` to be installed and authenticated (`gh auth login`).

## Testing Patterns

Tests use Go's standard `testing` package with a builder pattern for constructing test PRs:

```go
pr := makePR(
    withReview("me", "APPROVED", reviewTime),
    withLastCommit(commitTime),
)
state, include := classify(pr, "me")
```

Builders: `makePR()`, `withReview()`, `withComment()`, `withAuthor()`, `withLastCommit()` — defined in `classify_test.go`.

TUI tests use a `sendKey()` helper to simulate keyboard input on the BubbleTea model.
