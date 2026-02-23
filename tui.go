package main

import (
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	tagNew = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2"))
	tagCmt = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("3"))
	tagDis = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("1"))
	tagStl = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	selLine = lipgloss.NewStyle().Bold(true).Reverse(true)
	helpStyle = lipgloss.NewStyle().Faint(true)
)

type model struct {
	items     []ClassifiedPR
	cursor    int
	dismissed map[int]bool
	cols      colWidths
	width     int
	height    int
}

func newModel(items []ClassifiedPR) model {
	return model{
		items:     items,
		dismissed: make(map[int]bool),
		cols:      computeColumns(items),
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
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
				c := exec.Command("gh", "pr", "view", "--web",
					"-R", pr.RepoFullName,
					fmt.Sprintf("%d", pr.Number))
				return m, tea.ExecProcess(c, func(err error) tea.Msg { return nil })
			}
		case "d":
			if idx, ok := m.selectedIndex(); ok {
				m.dismissed[idx] = true
				vis := m.visibleItems()
				if m.cursor >= len(vis) && m.cursor > 0 {
					m.cursor--
				}
				if len(vis) == 0 {
					return m, tea.Quit
				}
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m model) View() string {
	vis := m.visibleItems()
	if len(vis) == 0 {
		return "No PRs pending your review.\n"
	}

	var b strings.Builder
	maxLines := m.height - 2 // reserve for help bar
	if maxLines <= 0 {
		maxLines = len(vis)
	}

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
		tag := formatTag(pr.State)
		repoCol := fmt.Sprintf("%s#%d", pr.RepoName, pr.Number)
		line := fmt.Sprintf("%s %-*s  %-*s  %s", tag, m.cols.repo, repoCol, m.cols.author, pr.Author, pr.Title)

		if m.width > 0 && len(line) > m.width {
			line = line[:m.width-1] + "…"
		}

		if i == m.cursor {
			line = selLine.Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	help := helpStyle.Render("j/k: navigate  enter: open  d: dismiss  q: quit")
	b.WriteString("\n")
	b.WriteString(help)

	return b.String()
}

func (m model) visibleItems() []ClassifiedPR {
	var vis []ClassifiedPR
	for i, pr := range m.items {
		if !m.dismissed[i] {
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

func (m model) selectedIndex() (int, bool) {
	vis := 0
	for i := range m.items {
		if m.dismissed[i] {
			continue
		}
		if vis == m.cursor {
			return i, true
		}
		vis++
	}
	return 0, false
}

func formatTag(s ReviewState) string {
	label := fmt.Sprintf("[%s]", s)
	switch s {
	case StateNew:
		return tagNew.Render(label)
	case StateCommented:
		return tagCmt.Render(label)
	case StateDismissed:
		return tagDis.Render(label)
	case StateStale:
		return tagStl.Render(label)
	default:
		return label
	}
}
