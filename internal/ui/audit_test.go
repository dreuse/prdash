package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dreuse/prdash/internal/config"
	"github.com/dreuse/prdash/internal/model"
)

func addRepoField(t *testing.T, m Model) settingsField {
	t.Helper()
	for _, f := range m.settingsFields() {
		if f.kind == fieldText && f.section == "REPOSITORIES" {
			return f
		}
	}
	t.Fatal("the settings panel has no add-repository field")
	return settingsField{}
}

func TestSettingsRefusesARepositoryThatWouldBrickTheNextStart(t *testing.T) {
	m := testModel(t, 120, 40, ViewBoard)
	field := addRepoField(t, m)

	before := len(m.settings.Repos)
	for _, bad := range []string{"typo-no-slash", "acme/api/../../users", "  ", "/api", "acme/"} {
		field.set(&m.settings, bad)
		if len(m.settings.Repos) != before {
			t.Fatalf("%q was accepted and would fail ParseRepos on the next start: %v",
				bad, m.settings.Repos)
		}
	}

	field.set(&m.settings, "acme/new-service")
	if len(m.settings.Repos) != before+1 {
		t.Fatalf("a valid repository must still be added, got %v", m.settings.Repos)
	}
}

func TestSettingsOffersTheAddKeyItAdvertises(t *testing.T) {
	m := testModel(t, 120, 40, ViewBoard)
	m.push(ovSettings)

	if !strings.Contains(stripANSI(m.renderSettings()), "a add") {
		t.Skip("the panel no longer advertises an add key")
	}

	out, _ := m.handleSettingsKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	got := out.(Model)
	if !got.panel.editing {
		t.Error("the advertised a key must open the add-repository field")
	}
	if field := got.settingsFields()[got.panel.idx]; field.kind != fieldText || field.section != "REPOSITORIES" {
		t.Errorf("a must land on the add-repository field, landed on %q", field.label)
	}
}

func TestSettingsRemovesARepositoryWhateverTheCase(t *testing.T) {
	m := testModel(t, 120, 40, ViewBoard)
	m.settings.Repos = []string{"Acme/Payments-API", "acme/device-gateway"}

	out, _ := m.removeRepo([]settingsField{{kind: fieldRepo, repo: "acme/payments-api"}})
	got := out.(Model)

	if len(got.settings.Repos) != 1 || got.settings.Repos[0] != "acme/device-gateway" {
		t.Errorf("github repository names are case insensitive, got %v", got.settings.Repos)
	}
}

func TestSpinnerStopsTickingWhenNothingIsPending(t *testing.T) {
	m := testModel(t, 120, 40, ViewBoard)
	m.loading = false

	if _, cmd := m.Update(spinnerMsg{}); cmd != nil {
		t.Error("an idle dashboard must not re-render eight times a second")
	}

	m.loading = true
	if _, cmd := m.Update(spinnerMsg{}); cmd == nil {
		t.Error("a loading dashboard still needs the spinner")
	}

	m.loading = false
	m.pending[model.Key{Repo: "acme/api", Number: 1}] = "merging"
	if _, cmd := m.Update(spinnerMsg{}); cmd == nil {
		t.Error("a pending action still needs the spinner")
	}
}

func TestRefreshRestartsTheSpinner(t *testing.T) {
	m := testModel(t, 120, 40, ViewBoard)
	m.loading = false
	if _, cmd := m.Update(spinnerMsg{}); cmd != nil {
		t.Fatal("precondition: the spinner should be stopped")
	}

	out, _ := m.refresh()
	if _, cmd := out.(Model).Update(spinnerMsg{}); cmd == nil {
		t.Error("a refresh must bring the spinner back")
	}
}

func TestLargePullRequestNumbersRender(t *testing.T) {
	m := testModel(t, 120, 40, ViewBoard)
	for i := range m.prs {
		m.prs[i].Number = 123456789
	}
	m.sel = model.Key{}
	m.rebuild()

	if !strings.Contains(stripANSI(m.View()), "123456789") {
		t.Error("a nine digit pull request number must render")
	}
}

