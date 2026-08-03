package ui

import (
	"context"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/dreuse/prdash/internal/github"
	"github.com/dreuse/prdash/internal/model"
)

const (
	logDebounce   = 300 * time.Millisecond
	logTabStop    = "  "
	logNameWidth  = 24
	logRefWidth   = 28
	maxCachedLogs = 16
)

type logKey struct {
	run        int64
	failedOnly bool
}

type logPane struct {
	open       bool
	focus      bool
	failedOnly bool
	run        model.WorkflowRun
	lines      []string
	scroll     int
	loading    bool
	err        error
	gen        int
	cache      map[logKey][]string
	order      []logKey
}

func (p *logPane) remember(key logKey, lines []string) {
	if p.cache == nil {
		return
	}
	if _, seen := p.cache[key]; !seen {
		p.order = append(p.order, key)
	}
	p.cache[key] = lines
	for len(p.order) > maxCachedLogs {
		delete(p.cache, p.order[0])
		p.order = p.order[1:]
	}
}

func (p logPane) key() logKey {
	return logKey{run: p.run.ID, failedOnly: p.failedOnly}
}

func (p logPane) modeLabel() string {
	if p.failedOnly {
		return "failing steps"
	}
	return "full log"
}

func (p *logPane) close() {
	p.open = false
	p.focus = false
}

type logLoadMsg struct{ gen int }

type logsMsg struct {
	key   logKey
	lines []string
	err   error
}

func (m Model) toggleLogs() (tea.Model, tea.Cmd) {
	if m.logs.open {
		m.logs.close()
		return m, nil
	}
	run, ok := m.selectedRun()
	if !ok {
		return m, m.notify("no run selected", toastInfo)
	}
	m.logs.open = true
	m.logs.focus = false
	m.logs.failedOnly = run.Failed()
	return m.bindLog(run)
}

func (m Model) cursorMoved() (tea.Model, tea.Cmd) {
	run, ok := m.selectedRun()
	if ok {
		m.ciSel = run.ID
	}
	if !m.logs.open || !ok {
		return m, nil
	}
	if run.ID == m.logs.run.ID && (m.logs.lines != nil || m.logs.loading || m.logs.err != nil) {
		return m, nil
	}
	m.logs.failedOnly = run.Failed()
	return m.bindLog(run)
}

func (m Model) bindLog(run model.WorkflowRun) (tea.Model, tea.Cmd) {
	m.logs.run = run
	m.logs.err = nil
	m.logs.loading = false
	m.logs.lines = nil
	m.logs.scroll = 0

	if lines, ok := m.logs.cache[m.logs.key()]; ok {
		m.logs.lines = lines
		m.logs.scroll = len(lines)
		return m, nil
	}
	if run.InProgress() {
		m.logs.err = github.ErrLogsNotReady
		return m, nil
	}

	m.logs.loading = true
	m.logs.gen++
	gen := m.logs.gen
	return m, tea.Tick(logDebounce, func(time.Time) tea.Msg { return logLoadMsg{gen: gen} })
}

func (m Model) applyLogLoad(msg logLoadMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.logs.gen || !m.logs.open || !m.logs.loading {
		return m, nil
	}
	return m, m.fetchLogCmd(m.logs.run, m.logs.failedOnly)
}

func (m Model) fetchLogCmd(run model.WorkflowRun, failedOnly bool) tea.Cmd {
	actor := m.actor
	key := logKey{run: run.ID, failedOnly: failedOnly}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		lines, err := actor.RunLog(ctx, run, failedOnly)
		return logsMsg{key: key, lines: lines, err: err}
	}
}

func (m Model) applyLogs(msg logsMsg) (tea.Model, tea.Cmd) {
	if msg.err == nil {
		m.logs.remember(msg.key, msg.lines)
	}
	if msg.key != m.logs.key() {
		return m, nil
	}
	m.logs.loading = false
	m.logs.err = msg.err
	m.logs.lines = msg.lines
	m.logs.scroll = len(msg.lines)
	return m, nil
}

