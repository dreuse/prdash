package ui

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dreuse/prdash/internal/github"
	"github.com/dreuse/prdash/internal/model"
	"github.com/dreuse/prdash/internal/readiness"
)

const (
	DefaultRefreshInterval = 15 * time.Second
	rateLimitBackoff       = 60 * time.Second
	fetchTimeout           = 45 * time.Second
	spinnerInterval        = 120 * time.Millisecond
)

type view int

const (
	viewBoard view = iota
	viewDetail
	viewActions
)

type Model struct {
	fetcher  github.Fetcher
	policy   readiness.Policy
	interval time.Duration
	repos    []string
	theme    Theme

	width, height int
	view          view
	col, row      int
	runRow        int

	groups map[model.Column][]model.PullRequest
	runs   []model.WorkflowRun

	loading     bool
	loadedOnce  bool
	err         error
	rateLimited bool
	lastUpdate  time.Time
	notice      string

	tickGen     int
	spinnerStep int
}

func New(f github.Fetcher, policy readiness.Policy, interval time.Duration, repos []string) Model {
	if interval <= 0 {
		interval = DefaultRefreshInterval
	}
	return Model{
		fetcher:  f,
		policy:   policy,
		interval: interval,
		repos:    repos,
		theme:    NewTheme(),
		loading:  true,
		groups:   policy.Group(nil),
	}
}

type dataMsg struct {
	snapshot github.Snapshot
	err      error
}

type tickMsg struct{ gen int }

type spinnerMsg struct{}

type noticeMsg struct{ text string }

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.fetchCmd(), m.scheduleTick(), spinnerTick())
}

func (m Model) fetchCmd() tea.Cmd {
	f := m.fetcher
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		snap, err := f.Fetch(ctx)
		return dataMsg{snapshot: snap, err: err}
	}
}

func (m Model) scheduleTick() tea.Cmd {
	gen := m.tickGen
	d := m.interval
	if m.rateLimited && d < rateLimitBackoff {
		d = rateLimitBackoff
	}
	return tea.Tick(d, func(time.Time) tea.Msg { return tickMsg{gen: gen} })
}

func spinnerTick() tea.Cmd {
	return tea.Tick(spinnerInterval, func(time.Time) tea.Msg { return spinnerMsg{} })
}

func (m Model) now() time.Time { return time.Now() }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case spinnerMsg:
		m.spinnerStep++
		return m, spinnerTick()

	case tickMsg:
		if msg.gen != m.tickGen {
			return m, nil
		}
		m.loading = true
		return m, tea.Batch(m.fetchCmd(), m.scheduleTick())

	case noticeMsg:
		m.notice = msg.text
		return m, nil

	case dataMsg:
		m.loading = false
		m.err = msg.err
		m.rateLimited = isRateLimit(msg.err)
		if msg.err == nil || len(msg.snapshot.PullRequests) > 0 || len(msg.snapshot.Runs) > 0 {
			m.groups = m.policy.Group(msg.snapshot.PullRequests)
			m.runs = msg.snapshot.Runs
			m.loadedOnce = true
			m.lastUpdate = time.Now()
			m.clampSelection()
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "r":
		m.loading = true
		m.notice = ""
		m.tickGen++
		return m, tea.Batch(m.fetchCmd(), m.scheduleTick())

	case "tab", "a":
		if m.view == viewActions {
			m.view = viewBoard
		} else {
			m.view = viewActions
		}
		return m, nil

	case "enter":
		if m.view == viewBoard {
			if _, ok := m.selectedPR(); ok {
				m.view = viewDetail
			}
		}
		return m, nil

	case "esc":
		if m.view != viewBoard {
			m.view = viewBoard
		}
		return m, nil

	case "o":
		return m, m.openSelected()

	case "?":
		m.notice = ""
		return m, nil
	}

	switch m.view {
	case viewActions:
		return m.moveActions(msg.String()), nil
	case viewBoard:
		return m.moveBoard(msg.String()), nil
	}
	return m, nil
}

func (m Model) moveBoard(key string) Model {
	switch key {
	case "left", "h":
		m.col = wrap(m.col-1, len(model.Columns))
		m.row = 0
	case "right", "l":
		m.col = wrap(m.col+1, len(model.Columns))
		m.row = 0
	case "up", "k":
		if m.row > 0 {
			m.row--
		}
	case "down", "j":
		if n := len(m.groups[model.Columns[m.col]]); m.row < n-1 {
			m.row++
		}
	case "g", "home":
		m.row = 0
	case "G", "end":
		m.row = maxInt(0, len(m.groups[model.Columns[m.col]])-1)
	}
	m.clampSelection()
	return m
}

func (m Model) moveActions(key string) Model {
	n := len(m.runs)
	switch key {
	case "up", "k":
		if m.runRow > 0 {
			m.runRow--
		}
	case "down", "j":
		if m.runRow < n-1 {
			m.runRow++
		}
	case "g", "home":
		m.runRow = 0
	case "G", "end":
		m.runRow = maxInt(0, n-1)
	}
	return m
}

func (m *Model) clampSelection() {
	if m.col < 0 || m.col >= len(model.Columns) {
		m.col = 0
	}
	n := len(m.groups[model.Columns[m.col]])
	if m.row >= n {
		m.row = maxInt(0, n-1)
	}
	if m.row < 0 {
		m.row = 0
	}
	if m.runRow >= len(m.runs) {
		m.runRow = maxInt(0, len(m.runs)-1)
	}
}

