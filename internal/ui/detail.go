package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/dreuse/prdash/internal/model"
)

const (
	passedChecks    = "checks"
	detailTwoColumn = 140
	detailColumnGap = 4
)

func (m Model) renderSplit(pr model.PullRequest, width, height int) string {
	t := m.theme
	rule := t.HorizontalRule(width)
	body := m.renderDetail(pr, width, maxInt(1, height-1))
	return rule + "\n" + body
}

func (m Model) renderDetail(pr model.PullRequest, width, height int) string {
	if width < detailTwoColumn {
		return lipgloss.NewStyle().MaxHeight(height).Render(
			strings.Join(m.detailBlocks(pr, width), "\n\n"))
	}

	half := (width - detailColumnGap) / 2
	left := m.detailBlocks(pr, half)
	right := left[2:]
	left = left[:2]

	return lipgloss.NewStyle().MaxHeight(height).Render(
		lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.NewStyle().Width(half).MarginRight(detailColumnGap).
				Render(strings.Join(left, "\n\n")),
			lipgloss.NewStyle().Width(half).Render(strings.Join(right, "\n\n"))))
}

func (m Model) detailBlocks(pr model.PullRequest, width int) []string {
	return []string{
		m.detailHeader(pr, width),
		m.detailChecks(pr, width),
		m.detailReview(pr, width),
		m.detailBranch(pr, width) + "\n\n" + m.actionChips(pr, width),
	}
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
