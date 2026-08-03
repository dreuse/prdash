package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/dreuse/prdash/internal/model"
)

const (
	cardRulePad     = 3
	behindKeepAbove = 50
	ageDangerDays   = 180
	laneHeaderRows  = 3
)

type signalToken struct {
	text      string
	droppable bool
}

func (m Model) renderBoard(height int) string {
	l := Layout{Width: m.width, Height: m.height}
	order := m.order
	if len(order) == 0 {
		return ""
	}

	filled := make([]model.Column, 0, len(order))
	for _, col := range order {
		if len(m.lanes[col]) > 0 {
			filled = append(filled, col)
		}
	}
	if len(filled) == 0 {
		return m.renderNoPRs(height)
	}
	order = filled

	laneIdx := 0
	for i, col := range order {
		if len(m.order) > 0 && col == m.order[clamp(m.laneIdx, 0, len(m.order)-1)] {
			laneIdx = i
		}
	}

	offset, visible := laneWindow(len(order), laneIdx, l.MaxVisibleLanes())
	window := order[offset : offset+visible]

	counts := make([]int, len(window))
	for i, col := range window {
		counts[i] = len(m.lanes[col])
	}
	widths := laneWidths(counts, m.width)

	cols := make([]string, 0, len(window))
	for i, col := range window {
		cols = append(cols, lipgloss.NewStyle().
			Width(widths[i]).
			MarginRight(laneGutter).
			Render(m.renderLane(col, widths[i], height)))
	}

	board := lipgloss.JoinHorizontal(lipgloss.Top, cols...)
	if visible < len(order) {
		pages := (len(order) + visible - 1) / visible
		page := offset/maxInt(1, visible) + 1
		board = lipgloss.JoinVertical(lipgloss.Left, board,
			m.theme.Faint.Render(fmt.Sprintf("lanes %d/%d  %s for more", page, pages,
				m.theme.Glyphs.LeftRight)))
	}
	return board
}

func (m Model) renderNoPRs(height int) string {
	t := m.theme
	msg := t.Dim.Render("No PRs")
	switch {
	case !m.filter.Empty():
		msg = t.Dim.Render("No PRs match this filter") + "\n\n" + t.Faint.Render("esc to clear")
	case m.scope != "":
		msg = t.Dim.Render("No open PRs in "+shortRepo(m.scope)) + "\n\n" +
			t.Faint.Render("R to switch repository")
	}
	return lipgloss.Place(m.width, height, lipgloss.Center, lipgloss.Center, msg)
}

func (m Model) renderLane(col model.Column, width, height int) string {
	t := m.theme
	prs := m.lanes[col]

	header := t.LaneHeader(col).Render(col.String()) + " " + t.Faint.Render(itoa(len(prs)))
	rule := t.LaneRule(col).Render(strings.Repeat(t.Glyphs.HRule, maxInt(1, width)))

	budget := maxInt(1, height-laneHeaderRows)
	inner := maxInt(6, width-cardRulePad)
	heights := make([]int, len(prs))
	for i, pr := range prs {
		heights[i] = m.cardHeight(pr, col) + 1
	}

	active := len(m.order) > 0 && m.order[clamp(m.laneIdx, 0, len(m.order)-1)] == col
	start, end := fitCards(heights, budget, m.laneRow(), active)

	body := make([]string, 0, end-start+1)
	for i := start; i < end; i++ {
		body = append(body, m.renderCard(prs[i], col, inner, prs[i].Key() == m.sel))
	}
	if hidden := len(prs) - end; hidden > 0 {
		body = append(body, t.Faint.Render(" +"+itoa(hidden)+" more"))
	}

	return header + "\n" + rule + "\n\n" + strings.Join(body, "\n\n")
}

func fitCards(heights []int, budget, selected int, active bool) (start, end int) {
	if !active {
		selected = 0
	}
	for start = 0; ; start++ {
		used := 0
		for end = start; end < len(heights) && used+heights[end] <= budget; end++ {
			used += heights[end]
		}
		if end == start {
			end = minInt(start+1, len(heights))
		}
		if selected < end || start >= len(heights)-1 {
			return start, end
		}
	}
}

func (m Model) renderCard(pr model.PullRequest, col model.Column, width int, selected bool) string {
	t := m.theme
	dim := pr.IsDraft || int(pr.Age().Hours()/24) > ageDangerDays

	lines := []string{
		m.cardTitle(pr, width, selected, dim),
		m.cardMeta(pr, width, dim),
	}
	if signals := m.cardSignals(pr, col, width); signals != "" {
		lines = append(lines, signals)
	}
	if label, busy := m.pending[pr.Key()]; busy {
		lines = append(lines, t.Warn.Render(m.spinnerFrame()+" "+label))
	}

	rule := t.LaneAccent(col).Render(t.Glyphs.LaneRule)
	marker := " "
	rowStyle := lipgloss.NewStyle()
	if selected {
		marker = t.Accent.Render(t.Glyphs.Selected)
		rowStyle = t.Selected
	}

	out := make([]string, len(lines))
	for i, line := range lines {
		lead := rule + "  "
		if i == 0 {
			lead = rule + marker + " "
		}
		out[i] = rowStyle.Render(fillLine(lead+line, width+cardRulePad))
	}
	return strings.Join(out, "\n")
}

