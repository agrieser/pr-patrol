# pr-patrol

See every PR waiting for you across an entire GitHub org — without clicking through dozens of repos.

pr-patrol fetches all open pull requests in an organization and classifies each one based on **your** review state — whether you've never seen it, left a comment, had your review dismissed, or the author pushed new commits since you last reviewed. PRs where your review is still current are hidden, so you only see what needs your attention.

```
$ pr-patrol --org kubernetes --plain
[NEW] kops#18008        justinsb     tests/ai-conformance: update test job for DRA v1
[NEW] website#54596     nojnhuh      [WIP] Docs update for KEP-5729: DRA ResourceClaim Support
[NEW] kubernetes#137192 kannon92     promote two test cases for ObservedGeneration to conformance
[STL] website#54321     liggitt      Update API reference docs for v1.33
[CMT] enhancements#5900 dims         KEP-4639: Update OCI VolumeSource
[DIS] kubernetes#136800 aojea        Fix endpoint reconciler for dual-stack
```

## Review States

| Tag | Meaning |
|-----|---------|
| `[NEW]` | You've never reviewed or commented on this PR |
| `[CMT]` | You've left comments but haven't submitted a formal review |
| `[DIS]` | Your review was dismissed by the PR author or a maintainer |
| `[STL]` | You reviewed, but new commits have been pushed since |

PRs where your review is still current (approved/changes-requested and no new commits) are automatically hidden.

## Install

Download a binary from [Releases](https://github.com/agrieser/pr-patrol/releases), or build from source:

```
go install github.com/agrieser/pr-patrol@latest
```

Requires the [GitHub CLI](https://cli.github.com) (`gh`) to be installed and authenticated (`gh auth login`).

## Usage

```
pr-patrol --org mycompany
```

This launches an interactive TUI with colored tags and keyboard navigation. Use `--plain` for scriptable text output:

```
# Count PRs awaiting your review
pr-patrol --org mycompany --plain | wc -l

# Set your org once
export GITHUB_ORG=mycompany
pr-patrol
```

### Flags

| Flag | Env Var | Description |
|------|---------|-------------|
| `--org` | `GITHUB_ORG` | GitHub organization (required) |
| `--plain` | | Plain text output, no TUI |
| `--self` | | Include your own PRs (excluded by default) |

### TUI Keys

| Key | Action |
|-----|--------|
| `j` / `k` / `↑` / `↓` | Navigate |
| `Enter` | Open PR in browser |
| `d` | Dismiss (session only) |
| `q` | Quit |
