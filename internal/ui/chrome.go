package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/dreuse/prdash/internal/model"
)

const appName = "prdash"

func (m Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	l := Layout{Width: m.width, Height: m.height}
	if l.TooSmall() {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			m.theme.Dim.Render(truncate("terminal too small - 60 columns minimum", m.width)))
	}

	header := m.renderHeader()
	filter := ""
	if l.ShowFilterBand() {
		filter = m.renderFilterBand()
	}
	footer := m.renderFooter()
	if m.comment.active {
		footer = m.renderCommentBar()
	}

	chrome := 1 + lipgloss.Height(footer)
	if filter != "" {
		chrome++
	}
	bodyHeight := maxInt(1, m.height-chrome)

	var body string
	switch {
	case !m.loadedOnce && m.err != nil:
		body = m.padBody(m.renderFatal(bodyHeight), bodyHeight)
	case !m.loadedOnce:
		body = m.padBody(m.renderSkeleton(bodyHeight), bodyHeight)
	case m.view == ViewCI:
		body = m.renderCIBody(l, bodyHeight)
	default:
		body = m.renderBoardBody(l, bodyHeight)
	}
	screen := header
	if filter != "" {
		screen += "\n" + filter
	}
	screen += "\n" + body + "\n" + footer
	screen = m.clampLines(screen)

	if kind, ok := m.overlay(); ok {
		screen = overlay(screen, m.renderOverlay(kind), m.width, m.height)
	}
	return screen
}

func (m Model) renderCIBody(l Layout, height int) string {
	if !m.logs.open {
		return m.padBody(m.renderCI(height), height)
	}

	budget := l.SplitDetailHeight(height)
	pane := m.renderLogSplit(m.width, budget)
	paneHeight := clamp(lipgloss.Height(pane), 1, budget)
	tableHeight := height - paneHeight

	return m.padBody(m.renderCI(tableHeight), tableHeight) + "\n" +
		m.padBody(pane, paneHeight)
}

func (m Model) renderBoardBody(l Layout, height int) string {
	pr, ok := m.selectedPR()
	if !m.split || !ok {
		return m.padBody(m.renderBoard(height), height)
	}

	budget := l.SplitDetailHeight(height)
	detail := m.renderSplit(pr, m.width, budget)
	detailHeight := clamp(lipgloss.Height(detail), 1, budget)
	boardHeight := height - detailHeight

	return m.padBody(m.renderBoard(boardHeight), boardHeight) + "\n" +
		m.padBody(detail, detailHeight)
}

func (m Model) clampLines(screen string) string {
	lines := strings.Split(screen, "\n")
	for i, line := range lines {
		if lipgloss.Width(line) > m.width {
			lines[i] = ansi.Truncate(line, m.width, "")
		}
	}
	return strings.Join(lines, "\n")
}

