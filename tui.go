package main

import (
	"fmt"
	"hash/fnv"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

var (
	styleGreen  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	styleRed    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	styleYellow = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	styleCyan   = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	styleDim    = lipgloss.NewStyle().Faint(true)
	styleWhite  = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))

	selBg     = lipgloss.NewStyle().Background(lipgloss.Color("238"))
	helpStyle = lipgloss.NewStyle().Faint(true)

	// Palette of distinguishable ANSI-256 colors for repo/author hashing.
	namePalette = []lipgloss.Color{
		"39",  // blue
		"168", // pink
		"114", // green
		"215", // orange
		"141", // purple
		"80",  // teal
		"203", // red
		"227", // yellow
		"75",  // sky
		"183", // lavender
	}
)

func nameColor(name string) lipgloss.Style {
	h := fnv.New32a()
	h.Write([]byte(name))
	return lipgloss.NewStyle().Foreground(namePalette[h.Sum32()%uint32(len(namePalette))])
}

type model struct {
	items        []ClassifiedPR
	cursor       int
	dismissed    map[string]bool
	dismissedRepos map[string]bool
	cols      colWidths
	width     int
	height    int

	rawPRs  []PRNode
	me      string
	myTeams map[string]bool

	showAuthored   bool
	showAssigned   bool
	showAuthor bool
	sortMode   SortMode

	loading      bool
	loadingCount int
	spinnerFrame int
	fetchID      int
	org          string
	limit        int
	errMsg       string
	showHelp     bool
	statusMsg    string
}

type modelConfig struct {
	rawPRs         []PRNode
	me             string
	myTeams        map[string]bool
	showAuthored   bool
	showAssigned   bool
	showAuthor     bool
	sortMode       SortMode
	loading        bool
	org            string
	limit          int
	dismissedRepos map[string]bool
}

type fetchPageMsg struct {
	prs     []PRNode
	me      string
	myTeams map[string]bool
	done    bool
	fetchID int
	ch      <-chan []PRNode
	errCh   <-chan error
}

type fetchErrMsg struct {
	err     error
	fetchID int
}

type commentPostedMsg struct {
	repo   string
	number int
	err    error
}

func postCommentCmd(repo string, number int, body string) tea.Cmd {
	return func() tea.Msg {
		err := postComment(repo, number, body)
		return commentPostedMsg{repo: repo, number: number, err: err}
	}
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("cmd", "/c", "start", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}

// startFetchCmd fetches user+teams in parallel, then streams PR pages on a channel.
// Returns the first fetchPageMsg once the first page (and user/teams) are ready.
func startFetchCmd(org string, limit int, fetchID int) tea.Cmd {
	return func() tea.Msg {
		var me string
		var myTeams map[string]bool
		var userErr, teamsErr error

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			me, userErr = fetchCurrentUser()
		}()
		go func() {
			defer wg.Done()
			myTeams, teamsErr = fetchUserTeams(org)
			if teamsErr != nil {
				myTeams = make(map[string]bool)
			}
		}()
		wg.Wait()

		if userErr != nil {
			return fetchErrMsg{err: userErr, fetchID: fetchID}
		}

		prCh := make(chan []PRNode, 1)
		errCh := make(chan error, 1)
		go func() {
			errCh <- fetchOpenPRsStreaming(org, limit, prCh)
		}()

		// Wait for first page
		prs, ok := <-prCh
		if !ok {
			// Channel closed immediately — check for error
			if err := <-errCh; err != nil {
				return fetchErrMsg{err: err, fetchID: fetchID}
			}
			return fetchPageMsg{
				me: me, myTeams: myTeams,
				done: true, fetchID: fetchID,
			}
		}
		return fetchPageMsg{
			prs: prs, me: me, myTeams: myTeams,
			done: false, fetchID: fetchID, ch: prCh, errCh: errCh,
		}
	}
}

// waitForPageCmd reads the next page from the channel.
func waitForPageCmd(ch <-chan []PRNode, errCh <-chan error, me string, myTeams map[string]bool, fetchID int) tea.Cmd {
	return func() tea.Msg {
		prs, ok := <-ch
		if !ok {
			// Channel closed — check for streaming error
			if errCh != nil {
				if err := <-errCh; err != nil {
					return fetchErrMsg{err: err, fetchID: fetchID}
				}
			}
			return fetchPageMsg{
				me: me, myTeams: myTeams,
				done: true, fetchID: fetchID,
			}
		}
		return fetchPageMsg{
			prs: prs, me: me, myTeams: myTeams,
			done: false, fetchID: fetchID, ch: ch, errCh: errCh,
		}
	}
}

