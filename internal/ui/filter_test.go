package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func openFilterBar(m Model) Model {
	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	return out.(Model)
}

func typeFilter(m Model, text string) Model {
	for _, r := range text {
		out, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = out.(Model)
	}
	return m
}

func TestFilterCompletesKeys(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m = openFilterBar(m)
	m = typeFilter(m, "assi")

	if len(m.filterBar.candidates) == 0 {
		t.Fatal("typing a key prefix should offer keys")
	}
	if got := m.filterBar.candidates[0].insert; got != "assignee:" {
		t.Fatalf("got %q, want assignee:", got)
	}
	if !strings.Contains(stripANSI(m.filterBar.input.View()), "assignee:") {
		t.Fatal("the key is not ghosted")
	}

	m = pressKey(m, tea.KeyTab)
	if got := m.filterBar.input.Value(); got != "assignee:" {
		t.Fatalf("tab produced %q", got)
	}
}

func TestFilterCompletesValues(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m = openFilterBar(m)
	m = typeFilter(m, "assignee:")

	if len(m.filterBar.candidates) == 0 {
		t.Fatal("a key should offer its values")
	}
	if m.filterBar.candidates[0].label != "@me" {
		t.Fatalf("@me should lead, got %q", m.filterBar.candidates[0].label)
	}

	m = typeFilter(m, "alf")
	if len(m.filterBar.candidates) == 0 {
		t.Fatal("no login matched alf")
	}
	if got := m.filterBar.candidates[0].insert; got != "assignee:alfoltran" {
		t.Fatalf("got %q", got)
	}
}

func TestFilterCompletesRealLoginsAndLabels(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)

	logins := m.knownLogins()
	for _, want := range []string{"alfoltran", "theorocha", "dreuse"} {
		if !containsFold(logins, want) {
			t.Errorf("%q is not offered as a login", want)
		}
	}
	if labels := m.knownLabels(); !containsFold(labels, "database") {
		t.Errorf("labels from the data are not offered: %v", labels)
	}
}

func TestFilterCompletesStateAndIs(t *testing.T) {
	for _, tc := range []struct{ typed, want string }{
		{"state:block", "state:blocked"},
		{"is:dra", "is:draft"},
		{"behind:>5", "behind:>50"},
		{"age:>3", "age:>30d"},
	} {
		m := testModel(t, 200, 40, ViewBoard)
		m = openFilterBar(m)
		m = typeFilter(m, tc.typed)

		if len(m.filterBar.candidates) == 0 {
			t.Errorf("%q offered nothing", tc.typed)
			continue
		}
		if got := m.filterBar.candidates[0].insert; got != tc.want {
			t.Errorf("%q offered %q, want %q", tc.typed, got, tc.want)
		}
	}
}

func TestFilterCompletionKeepsEarlierTokens(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m = openFilterBar(m)
	m = typeFilter(m, "is:draft assi")

	m = pressKey(m, tea.KeyTab)
	if got := m.filterBar.input.Value(); got != "is:draft assignee:" {
		t.Fatalf("completion clobbered earlier tokens: %q", got)
	}
}

func TestFilterAppliesLive(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	total := m.countAll()

	m = openFilterBar(m)
	m = typeFilter(m, "is:draft")

	if m.countAll() == 0 || m.countAll() >= total {
		t.Fatalf("filter did not narrow live: %d of %d", m.countAll(), total)
	}
}

func TestFreeTextFilterIsFuzzy(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m = openFilterBar(m)
	m = typeFilter(m, "idempo")

	if m.countAll() != 1 {
		t.Fatalf("free text should find one pull request, got %d", m.countAll())
	}
	if len(m.filterBar.candidates) != 0 {
		t.Fatal("free text should not be mistaken for a key")
	}
}

func TestFilterByAssigneeEndToEnd(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	total := m.countAll()

	m = openFilterBar(m)
	m = typeFilter(m, "assignee:theorocha")

	matched := m.countAll()
	if matched == 0 || matched >= total {
		t.Fatalf("assignee filter matched %d of %d", matched, total)
	}
	for _, prs := range m.lanes {
		for _, pr := range prs {
			if !pr.AssignedTo("theorocha") {
				t.Fatalf("#%d is not assigned to theorocha", pr.Number)
			}
		}
	}
}

