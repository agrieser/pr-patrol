# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
go build .                          # Build binary
go test ./...                       # Run all tests
go test -v ./...                    # Verbose test output
go test -run TestComputeMyReview    # Run a single test by name
go test -run "TestClassifyAll_"     # Run tests matching a pattern
go vet ./...                        # Static analysis
```

## Architecture

Single-package Go CLI (`package main`) that fetches open PRs across a GitHub org and shows which ones need your review using a three-column indicator system.

**Auth:** `GITHUB_TOKEN` environment variable (required). No external CLI dependencies.

**Data flow:** `main.go` orchestrates:

1. **github.go** — Direct HTTP calls to the GitHub API (`net/http`). `ghToken()` reads env var, `ghRequest()` makes authenticated requests, `ghRequestPaginated()` handles Link-header pagination. Fetches current user (`fetchCurrentUser`), team memberships (`fetchUserTeams`), and paginated open PRs via GraphQL (`fetchOpenPRs`). Also `postComment()` for the @claude review feature. Defines `PRNode`, `ReviewNode`, `CommentNode`, `CommitNode`, `ReviewRequestNode` types matching the GraphQL response shape.

2. **classify.go** — Three independent compute functions determine per-PR indicators:
   - `computeMyReview()` → `MyReviewIndicator` (none/approved/changes/stale)
   - `computeOthReview()` → `OthReviewIndicator` (none/approved/changes/mixed)
   - `computeActivity()` → `ActivityIndicator` (none/others/mine)
   - `computeAuthorActivity()` — variant for author mode

   `classifyAll()` filters, classifies, and sorts PRs for reviewer mode. `classifyAllAuthor()` does the same for author mode. Both return `[]ClassifiedPR`.

3. **tui.go** — BubbleTea interactive terminal UI. The `model` struct holds raw `PRNode` data for runtime re-classification when toggling filters. Key features:
   - Three-column color-coded indicators (`formatIndicators()`)
   - Color-coded repo names and authors via FNV hash → ANSI-256 palette (`nameColor()`)
   - Runtime toggles: `s` (self), `m` (mine), `a` (author mode)
   - Async data fetch with loading state (`fetchDataCmd`, `dataLoadedMsg`)
   - `openBrowser()` for platform-specific URL opening
   - Legend overlay (`?` key), refresh (`r`), @claude comment (`c`)

4. **plain.go** — `renderPlain()` writes tabular text with `plainIndicators()` to an `io.Writer` for scripting/piping.

## Key Design Decisions

- **Raw PRs stored in model** — The TUI keeps `rawPRs []PRNode` so toggling filters (s/m/a) can instantly re-classify without re-fetching.
- **Dismissed by URL** — `dismissed map[string]bool` uses PR URL as key so dismissals survive re-classification.
- **No gh CLI dependency** — All GitHub API access is direct HTTP via `GITHUB_TOKEN`.

## Testing Patterns

Tests use Go's standard `testing` package with a builder pattern for constructing test PRs:

```go
pr := makePR(
    withAuthor("alice"),
    withReview("me", "APPROVED", reviewTime),
    withLastCommit(commitTime),
    withComment("bob"),
    withURL("https://github.com/org/repo/pull/1"),
)
result := computeMyReview(pr, "me")
```

Builders: `makePR()`, `withReview()`, `withComment()`, `withCommentAt()`, `withAuthor()`, `withLastCommit()`, `withReviewRequest()`, `withURL()` — defined in `classify_test.go` and `tui_test.go`.

TUI tests use `sendKey()` and `sendMsg()` helpers to simulate keyboard input and async messages on the BubbleTea model.

## Release

Push a semver tag to trigger the GitHub Actions release workflow:

```bash
git tag v0.X.0 -m "description" && git push origin v0.X.0
```

This runs tests, cross-compiles for darwin/linux/windows (amd64+arm64), and creates a GitHub Release with binaries.
