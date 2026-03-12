# Focus Mode, Undo Dismissals, Key Remapping

## Problem

Three UX gaps:
1. No way to undo dismissals (d/D/A) without restarting
2. No way to focus on a single repo or author (only negative filtering via dismiss)
3. Author mode (`a`) is a special mode that focus-by-author subsumes

## Design

### Key Remapping

| Key | Old | New |
|-----|-----|-----|
| `f` | toggle assigned-only | focus on repo of selected PR |
| `F` | unused | focus on author of selected PR |
| `a` | toggle author mode | toggle assigned-only (was `f`) |
| `u` | unused | clear all dismissals |
| author mode | `a` key | **removed** |

All other keys unchanged: `d`, `D`, `A`, `s`, `o`, `r`, `c`, `?`, `q`, `j/k`, `enter`.

### Focus Mode

- `f` on a PR sets `focusRepo` to that PR's repo name. Only PRs from that repo are shown.
- `F` on a PR sets `focusAuthor` to that PR's author. Only PRs from that author are shown.
- Pressing `f`/`F` again or `Esc` clears the focus.
- Only one focus active at a time (`f` clears author focus and vice versa).
- Focus is additive with dismissals — dismissed items stay hidden even within a focused view.
- Status bar shows "Focus: repo sam-web-app" or "Focus: author dependabot" while active.
- Cursor resets to 0 when entering/exiting focus.

### Undo Dismissals

- `u` clears all three dismiss maps (dismissed, dismissedRepos, dismissedAuthors).
- Shows status message "Cleared all dismissals".
- Cursor resets to 0.

### Author Mode Removal

- Remove `showAuthor` field and all author mode branches from TUI.
- Remove `classifyAllAuthor` call from `reclassify()`.
- Remove author mode header variant (`📝 👥 💬`).
- Remove author mode branch from `formatIndicators()`.
- Keep `classifyAllAuthor` and `computeAuthorActivity` functions in classify.go (may be useful for plain mode or future use).
- Remove `--author` CLI flag if it exists.

### View Changes

- `visibleItems()` adds focus filtering: if `focusRepo != ""`, only show matching repo; if `focusAuthor != ""`, only show matching author.
- Help bar simplified: no author mode conditional, just one format string.
- Help bar shows focus state labels: `f: focus:repo-name` or `f: focus:off`.
- Legend updated with new keybindings.
- "No PRs" empty state message updated (remove author mode reference).
