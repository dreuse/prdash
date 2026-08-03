package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dreuse/prdash/internal/model"
)

const (
	passedChecks      = "checks"
	detailTwoColumn   = 140
	detailColumnGap   = 4
	maxDetailComments = 3
	maxDetailCommits  = 5
	commentBodyLines  = 2
	commentGutter     = 2
	oidWidth          = 7
	detailChromeRows  = 2
)

type detailPane struct {
	focus   bool
	diff    bool
	pr      model.PullRequest
	lines   []string
	scroll  int
	loading bool
	err     error
	gen     int
}

func (p *detailPane) rewind() {
	p.scroll = 0
	p.lines = nil
	p.err = nil
	p.loading = false
}

func (m Model) renderSplit(pr model.PullRequest, width, height int) string {
	t := m.theme
	rows := maxInt(1, height-detailChromeRows)
	body, indicator := m.detailBody(pr, rows, width)

	head := fillLine(spread(width, m.detailLabel(pr), indicator), width)
	if m.detail.focus {
		head = t.Selected.Render(head)
	}
	return t.HorizontalRule(width) + "\n" + head + "\n" + body
}

func (m Model) detailLabel(pr model.PullRequest) string {
	t := m.theme
	label := "DETAILS"
	if m.detail.diff {
		label = "DIFF"
	}
	return t.Faint.Render(label) + " " + t.Accent.Render("#"+itoa(pr.Number))
}

func (m Model) detailBody(pr model.PullRequest, rows, width int) (string, string) {
	t := m.theme
	hint := t.Faint.Render("tab " + t.Glyphs.LeftRight)
	if m.detail.focus {
		hint = t.Accent.Render("focused") + t.Faint.Render("  "+t.Glyphs.UpDown+" scroll  tab "+t.Glyphs.LeftRight)
	}

	if m.detail.diff {
		return m.diffBody(rows, width, hint)
	}

	lines := m.detailLines(pr, width)
	start, end := window(m.detail.scroll, len(lines), rows)
	return strings.Join(lines[start:end], "\n"), scrollIndicator(t, end, len(lines)) + hint
}

func (m Model) detailLines(pr model.PullRequest, width int) []string {
	if width < detailTwoColumn {
		left, right := m.detailColumns(pr, width)
		return strings.Split(strings.Join(append(left, right...), "\n\n"), "\n")
	}

	half := (width - detailColumnGap) / 2
	left, right := m.detailColumns(pr, half)

	return strings.Split(lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(half).MarginRight(detailColumnGap).
			Render(strings.Join(left, "\n\n")),
		lipgloss.NewStyle().Width(half).Render(strings.Join(right, "\n\n"))), "\n")
}

func window(scroll, total, rows int) (int, int) {
	start := clamp(scroll, 0, maxInt(0, total-rows))
	return start, minInt(start+rows, total)
}

func scrollIndicator(t Theme, end, total int) string {
	if total == 0 {
		return ""
	}
	return t.Faint.Render(itoa(end) + "/" + itoa(total) + "  ")
}

func (m Model) handleDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := m.keys
	pr, ok := m.selectedPR()
	if !ok {
		return m, nil
	}

	switch {
	case key.Matches(msg, k.Focus):
		m.detail.focus = false
		return m, nil
	case key.Matches(msg, k.Split):
		m.split = false
		m.detail.focus = false
		return m, nil
	case key.Matches(msg, k.Diff):
		return m.toggleDiff(pr)
	case key.Matches(msg, k.SplitGrow):
		return m.resizeSplit(1)
	case key.Matches(msg, k.SplitShrink):
		return m.resizeSplit(-1)
	case key.Matches(msg, k.Expand):
		m.expanded[passedChecks] = !m.expanded[passedChecks]
		return m, nil
	case key.Matches(msg, k.NextHunk):
		return m.jumpHunk(1)
	case key.Matches(msg, k.PrevHunk):
		return m.jumpHunk(-1)
	}

	rows := m.detailRows()
	last := maxInt(0, m.detailTotal(pr, m.width)-rows)
	if next, moved := scrollFor(msg, k, m.detail.scroll, rows, last); moved {
		m.detail.scroll = next
	}
	return m, nil
}

func (m Model) selectionMoved() (tea.Model, tea.Cmd) {
	m.detail.rewind()
	save := m.persist()

	pr, ok := m.selectedPR()
	if !m.split || !m.detail.diff || !ok {
		return m, save
	}
	next, cmd := m.bindDiff(pr)
	return next, tea.Batch(save, cmd)
}

func (m Model) resizeSplit(delta int) (tea.Model, tea.Cmd) {
	if !m.split {
		return m, nil
	}
	l := Layout{Width: m.width, Height: m.height}
	body := m.bodyHeight()
	ceiling := maxInt(minSplitDetail, body-minSplitBoard)

	m.state.SplitRows = clamp(l.SplitDetailHeight(body, m.state.SplitRows)+delta, minSplitDetail, ceiling)
	return m, m.persist()
}

