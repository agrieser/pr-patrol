# pr-patrol

See every PR waiting for you across an entire GitHub org — without clicking through dozens of repos.

pr-patrol fetches all open pull requests in an organization and shows each one with a three-column indicator display: **your review status**, **others' reviews**, and **comment activity**. Repo names and authors are color-coded so you can scan at a glance.

```
👤 👥 💬
· ✓ ● sam-treatment-svc#199  orangejenny     SA3-3847: Email notifications MVP
· ● ○ sam-web-app#165        mhayto          SA3-3797: Fix left nav home icon
~ · · sam-treatment-svc#190  Robert-Costello  update functions to use currentClientId
✓ ✓ · sam-iam-svc#72         Robert-Costello  add flag for multi-client
✗ ● · sam-treatment-svc#195  cws1121          SA3-3792: Update PatientDoseReportController
```

## Indicator Columns

### Column 1 — Your Review (👤)

| Symbol | Meaning |
|--------|---------|
| `·` | No review yet |
| `✓` | You approved |
| `✗` | You requested changes |
| `~` | Your review is stale (new commits pushed since) |

### Column 2 — Others' Reviews (👥)

| Symbol | Meaning |
|--------|---------|
| `·` | No reviews yet |
| `✓` | All approved |
| `✗` | Changes requested |
| `±` | Mixed reviews |

### Column 3 — Comments (💬)

| Symbol | Meaning |
|--------|---------|
| `·` | No comments |
| `○` | Others commented |
| `●` | You commented |

Press `?` in the TUI to see this legend at any time.

## Install

Download a binary from [Releases](https://github.com/agrieser/pr-patrol/releases), or build from source:

```
go install github.com/agrieser/pr-patrol@latest
```

Requires a `GITHUB_TOKEN` environment variable with `repo` scope. You can create one at [github.com/settings/tokens](https://github.com/settings/tokens).

## Usage

```
pr-patrol --org mycompany
```

This launches an interactive TUI with colored indicators and keyboard navigation. Use `--plain` for scriptable text output:

```
# Set credentials once
export GITHUB_TOKEN=ghp_...
export GITHUB_ORG=mycompany

# Launch TUI
pr-patrol

# Or use plain text for scripting
pr-patrol --plain | wc -l
```

### Flags

| Flag | Env Var | Description |
|------|---------|-------------|
| | `GITHUB_TOKEN` | GitHub personal access token with `repo` scope (required) |
| `--org` | `GITHUB_ORG` | GitHub organization (required) |
| `--plain` | | Plain text output, no TUI |
| `--self` | | Include your own PRs (excluded by default) |
| `--mine` | | Only show PRs where you are a requested reviewer |
| `--author` | | Show your own PRs and their review status |

### TUI Keys

| Key | Action |
|-----|--------|
| `j` / `k` / `↑` / `↓` | Navigate |
| `Enter` | Open PR in browser |
| `d` | Dismiss PR (session only) |
| `c` | Comment `@claude please review this PR` |
| `s` | Toggle showing self-authored PRs |
| `m` | Toggle filtering to requested-reviewer PRs |
| `a` | Toggle author mode (see your PRs' review status) |
| `r` | Refresh data |
| `?` | Show indicator legend |
| `q` | Quit |
