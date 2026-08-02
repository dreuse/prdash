package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/dreuse/prdash/internal/model"
	"github.com/dreuse/prdash/internal/readiness"
)

const (
	minColumnWidth = 26
	columnGap      = 1
)

func (m Model) multiRepo() bool {
	seen := make(map[string]struct{}, 2)
	for _, prs := range m.groups {
		for _, pr := range prs {
			seen[pr.Repo] = struct{}{}
			if len(seen) > 1 {
				return true
			}
		}
	}
	for _, r := range m.runs {
		seen[r.Repo] = struct{}{}
		if len(seen) > 1 {
			return true
		}
	}
	return false
}

func (m Model) renderBoard(height int) string {
	visible, offset := m.columnWindow()
	colWidth := m.columnWidth(visible)

	cols := make([]string, 0, visible)
	for i := offset; i < offset+visible && i < len(model.Columns); i++ {
		cols = append(cols, m.renderColumn(i, colWidth, height))
	}

	board := lipgloss.JoinHorizontal(lipgloss.Top, cols...)
	if visible < len(model.Columns) {
		board = lipgloss.JoinVertical(lipgloss.Left, board,
			m.theme.Dim.Render(fmt.Sprintf("columns %d-%d of %d", offset+1, offset+visible, len(model.Columns))))
	}
	return board
}

func (m Model) columnWidth(visible int) int {
	if visible == 0 {
		return minColumnWidth
	}
	w := (m.width - columnGap*(visible-1)) / visible
	if w < minColumnWidth {
		w = minColumnWidth
	}
	return w
}

func (m Model) columnWindow() (visible, offset int) {
	total := len(model.Columns)
	visible = (m.width + columnGap) / (minColumnWidth + columnGap)
	if visible >= total {
		return total, 0
	}
	if visible < 1 {
		visible = 1
	}
	offset = m.col - visible + 1
	if offset < 0 {
		offset = 0
	}
	if m.col < offset {
		offset = m.col
	}
	return visible, offset
}

func (m Model) renderColumn(idx, width, height int) string {
	col := model.Columns[idx]
	prs := m.groups[col]

	header := m.theme.ColumnStyle(col).Render(col.String()) + " " +
		m.theme.ColumnCount.Render(fmt.Sprintf("(%d)", len(prs)))
	if idx == m.col {
		header = m.theme.Icons.Selected + header
	} else {
		header = " " + header
	}

	body := make([]string, 0, len(prs)+1)
	if len(prs) == 0 {
		body = append(body, m.theme.Empty.Render("  "+m.emptyLabel()))
	}

	cards := make([]string, len(prs))
	heights := make([]int, len(prs))
	for i, pr := range prs {
		cards[i] = m.renderCard(pr, width-2, idx == m.col && i == m.row)
		heights[i] = lipgloss.Height(cards[i])
	}

	budget := height - 2
	start, end := fitCards(heights, budget, idx == m.col, m.row)
	if end < len(cards) {
		budget--
		start, end = fitCards(heights, budget, idx == m.col, m.row)
	}
	for i := start; i < end; i++ {
		body = append(body, cards[i])
	}
	if hidden := len(cards) - (end - start); hidden > 0 {
		body = append(body, m.theme.Dim.Render(fmt.Sprintf("  +%d more", hidden)))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, append([]string{header, ""}, body...)...)
	return lipgloss.NewStyle().Width(width).MaxHeight(height).MarginRight(columnGap).Render(content)
}

func fitCards(heights []int, budget int, active bool, selected int) (start, end int) {
	if budget < 1 {
		budget = 1
	}
	for start = 0; start < len(heights); start++ {
		used := 0
		for end = start; end < len(heights) && used+heights[end] <= budget; end++ {
			used += heights[end]
		}
		if end == start {
			end = start + 1
		}
		if !active || selected < end || start >= len(heights)-1 {
			return start, end
		}
	}
	return 0, len(heights)
}

