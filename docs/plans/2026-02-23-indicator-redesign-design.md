# Indicator Redesign + Author Mode — Design Document

## Problem

The current `[TAG]` system only reflects the user's own review state. PRs they haven't touched all show `[NEW]`, even if others have approved or requested changes. A wall of 27 `[NEW]` tags gives no triage signal.

## Solution

Replace the single `[TAG]` column with three single-character indicator columns. Each column answers a different question, is color-coded, and provides at-a-glance triage info.

## Display Format

```
👤👥💬
~  ✓ ○   sam-web-app#162       alice   Add login page
·  · ·   sam-iam-svc#72        bob     Fix null check
✓  ✓ ●   sam-treatment-svc#190 carol   Update API docs
✗  ± ●   sam-devops-scripts#136 dave   Refactor auth
```

Three columns with emoji headers: 👤 (my review), 👥 (others' reviews), 💬 (activity).

---

## Reviewer Mode (default)

### 👤 My Review

| Symbol | Color     | Meaning                                    |
|--------|-----------|--------------------------------------------|
| `·`    | dim/gray  | Haven't reviewed                           |
| `✓`    | green     | I approved (current — no new commits)      |
| `✓`    | dim/gray  | I approved but stale (new commits since)   |
| `✗`    | red       | I requested changes (current)              |
| `✗`    | dim/gray  | I requested changes but stale              |

### 👥 Others' Reviews

Deduplicated to latest review per reviewer. Skips PENDING and DISMISSED.

| Symbol | Color    | Meaning                                      |
|--------|----------|----------------------------------------------|
| `·`    | dim/gray | No reviews from anyone else                  |
| `✓`    | green    | All reviewers approved                       |
| `✗`    | red      | At least one reviewer requested changes      |
| `±`    | yellow   | Mixed/partial (some approved, no rejections) |

### 💬 Activity

| Symbol | Color     | Meaning                        |
|--------|-----------|---------------------------------|
| `·`    | dim/gray  | No comments                    |
| `○`    | white     | Comments exist, none are mine  |
| `●`    | cyan      | I've commented                 |

### Draft Indicator

PRs with `isDraft: true` get a `D` marker (details TBD — inline with repo name or separate). Drafts sort to bottom, visually dimmed.

### Sort Order (Reviewer Mode)

1. **Stale reviews** — I reviewed, new commits since. Most actionable.
2. **Untouched, no other reviews** — needs first eyes.
3. **Untouched, others have looked** — has activity, may still need me.
4. **I requested changes (current)** — ball's in their court.
5. **I approved (current)** — dimmed, at bottom.

Within each group, newest first (by created date).

---

## Author Mode (press `a`)

Shows the user's own PRs with review status.

### 👤 becomes Draft Status

| Symbol | Color     | Meaning          |
|--------|-----------|-------------------|
| `○`    | dim/gray  | Draft             |
| `●`    | white     | Ready for review  |

### 👥 Others' Reviews

Same as reviewer mode — shows who has reviewed your PR and their status.

### 💬 becomes New-Comments-Since-Push

| Symbol | Color     | Meaning                                   |
|--------|-----------|--------------------------------------------|
| `·`    | dim/gray  | No comments                               |
| `○`    | white     | Comments exist, all before my last push   |
| `●`    | cyan      | New comments since my last push           |

### Sort Order (Author Mode)

1. **Changes requested** — action needed from you.
2. **Has reviews, not all approved** — in progress.
3. **No reviews** — waiting.
4. **All approved** — ready to merge.

---

## Data Changes

### GraphQL Query Additions

- Add `isDraft` boolean to PR fragment
- Add `createdAt` to `comments` nodes (needed for "new since push" logic in author mode)

### New Fields on PRNode

```go
IsDraft bool `json:"isDraft"`
```

Comment nodes gain:
```go
CreatedAt time.Time `json:"createdAt"`
```

---

## Refresh (`r` key)

- Press `r` to re-fetch data from GitHub.
- Shows a spinner in the help bar while fetching. Current list stays visible.
- On startup, the TUI launches immediately with a spinner instead of blocking on stderr with "Fetching PRs...".
- Implemented via BubbleTea async `Cmd` pattern: `Init()` returns a fetch command, model renders loading state until data arrives.

---

## Toggles

| Key | Action                                        |
|-----|-----------------------------------------------|
| `j/k` | Navigate                                   |
| `enter` | Open PR in browser                       |
| `d` | Dismiss PR                                    |
| `s` | Toggle show self-authored PRs (reviewer mode) |
| `m` | Toggle "mine" filter (reviewer mode)          |
| `a` | Toggle author mode                            |
| `r` | Refresh data                                  |
| `q` | Quit                                          |

`s` and `m` are disabled in author mode (not applicable).

---

## What This Replaces

The entire `ReviewState` / `[TAG]` system is replaced:

- `StateNew`, `StateCommented`, `StateDismissed`, `StateStale` constants — removed
- `classify()` function — replaced by indicator computation
- `classifyAll()` — replaced by new classification that computes three indicators per PR
- `ClassifiedPR.State` field — replaced by three indicator fields
- `formatTag()` — removed
- `statePriority` sort map — replaced by new sort logic

The `ClassifiedPR` struct gets indicator fields instead of a single `State`:
```go
type ClassifiedPR struct {
    MyReview    IndicatorState
    OthReview   IndicatorState
    Activity    IndicatorState
    IsDraft     bool
    // ... existing fields (RepoName, Number, Title, Author, URL, CreatedAt)
}
```
