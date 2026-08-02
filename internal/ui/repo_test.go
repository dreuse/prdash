package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dreuse/prdash/internal/config"
	"github.com/dreuse/prdash/internal/model"
)

func openPicker(m Model) Model {
	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	return out.(Model)
}

func typePicker(m Model, text string) Model {
	for _, r := range text {
		out, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = out.(Model)
	}
	return m
}

func TestRepoPickerListsAllAndEachRepo(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m = openPicker(m)

	if _, open := m.overlay(); !open {
		t.Fatal("R should open the picker")
	}
	entries := m.repoEntries()
	if len(entries) < 3 {
		t.Fatalf("expected all plus two repos, got %d", len(entries))
	}
	if entries[0].repo != "" || !strings.Contains(entries[0].label, "All") {
		t.Fatalf("the first entry should be all repositories, got %+v", entries[0])
	}
	for _, e := range entries[1:] {
		if e.detail == "" {
			t.Errorf("%q shows no open count", e.label)
		}
	}
}

func TestRepoPickerScopesTheBoard(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	total := len(m.prs)

	m = openPicker(m)
	m = typePicker(m, "device-gateway")

	entries := m.repoEntries()
	if len(entries) == 0 {
		t.Fatal("filtering found nothing")
	}
	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = out.(Model)

	if m.scope == "" {
		t.Fatal("selecting a repository should set the scope")
	}
	if _, open := m.overlay(); open {
		t.Fatal("the picker should close after selecting")
	}

	scoped := m.countAll()
	if scoped == 0 || scoped >= total {
		t.Fatalf("scope did not narrow the board: %d of %d", scoped, total)
	}
	for _, prs := range m.lanes {
		for _, pr := range prs {
			if pr.Repo != m.scope {
				t.Fatalf("%s leaked into a board scoped to %s", pr.Repo, m.scope)
			}
		}
	}
}

func TestRepoScopeAlsoNarrowsCI(t *testing.T) {
	m := testModel(t, 200, 40, ViewCI)
	all := len(m.scopedRuns())

	m.scope = "acme/device-gateway"
	scoped := m.scopedRuns()
	if len(scoped) == 0 || len(scoped) >= all {
		t.Fatalf("ci runs did not narrow: %d of %d", len(scoped), all)
	}
	for _, r := range scoped {
		if r.Repo != m.scope {
			t.Fatalf("%s leaked into ci scoped to %s", r.Repo, m.scope)
		}
	}
}

func TestRepoPickerReturnsToAll(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m.scope = "acme/device-gateway"
	m.rebuild()
	narrowed := m.countAll()

	m = openPicker(m)
	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = out.(Model)

	if m.scope != "" {
		t.Fatalf("selecting all should clear the scope, got %q", m.scope)
	}
	if m.countAll() <= narrowed {
		t.Fatal("clearing the scope should widen the board")
	}
}

func TestRepoPickerOffersToAddAnUntrackedRepo(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m = openPicker(m)
	m = typePicker(m, "acme/api")

	entries := m.repoEntries()
	if len(entries) == 0 || !entries[len(entries)-1].add {
		t.Fatalf("owner/name should offer an add entry, got %+v", entries)
	}

	m.repoPick.idx = len(entries) - 1
	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = out.(Model)

	if !containsFold(m.settings.Repos, "acme/api") {
		t.Fatalf("the repository was not tracked: %v", m.settings.Repos)
	}
	if m.scope != "acme/api" {
		t.Fatalf("adding should switch to it, scope is %q", m.scope)
	}
}

func TestRepoPickerDoesNotOfferAddForBareText(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m = openPicker(m)
	m = typePicker(m, "payments")

	for _, e := range m.repoEntries() {
		if e.add {
			t.Fatalf("%q is not owner/name and must not offer add", "payments")
		}
	}
}

func TestScopeSurvivesInTheHeaderAndState(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m.scope = "acme/device-gateway"
	m.rebuild()

	if !strings.Contains(stripANSI(m.View()), "device-gateway") {
		t.Fatal("the header does not show the active repository")
	}

	m.scope = ""
	if got := m.scopeLabel(); got != "all repos" {
		t.Fatalf("scopeLabel with several repos = %q", got)
	}
}