func (m Model) openSelected() tea.Cmd {
	var url string
	switch m.view {
	case viewActions:
		runs := m.orderedRuns()
		if m.runRow >= 0 && m.runRow < len(runs) {
			url = runs[m.runRow].URL
		}
	default:
		if pr, ok := m.selectedPR(); ok {
			url = pr.URL
		}
	}
	if url == "" {
		return func() tea.Msg { return noticeMsg{text: "nothing selected to open"} }
	}
	return func() tea.Msg {
		if err := openBrowser(url); err != nil {
			return noticeMsg{text: "could not open browser: " + err.Error()}
		}
		return noticeMsg{text: "opened " + url}
	}
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

func (m Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}

	header := m.renderHeader()
	footer := m.renderFooter()
	bodyHeight := maxInt(1, m.height-lipgloss.Height(header)-lipgloss.Height(footer))

	var body string
	switch {
	case !m.loadedOnce && m.err != nil:
		body = m.renderFatal()
	case !m.loadedOnce:
		body = m.renderLoading(bodyHeight)
	case m.view == viewDetail:
		body = m.renderDetail(bodyHeight)
	case m.view == viewActions:
		body = m.renderActions(bodyHeight)
	default:
		body = m.renderBoard(bodyHeight)
	}

	return m.fitScreen(header, body, footer)
}

func (m Model) fitScreen(header, body, footer string) string {
	clamp := lipgloss.NewStyle().MaxWidth(m.width)
	header, footer = clamp.Render(header), clamp.Render(footer)

	free := m.height - lipgloss.Height(header) - lipgloss.Height(footer)
	body = lipgloss.NewStyle().MaxWidth(m.width).MaxHeight(maxInt(1, free)).Render(body)

	lines := strings.Split(body, "\n")
	for len(lines) < free {
		lines = append(lines, "")
	}

	all := append(strings.Split(header, "\n"), lines...)
	all = append(all, strings.Split(footer, "\n")...)
	if len(all) > m.height {
		all = all[:m.height]
	}
	return strings.Join(all, "\n")
}

func (m Model) renderHeader() string {
	title := m.theme.Title.Render("prdash")
	repos := m.theme.Dim.Render(strings.Join(m.repos, "  "))

	var right string
	switch {
	case m.loading:
		right = m.theme.Status.Render(m.spinnerFrame() + " refreshing")
	case m.rateLimited:
		right = m.theme.StatusWarn.Render(fmt.Sprintf("%s rate limited, retrying in %s",
			m.theme.Icons.Conflict, rateLimitBackoff))
	case m.err != nil:
		right = m.theme.StatusError.Render(m.theme.Icons.Failed + " " + truncate(m.err.Error(), 60))
	case !m.lastUpdate.IsZero():
		right = m.theme.Status.Render(fmt.Sprintf("updated %s ago", model.ShortAge(time.Since(m.lastUpdate))))
	}

	left := title + "  " + repos
	gap := maxInt(1, m.width-lipgloss.Width(left)-lipgloss.Width(right))
	return left + strings.Repeat(" ", gap) + right
}

func (m Model) renderFooter() string {
	var keys string
	switch m.view {
	case viewDetail:
		keys = m.joinKeys("esc back", "o open", "r refresh", "q quit")
	case viewActions:
		keys = m.joinKeys("j/k move", "tab board", "o open", "r refresh", "q quit")
	default:
		keys = m.joinKeys("h/j/k/l move", "enter details", "tab actions", "o open", "r refresh", "q quit")
	}
	if lipgloss.Width(keys) > m.width {
		keys = m.joinKeys("move", "enter", "tab", "o", "r", "q")
	}
	line := m.theme.Help.Render(keys)
	if m.notice != "" {
		line = m.theme.Status.Render(truncate(m.notice, maxInt(10, m.width))) + "\n" + line
	} else if m.err != nil && m.loadedOnce {
		line = m.theme.StatusError.Render(truncate("error: "+m.err.Error(), maxInt(10, m.width))) + "\n" + line
	}
	return line
}

func (m Model) renderLoading(height int) string {
	msg := fmt.Sprintf("%s loading pull requests from %s", m.spinnerFrame(), strings.Join(m.repos, ", "))
	return lipgloss.Place(m.width, height, lipgloss.Center, lipgloss.Center, m.theme.Status.Render(msg))
}

func (m Model) renderFatal() string {
	var b strings.Builder
	b.WriteString(m.theme.StatusError.Render(m.theme.Icons.Failed + " could not load data"))
	b.WriteString("\n\n")
	b.WriteString(m.err.Error())
	b.WriteString("\n\n")

	var ghErr *github.Error
	switch {
	case errors.As(m.err, &ghErr) && ghErr.Auth:
		b.WriteString(m.theme.Status.Render("run `gh auth login` and try again"))
	case errors.As(m.err, &ghErr) && ghErr.RateLimit:
		b.WriteString(m.theme.Status.Render("github rate limit hit; the dashboard will retry automatically"))
	default:
		b.WriteString(m.theme.Status.Render("press r to retry, q to quit"))
	}
	return b.String()
}

func (m Model) spinnerFrame() string {
	frames := m.theme.Icons.Spinner
	return frames[m.spinnerStep%len(frames)]
}

func isRateLimit(err error) bool {
	var ghErr *github.Error
	return errors.As(err, &ghErr) && ghErr.RateLimit
}

func wrap(i, n int) int {
	if n == 0 {
		return 0
	}
	return ((i % n) + n) % n
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