func (m Model) renderCard(pr model.PullRequest, width int, selected bool) string {
	inner := width - 4
	if inner < 8 {
		inner = 8
	}

	title := m.theme.CardTitle.Render(truncate(fmt.Sprintf("#%d %s", pr.Number, pr.Title), inner))
	dot := " " + m.theme.Icons.Dot + " "
	metaParts := []string{"@" + pr.Author, model.ShortAge(pr.Age())}
	if m.multiRepo() {
		metaParts = append([]string{pr.Repo}, metaParts...)
	}
	meta := m.theme.CardMeta.Render(truncate(strings.Join(metaParts, dot), inner))

	checks := pr.ChecksState()
	if len(pr.Checks) == 0 {
		checks = model.CheckNeutral
	}
	checkBadge := m.theme.CheckStyle(checks).Render(m.theme.CheckIcon(checks) + " " + m.checkSummary(pr))
	approvalBadge := m.approvalBadge(pr)

	lines := []string{title, meta}
	if lipgloss.Width(checkBadge)+2+lipgloss.Width(approvalBadge) > inner {
		lines = append(lines, checkBadge, approvalBadge)
	} else {
		lines = append(lines, checkBadge+"  "+approvalBadge)
	}

	if rev := m.reviewerLine(pr, inner); rev != "" {
		lines = append(lines, rev)
	}
	if flags := m.flagLine(pr); flags != "" {
		lines = append(lines, flags)
	}

	style := m.theme.Card
	if selected {
		style = m.theme.CardSelected
	}
	return style.Width(width).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func (m Model) checkSummary(pr model.PullRequest) string {
	if len(pr.Checks) == 0 {
		return "no checks"
	}
	var passed, failed, running int
	for _, c := range pr.Checks {
		switch c.State {
		case model.CheckPassed:
			passed++
		case model.CheckFailed:
			failed++
		case model.CheckRunning:
			running++
		}
	}
	switch {
	case failed > 0:
		return fmt.Sprintf("%d failing", failed)
	case running > 0:
		return fmt.Sprintf("%d running", running)
	default:
		return fmt.Sprintf("%d/%d checks", passed, len(pr.Checks))
	}
}

func (m Model) approvalBadge(pr model.PullRequest) string {
	label := fmt.Sprintf("%s %d/%d", m.theme.Icons.Approved, pr.Approvals, m.policy.RequiredApprovals)
	if pr.ChangesRequested > 0 {
		return m.theme.Failed.Render(fmt.Sprintf("%s %d changes", m.theme.Icons.Failed, pr.ChangesRequested))
	}
	if pr.Approvals >= m.policy.RequiredApprovals {
		return m.theme.Passed.Render(label)
	}
	return m.theme.Dim.Render(label)
}

func (m Model) reviewerLine(pr model.PullRequest, width int) string {
	if len(pr.RequestedReviewers) == 0 {
		return ""
	}
	names := strings.Join(pr.RequestedReviewers, ", ")
	return m.theme.CardMeta.Render(truncate(m.theme.Icons.Reviewer+" "+names, width))
}

func (m Model) flagLine(pr model.PullRequest) string {
	var parts []string
	if pr.HasConflicts() {
		parts = append(parts, m.theme.Failed.Render(m.theme.Icons.Conflict+" conflict"))
	}
	if pr.BehindBy > 0 {
		parts = append(parts, m.theme.Warn.Render(fmt.Sprintf("%s%d behind", m.theme.Icons.Behind, pr.BehindBy)))
	}
	if pr.IsDraft {
		parts = append(parts, m.theme.Dim.Render(m.theme.Icons.Draft+" draft"))
	}
	return strings.Join(parts, "  ")
}

func (m Model) selectedPR() (model.PullRequest, bool) {
	prs := m.groups[model.Columns[m.col]]
	if m.row < 0 || m.row >= len(prs) {
		return model.PullRequest{}, false
	}
	return prs[m.row], true
}

func blockerText(p readiness.Policy, pr model.PullRequest) []string {
	blockers := p.Blockers(pr)
	out := make([]string, 0, len(blockers))
	for _, b := range blockers {
		out = append(out, string(b))
	}
	return out
}

func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= width {
		return s
	}
	if width <= 2 {
		return string(r[:width])
	}
	return string(r[:width-2]) + ".."
}
