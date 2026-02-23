package main

import (
	"fmt"
	"hash/fnv"
	"os/exec"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	styleGreen  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	styleRed    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	styleYellow = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	styleCyan   = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	styleDim    = lipgloss.NewStyle().Faint(true)
	styleWhite  = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))

	selLine   = lipgloss.NewStyle().Bold(true).Reverse(true)
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
	items     []ClassifiedPR
	cursor    int
	dismissed map[string]bool
	cols      colWidths
	width     int
	height    int

	rawPRs  []PRNode
	me      string
	myTeams map[string]bool

	showSelf   bool
	showMine   bool
	showAuthor bool
	sortMode   SortMode

	loading   bool
	org       string
	limit     int
	errMsg    string
	showHelp  bool
	statusMsg string
}

type modelConfig struct {
	rawPRs     []PRNode
	me         string
	myTeams    map[string]bool
	showSelf   bool
	showMine   bool
	showAuthor bool
	sortMode   SortMode
	loading    bool
	org        string
	limit      int
}

type dataLoadedMsg struct {
	prs     []PRNode
	me      string
	myTeams map[string]bool
}

type fetchErrMsg struct {
	err error
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

func fetchDataCmd(org string, limit int) tea.Cmd {
	return func() tea.Msg {
		me, err := fetchCurrentUser()
		if err != nil {
			return fetchErrMsg{err}
		}
		prs, err := fetchOpenPRs(org, limit)
		if err != nil {
			return fetchErrMsg{err}
		}
		myTeams, err := fetchUserTeams(org)
		if err != nil {
			myTeams = make(map[string]bool)
		}
		return dataLoadedMsg{prs: prs, me: me, myTeams: myTeams}
	}
}

func newModel(cfg modelConfig) model {
	m := model{
		dismissed:  make(map[string]bool),
		rawPRs:     cfg.rawPRs,
		me:         cfg.me,
		myTeams:    cfg.myTeams,
		showSelf:   cfg.showSelf,
		showMine:   cfg.showMine,
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
		if m.showMine {
			me, teams := m.me, m.myTeams
			filter = func(pr PRNode) bool {
				return isRequestedReviewer(pr, me, teams)
			}
		}
		m.items = classifyAll(m.rawPRs, m.me, m.showSelf, filter, m.sortMode)
	}
	m.cols = computeColumns(m.items)
}

func (m model) Init() tea.Cmd {
	if m.loading {
		return fetchDataCmd(m.org, m.limit)
	}
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case dataLoadedMsg:
		m.loading = false
		m.errMsg = ""
		m.rawPRs = msg.prs
		m.me = msg.me
		m.myTeams = msg.myTeams
		m.reclassify()
		m.cursor = 0
	case fetchErrMsg:
		m.loading = false
		m.errMsg = msg.err.Error()
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
				m.showSelf = !m.showSelf
				m.reclassify()
				m.cursor = 0
			}
		case "m":
			if !m.showAuthor {
				m.showMine = !m.showMine
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
				m.loading = true
				m.errMsg = ""
				return m, fetchDataCmd(m.org, m.limit)
			}
		case "d":
			if pr, ok := m.selectedPR(); ok {
				m.dismissed[pr.URL] = true
				vis := m.visibleItems()
				if m.cursor >= len(vis) && m.cursor > 0 {
					m.cursor--
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

const hideCursor = "\033[?25l"

func (m model) View() string {
	if m.loading {
		return hideCursor + "Fetching PRs...\n"
	}
	if m.errMsg != "" {
		msg := strings.ReplaceAll(m.errMsg, "\n", " ")
		if len(msg) > 200 {
			msg = msg[:200] + "..."
		}
		return hideCursor + fmt.Sprintf("Error: %s\n\nPress r to retry, q to quit.\n", msg)
	}

	if m.showHelp {
		return hideCursor + m.renderLegend()
	}

	vis := m.visibleItems()
	if len(vis) == 0 {
		if m.showAuthor {
			return hideCursor + "No open PRs authored by you. Press a to switch to reviewer mode.\n"
		}
		return hideCursor + "No PRs match current filters. Press s/m to adjust, or a for author mode.\n"
	}

	var b strings.Builder
	b.WriteString(hideCursor)
	maxLines := m.height - 3 // reserve for header + help bar
	if maxLines <= 0 {
		maxLines = len(vis)
	}

	// Header
	header := "👤 👥 💬"
	if m.showAuthor {
		header = "📝 👥 💬"
	}
	b.WriteString(helpStyle.Render(header))
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

	for i := start; i < end; i++ {
		pr := vis[i]
		indicators := formatIndicators(pr, m.showAuthor)
		repoCol := fmt.Sprintf("%-*s", m.cols.repo, fmt.Sprintf("%s#%d", pr.RepoName, pr.Number))
		authorCol := fmt.Sprintf("%-*s", m.cols.author, pr.Author)

		// Build plain line for truncation check, then colorized version for display
		plainLine := fmt.Sprintf("%s %s  %s  %s", "     ", repoCol, authorCol, pr.Title)
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

		coloredRepo := nameColor(pr.RepoName).Render(repoCol)
		coloredAuthor := nameColor(pr.Author).Render(authorCol)
		line := fmt.Sprintf("%s %s  %s  %s", indicators, coloredRepo, coloredAuthor, titleText)

		if i == m.cursor {
			line = selLine.Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
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
			"j/k: navigate  enter: open  d: dismiss  c: @claude  o: %s  a: %s  r: refresh  ?: legend  q: quit",
			sortLabel, authorLabel,
		))
	} else {
		selfLabel := "self:off"
		if m.showSelf {
			selfLabel = "self:on"
		}
		mineLabel := "mine:off"
		if m.showMine {
			mineLabel = "mine:on"
		}
		help = helpStyle.Render(fmt.Sprintf(
			"j/k: navigate  enter: open  d: dismiss  c: @claude  s: %s  m: %s  o: %s  a: %s  r: refresh  ?: legend  q: quit",
			selfLabel, mineLabel, sortLabel, authorLabel,
		))
	}
	if m.statusMsg != "" {
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
	b.WriteString(fmt.Sprintf("  %s  No review yet\n", styleDim.Render("·")))
	b.WriteString(fmt.Sprintf("  %s  You approved\n", styleGreen.Render("✓")))
	b.WriteString(fmt.Sprintf("  %s  You requested changes\n", styleRed.Render("✗")))
	b.WriteString(fmt.Sprintf("  %s  Your review is stale (new commits since)\n", styleDim.Render("~")))
	b.WriteString("\n")
	b.WriteString("Column 2 — Others' Reviews:\n")
	b.WriteString(fmt.Sprintf("  %s  No reviews yet\n", styleDim.Render("·")))
	b.WriteString(fmt.Sprintf("  %s  All approved\n", styleGreen.Render("✓")))
	b.WriteString(fmt.Sprintf("  %s  Changes requested\n", styleRed.Render("✗")))
	b.WriteString(fmt.Sprintf("  %s  Mixed reviews\n", styleYellow.Render("±")))
	b.WriteString("\n")
	b.WriteString("Column 3 — Comments:\n")
	b.WriteString(fmt.Sprintf("  %s  No comments\n", styleDim.Render("·")))
	b.WriteString(fmt.Sprintf("  %s  Others commented\n", styleWhite.Render("○")))
	b.WriteString(fmt.Sprintf("  %s  You commented\n", styleCyan.Render("●")))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("Press any key to close"))
	return b.String()
}

func (m model) visibleItems() []ClassifiedPR {
	var vis []ClassifiedPR
	for _, pr := range m.items {
		if !m.dismissed[pr.URL] {
			vis = append(vis, pr)
		}
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

func formatIndicators(pr ClassifiedPR, authorMode bool) string {
	var col1, col2, col3 string

	if authorMode {
		if pr.IsDraft {
			col1 = styleDim.Render("○")
		} else {
			col1 = styleWhite.Render("●")
		}
	} else {
		switch pr.MyReview {
		case MyNone:
			col1 = styleDim.Render("·")
		case MyApproved:
			col1 = styleGreen.Render("✓")
		case MyChanges:
			col1 = styleRed.Render("✗")
		case MyStale:
			col1 = styleDim.Render("~")
		default:
			col1 = styleDim.Render("·")
		}
	}

	switch pr.OthReview {
	case OthNone:
		col2 = styleDim.Render("·")
	case OthApproved:
		col2 = styleGreen.Render("✓")
	case OthChanges:
		col2 = styleRed.Render("✗")
	case OthMixed:
		col2 = styleYellow.Render("±")
	default:
		col2 = styleDim.Render("·")
	}

	switch pr.Activity {
	case ActNone:
		col3 = styleDim.Render("·")
	case ActOthers:
		col3 = styleWhite.Render("○")
	case ActMine:
		col3 = styleCyan.Render("●")
	default:
		col3 = styleDim.Render("·")
	}

	return col1 + " " + col2 + " " + col3
}
