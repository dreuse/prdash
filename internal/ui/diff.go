package ui

import (
	"context"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dreuse/prdash/internal/model"
)

const (
	diffTabStop     = "  "
	diffFileHeader  = "diff --git"
	minimapWidth    = 2
	minimapMinWidth = 60
)

type diffLoadMsg struct{ gen int }

type diffMsg struct {
	key   model.Key
	lines []string
	err   error
}

func (m Model) toggleDiff(pr model.PullRequest) (tea.Model, tea.Cmd) {
	if m.detail.diff {
		m.detail.diff = false
		m.detail.rewind()
		return m, nil
	}
	m.detail.diff = true
	return m.bindDiff(pr)
}

func (m Model) bindDiff(pr model.PullRequest) (tea.Model, tea.Cmd) {
	m.detail.rewind()
	m.detail.pr = pr
	m.detail.loading = true
	m.detail.gen++

	gen := m.detail.gen
	return m, tea.Tick(paneDebounce, func(time.Time) tea.Msg { return diffLoadMsg{gen: gen} })
}

func (m Model) applyDiffLoad(msg diffLoadMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.detail.gen || !m.detail.diff || !m.detail.loading {
		return m, nil
	}
	return m, m.fetchDiffCmd(m.detail.pr)
}

func (m Model) fetchDiffCmd(pr model.PullRequest) tea.Cmd {
	actor := m.actor
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		lines, err := actor.Diff(ctx, pr)
		return diffMsg{key: pr.Key(), lines: lines, err: err}
	}
}

func (m Model) applyDiff(msg diffMsg) (tea.Model, tea.Cmd) {
	if msg.key != m.detail.pr.Key() {
		return m, nil
	}
	m.detail.loading = false
	m.detail.err = msg.err
	m.detail.lines = msg.lines
	m.detail.scroll = 0
	return m, nil
}

func (m Model) diffBody(rows, width int, hint string) (string, string) {
	t := m.theme

	switch {
	case m.detail.loading:
		return t.Dim.Render(" " + m.spinnerFrame() + " loading diff"), hint
	case m.detail.err != nil:
		return t.Danger.Render(" "+truncate(m.detail.err.Error(), maxInt(1, width-2))) + "\n" +
			t.Faint.Render(" d to retry"), hint
	case len(m.detail.lines) == 0:
		return t.Faint.Render(" no textual changes"), hint
	}

	total := len(m.detail.lines)
	start, end := window(m.detail.scroll, total, rows)

	var cells []string
	body := width
	if width >= minimapMinWidth {
		if cells = minimap(t, m.detail.lines, start, end, rows); cells != nil {
			body = width - minimapWidth
		}
	}

	var b strings.Builder
	for i := start; i < end; i++ {
		line := strings.ReplaceAll(m.detail.lines[i], "\t", diffTabStop)
		row := diffLineStyle(t, m.detail.lines[i]).Render(" " + truncate(line, maxInt(1, body-1)))
		if cells != nil {
			row = padStyled(row, body) + " " + cells[i-start]
		}
		b.WriteString(row)
		if i < end-1 {
			b.WriteString("\n")
		}
	}
	return b.String(), scrollIndicator(t, end, total) + hint
}

func minimap(t Theme, lines []string, start, end, rows int) []string {
	total := len(lines)
	if total <= rows || rows < 1 {
		return nil
	}

	cells := make([]string, rows)
	for r := range cells {
		from, to := r*total/rows, maxInt((r+1)*total/rows, r*total/rows+1)
		adds, dels := 0, 0
		for _, line := range lines[from:minInt(to, total)] {
			switch {
			case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"):
			case strings.HasPrefix(line, "+"):
				adds++
			case strings.HasPrefix(line, "-"):
				dels++
			}
		}

		glyph := t.Glyphs.Track
		if from < end && to > start {
			glyph = t.Glyphs.Thumb
		}
		cells[r] = minimapStyle(t, adds, dels).Render(glyph)
	}
	return cells
}

func minimapStyle(t Theme, adds, dels int) lipgloss.Style {
	switch {
	case adds > 0 && dels > 0:
		return t.Accent
	case adds > 0:
		return t.OK
	case dels > 0:
		return t.Danger
	}
	return t.Faint
}

func diffFileAt(lines []string, at int) string {
	for i := minInt(at, len(lines)-1); i >= 0; i-- {
		if !strings.HasPrefix(lines[i], diffFileHeader) {
			continue
		}
		_, path, _ := strings.Cut(lines[i], " b/")
		return path
	}
	return ""
}

func linesStarting(lines []string, prefix string) []int {
	var at []int
	for i, line := range lines {
		if strings.HasPrefix(line, prefix) {
			at = append(at, i)
		}
	}
	return at
}

func blockStarts(lines []string) []int {
	var at []int
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "" && strings.TrimSpace(lines[i-1]) == "" {
			at = append(at, i)
		}
	}
	return at
}

func nextTarget(targets []int, cur, delta int) (int, bool) {
	if delta > 0 {
		for _, at := range targets {
			if at > cur {
				return at, true
			}
		}
		return cur, false
	}
	for i := len(targets) - 1; i >= 0; i-- {
		if targets[i] < cur {
			return targets[i], true
		}
	}
	return 0, cur > 0
}

func (m Model) paneTargets(pr model.PullRequest, prefix string) []int {
	if !m.detail.diff {
		return blockStarts(m.detailLines(pr, m.width))
	}
	return linesStarting(m.detail.lines, prefix)
}

func (m Model) jumpBy(prefix string, delta int) (tea.Model, tea.Cmd) {
	pr, ok := m.selectedPR()
	if !ok {
		return m, nil
	}
	next, moved := nextTarget(m.paneTargets(pr, prefix), m.detail.scroll, delta)
	if !moved {
		return m, nil
	}
	rows := m.detailRows()
	m.detail.scroll = clamp(next, 0, maxInt(0, m.detailTotal(pr, m.width)-rows))
	return m, nil
}

func (m Model) jumpFile(delta int) (tea.Model, tea.Cmd) {
	if !m.split || !m.detail.focus {
		return m, nil
	}
	return m.jumpBy(diffFileHeader, delta)
}

func (m Model) jumpHunk(delta int) (tea.Model, tea.Cmd) {
	return m.jumpBy("@@", delta)
}

func diffLineStyle(t Theme, line string) lipgloss.Style {
	switch {
	case strings.HasPrefix(line, "diff --git"), strings.HasPrefix(line, "index "):
		return t.Strong
	case strings.HasPrefix(line, "---"), strings.HasPrefix(line, "+++"):
		return t.Faint
	case strings.HasPrefix(line, "@@"):
		return t.Accent
	case strings.HasPrefix(line, "+"):
		return t.OK
	case strings.HasPrefix(line, "-"):
		return t.Danger
	}
	return t.Body
}