func (m Model) handleLogKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := m.keys
	page := m.logPageSize()
	last := len(m.logs.lines)

	switch {
	case key.Matches(msg, k.Focus):
		m.logs.focus = false
	case key.Matches(msg, k.Logs):
		m.logs.close()
	case key.Matches(msg, k.FailuresOnly):
		m.logs.failedOnly = !m.logs.failedOnly
		mm, cmd := m.bindLog(m.logs.run)
		return mm, tea.Batch(cmd, m.notify(m.logs.modeLabel(), toastInfo))
	case key.Matches(msg, k.Down):
		m.logs.scroll = clamp(m.logs.scroll+1, 0, last)
	case key.Matches(msg, k.Up):
		m.logs.scroll = clamp(m.logs.scroll-1, 0, last)
	case key.Matches(msg, k.PageDown):
		m.logs.scroll = clamp(m.logs.scroll+page, 0, last)
	case key.Matches(msg, k.PageUp):
		m.logs.scroll = clamp(m.logs.scroll-page, 0, last)
	case key.Matches(msg, k.Top):
		m.logs.scroll = 0
	case key.Matches(msg, k.End):
		m.logs.scroll = last
	}
	return m, nil
}

func (m Model) logPageSize() int {
	l := Layout{Width: m.width, Height: m.height}
	return maxInt(1, l.SplitDetailHeight(maxInt(1, m.height-ciChromeRows))-2)
}

func (m Model) renderLogSplit(width, height int) string {
	t := m.theme
	return t.HorizontalRule(width) + "\n" + m.renderLogPane(width, maxInt(1, height-1))
}

func (m Model) renderLogPane(width, height int) string {
	t := m.theme
	run := m.logs.run

	glyph, style := t.Glyphs.Pass, t.OK
	switch {
	case run.Failed():
		glyph, style = t.Glyphs.Fail, t.Danger
	case run.InProgress():
		glyph, style = t.Glyphs.Running, t.Warn
	}

	dot := t.Faint.Render(" " + t.Glyphs.Dot + " ")
	left := t.Faint.Render("LOGS") + "  " +
		t.Strong.Render(truncate(run.Name, logNameWidth)) +
		dot + t.Dim.Render(truncate(run.Branch, logRefWidth)) +
		dot + style.Render(glyph+" "+runStatus(run)) +
		dot + t.Accent.Render(m.logs.modeLabel())

	body, right := m.logBody(maxInt(1, height-1), width)
	head := fillLine(spread(width, left, right), width)
	if m.logs.focus {
		head = t.Selected.Render(head)
	}
	return head + "\n" + body
}

func (m Model) logBody(rows, width int) (string, string) {
	t := m.theme
	hint := t.Faint.Render("f mode  tab " + t.Glyphs.LeftRight)
	if m.logs.focus {
		hint = t.Accent.Render("focused") + t.Faint.Render("  f mode  tab "+t.Glyphs.LeftRight)
	}

	switch {
	case m.logs.loading:
		return t.Dim.Render(" " + m.spinnerFrame() + " loading logs"), hint
	case m.logs.err != nil:
		return t.Danger.Render(" "+truncate(m.logs.err.Error(), maxInt(1, width-2))) + "\n" +
			t.Faint.Render(" L to retry"), hint
	case len(m.logs.lines) == 0 && m.logs.failedOnly:
		return t.Faint.Render(" no failing steps") + "\n" +
			t.Faint.Render(" f for the full log"), hint
	case len(m.logs.lines) == 0:
		return t.Faint.Render(" no output"), hint
	}

	total := len(m.logs.lines)
	start := clamp(m.logs.scroll, 0, maxInt(0, total-rows))
	end := minInt(start+rows, total)

	var b strings.Builder
	for i := start; i < end; i++ {
		line := strings.ReplaceAll(m.logs.lines[i], "\t", logTabStop)
		b.WriteString(t.Body.Render(" " + truncate(line, maxInt(1, width-1))))
		if i < end-1 {
			b.WriteString("\n")
		}
	}
	return b.String(), t.Faint.Render(itoa(end)+"/"+itoa(total)+"  ") + hint
}