func (m Model) detailRows() int {
	l := Layout{Width: m.width, Height: m.height}
	return maxInt(1, l.SplitDetailHeight(m.bodyHeight(), m.state.SplitRows)-detailChromeRows)
}

func (m Model) detailTotal(pr model.PullRequest, width int) int {
	if m.detail.diff {
		return len(m.detail.lines)
	}
	return len(m.detailLines(pr, width))
}

func (m Model) detailColumns(pr model.PullRequest, width int) ([]string, []string) {
	return []string{
			m.detailHeader(pr, width),
			m.detailChecks(pr, width),
			m.detailReview(pr, width),
			m.detailBranch(pr, width) + "\n\n" + m.actionChips(pr, width),
		}, []string{
			m.detailCommits(pr, width),
			m.detailComments(pr, width),
		}
}

func (m Model) detailCommits(pr model.PullRequest, width int) string {
	t := m.theme
	var b strings.Builder
	b.WriteString(t.Faint.Render("COMMITS") + " " + t.Strong.Render(itoa(pr.CommitCount)))

	if len(pr.Commits) == 0 {
		b.WriteString("\n" + t.Faint.Render("no commits reported"))
		return b.String()
	}

	shown := pr.Commits
	if len(shown) > maxDetailCommits {
		shown = shown[len(shown)-maxDetailCommits:]
	}
	for i := len(shown) - 1; i >= 0; i-- {
		c := shown[i]
		age := ""
		if !c.CommittedAt.IsZero() {
			age = " " + t.Glyphs.Dot + " " + model.ShortAge(nowSince(c.CommittedAt))
		}
		room := maxInt(1, width-oidWidth-textWidth(age)-commentGutter)
		b.WriteString("\n" + t.Accent.Render(truncate(c.OID, oidWidth)) + " " +
			t.Body.Render(truncate(c.Headline, room)) + t.Faint.Render(age))
	}
	if left := pr.CommitCount - len(shown); left > 0 {
		b.WriteString("\n" + t.Faint.Render("+"+itoa(left)+" more"))
	}
	return b.String()
}

func (m Model) detailComments(pr model.PullRequest, width int) string {
	t := m.theme
	var b strings.Builder
	b.WriteString(t.Faint.Render("COMMENTS") + " " + t.Strong.Render(itoa(pr.CommentCount)))

	if len(pr.Comments) == 0 {
		b.WriteString("\n" + t.Faint.Render("no comments yet"))
		return b.String()
	}

	shown := pr.Comments
	if len(shown) > maxDetailComments {
		shown = shown[len(shown)-maxDetailComments:]
	}
	for i := len(shown) - 1; i >= 0; i-- {
		c := shown[i]
		who := c.Author
		if strings.EqualFold(who, m.viewer) {
			who = "you"
		}
		age := ""
		if !c.CreatedAt.IsZero() {
			age = " " + t.Glyphs.Dot + " " + model.ShortAge(nowSince(c.CreatedAt)) + " ago"
		}
		b.WriteString("\n" + t.Strong.Render(truncate(who, maxInt(1, width-textWidth(age)))) +
			t.Faint.Render(age))
		for _, line := range wrapBody(t.Dim, c.Body, maxInt(1, width-commentGutter)) {
			b.WriteString("\n" + strings.Repeat(" ", commentGutter) + line)
		}
	}
	return b.String()
}

func wrapBody(style lipgloss.Style, body string, width int) []string {
	if strings.TrimSpace(body) == "" {
		return nil
	}
	lines := strings.Split(style.Width(width).Render(body), "\n")
	if len(lines) > commentBodyLines {
		lines = lines[:commentBodyLines]
	}
	return lines
}

func (m Model) detailHeader(pr model.PullRequest, width int) string {
	t := m.theme
	title := t.Accent.Render("#"+itoa(pr.Number)) + " " +
		t.Strong.Render(truncate(pr.Title, maxInt(1, width-numberGutter)))

	meta := []string{pr.Author, "opened " + model.ShortAge(pr.Age()) + " ago"}
	if m.multi {
		meta = append([]string{shortRepo(pr.Repo)}, meta...)
	}
	meta = append(meta, pr.HeadRef+" "+t.Glyphs.Arrow+" "+pr.BaseRef)

	return title + "\n" + t.Dim.Render(truncate(strings.Join(meta, " "+t.Glyphs.Dot+" "), width))
}