func (m Model) cardTitle(pr model.PullRequest, width int, selected, dim bool) string {
	t := m.theme
	num := "#" + itoa(pr.Number)
	numStyle, titleStyle := t.Faint, t.Strong
	switch {
	case selected:
		numStyle, titleStyle = t.Accent.Background(selectedBg(t)), t.SelectedTitle
	case dim:
		numStyle, titleStyle = t.Faint, t.Faint
	}
	title := truncate(pr.Title, maxInt(1, width-textWidth(num)-1))
	return numStyle.Render(num) + " " + titleStyle.Render(title)
}

func selectedBg(t Theme) lipgloss.TerminalColor {
	if t.NoColor {
		return lipgloss.NoColor{}
	}
	return toneBgSelected.adaptive()
}

func (m Model) cardMeta(pr model.PullRequest, width int, dim bool) string {
	t := m.theme
	days := int(pr.Age().Hours() / 24)

	authorStyle := t.Dim
	ageStyle := t.AgeStyle(days)
	if dim {
		authorStyle, ageStyle = t.Faint, t.Faint
	}

	stake := pr.StakeFor(m.viewer).String()
	repo := ""
	if m.multi {
		repo = shortRepo(pr.Repo)
	}

	dot := t.Faint.Render(" " + t.Glyphs.Dot + " ")
	line := ""
	for drop := 0; drop <= 2; drop++ {
		var parts []string
		if repo != "" && drop < 1 {
			parts = append(parts, t.Faint.Render(truncate(repo, 14)))
		}
		parts = append(parts, authorStyle.Render(pr.Author))
		if drop < 2 {
			parts = append(parts, ageStyle.Render(model.ShortAge(pr.Age())))
		}
		if stake != "" {
			parts = append(parts, t.Dim.Render(stake))
		}
		line = strings.Join(parts, dot)
		if lipgloss.Width(line) <= width {
			return line
		}
	}
	return truncateStyled(line, width)
}

func (m Model) hasSignals(pr model.PullRequest, col model.Column) bool {
	counts := pr.CheckCounts()
	switch {
	case counts.Failed > 0,
		pr.HasConflicts(),
		pr.ChangesRequested > 0,
		col == model.ColReadyToMerge && pr.Approvals >= m.policy.RequiredApprovals,
		counts.Total > 0 && counts.Passed < counts.Total,
		pr.BehindBy > 0:
		return true
	}
	return false
}

func (m Model) cardHeight(pr model.PullRequest, col model.Column) int {
	lines := 2
	if m.hasSignals(pr, col) {
		lines++
	}
	if _, busy := m.pending[pr.Key()]; busy {
		lines++
	}
	return lines
}

func (m Model) cardSignals(pr model.PullRequest, col model.Column, width int) string {
	if !m.hasSignals(pr, col) {
		return ""
	}
	t := m.theme
	counts := pr.CheckCounts()
	var tokens []signalToken

	if counts.Failed > 0 {
		tokens = append(tokens, signalToken{t.Danger.Render(fmt.Sprintf("%s %d %s failing",
			t.Glyphs.Fail, counts.Failed, plural(counts.Failed, "check", "checks"))), false})
	}
	if pr.HasConflicts() {
		tokens = append(tokens, signalToken{t.Warn.Render(t.Glyphs.Conflict + " conflict"), false})
	}
	if pr.ChangesRequested > 0 {
		tokens = append(tokens, signalToken{
			t.Review.Render(fmt.Sprintf("%d change req", pr.ChangesRequested)), true})
	}
	if col == model.ColReadyToMerge && pr.Approvals >= m.policy.RequiredApprovals {
		tokens = append(tokens, signalToken{t.OK.Render(t.Glyphs.Pass + " approved"), true})
	}
	if counts.Failed == 0 && counts.Total > 0 && counts.Passed < counts.Total {
		tokens = append(tokens, signalToken{t.Faint.Render(fmt.Sprintf("%s %d/%d",
			t.Glyphs.Pass, counts.Passed, counts.Total)), true})
	}
	if pr.BehindBy > 0 {
		tokens = append(tokens, signalToken{
			t.BehindStyle(pr.BehindBy).Render(t.Glyphs.Behind + itoa(pr.BehindBy)),
			pr.BehindBy <= behindKeepAbove,
		})
	}
	if len(tokens) == 0 {
		return ""
	}

	dot := t.Faint.Render(" " + t.Glyphs.Dot + " ")
	for {
		parts := make([]string, len(tokens))
		for i, tk := range tokens {
			parts[i] = tk.text
		}
		line := strings.Join(parts, dot)
		if lipgloss.Width(line) <= width {
			return line
		}
		dropped := false
		for i := len(tokens) - 1; i >= 0; i-- {
			if tokens[i].droppable {
				tokens = append(tokens[:i], tokens[i+1:]...)
				dropped = true
				break
			}
		}
		if !dropped {
			return truncateStyled(line, width)
		}
	}
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func shortRepo(full string) string {
	if _, name, ok := strings.Cut(full, "/"); ok {
		return name
	}
	return full
}