func TestFilterByAssigneeMe(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m = openFilterBar(m)
	m = typeFilter(m, "assignee:@me")

	if m.countAll() == 0 {
		t.Fatal("assignee:@me found nothing")
	}
	for _, prs := range m.lanes {
		for _, pr := range prs {
			if !pr.AssignedTo(m.viewer) {
				t.Fatalf("#%d is not assigned to %s", pr.Number, m.viewer)
			}
		}
	}
}

func TestFilterEnterClosesAndKeeps(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m = openFilterBar(m)
	m = typeFilter(m, "is:draft")

	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = out.(Model)
	if m.filterBar.on {
		t.Fatal("enter should close the bar")
	}
	if m.filter.Raw != "is:draft" {
		t.Fatalf("the filter was lost: %q", m.filter.Raw)
	}
	if len(m.filterBar.candidates) != 0 {
		t.Fatal("candidates should be cleared when the bar closes")
	}
}

func TestFilterEscRestoresThePrevious(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m = openFilterBar(m)
	m = typeFilter(m, "is:draft")
	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = out.(Model)

	m = openFilterBar(m)
	m = typeFilter(m, " zzz")
	out, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = out.(Model)

	if m.filterBar.input.Value() != "is:draft" {
		t.Fatalf("esc did not restore the applied filter: %q", m.filterBar.input.Value())
	}
}

func TestFilterBarStaysInsideTheScreen(t *testing.T) {
	for _, width := range []int{62, 100, 200} {
		m := testModel(t, width, 24, ViewBoard)
		m = openFilterBar(m)
		m = typeFilter(m, "assignee:a")

		for i, line := range strings.Split(m.View(), "\n") {
			if w := lipglossWidth(line); w > width {
				t.Fatalf("at %d cols line %d is %d wide:\n%q", width, i, w, line)
			}
		}
	}
}

func TestSuggestionStripIsNotCutOffByTheGhost(t *testing.T) {
	for _, width := range []int{100, 160, 200, 260} {
		m := testModel(t, width, 24, ViewBoard)
		m = openFilterBar(m)
		m = typeFilter(m, "assignee:")

		if len(m.filterBar.candidates) == 0 {
			t.Fatalf("at %d cols there were no candidates", width)
		}
		band := stripANSI(m.renderFilterBand())
		if !strings.HasSuffix(strings.TrimRight(band, " "), "tab") {
			t.Fatalf("at %d cols the strip lost its tab hint:\n%q", width, band)
		}
	}
}

func TestCommentStripIsNotCutOffByTheGhost(t *testing.T) {
	for _, width := range []int{100, 160, 200, 260} {
		m := testModel(t, width, 24, ViewBoard)
		m = press(m, "c")
		m = press(m, "cc @a")

		if len(m.comment.candidates) == 0 {
			t.Fatalf("at %d cols there were no candidates", width)
		}
		bar := stripANSI(m.renderCommentBar())
		if !strings.HasSuffix(strings.TrimRight(bar, " "), "tab") {
			t.Fatalf("at %d cols the comment strip lost its tab hint:\n%q", width, bar)
		}
	}
}

func TestFilterCompletesNegatedTokens(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m = openFilterBar(m)
	m = typeFilter(m, "-assi")

	if len(m.filterBar.candidates) == 0 {
		t.Fatal("a negated prefix should still offer keys")
	}
	if got := m.filterBar.candidates[0].insert; got != "-assignee:" {
		t.Fatalf("got %q, want -assignee:", got)
	}

	m = pressKey(m, tea.KeyTab)
	if got := m.filterBar.input.Value(); got != "-assignee:" {
		t.Fatalf("tab produced %q", got)
	}
}

func TestNegatedFilterNarrowsTheBoard(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	total := m.countAll()

	m = openFilterBar(m)
	m = typeFilter(m, "-is:draft")

	narrowed := m.countAll()
	if narrowed == 0 || narrowed >= total {
		t.Fatalf("-is:draft matched %d of %d", narrowed, total)
	}
	for _, prs := range m.lanes {
		for _, pr := range prs {
			if pr.IsDraft {
				t.Fatalf("#%d is a draft and should be excluded", pr.Number)
			}
		}
	}
}