func TestRefreshIsOfferedInTheFooter(t *testing.T) {
	for _, view := range Views {
		m := testModel(t, 200, 40, view)
		footer := stripANSI(m.renderFooter())
		if !strings.Contains(footer, "refresh") {
			t.Errorf("%s footer does not offer refresh:\n%s", view, footer)
		}
		if !strings.Contains(footer, "repo") {
			t.Errorf("%s footer does not offer the repo switcher:\n%s", view, footer)
		}
	}
}

func TestForceRefreshFetchesImmediately(t *testing.T) {
	for _, k := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune{'u'}},
		{Type: tea.KeyCtrlR},
		{Type: tea.KeyF5},
	} {
		m := testModel(t, 200, 40, ViewBoard)
		m.loading = false
		before := m.tickGen

		out, cmd := m.Update(k)
		m = out.(Model)
		if !m.loading {
			t.Fatalf("%v did not start a fetch", k)
		}
		if cmd == nil {
			t.Fatalf("%v produced no command", k)
		}
		if m.tickGen == before {
			t.Fatalf("%v did not reset the poll timer", k)
		}
	}
}

func TestRepoPickerRendersCleanly(t *testing.T) {
	for _, scope := range []string{"", "acme/device-gateway"} {
		for _, typed := range []string{"", "other", "acme/api"} {
			m := testModel(t, 200, 30, ViewBoard)
			m.scope = scope
			m.rebuild()
			m = openPicker(m)
			m = typePicker(m, typed)

			panel := m.renderRepoPicker()
			plain := stripANSI(panel)
			if strings.ContainsRune(plain, '�') {
				t.Fatalf("scope=%q typed=%q produced a broken rune:\n%s", scope, typed, plain)
			}
			for _, line := range strings.Split(panel, "\n") {
				if w := lipglossWidth(line); w > pickerWidth {
					t.Fatalf("scope=%q typed=%q: line is %d wide:\n%q", scope, typed, w, line)
				}
			}
		}
	}
}

func TestRepoPickerMarksTheActiveScope(t *testing.T) {
	m := testModel(t, 200, 30, ViewBoard)
	m.scope = "acme/device-gateway"
	m.rebuild()
	m = openPicker(m)

	entries := m.repoEntries()
	active := -1
	for i, e := range entries {
		if e.repo == m.scope {
			active = i
		}
	}
	if active < 0 {
		t.Fatal("the active repository is not listed")
	}

	m.repoPick.idx = 0
	plain := stripANSI(m.renderRepoPicker())
	if strings.Count(plain, m.theme.Glyphs.Pass) != 1 {
		t.Fatalf("exactly one row should be marked active:\n%s", plain)
	}
}

func TestRepoNamesDedupeIgnoringCase(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m.repos = []string{"acme/timesheet", "acme/payments-api"}
	m.prs = append(m.prs, model.PullRequest{
		Repo: "acme/Timesheet", Number: 5, Title: "t", Author: "a",
		CreatedAt: nowMinusDays(1), UpdatedAt: nowMinusDays(1),
	})

	names := m.repoNames()
	var seen int
	for _, n := range names {
		if strings.EqualFold(n, "acme/timesheet") {
			seen++
			if n != "acme/Timesheet" {
				t.Errorf("the switcher should prefer github's spelling, got %q", n)
			}
		}
	}
	if seen != 1 {
		t.Fatalf("timesheet appears %d times in %v", seen, names)
	}
}

func TestTrackedReposAreCanonicalisedFromGithub(t *testing.T) {
	fixed := canonicalRepos(
		[]string{"acme/timesheet", "acme/TIMESHEET", "acme/other"},
		[]model.PullRequest{{Repo: "acme/Timesheet"}},
	)
	if len(fixed) != 2 {
		t.Fatalf("duplicates were not merged: %v", fixed)
	}
	if fixed[0] != "acme/Timesheet" {
		t.Fatalf("casing was not corrected: %v", fixed)
	}
}

func TestSettingsDedupeReposIgnoringCase(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	s := config.DefaultSettings()
	s.Repos = []string{"acme/timesheet", "acme/Timesheet", "acme/other"}
	if err := config.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	if got := config.LoadSettings().Repos; len(got) != 2 {
		t.Fatalf("a settings file with case duplicates should heal, got %v", got)
	}
}