func (m Model) detailChecks(pr model.PullRequest, width int) string {
	t := m.theme
	counts := pr.CheckCounts()

	var b strings.Builder
	b.WriteString(t.Faint.Render("CHECKS") + " " +
		t.Strong.Render(itoa(counts.Passed)) + t.Faint.Render(" / "+itoa(counts.Total)))

	if counts.Total == 0 {
		b.WriteString("\n" + t.Faint.Render("no checks reported"))
		return b.String()
	}
	for _, c := range pr.Checks {
		if c.State == model.CheckPassed {
			continue
		}
		detail := ""
		if c.State == model.CheckRunning && !c.StartedAt.IsZero() {
			detail = " (running " + model.ShortAge(nowSince(c.StartedAt)) + ")"
		}
		b.WriteString("\n" + t.CheckStyle(c.State).Render(t.CheckGlyph(c.State)) + " " +
			t.Body.Render(truncate(c.Name+detail, maxInt(1, width-2))))
	}
	if counts.Passed == 0 {
		return b.String()
	}
	if !m.expanded[passedChecks] {
		b.WriteString("\n" + t.Faint.Render(fmt.Sprintf("%s %d passed - collapsed, x to expand",
			t.Glyphs.Pass, counts.Passed)))
		return b.String()
	}
	for _, c := range pr.Checks {
		if c.State == model.CheckPassed {
			b.WriteString("\n" + t.Faint.Render(t.Glyphs.Pass+" "+truncate(c.Name, maxInt(1, width-2))))
		}
	}
	return b.String()
}

func (m Model) detailReview(pr model.PullRequest, width int) string {
	t := m.theme
	var b strings.Builder
	b.WriteString(t.Faint.Render("REVIEW") + " " +
		t.Strong.Render(itoa(pr.Approvals)) + t.Faint.Render(" / "+itoa(m.policy.RequiredApprovals)))

	for _, r := range pr.Reviews {
		glyph, style := t.Glyphs.Pending, t.Faint
		switch r.State {
		case model.ReviewApproved:
			glyph, style = t.Glyphs.Pass, t.OK
		case model.ReviewChangesRequested:
			glyph, style = t.Glyphs.Fail, t.Review
		}
		name := r.Login
		if strings.EqualFold(name, m.viewer) {
			name = "you"
		}
		age := ""
		if !r.SubmittedAt.IsZero() {
			age = " " + t.Glyphs.Dot + " " + model.ShortAge(nowSince(r.SubmittedAt)) + " ago"
		}
		b.WriteString("\n" + style.Render(glyph) + " " +
			t.Body.Render(truncate(name+age, maxInt(1, width-2))))
	}

	if len(pr.RequestedReviewers) > 0 {
		names := make([]string, 0, len(pr.RequestedReviewers))
		for _, r := range pr.RequestedReviewers {
			if strings.EqualFold(r, m.viewer) {
				names = append([]string{"you - requested"}, names...)
				continue
			}
			names = append(names, r)
		}
		b.WriteString("\n" + t.Faint.Render(t.Glyphs.Pending) + " " +
			t.Dim.Render(truncate(strings.Join(names, ", "), maxInt(1, width-2))))
	}
	if len(pr.Reviews) == 0 && len(pr.RequestedReviewers) == 0 {
		b.WriteString("\n" + t.Faint.Render("nobody has been asked yet"))
	}
	return b.String()
}

func (m Model) detailBranch(pr model.PullRequest, width int) string {
	t := m.theme
	dot := t.Faint.Render(" " + t.Glyphs.Dot + " ")

	var parts []string
	if pr.BehindBy > 0 {
		parts = append(parts, t.BehindStyle(pr.BehindBy).Render(
			t.Glyphs.Behind+itoa(pr.BehindBy)+" behind "+pr.BaseRef))
	} else {
		parts = append(parts, t.Faint.Render("up to date with "+pr.BaseRef))
	}
	if pr.HasConflicts() {
		parts = append(parts, t.Warn.Render(t.Glyphs.Conflict+" conflicts"))
	} else {
		parts = append(parts, t.OK.Render("no conflicts"))
	}
	if pr.Changed > 0 {
		parts = append(parts, t.Faint.Render(fmt.Sprintf("+%d -%d in %d files",
			pr.Additions, pr.Deletions, pr.Changed)))
	}
	return t.Faint.Render("BRANCH") + "\n" + truncateStyled(strings.Join(parts, dot), width)
}

func (m Model) actionChips(pr model.PullRequest, width int) string {
	t := m.theme
	type chip struct {
		key     string
		label   string
		primary bool
	}

	chips := []chip{}
	if m.canApprove(pr) {
		chips = append(chips, chip{"a", "approve", pr.RequestedFrom(m.viewer)})
	}
	if m.policy.ReadyToMerge(pr) {
		chips = append(chips, chip{"m", "merge", true})
	}
	if pr.CheckCounts().Failed > 0 {
		chips = append(chips, chip{"r", "re-run", true})
	}
	chips = append(chips, chip{"c", "comment", false}, chip{"X", "close", false})
	if pr.BehindBy > 0 || pr.HasConflicts() {
		chips = append(chips, chip{"b", "rebase", false})
	}
	chips = append(chips, chip{"y", "copy branch", false}, chip{t.Glyphs.Enter, "open", false})

	var lines []string
	current := ""
	for _, c := range chips {
		style := t.Chip
		if c.primary {
			style = t.ChipFilled
		}
		rendered := style.Render(c.key + " " + c.label)
		if current != "" && lipgloss.Width(current)+lipgloss.Width(rendered)+1 > width {
			lines = append(lines, current)
			current = ""
		}
		if current != "" {
			current += " "
		}
		current += rendered
	}
	if current != "" {
		lines = append(lines, current)
	}
	return strings.Join(lines, "\n")
}