func filterDismissedRepos(prs []ClassifiedPR, repos map[string]bool) []ClassifiedPR {
	if len(repos) == 0 {
		return prs
	}
	var out []ClassifiedPR
	for _, pr := range prs {
		if !repos[pr.RepoName] {
			out = append(out, pr)
		}
	}
	return out
}

func newModel(cfg modelConfig) model {
	dismissedRepos := cfg.dismissedRepos
	if dismissedRepos == nil {
		dismissedRepos = make(map[string]bool)
	}
	m := model{
		dismissed:      make(map[string]bool),
		dismissedRepos: dismissedRepos,
		rawPRs:     cfg.rawPRs,
		me:         cfg.me,
		myTeams:    cfg.myTeams,
		showAuthored:   cfg.showAuthored,
		showAssigned:   cfg.showAssigned,
		showAuthor: cfg.showAuthor,
		sortMode:   cfg.sortMode,
		loading:    cfg.loading,
		org:        cfg.org,
		limit:      cfg.limit,
	}
	if !m.loading {
		m.reclassify()
	}
	return m
}

func (m *model) reclassify() {
	if m.showAuthor {
		m.items = classifyAllAuthor(m.rawPRs, m.me, m.sortMode)
	} else {
		var filter func(PRNode) bool
		if m.showAssigned {
			me, teams := m.me, m.myTeams
			filter = func(pr PRNode) bool {
				return isRequestedReviewer(pr, me, teams)
			}
		}
		m.items = classifyAll(m.rawPRs, m.me, m.showAuthored, filter, m.sortMode)
	}
	m.cols = computeColumns(m.items)
}

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{tea.HideCursor}
	if m.loading {
		cmds = append(cmds, startFetchCmd(m.org, m.limit, m.fetchID), tickCmd())
	}
	return tea.Batch(cmds...)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case fetchPageMsg:
		if msg.fetchID != m.fetchID {
			return m, nil // stale fetch, ignore
		}
		m.errMsg = ""
		m.me = msg.me
		m.myTeams = msg.myTeams
		if msg.prs != nil {
			m.rawPRs = msg.prs
			m.loadingCount = len(msg.prs)
			m.reclassify()
		}
		// Clamp cursor
		vis := m.visibleItems()
		if m.cursor >= len(vis) && m.cursor > 0 {
			m.cursor = len(vis) - 1
		}
		if msg.done {
			m.loading = false
			return m, nil
		}
		return m, waitForPageCmd(msg.ch, msg.errCh, msg.me, msg.myTeams, msg.fetchID)
	case fetchErrMsg:
		if msg.fetchID != m.fetchID {
			return m, nil
		}
		m.loading = false
		m.errMsg = msg.err.Error()
	case tickMsg:
		if m.loading {
			m.spinnerFrame = (m.spinnerFrame + 1) % len(spinnerFrames)
			return m, tickCmd()
		}
	case commentPostedMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Failed to comment on %s#%d: %v", msg.repo, msg.number, msg.err)
		} else {
			m.statusMsg = fmt.Sprintf("Commented on %s#%d", msg.repo, msg.number)
		}
	case tea.KeyMsg:
		m.statusMsg = "" // clear status on any keypress
		if m.showHelp {
			m.showHelp = false
			return m, nil
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "?":
			m.showHelp = true
			return m, nil
		case "j", "down":
			vis := m.visibleItems()
			if m.cursor < len(vis)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "enter":
			if pr, ok := m.selectedPR(); ok {
				_ = openBrowser(pr.URL)
			}
		case "a":
			m.showAuthor = !m.showAuthor
			m.reclassify()
			m.cursor = 0
		case "s":
			if !m.showAuthor {
				m.showAuthored = !m.showAuthored
				m.reclassify()
				m.cursor = 0
			}
		case "f":
			if !m.showAuthor {
				m.showAssigned = !m.showAssigned
				m.reclassify()
				m.cursor = 0
			}
		case "o":
			if m.sortMode == SortDate {
				m.sortMode = SortPriority
			} else {
				m.sortMode = SortDate
			}
			m.reclassify()
			m.cursor = 0
		case "r":
			if m.org != "" {
				m.fetchID++
				m.loading = true
				m.loadingCount = 0
				m.spinnerFrame = 0
				m.errMsg = ""
				return m, tea.Batch(startFetchCmd(m.org, m.limit, m.fetchID), tickCmd())
			}
		case "d":
			if pr, ok := m.selectedPR(); ok {
				m.dismissed[pr.URL] = true
				vis := m.visibleItems()
				if m.cursor >= len(vis) && m.cursor > 0 {
					m.cursor--
				}
			}
		case "D":
			if pr, ok := m.selectedPR(); ok {
				m.dismissedRepos[pr.RepoName] = true
				m.statusMsg = fmt.Sprintf("Dismissed repo %s", pr.RepoName)
				vis := m.visibleItems()
				if m.cursor >= len(vis) && m.cursor > 0 {
					m.cursor = len(vis) - 1
				}
			}
		case "c":
			if pr, ok := m.selectedPR(); ok {
				m.statusMsg = fmt.Sprintf("Commenting on %s#%d...", pr.RepoName, pr.Number)
				return m, postCommentCmd(pr.RepoFullName, pr.Number, "@claude please review this PR")
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m model) View() string {
	if m.errMsg != "" {
		msg := strings.ReplaceAll(m.errMsg, "\n", " ")
		if len(msg) > 200 {
			msg = msg[:200] + "..."
		}
		return fmt.Sprintf("Error: %s\n\nPress r to retry, q to quit.\n", msg)
	}

	if m.showHelp {
		return m.renderLegend()
	}

	vis := m.visibleItems()
	if len(vis) == 0 && !m.loading {
		if m.showAuthor {
			return "No open PRs authored by you. Press a to switch to reviewer mode.\n"
		}
		return "No PRs match current filters. Press s/f to adjust, or a for author mode.\n"
	}

	var b strings.Builder
	// Layout: 1 header + N items + 1 status + 1 help = height
	// Reserve 3 lines for header, status bar, and help bar
	maxLines := m.height - 3
	if maxLines <= 0 {
		maxLines = len(vis)
	}

	// Header
	header := "👤 👥 💬"
	if m.showAuthor {
		header = "📝 👥 💬"
	}
	// Pad to align age label over the age column
	ageLabel := "age"
	if m.sortMode == SortDate {
		ageLabel = "act"
	}
	headerPad := 6 + m.cols.repo + 2 + m.cols.author + 2 // indicators + space + repo + sep + author + sep
	padNeeded := headerPad - lipgloss.Width(header)
	if padNeeded < 1 {
		padNeeded = 1
	}
	headerLine := header + strings.Repeat(" ", padNeeded) + fmt.Sprintf("%4s", ageLabel)
	b.WriteString(helpStyle.Render(headerLine))
	b.WriteString("\n")

	// Scrolling: determine visible window
	start := 0
	if m.cursor >= maxLines {
		start = m.cursor - maxLines + 1
	}
	end := start + maxLines
	if end > len(vis) {
		end = len(vis)
	}
	itemsRendered := end - start

	for i := start; i < end; i++ {
		pr := vis[i]
		selected := i == m.cursor
		var bg *lipgloss.Style
		if selected {
			bg = &selBg
		}

		indicators := formatIndicators(pr, m.showAuthor, bg)
		repoCol := fmt.Sprintf("%-*s", m.cols.repo, fmt.Sprintf("%s#%d", pr.RepoName, pr.Number))
		authorCol := fmt.Sprintf("%-*s", m.cols.author, pr.Author)
		ageTime := pr.CreatedAt
		if m.sortMode == SortDate {
			ageTime = pr.LastActivity
		}
		ageCol := fmt.Sprintf("%4s", formatAge(ageTime))

		// Build plain line for truncation check, then colorized version for display
		plainLine := fmt.Sprintf("%s %s  %s  %s  %s", "     ", repoCol, authorCol, ageCol, pr.Title)
		titleText := pr.Title
		if m.width > 0 && len(plainLine) > m.width {
			// Truncate title to fit
			overhead := len(plainLine) - len(pr.Title)
			maxTitle := m.width - overhead - 1
			if maxTitle > 0 && maxTitle < len(pr.Title) {
				titleText = pr.Title[:maxTitle] + "…"
			} else if maxTitle <= 0 {
				titleText = "…"
			}
		}

		var coloredRepo, coloredAuthor, line string
		if selected {
			if pr.IsDraft {
				coloredRepo = styleDim.Inherit(selBg).Render(repoCol)
				coloredAuthor = styleDim.Inherit(selBg).Render(authorCol)
			} else {
				coloredRepo = nameColor(pr.RepoName).Inherit(selBg).Render(repoCol)
				coloredAuthor = nameColor(pr.Author).Inherit(selBg).Render(authorCol)
			}
			sep := selBg.Render("  ")
			ageRendered := styleDim.Inherit(selBg).Render(ageCol)
			var titleRendered string
			if pr.IsDraft {
				titleRendered = styleDim.Inherit(selBg).Render(titleText)
			} else {
				titleRendered = selBg.Render(titleText)
			}
			line = indicators + selBg.Render(" ") + coloredRepo + sep + coloredAuthor + sep + ageRendered + sep + titleRendered
			// Pad to full width
			if m.width > 0 {
				lineLen := lipgloss.Width(line)
				if lineLen < m.width {
					line += selBg.Render(strings.Repeat(" ", m.width-lineLen))
				}
			}
		} else if pr.IsDraft {
			coloredRepo = styleDim.Render(repoCol)
			coloredAuthor = styleDim.Render(authorCol)
			ageRendered := styleDim.Render(ageCol)
			line = fmt.Sprintf("%s %s  %s  %s  %s", indicators, coloredRepo, coloredAuthor, ageRendered, styleDim.Render(titleText))
		} else {
			coloredRepo = nameColor(pr.RepoName).Render(repoCol)
			coloredAuthor = nameColor(pr.Author).Render(authorCol)
			ageRendered := styleDim.Render(ageCol)
			line = fmt.Sprintf("%s %s  %s  %s  %s", indicators, coloredRepo, coloredAuthor, ageRendered, titleText)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	// Pad to fill terminal height: pin status+help to bottom
	if m.height > 0 {
		pad := maxLines - itemsRendered
		for i := 0; i < pad; i++ {
			b.WriteString("\n")
		}
	}

	// Help bar
	authorLabel := "author:off"
	if m.showAuthor {
		authorLabel = "author:on"
	}
	sortLabel := "sort:priority"
	if m.sortMode == SortDate {
		sortLabel = "sort:date"
	}

	var help string
	if m.showAuthor {
		help = helpStyle.Render(fmt.Sprintf(
			"j/k: navigate  enter: open  d/D: dismiss PR/repo  c: @claude  o: %s  a: %s  r: refresh  ?: legend  q: quit",
			sortLabel, authorLabel,
		))
	} else {
		authoredLabel := "authored:off"
		if m.showAuthored {
			authoredLabel = "authored:on"
		}
		assignedLabel := "assigned:off"
		if m.showAssigned {
			assignedLabel = "assigned:on"
		}
		help = helpStyle.Render(fmt.Sprintf(
			"j/k: navigate  enter: open  d/D: dismiss PR/repo  c: @claude  s: %s  f: %s  o: %s  a: %s  r: refresh  ?: legend  q: quit",
			authoredLabel, assignedLabel, sortLabel, authorLabel,
		))
	}
	if m.loading {
		spin := styleCyan.Render(spinnerFrames[m.spinnerFrame])
		var loadText string
		if m.loadingCount > 0 {
			loadText = fmt.Sprintf("Fetching PRs... %d found", m.loadingCount)
		} else {
			loadText = "Fetching PRs..."
		}
		b.WriteString(fmt.Sprintf("%s %s", spin, helpStyle.Render(loadText)))
		b.WriteString("\n")
	} else if m.statusMsg != "" {
		b.WriteString(styleCyan.Render(m.statusMsg))
		b.WriteString("\n")
	} else {
		b.WriteString("\n")
	}
	b.WriteString(help)

	return b.String()
}

func (m model) renderLegend() string {
	var b strings.Builder
	b.WriteString("Column 1 — Your Review:\n")
	b.WriteString(fmt.Sprintf("  %s  You approved\n", styleGreen.Render("✓")))
	b.WriteString(fmt.Sprintf("  %s  You requested changes\n", styleRed.Render("✗")))
	b.WriteString(fmt.Sprintf("  %s  You left review comments\n", styleYellow.Render("◆")))
	b.WriteString(fmt.Sprintf("  %s  No review yet\n", styleDim.Render("·")))
	b.WriteString("  Color: bright = current, gray = stale\n")
	b.WriteString("\n")
	b.WriteString("Column 2 — Others' Reviews:\n")
	b.WriteString(fmt.Sprintf("  %s  All approved\n", styleGreen.Render("✓")))
	b.WriteString(fmt.Sprintf("  %s  Changes requested\n", styleRed.Render("✗")))
	b.WriteString(fmt.Sprintf("  %s  Mixed reviews\n", styleYellow.Render("±")))
	b.WriteString(fmt.Sprintf("  %s  No reviews yet\n", styleDim.Render("·")))
	b.WriteString("\n")
	b.WriteString("Column 3 — Comments:\n")
	b.WriteString(fmt.Sprintf("  %s  You commented\n", styleCyan.Render("●")))
	b.WriteString(fmt.Sprintf("  %s  Others commented\n", styleWhite.Render("○")))
	b.WriteString(fmt.Sprintf("  %s  No comments\n", styleDim.Render("·")))
	b.WriteString("  Color: bright = fresh, gray = stale\n")
	b.WriteString("\n")
	b.WriteString("Keys:\n")
	b.WriteString("  j/k     Navigate up/down\n")
	b.WriteString("  enter   Open PR in browser\n")
	b.WriteString("  d       Dismiss current PR (hide it)\n")
	b.WriteString("  D       Dismiss entire repo\n")
	b.WriteString("  s       Toggle showing PRs you authored\n")
	b.WriteString("  f       Toggle showing only PRs assigned to you\n")
	b.WriteString("  a       Toggle author mode (your PRs + their review status)\n")
	b.WriteString("  o       Toggle sort: priority vs date\n")
	b.WriteString("  c       Post @claude review comment on current PR\n")
	b.WriteString("  r       Refresh data from GitHub\n")
	b.WriteString("  q       Quit\n")
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("Press any key to close"))
	return b.String()
}

func (m model) visibleItems() []ClassifiedPR {
	var vis []ClassifiedPR
	for _, pr := range m.items {
		if m.dismissed[pr.URL] || m.dismissedRepos[pr.RepoName] {
			continue
		}
		vis = append(vis, pr)
	}
	return vis
}

func (m model) selectedPR() (ClassifiedPR, bool) {
	vis := m.visibleItems()
	if m.cursor < 0 || m.cursor >= len(vis) {
		return ClassifiedPR{}, false
	}
	return vis[m.cursor], true
}

func withBg(s lipgloss.Style, bg *lipgloss.Style) lipgloss.Style {
	if bg != nil {
		return s.Inherit(*bg)
	}
	return s
}

func formatIndicators(pr ClassifiedPR, authorMode bool, bg *lipgloss.Style) string {
	var col1, col2, col3 string

	// Draft PRs (non-author mode): dim all indicators
	if pr.IsDraft && !authorMode {
		col1 = withBg(styleDim, bg).Render("·")
		col2 = withBg(styleDim, bg).Render("·")
		col3 = withBg(styleDim, bg).Render("·")
		sep := " "
		if bg != nil {
			sep = bg.Render(" ")
		}
		return col1 + sep + col2 + sep + col3
	}

	if authorMode {
		if pr.IsDraft {
			col1 = withBg(styleDim, bg).Render("○")
		} else {
			col1 = withBg(styleWhite, bg).Render("●")
		}
	} else {
		switch pr.MyReview {
		case MyNone:
			col1 = withBg(styleDim, bg).Render("·")
		case MyApproved:
			col1 = withBg(styleGreen, bg).Render("✓")
		case MyChanges:
			col1 = withBg(styleRed, bg).Render("✗")
		case MyCommented:
			col1 = withBg(styleYellow, bg).Render("◆")
		case MyApprovedStale:
			col1 = withBg(styleDim, bg).Render("✓")
		case MyChangesStale:
			col1 = withBg(styleDim, bg).Render("✗")
		case MyCommentedStale:
			col1 = withBg(styleDim, bg).Render("◆")
		default:
			col1 = withBg(styleDim, bg).Render("·")
		}
	}

	switch pr.OthReview {
	case OthNone:
		col2 = withBg(styleDim, bg).Render("·")
	case OthApproved:
		col2 = withBg(styleGreen, bg).Render("✓")
	case OthChanges:
		col2 = withBg(styleRed, bg).Render("✗")
	case OthMixed:
		col2 = withBg(styleYellow, bg).Render("±")
	default:
		col2 = withBg(styleDim, bg).Render("·")
	}

	switch pr.Activity {
	case ActNone:
		col3 = withBg(styleDim, bg).Render("·")
	case ActOthers:
		col3 = withBg(styleWhite, bg).Render("○")
	case ActMine:
		col3 = withBg(styleCyan, bg).Render("●")
	case ActOthersStale:
		col3 = withBg(styleDim, bg).Render("○")
	case ActMineStale:
		col3 = withBg(styleDim, bg).Render("●")
	default:
		col3 = withBg(styleDim, bg).Render("·")
	}

	sep := " "
	if bg != nil {
		sep = bg.Render(" ")
	}
	return col1 + sep + col2 + sep + col3
}
