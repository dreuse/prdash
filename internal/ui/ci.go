package ui

import (
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dreuse/prdash/internal/model"
)

const (
	sparkWidth  = 20
	trendWidth  = 12
	colRuns     = 6
	colPass     = 7
	colMedian   = 9
	colLast     = 14
	colCIMarker = 2
	colCIPR     = 7
	colCIRepo   = 16
	colCIName   = 22
	colCIEvent  = 9
	colCIStatus = 12
	colCIDur    = 8
	colCIAge    = 6
	minCIBranch = 12
	maxCIBranch = 44

	minWorkflowW = 12
	maxWorkflowW = 46
	maxTrendRows = 6

	ciChromeRows = 4
)

func (m Model) recentWindow() time.Duration {
	hours := m.settings.CIRecentHours
	if hours < 1 {
		hours = 24
	}
	return time.Duration(hours) * time.Hour
}

func (m Model) visibleRuns() []model.WorkflowRun {
	window := m.recentWindow()
	out := make([]model.WorkflowRun, 0, len(m.runs))
	for _, r := range m.runs {
		if !m.inScope(r.Repo) {
			continue
		}
		if m.ciFailuresOnly && !r.Failed() {
			continue
		}
		if !r.InProgress() && nowSince(r.UpdatedAt) > window {
			continue
		}
		out = append(out, r)
	}
	return out
}

func (m Model) sortedVisibleRuns() []model.WorkflowRun {
	runs := m.visibleRuns()
	sort.SliceStable(runs, func(i, j int) bool {
		a, b := runs[i], runs[j]
		if a.InProgress() != b.InProgress() {
			return a.InProgress()
		}
		return a.UpdatedAt.After(b.UpdatedAt)
	})
	return runs
}

func (m Model) ciRows() []model.WorkflowRun { return m.ciCache }

func (m Model) trendRows() []model.WorkflowStats {
	return model.AggregateWorkflows(m.scopedRuns())
}

func (m Model) scopedRuns() []model.WorkflowRun {
	out := make([]model.WorkflowRun, 0, len(m.runs))
	for _, r := range m.runs {
		if m.inScope(r.Repo) {
			out = append(out, r)
		}
	}
	return out
}

func (m Model) selectedRun() (model.WorkflowRun, bool) {
	rows := m.ciRows()
	if m.ciRow < 0 || m.ciRow >= len(rows) {
		return model.WorkflowRun{}, false
	}
	return rows[m.ciRow], true
}

type ciColumns struct {
	pr     bool
	repo   bool
	event  bool
	dur    bool
	branch int
}

func (m Model) ciColumns() ciColumns {
	c := ciColumns{pr: true, repo: m.scope == "" && m.multi, event: true, dur: true}
	drop := []*bool{&c.event, &c.repo, &c.dur, &c.pr}

	for i := 0; ; i++ {
		width := m.width - colCIMarker - colCIName - colCIStatus - colCIAge
		if c.pr {
			width -= colCIPR
		}
		if c.repo {
			width -= colCIRepo
		}
		if c.event {
			width -= colCIEvent
		}
		if c.dur {
			width -= colCIDur
		}
		if width >= minCIBranch || i >= len(drop) {
			c.branch = clamp(width, minCIBranch, maxCIBranch)
			return c
		}
		*drop[i] = false
	}
}

func eventLabel(e string) string {
	switch e {
	case "pull_request", "pull_request_target":
		return "pr"
	case "workflow_dispatch":
		return "manual"
	case "schedule":
		return "cron"
	case "":
		return "-"
	}
	return e
}

func runStatus(r model.WorkflowRun) string {
	switch {
	case r.Failed():
		return r.Conclusion
	case r.InProgress():
		switch r.Status {
		case "queued", "waiting", "pending", "requested":
			return r.Status
		}
		return "running"
	case r.Succeeded():
		return "success"
	}
	if r.Conclusion != "" {
		return r.Conclusion
	}
	return r.Status
}

func (m Model) renderCI(height int) string {
	t := m.theme
	var b strings.Builder

	b.WriteString(m.renderHealthStrip())
	b.WriteString("\n")
	b.WriteString(t.HorizontalRule(m.width))
	b.WriteString("\n")

	rows := m.ciRows()
	if len(rows) == 0 {
		return lipgloss.NewStyle().MaxHeight(height).Width(m.width).Render(
			b.String() + m.renderNoRuns(maxInt(1, height-2)))
	}

	trends := ""
	trendHeight := 0
	if !m.logs.open {
		trends = m.renderTrends()
		if trends != "" {
			trendHeight = lipgloss.Height(trends) + 1
		}
	}

	visible := maxInt(1, height-ciChromeRows-trendHeight)
	offset, shown := laneWindow(len(rows), m.ciRow, visible)

	cols := m.ciColumns()
	b.WriteString(m.renderTableHeader(cols))
	b.WriteString("\n")
	b.WriteString(t.HorizontalRule(m.width))
	b.WriteString("\n")

	for i := offset; i < offset+shown; i++ {
		b.WriteString(m.renderRunRow(rows[i], cols, i == m.ciRow))
		b.WriteString("\n")
	}
	if hidden := len(rows) - (offset + shown); hidden > 0 {
		b.WriteString(t.Faint.Render("  +" + itoa(hidden) + " more " + plural(hidden, "run", "runs")))
		b.WriteString("\n")
	}

	if trends != "" {
		b.WriteString("\n")
		b.WriteString(trends)
	}

	return lipgloss.NewStyle().MaxHeight(height).Width(m.width).Render(b.String())
}

