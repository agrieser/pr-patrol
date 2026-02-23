# pr-patrol

See every PR waiting for you across an entire GitHub org — without clicking through dozens of repos.

pr-patrol fetches all open pull requests in an organization and shows each one with a three-column indicator display: **your review status**, **others' reviews**, and **comment activity**. Repo names and authors are color-coded so you can scan at a glance.

```
👤 👥 💬
· ✓ ● billing-svc#342   samantha  Add invoice PDF generation endpoint
· ± ○ web-app#891       danielk   Fix timezone handling in scheduler
~ · · billing-svc#339   rchen     Update Stripe webhook handler for new API version
✓ ✓ · auth-svc#156      rchen     Add SAML SSO support for enterprise accounts
✗ ± · billing-svc#337   mlopez    Refactor subscription tier logic
· · · web-app#885       jpark     Dark mode toggle in user preferences
· · ○ deploy-tools#78   danielk   Add canary deployment support to rollout script
✓ ✗ ● api-gateway#214   samantha  Rate limiting per API key
· ✓ · web-app#882       mlopez    Migrate user settings page to React 19
~ · ● auth-svc#153      jpark     Fix session expiry race condition
· · · data-pipeline#45  rchen     Add retry logic for failed ETL jobs
✗ · ○ web-app#878       danielk   Accessibility improvements for nav components
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
| `--authored` | | Include PRs you authored (excluded by default) |
| `--assigned` | | Only show PRs assigned to you for review |
| `--author` | | Show your own PRs and their review status |
| `--limit` | | Maximum PRs to fetch (default 500) |

### TUI Keys

| Key | Action |
|-----|--------|
| `j` / `k` / `↑` / `↓` | Navigate |
| `Enter` | Open PR in browser |
| `d` | Dismiss PR (session only) |
| `D` | Dismiss entire repo (session only) |
| `c` | Comment `@claude please review this PR` |
| `s` | Toggle showing PRs you authored |
| `f` | Toggle filtering to PRs assigned to you for review |
| `o` | Toggle sort order (priority / date) |
| `a` | Toggle author mode (see your PRs' review status) |
| `r` | Refresh data |
| `?` | Show indicator legend |
| `q` | Quit |