func (m Model) padBody(body string, height int) string {
	lines := strings.Split(body, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, m.width, "")
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderHeader() string {
	t := m.theme
	repos := m.repoNames()

	for drop := 0; ; drop++ {
		left := t.Brand.Render(appName)
		if len(repos) > 0 && drop < 2 {
			label := m.scopeLabel() + " " + t.Glyphs.Expand
			if m.scope == "" && len(repos) > 1 {
				label = "all repos " + t.Glyphs.Expand + " " + itoa(len(repos))
			}
			left += t.Chrome.Render("  ") + t.ChromeDim.Render(label)
		}

		tabs := make([]string, 0, len(Views))
		for _, v := range Views {
			label := " " + itoa(int(v)+1) + " " + v.Label() + " "
			if drop >= 3 {
				label = " " + itoa(int(v)+1) + " "
			}
			style := t.TabIdle
			if v == m.view {
				style = t.TabActive
			}
			tabs = append(tabs, style.Render(label))
		}
		left += t.Chrome.Render("   ") + strings.Join(tabs, t.Chrome.Render(" "))

		right := m.headerStatus(drop)
		line := spread(m.width, left, right)
		if lipgloss.Width(line) <= m.width || drop >= 5 {
			return t.Chrome.Render(fillLine(line, m.width))
		}
	}
}

func (m Model) headerStatus(drop int) string {
	t := m.theme
	var parts []string

	if n := m.needsYou(); n > 0 && drop < 4 {
		parts = append(parts, t.OK.Background(chromeBg(t)).Render(fmt.Sprintf("%d need you", n)))
	}
	if blocked := len(m.lanes[model.ColBlocked]); blocked > 0 && drop < 1 {
		parts = append(parts, t.ChromeDim.Render(fmt.Sprintf("%d blocked", blocked)))
	}
	if drop < 2 {
		parts = append(parts, t.ChromeDim.Render(fmt.Sprintf("%d open", m.countAll())))
	}

	switch {
	case m.loading:
		parts = append(parts, t.ChromeDim.Render(m.spinnerFrame()+" refreshing"))
	case m.rateLimited:
		parts = append(parts, t.Warn.Background(chromeBg(t)).Render(
			t.Glyphs.Stale+" rate limited "+t.Glyphs.Dot+" retry in "+model.ShortAge(rateLimitBackoff)))
	case m.stale && !m.lastUpdate.IsZero():
		parts = append(parts, t.Warn.Background(chromeBg(t)).Render(
			t.Glyphs.Stale+" stale "+model.ShortAge(nowSince(m.lastUpdate))))
	case !m.lastUpdate.IsZero():
		parts = append(parts, t.ChromeFaint.Render(t.Glyphs.Refresh+" "+model.ShortAge(nowSince(m.lastUpdate))))
	}
	return strings.Join(parts, t.ChromeFaint.Render(" "+t.Glyphs.Dot+" "))
}

func (m Model) needsYou() int {
	n := 0
	for _, prs := range m.lanes {
		for _, pr := range prs {
			if pr.RequestedFrom(m.viewer) {
				n++
			}
		}
	}
	return n
}

func (m Model) countAll() int {
	n := 0
	for _, prs := range m.lanes {
		n += len(prs)
	}
	return n
}

func (m Model) renderFilterBand() string {
	t := m.theme
	if m.filterBar.on {
		label := t.Accent.Background(chromeBg(t)).Render("/ ")
		right := t.ChromeFaint.Render("tab completes " + t.Glyphs.Dot + " " +
			t.Glyphs.Enter + " apply " + t.Glyphs.Dot + " esc cancel")
		if len(m.filterBar.candidates) > 0 {
			right = m.renderSuggestions(m.filterBar.suggestBox, true)
		}

		m.filterBar.input.Width = maxInt(8, m.width-lipgloss.Width(label)-
			lipgloss.Width(right)-m.filterBar.ghostWidth()-2)
		return t.Chrome.Render(fillLine(
			spread(m.width, label+m.filterBar.input.View(), right), m.width))
	}

	left := t.ChromeFaint.Render("/ ")
	if m.filter.Empty() {
		left += t.ChromeFaint.Render("filter")
	} else {
		rendered := make([]string, 0, len(m.filter.Tokens))
		for _, tok := range m.filter.Tokens {
			if !tok.Valid {
				rendered = append(rendered, t.Danger.Background(chromeBg(t)).Render(tok.Text))
				continue
			}
			if tok.Key == "" {
				rendered = append(rendered, t.Chrome.Render(tok.Text))
				continue
			}
			rendered = append(rendered, t.ChromeDim.Render(tok.Key+":")+t.Warn.Background(chromeBg(t)).Render(tok.Value))
		}
		left += strings.Join(rendered, t.Chrome.Render("  "))
	}
	left += t.ChromeFaint.Render("  "+t.Glyphs.Dot+" sort:") + t.Warn.Background(chromeBg(t)).Render(m.sortMode.String())

	right := t.ChromeFaint.Render("/ to edit " + t.Glyphs.Dot + " esc to clear")
	if !m.filter.Empty() {
		right = t.ChromeFaint.Render(fmt.Sprintf("%d of %d match", m.countAll(), len(m.prs))) +
			t.ChromeFaint.Render("  "+t.Glyphs.Dot+" esc to clear")
	}
	return t.Chrome.Render(fillLine(spread(m.width, left, right), m.width))
}

func chromeBg(t Theme) lipgloss.TerminalColor {
	if t.NoColor {
		return lipgloss.NoColor{}
	}
	return toneBgChrome.adaptive()
}

func (m Model) renderFooter() string {
	t := m.theme
	keys := m.contextualKeys()
	right := t.ChromeFaint.Render("? keys")

	bar := ""
	for {
		left := strings.Join(keys, t.ChromeFaint.Render("   "))
		line := spread(m.width, left, right)
		if lipgloss.Width(line) <= m.width || len(keys) <= 1 {
			bar = t.Chrome.Render(fillLine(line, m.width))
			break
		}
		keys = keys[:len(keys)-1]
	}

	if m.toast.text == "" {
		return bar
	}
	style := t.Dim
	switch m.toast.kind {
	case toastGood:
		style = t.OK
	case toastBad:
		style = t.Danger
	}
	return style.Render(truncate(m.toast.text, m.width)) + "\n" + bar
}

func (m Model) splitLabel() string {
	if m.split {
		return "hide details"
	}
	return "details"
}

func (m Model) chip(k, label string) string {
	return m.theme.ChromeDim.Render(k) + m.theme.ChromeFaint.Render(" "+label)
}

func (m Model) contextualKeys() []string {
	t := m.theme
	if m.filterBar.on {
		return []string{m.chip(t.Glyphs.Enter, "apply"), m.chip("tab", "complete"), m.chip("esc", "cancel")}
	}
	if m.view == ViewCI {
		out := []string{m.chip(t.Glyphs.UpDown, "move")}
		if m.logs.open {
			out = append(out, m.chip("tab", "pane"), m.chip("esc", "close logs"))
		} else {
			out = append(out, m.chip("L", "logs"))
		}

		failures := m.chip("f", "failures only")
		if m.logs.open && m.logs.focus {
			failures = m.chip("f", m.logs.modeLabel())
		}
		return append(out,
			m.chip(t.Glyphs.Enter, "open run"), m.chip("r", "re-run"),
			failures, m.chip("R", "repo"),
			m.chip("u", "refresh"), m.chip("1", "board"))
	}

	out := []string{
		m.chip(t.Glyphs.UpDown, "pr"),
		m.chip(t.Glyphs.LeftRight, "lane"),
		m.chip("v", m.splitLabel()),
		m.chip(t.Glyphs.Enter, "open"),
	}

	pr, ok := m.selectedPR()
	if !ok {
		return append(out, m.chip("/", "filter"), m.chip(",", "settings"))
	}
	if label, busy := m.pending[pr.Key()]; busy {
		return append(out, t.Warn.Background(chromeBg(t)).Render(m.spinnerFrame()+" "+label))
	}

	if m.canApprove(pr) {
		out = append(out, m.chip("a", "approve"))
	}
	if m.policy.ReadyToMerge(pr) {
		out = append(out, m.chip("m", "merge"))
	}
	if pr.CheckCounts().Failed > 0 {
		out = append(out, m.chip("r", "re-run checks"))
	}
	if pr.BehindBy > 0 || pr.HasConflicts() {
		out = append(out, m.chip("b", "rebase"))
	}
	out = append(out,
		m.chip("c", "comment"),
		m.chip("X", "close"),
		m.chip("y", "copy branch"),
		m.chip("/", "filter"),
		m.chip("R", "repo"),
		m.chip("u", "refresh"),
	)
	return out
}

func (m Model) renderSkeleton(height int) string {
	t := m.theme
	if m.view != ViewBoard {
		lines := make([]string, 0, height)
		for i := 0; i < minInt(height, 8); i++ {
			lines = append(lines, t.Rule.Render(strings.Repeat(t.Glyphs.HRule, minInt(m.width-2, 40+i*3))))
		}
		return strings.Join(lines, "\n\n")
	}

	order := m.laneOrder()
	widths := laneWidths(make([]int, len(order)), m.width)
	cols := make([]string, 0, len(order))
	for i, col := range order {
		var b strings.Builder
		b.WriteString(t.LaneHeader(col).Render(col.String()))
		b.WriteString("\n")
		b.WriteString(t.LaneRule(col).Render(strings.Repeat(t.Glyphs.HRule, maxInt(1, widths[i]))))
		for row := 0; row < minInt(4, maxInt(0, height/4)); row++ {
			b.WriteString("\n\n")
			b.WriteString(t.Rule.Render(strings.Repeat(t.Glyphs.HRule, maxInt(1, widths[i]-4))))
		}
		cols = append(cols, lipgloss.NewStyle().Width(widths[i]).MarginRight(laneGutter).Render(b.String()))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, cols...) + "\n\n" +
		t.Dim.Render(m.spinnerFrame()+" loading "+strings.Join(m.repos, ", "))
}

func (m Model) renderFatal(height int) string {
	t := m.theme
	var b strings.Builder
	b.WriteString(t.Danger.Render(t.Glyphs.Fail + " could not load data"))
	b.WriteString("\n\n")
	b.WriteString(t.Body.Render(truncate(m.err.Error(), m.width)))
	b.WriteString("\n\n")
	b.WriteString(t.Dim.Render(m.recovery()))
	return lipgloss.Place(m.width, height, lipgloss.Center, lipgloss.Center, b.String())
}

func overlay(base, panel string, width, height int) string {
	baseLines := strings.Split(base, "\n")
	panelLines := strings.Split(panel, "\n")

	panelWidth := lipgloss.Width(panel)
	left := maxInt(0, (width-panelWidth)/2)
	top := maxInt(0, (height-len(panelLines))/2)

	for i, line := range panelLines {
		row := top + i
		if row < 0 || row >= len(baseLines) {
			continue
		}
		prefix := ansi.Truncate(baseLines[row], left, "")
		prefix += strings.Repeat(" ", maxInt(0, left-lipgloss.Width(prefix)))
		suffix := ansi.TruncateLeft(baseLines[row], left+panelWidth, "")
		baseLines[row] = prefix + line + suffix
	}
	return strings.Join(baseLines, "\n")
}