func (m Model) renderNoRuns(height int) string {
	t := m.theme
	window := model.ShortAge(m.recentWindow())

	msg := t.Dim.Render("No runs in the last "+window) + "\n\n" +
		t.Faint.Render("u to refresh")
	if m.ciFailuresOnly {
		msg = t.Dim.Render("No failing runs in the last "+window) + "\n\n" +
			t.Faint.Render("f to show all runs")
	}
	return lipgloss.Place(m.width, height, lipgloss.Center, lipgloss.Center, msg)
}

func (m Model) renderTableHeader(c ciColumns) string {
	t := m.theme
	head := strings.Repeat(" ", colCIMarker)
	if c.pr {
		head += pad("PR", colCIPR)
	}
	if c.repo {
		head += pad("REPO", colCIRepo)
	}
	head += pad("WORKFLOW", colCIName) + pad("BRANCH", c.branch)
	if c.event {
		head += pad("EVENT", colCIEvent)
	}
	head += pad("STATUS", colCIStatus)
	if c.dur {
		head += pad("DUR", colCIDur)
	}
	head += "AGE"
	return t.Faint.Render(head)
}

func (m Model) renderRunRow(run model.WorkflowRun, c ciColumns, selected bool) string {
	t := m.theme

	marker := "  "
	rowStyle := lipgloss.NewStyle()
	if selected {
		marker = t.Accent.Render(t.Glyphs.Selected) + " "
		rowStyle = t.Selected
	}

	line := marker
	if c.pr {
		if pr, ok := m.prByBranch(run.Repo, run.Branch); ok {
			line += t.Accent.Render(pad("#"+itoa(pr.Number), colCIPR))
		} else {
			line += t.Faint.Render(pad(t.Glyphs.Dot, colCIPR))
		}
	}
	if c.repo {
		line += t.Faint.Render(pad(truncate(shortRepo(run.Repo), colCIRepo-1), colCIRepo))
	}

	line += t.Body.Render(pad(truncate(run.Name, colCIName-1), colCIName)) +
		t.Dim.Render(pad(truncate(run.Branch, c.branch-1), c.branch))

	if c.event {
		line += t.Faint.Render(pad(truncate(eventLabel(run.Event), colCIEvent-1), colCIEvent))
	}

	glyph, style := t.Glyphs.Pass, t.OK
	switch {
	case run.Failed():
		glyph, style = t.Glyphs.Fail, t.Danger
	case run.InProgress():
		glyph, style = t.Glyphs.Running, t.Warn
	}
	line += style.Render(pad(glyph+" "+truncate(runStatus(run), colCIStatus-3), colCIStatus))

	if c.dur {
		line += t.Faint.Render(pad(model.FormatDuration(run.Duration()), colCIDur))
	}
	line += t.Faint.Render(model.ShortAge(nowSince(run.UpdatedAt)))

	return rowStyle.Render(fillLine(line, m.width))
}

func (m Model) renderTrends() string {
	rows := m.trendRows()
	if len(rows) == 0 {
		return ""
	}
	t := m.theme
	nameWidth := clamp(m.width-colRuns-colPass-colMedian-trendWidth-colLast-colCIRepo-4,
		minWorkflowW, maxWorkflowW)

	var b strings.Builder
	b.WriteString(t.Faint.Render("TRENDS"))
	b.WriteString("\n")
	b.WriteString(t.Faint.Render(
		pad("WORKFLOW", nameWidth) +
			pad("REPO", colCIRepo) +
			pad("RUNS", colRuns) +
			pad("PASS", colPass) +
			pad("MEDIAN", colMedian) +
			pad("TREND", trendWidth) +
			"LAST"))

	hidden := 0
	if len(rows) > maxTrendRows {
		hidden = len(rows) - maxTrendRows
		rows = rows[:maxTrendRows]
	}

	for _, w := range rows {
		passStyle := t.OK
		switch {
		case w.PassRate() < 80:
			passStyle = t.Danger
		case w.PassRate() < 100:
			passStyle = t.Warn
		}

		lastGlyph, lastStyle := t.Glyphs.Pass, t.Faint
		if w.Last.Failed() {
			lastGlyph, lastStyle = t.Glyphs.Fail, t.Danger
		} else if w.Last.InProgress() {
			lastGlyph, lastStyle = t.Glyphs.Running, t.Warn
		}

		b.WriteString("\n")
		b.WriteString(t.Body.Render(pad(truncate(w.Name, nameWidth-1), nameWidth)) +
			t.Faint.Render(pad(truncate(shortRepo(w.Repo), colCIRepo-1), colCIRepo)) +
			t.Faint.Render(pad(itoa(w.Runs), colRuns)) +
			passStyle.Render(pad(itoa(w.PassRate())+"%", colPass)) +
			t.Dim.Render(pad(model.FormatDuration(w.Median), colMedian)) +
			padStyled(m.sparkline(w.Trend, trendWidth-2), trendWidth) +
			lastStyle.Render(lastGlyph+" ") +
			t.Faint.Render(model.ShortAge(nowSince(w.Last.UpdatedAt))))
	}
	if hidden > 0 {
		b.WriteString("\n")
		b.WriteString(t.Faint.Render("+" + itoa(hidden) + " more " +
			plural(hidden, "workflow", "workflows")))
	}
	return b.String()
}

