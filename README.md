# pr-patrol

See every PR waiting for you across an entire GitHub org — without clicking through dozens of repos.

pr-patrol fetches all open pull requests in an organization and shows each one with a three-column indicator display: **your review status**, **others' reviews**, and **comment activity**. Repo names and authors are color-coded so you can scan at a glance.

```ansi
[2m👤 👥 💬[0m
[2m·[0m [32m✓[0m [36m●[0m [35mbilling-svc#342 [0m [32msamantha[0m  Add invoice PDF generation endpoint
[2m·[0m [33m±[0m [37m○[0m [36mweb-app#891     [0m [33mdanielk [0m  Fix timezone handling in scheduler
[2m~[0m [2m·[0m [2m·[0m [35mbilling-svc#339 [0m [36mrchen   [0m  Update Stripe webhook handler for new API version
[32m✓[0m [32m✓[0m [2m·[0m [34mauth-svc#156    [0m [36mrchen   [0m  Add SAML SSO support for enterprise accounts
[31m✗[0m [33m±[0m [2m·[0m [35mbilling-svc#337 [0m [31mmlopez  [0m  Refactor subscription tier logic
[2m·[0m [2m·[0m [2m·[0m [36mweb-app#885     [0m [33mjpark   [0m  Dark mode toggle in user preferences
[2m·[0m [2m·[0m [37m○[0m [34mdeploy-tools#78 [0m [33mdanielk [0m  Add canary deployment support to rollout script
[32m✓[0m [31m✗[0m [36m●[0m [31mapi-gateway#214 [0m [32msamantha[0m  Rate limiting per API key
[2m·[0m [32m✓[0m [2m·[0m [36mweb-app#882     [0m [31mmlopez  [0m  Migrate user settings page to React 19
[2m~[0m [2m·[0m [36m●[0m [34mauth-svc#153    [0m [33mjpark   [0m  Fix session expiry race condition
[2m·[0m [2m·[0m [2m·[0m [31mdata-pipeline#45[0m [36mrchen   [0m  Add retry logic for failed ETL jobs
[31m✗[0m [2m·[0m [37m○[0m [36mweb-app#878     [0m [33mdanielk [0m  Accessibility improvements for nav components
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

### Download binary

Download from [Releases](https://github.com/agrieser/pr-patrol/releases).

On macOS, remove the quarantine flag before running:

```
xattr -d com.apple.quarantine ./pr-patrol-darwin-arm64
chmod +x ./pr-patrol-darwin-arm64
```

### Build from source

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
| `--limit` | | Maximum PRs to fetch (default 500) |

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
