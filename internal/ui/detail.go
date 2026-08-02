package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/dreuse/prdash/internal/model"
)

func (m Model) renderDetail(height int) string {
	pr, ok := m.selectedPR()
	if !ok {
		return m.theme.Empty.Render("no pull request selected")
	}

	label := m.theme.Dim
	rows := [][2]string{
		{"Repository", pr.Repo},
		{"Author", "@" + pr.Author},
		{"Branch", fmt.Sprintf("%s %s %s", pr.HeadRef, m.theme.Icons.Arrow, pr.BaseRef)},
		{"Opened", fmt.Sprintf("%s ago (%s)", model.ShortAge(pr.Age()), pr.CreatedAt.Format("2006-01-02 15:04"))},
		{"Updated", model.ShortAge(m.now().Sub(pr.UpdatedAt)) + " ago"},
		{"Column", m.policy.Classify(pr).String()},
		{"Approvals", m.approvalDetail(pr)},
		{"Reviewers", m.reviewerDetail(pr)},
		{"Mergeable", m.mergeableDetail(pr)},
		{"Behind base", m.behindDetail(pr)},
		{"URL", pr.URL},
	}

	var b strings.Builder
	b.WriteString(m.theme.Title.Render(fmt.Sprintf("#%d %s", pr.Number, pr.Title)))
	b.WriteString("\n\n")
	for _, r := range rows {
		b.WriteString(label.Render(fmt.Sprintf("%-12s", r[0])))
		b.WriteString(" ")
		b.WriteString(r[1])
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(m.theme.Title.Render("Merge readiness"))
	b.WriteString("\n")
	blockers := blockerText(m.policy, pr)
	if len(blockers) == 0 {
		b.WriteString(m.theme.Passed.Render(m.theme.Icons.Passed + " ready to merge"))
		b.WriteString("\n")
	} else {
		for _, blocker := range blockers {
			b.WriteString(m.theme.Failed.Render(m.theme.Icons.Failed+" ") + blocker + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(m.theme.Title.Render(fmt.Sprintf("Checks (%d)", len(pr.Checks))))
	b.WriteString("\n")
	if len(pr.Checks) == 0 {
		b.WriteString(m.theme.Empty.Render("no checks reported for the head commit"))
		b.WriteString("\n")
	}
	for _, c := range pr.Checks {
		b.WriteString(m.theme.CheckStyle(c.State).Render(m.theme.CheckIcon(c.State)))
		b.WriteString(" ")
		b.WriteString(c.Name)
		b.WriteString(" ")
		b.WriteString(m.theme.Dim.Render(string(c.State)))
		b.WriteString("\n")
	}

	return lipgloss.NewStyle().MaxHeight(height).Width(m.width).Render(b.String())
}

func (m Model) approvalDetail(pr model.PullRequest) string {
	s := fmt.Sprintf("%d of %d required", pr.Approvals, m.policy.RequiredApprovals)
	if pr.ChangesRequested > 0 {
		s += m.theme.Failed.Render(fmt.Sprintf("  (%d change requests outstanding)", pr.ChangesRequested))
	}
	return s
}

func (m Model) reviewerDetail(pr model.PullRequest) string {
	if len(pr.RequestedReviewers) == 0 {
		return m.theme.Dim.Render("none requested")
	}
	return strings.Join(pr.RequestedReviewers, ", ")
}

func (m Model) mergeableDetail(pr model.PullRequest) string {
	switch pr.Mergeable {
	case model.MergeableConflict:
		return m.theme.Failed.Render("conflicts with " + pr.BaseRef)
	case model.MergeableYes:
		return m.theme.Passed.Render("no conflicts")
	default:
		return m.theme.Dim.Render("unknown (github is still computing)")
	}
}

func (m Model) behindDetail(pr model.PullRequest) string {
	if pr.BehindBy <= 0 {
		return m.theme.Passed.Render("up to date")
	}
	return m.theme.Warn.Render(fmt.Sprintf("%d commits behind %s", pr.BehindBy, pr.BaseRef))
}