func TestScopeMatchesIgnoringCase(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m.scope = "acme/DEVICE-GATEWAY"
	if !m.inScope("acme/device-gateway") {
		t.Fatal("scope should match regardless of case")
	}
}

func TestRepoPickerRemovesARepo(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m.settings.Repos = []string{"acme/payments-api", "acme/device-gateway"}
	m.scope = "acme/device-gateway"

	m = openPicker(m)
	m = typePicker(m, "device")
	entries := m.repoEntries()
	if len(entries) == 0 {
		t.Fatal("nothing to remove")
	}

	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = out.(Model)

	if containsFold(m.settings.Repos, "acme/device-gateway") {
		t.Fatalf("the repository was not removed: %v", m.settings.Repos)
	}
	if m.scope != "" {
		t.Fatalf("removing the scoped repository should clear the scope, got %q", m.scope)
	}
}

func TestRepoPickerKeepsTheLastRepo(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m.settings.Repos = []string{"acme/payments-api"}

	m = openPicker(m)
	m = typePicker(m, "payments")
	out, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = out.(Model)

	if len(m.settings.Repos) != 1 {
		t.Fatal("the last repository must stay tracked")
	}
	if cmd == nil {
		t.Fatal("expected an explanation toast")
	}
}

func TestRepoPickerShowsRemoveHint(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m = openPicker(m)
	if !strings.Contains(stripANSI(m.renderRepoPicker()), "d remove") {
		t.Fatal("the picker does not advertise removal")
	}
}

func TestCloseAsksBeforeClosing(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	pr, ok := m.selectedPR()
	if !ok {
		t.Fatal("nothing selected")
	}

	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}})
	m = out.(Model)

	kind, open := m.overlay()
	if !open || kind != ovConfirm {
		t.Fatal("X should ask first")
	}
	if !strings.Contains(m.confirm.title, itoa(pr.Number)) {
		t.Fatalf("the confirm names the wrong pull request: %q", m.confirm.title)
	}
	if !strings.Contains(m.confirm.body, pr.HeadRef) || !strings.Contains(m.confirm.body, "kept") {
		t.Fatalf("the confirm must say the branch is kept: %q", m.confirm.body)
	}

	panel := stripANSI(m.renderConfirm())
	if !strings.Contains(panel, "the branch is kept") {
		t.Fatalf("truncation ate the guarantee:\n%s", panel)
	}
}

func TestCloseCancelDoesNothing(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	before := len(m.prs)

	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}})
	m = out.(Model)
	out, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = out.(Model)

	if _, open := m.overlay(); open {
		t.Fatal("n should dismiss the confirm")
	}
	if cmd != nil {
		t.Fatal("n must not run anything")
	}
	if len(m.prs) != before {
		t.Fatal("nothing should have been closed")
	}
}

func TestCloseRemovesThePullRequest(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	pr, _ := m.selectedPR()
	before := len(m.prs)

	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}})
	m = out.(Model)
	out, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = out.(Model)
	if cmd == nil {
		t.Fatal("confirming should run the close")
	}

	msg, ok := cmd().(actionMsg)
	if !ok || msg.err != nil || msg.verb != "closed" {
		t.Fatalf("unexpected result: %#v", cmd())
	}

	out, _ = m.Update(msg)
	m = out.(Model)
	if len(m.prs) != before-1 {
		t.Fatalf("the closed pull request is still listed: %d of %d", len(m.prs), before)
	}
	for _, p := range m.prs {
		if p.Key() == pr.Key() {
			t.Fatalf("#%d survived the close", pr.Number)
		}
	}
}

func TestCloseIsOfferedInTheFooterAndDetail(t *testing.T) {
	m := testModel(t, 220, 40, ViewBoard)
	pr, _ := m.selectedPR()

	if !strings.Contains(stripANSI(m.renderFooter()), "close") {
		t.Error("the footer does not offer close")
	}
	if !strings.Contains(stripANSI(m.actionChips(pr, 160)), "close") {
		t.Error("the detail pane does not offer close")
	}
}
