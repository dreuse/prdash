package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/dreuse/prdash/internal/model"
)

const (
	minRunColumnWidth = 8
	runColumnGap      = 1
	rowMarkerWidth    = 2
)

type runColumn struct {
	title string
	value func(model.WorkflowRun) string
	flex  bool
}

func (m Model) runColumns() []runColumn {
	cols := make([]runColumn, 0, 5)
	if m.multiRepo() {
		cols = append(cols, runColumn{"REPOSITORY", func(r model.WorkflowRun) string { return r.Repo }, true})
	}
	return append(cols,
		runColumn{"WORKFLOW", func(r model.WorkflowRun) string { return r.Name }, true},
		runColumn{"BRANCH", func(r model.WorkflowRun) string { return r.Branch }, true},
		runColumn{"EVENT", func(r model.WorkflowRun) string { return r.Event }, false},
		runColumn{"DURATION", func(r model.WorkflowRun) string { return model.FormatDuration(r.Duration()) }, false},
	)
}

func (m Model) renderActions(height int) string {
	running, done := splitRuns(m.runs)
	cols := m.runColumns()
	widths := m.runColumnWidths(cols)

	var b strings.Builder
	b.WriteString(m.theme.Title.Render(fmt.Sprintf("Running workflows (%d)", len(running))))
	b.WriteString("\n")
	b.WriteString(m.runTable(cols, widths, running, 0))
	b.WriteString("\n")
	b.WriteString(m.theme.Title.Render(fmt.Sprintf("Recently completed (%d)", len(done))))
	b.WriteString("\n")
	b.WriteString(m.runTable(cols, widths, done, len(running)))

	return lipgloss.NewStyle().MaxHeight(height).Width(m.width).Render(b.String())
}

func splitRuns(runs []model.WorkflowRun) (running, done []model.WorkflowRun) {
	for _, r := range runs {
		if r.InProgress() {
			running = append(running, r)
		} else {
			done = append(done, r)
		}
	}
	return running, done
}

func (m Model) orderedRuns() []model.WorkflowRun {
	running, done := splitRuns(m.runs)
	return append(running, done...)
}

func (m Model) runColumnWidths(cols []runColumn) []int {
	widths := make([]int, len(cols))
	for i, c := range cols {
		widths[i] = runeLen(c.title)
		for _, r := range m.runs {
			if n := runeLen(c.value(r)); n > widths[i] {
				widths[i] = n
			}
		}
	}

	statusWidth := runeLen("STATUS")
	for _, r := range m.runs {
		if n := lipgloss.Width(m.runStatus(r)); n > statusWidth {
			statusWidth = n
		}
	}

	available := m.width - rowMarkerWidth - statusWidth - runColumnGap*len(cols)
	shrinkColumns(widths, cols, available)
	return widths
}

func shrinkColumns(widths []int, cols []runColumn, available int) {
	total, flexTotal := 0, 0
	for i, c := range cols {
		total += widths[i]
		if c.flex {
			flexTotal += widths[i]
		}
	}
	excess := total - available
	if excess <= 0 || flexTotal == 0 {
		return
	}

	for i, c := range cols {
		if !c.flex || excess <= 0 {
			continue
		}
		share := excess * widths[i] / flexTotal
		if share > widths[i]-minRunColumnWidth {
			share = widths[i] - minRunColumnWidth
		}
		if share > 0 {
			widths[i] -= share
		}
	}

	for again := true; again; {
		again = false
		total = 0
		for _, w := range widths {
			total += w
		}
		for i, c := range cols {
			if total <= available {
				return
			}
			if c.flex && widths[i] > minRunColumnWidth {
				widths[i]--
				total--
				again = true
			}
		}
	}
}

func (m Model) runTable(cols []runColumn, widths []int, runs []model.WorkflowRun, indexOffset int) string {
	if len(runs) == 0 {
		return m.theme.Empty.Render("  "+m.emptyLabel()) + "\n"
	}

	titles := make([]string, len(cols))
	for i, c := range cols {
		titles[i] = c.title
	}

	var b strings.Builder
	b.WriteString("  " + m.theme.TableHeader.Render(runRow(widths, titles...)+"STATUS") + "\n")

	for i, r := range runs {
		style := m.theme.TableRow
		marker := "  "
		if indexOffset+i == m.runRow {
			style = m.theme.TableRowActive
			marker = m.theme.Icons.Selected + " "
		}
		cells := make([]string, len(cols))
		for j, c := range cols {
			cells[j] = c.value(r)
		}
		b.WriteString(marker + style.Render(runRow(widths, cells...)) + m.runStatus(r) + "\n")
	}
	return b.String()
}

func runRow(widths []int, cells ...string) string {
	var b strings.Builder
	for i, c := range cells {
		if i >= len(widths) {
			break
		}
		b.WriteString(pad(truncate(c, widths[i]), widths[i]))
		b.WriteString(strings.Repeat(" ", runColumnGap))
	}
	return b.String()
}

func (m Model) runStatus(r model.WorkflowRun) string {
	if r.InProgress() {
		return m.theme.Running.Render(m.theme.Icons.Running + " " + r.Status)
	}
	switch r.Conclusion {
	case "success":
		return m.theme.Passed.Render(m.theme.Icons.Passed + " success")
	case "failure", "timed_out", "startup_failure", "action_required":
		return m.theme.Failed.Render(m.theme.Icons.Failed + " " + r.Conclusion)
	case "":
		return m.theme.Dim.Render(m.theme.Icons.Neutral + " " + r.Status)
	default:
		return m.theme.Dim.Render(m.theme.Icons.Neutral + " " + r.Conclusion)
	}
}

func runeLen(s string) int { return len([]rune(s)) }

func pad(s string, width int) string {
	if n := width - runeLen(s); n > 0 {
		return s + strings.Repeat(" ", n)
	}
	return s
}