func (m Model) renderHealthStrip() string {
	t := m.theme
	h := model.Health(m.scopedRuns(), m.settings.CIRunsWindow)
	dot := t.Faint.Render(" " + t.Glyphs.Dot + " ")

	rate := t.OK
	switch {
	case h.PassRate() < 80:
		rate = t.Danger
	case h.PassRate() < 95:
		rate = t.Warn
	}

	left := rate.Render(itoa(h.PassRate())+"%") + t.Dim.Render(" pass") +
		dot + t.Faint.Render("last "+itoa(h.Total)) +
		"   " + t.Strong.Render(model.FormatDuration(h.Median)) + t.Dim.Render(" median") +
		"   " + t.Strong.Render(itoa(h.Running)) + t.Dim.Render(" running")

	return spread(m.width, left, m.sparkline(m.scopedRuns(), sparkWidth))
}

func (m Model) sparkline(runs []model.WorkflowRun, n int) string {
	t := m.theme
	if len(runs) > n {
		runs = runs[:n]
	}
	if len(runs) == 0 {
		return ""
	}

	var longest float64
	for _, r := range runs {
		if d := r.Duration().Seconds(); d > longest {
			longest = d
		}
	}
	blocks := t.Glyphs.Sparkline
	var b strings.Builder
	for i := len(runs) - 1; i >= 0; i-- {
		level := 0
		if longest > 0 {
			level = clamp(int(runs[i].Duration().Seconds()/longest*float64(len(blocks)-1)+0.5), 0, len(blocks)-1)
		}
		style := t.OK
		if runs[i].Failed() {
			style = t.Danger
		} else if runs[i].InProgress() {
			style = t.Warn
		}
		b.WriteString(style.Render(blocks[level]))
	}
	return b.String()
}

func (m Model) handleCIKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := m.keys
	rows := m.ciRows()

	if m.logs.open && m.logs.focus {
		return m.handleLogKey(msg)
	}

	switch {
	case key.Matches(msg, k.Down):
		m.ciRow = clamp(m.ciRow+1, 0, maxInt(0, len(rows)-1))
		return m.cursorMoved()
	case key.Matches(msg, k.Up):
		m.ciRow = clamp(m.ciRow-1, 0, maxInt(0, len(rows)-1))
		return m.cursorMoved()
	case key.Matches(msg, k.Top):
		m.ciRow = 0
		return m.cursorMoved()
	case key.Matches(msg, k.End):
		m.ciRow = maxInt(0, len(rows)-1)
		return m.cursorMoved()
	case key.Matches(msg, k.PageDown):
		m.ciRow = clamp(m.ciRow+m.ciPage(), 0, maxInt(0, len(rows)-1))
		return m.cursorMoved()
	case key.Matches(msg, k.PageUp):
		m.ciRow = clamp(m.ciRow-m.ciPage(), 0, maxInt(0, len(rows)-1))
		return m.cursorMoved()

	case key.Matches(msg, k.Focus):
		if m.logs.open {
			m.logs.focus = true
		}
		return m, nil

	case key.Matches(msg, k.Logs):
		return m.toggleLogs()

	case key.Matches(msg, k.FailuresOnly):
		m.ciFailuresOnly = !m.ciFailuresOnly
		m.ciRow = 0
		m.rebuild()
		label := "all runs"
		if m.ciFailuresOnly {
			label = "failures only"
		}
		mm, cmd := m.cursorMoved()
		return mm, tea.Batch(cmd, m.notify(label, toastInfo))

	case key.Matches(msg, k.Open):
		if run, ok := m.selectedRun(); ok {
			if pr, found := m.prByBranch(run.Repo, run.Branch); found {
				m.sel = pr.Key()
				m.syncLaneToSelection()
				return m.switchView(ViewBoard)
			}
			return m, openURL(run.URL)
		}

	case key.Matches(msg, k.Rerun):
		if run, ok := m.selectedRun(); ok {
			return m.ask(confirmState{
				title: "Re-run " + run.Name + "?",
				body:  "starts a new workflow run on\n" + run.Branch,
				verb:  "re-run",
				run:   func(mm Model) tea.Cmd { return mm.rerunRunCmd(run) },
			})
		}
	}
	return m, nil
}

func (m Model) ciPage() int {
	return maxInt(1, (m.height-ciChromeRows-4)/2)
}