func TestCIMoreCounterOnlyCountsRunsBelowTheWindow(t *testing.T) {
	m := testModel(t, 120, 14, ViewCI)
	rows := m.ciRows()
	if len(rows) < 4 {
		t.Skipf("need more runs to scroll, got %d", len(rows))
	}

	m.ciRow = len(rows) - 1
	m.ciSel = rows[len(rows)-1].ID
	out := stripANSI(m.View())

	if strings.Contains(out, "+"+itoa(len(rows))+" more") {
		t.Errorf("scrolled to the last run, nothing is hidden below it:\n%s", out)
	}
}

func TestLogCacheStopsGrowing(t *testing.T) {
	m := testModel(t, 120, 40, ViewCI)
	lines := []string{"one", "two"}

	for i := 0; i < maxCachedLogs*3; i++ {
		out, _ := m.applyLogs(logsMsg{key: logKey{run: int64(i)}, lines: lines})
		m = out.(Model)
	}
	if len(m.logs.cache) > maxCachedLogs {
		t.Errorf("the log cache holds %d runs, want at most %d", len(m.logs.cache), maxCachedLogs)
	}
}

func TestCardHeightAgreesWithTheRenderedCard(t *testing.T) {
	m := testModel(t, 180, 50, ViewBoard)
	busy := false

	for _, col := range m.order {
		for _, pr := range m.lanes[col] {
			for _, selected := range []bool{false, true} {
				if !busy {
					m.pending[pr.Key()] = "merging"
					busy = true
				}
				card := m.renderCard(pr, col, 40, selected)
				if got, want := m.cardHeight(pr, col), lipgloss.Height(card); got != want {
					t.Fatalf("#%d in %s: cardHeight says %d, the card is %d lines:\n%s",
						pr.Number, col, got, want, stripANSI(card))
				}
			}
			delete(m.pending, pr.Key())
			busy = false
		}
	}
}

func TestLanesOnlyRenderWhatFits(t *testing.T) {
	m := testModel(t, 180, 24, ViewBoard)
	col := m.order[0]

	crowd := make([]model.PullRequest, 0, 200)
	for i := 0; i < 200; i++ {
		pr := m.lanes[col][0]
		pr.Number = 5000 + i
		crowd = append(crowd, pr)
	}
	m.lanes[col] = crowd

	rendered := strings.Count(stripANSI(m.renderLane(col, 40, 20)), "#5")
	if rendered > 40 {
		t.Errorf("a 20 row lane rendered %d cards from a 200 card column", rendered)
	}
}

func TestMultiRepoIsResolvedOncePerRebuild(t *testing.T) {
	m := testModel(t, 120, 40, ViewBoard)
	if !m.multi {
		t.Fatal("the fixture tracks two repositories")
	}

	m.prs = m.prs[:0]
	m.rebuild()
	if m.multi {
		t.Error("an empty board is not multi repo")
	}
}

func TestCachedCIRowsMatchTheLiveOnes(t *testing.T) {
	m := testModel(t, 120, 40, ViewCI)
	live := m.visibleRuns()
	cached := m.ciRows()

	if len(live) != len(cached) {
		t.Fatalf("cached rows drifted: %d live, %d cached", len(live), len(cached))
	}

	m.ciFailuresOnly = true
	m.rebuild()
	for _, r := range m.ciRows() {
		if !r.Failed() {
			t.Errorf("failures-only must not serve a stale row: %s", r.Name)
		}
	}
}

func TestSettingsSurviveARoundTripThroughTheStore(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	s := config.DefaultSettings()
	s.Repos = []string{"acme/api"}
	if err := config.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	if got := config.LoadSettings(); len(got.Repos) != 1 || got.Repos[0] != "acme/api" {
		t.Fatalf("tightened permissions must not break reading, got %v", got.Repos)
	}
}
