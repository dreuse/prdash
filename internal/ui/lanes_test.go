package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dreuse/prdash/internal/config"
	"github.com/dreuse/prdash/internal/model"
)

func customLaneModel(t *testing.T, lanes ...config.Lane) Model {
	t.Helper()
	t.Cleanup(func() { model.SetLanes(nil) })
	m := testModel(t, 120, 40, ViewBoard)
	m.settings.LaneOrder = config.LaneOrderCustom
	m.settings.Lanes = lanes
	m.rebuild()
	return m
}

func submitLaneEdit(m Model, value string) Model {
	m.panel.input.SetValue(value)
	out, _ := m.handleSettingsEdit(tea.KeyMsg{Type: tea.KeyEnter}, m.settingsFields())
	return out.(Model)
}

func laneNames(m Model) []string {
	out := make([]string, 0, len(m.order))
	for _, c := range m.order {
		out = append(out, c.String())
	}
	return out
}

func fieldIndex(m Model, label string) int {
	for i, f := range m.settingsFields() {
		if f.label == label {
			return i
		}
	}
	return -1
}

func TestAddLaneFromTheSettingsPanel(t *testing.T) {
	m := customLaneModel(t)
	m.push(ovSettings)

	idx := fieldIndex(m, "add lane…")
	if idx < 0 {
		t.Fatal("custom lane order must offer an add lane field")
	}
	m.panel.idx = idx
	out, _ := m.cycleField(m.settingsFields(), 1)
	m = out.(Model)
	if !m.panel.editing {
		t.Fatal("enter on add lane… must open the editor")
	}

	m = submitLaneEdit(m, "MERGE NOW | is:ready")
	m = submitLaneEdit(setEditing(m, fieldIndex(m, "add lane…")), "ON ME | reviewer:@me")

	want := []string{"MERGE NOW", "ON ME", model.OtherLaneName}
	if got := laneNames(m); !sameStrings(got, want) {
		t.Fatalf("board lanes = %v, want %v", got, want)
	}
	if len(m.lanes[0]) == 0 {
		t.Error("the mock data has ready pull requests, MERGE NOW should not be empty")
	}
}

func setEditing(m Model, idx int) Model {
	m.panel.idx = idx
	m.panel.editing = true
	return m
}

func TestAddLaneRejectsMalformedInput(t *testing.T) {
	m := customLaneModel(t)
	m.push(ovSettings)

	for _, bad := range []string{"no separator", "| is:ready", "NAME |", ""} {
		m = submitLaneEdit(setEditing(m, fieldIndex(m, "add lane…")), bad)
		if len(m.settings.Lanes) != 0 {
			t.Fatalf("%q must not create a lane, got %v", bad, m.settings.Lanes)
		}
	}
}

func TestLanesMoveAndRemove(t *testing.T) {
	m := customLaneModel(t,
		config.Lane{Name: "FIRST", Rule: "is:draft"},
		config.Lane{Name: "SECOND", Rule: "is:conflict"},
		config.Lane{Name: "THIRD", Rule: model.CatchAllRule},
	)
	m.push(ovSettings)
	m.panel.idx = fieldIndex(m, "FIRST")

	out, _ := m.handleSettingsKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}})
	m = out.(Model)
	if got := laneNames(m); got[0] != "SECOND" || got[1] != "FIRST" {
		t.Fatalf("J must move the lane down, got %v", got)
	}
	if m.settingsFields()[m.panel.idx].label != "FIRST" {
		t.Error("the cursor must follow the lane it moved")
	}

	out, _ = m.handleSettingsKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'K'}})
	m = out.(Model)
	if got := laneNames(m); got[0] != "FIRST" || got[1] != "SECOND" {
		t.Fatalf("K must move the lane back up, got %v", got)
	}

	m.panel.idx = fieldIndex(m, "SECOND")
	out, _ = m.handleSettingsKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = out.(Model)
	if got := laneNames(m); len(got) != 2 || got[0] != "FIRST" || got[1] != "THIRD" {
		t.Fatalf("d must remove the focused lane, got %v", got)
	}
}

func TestEveryPullRequestLandsInALane(t *testing.T) {
	m := customLaneModel(t,
		config.Lane{Name: "DRAFTS", Rule: "is:draft"},
		config.Lane{Name: "REST", Rule: "author:nobody-at-all"},
	)
	counted := 0
	for _, prs := range m.lanes {
		counted += len(prs)
	}
	if counted != m.countAll() {
		t.Fatalf("lanes hold %d pull requests but %d are visible", counted, m.countAll())
	}
	if len(m.lanes[1]) != 0 {
		t.Error("a lane whose rule matches nothing must stay empty")
	}
	if len(m.lanes[2]) == 0 {
		t.Errorf("the %s lane must catch whatever no rule claimed", model.OtherLaneName)
	}
}

func TestOneNarrowLaneKeepsTheRestSeparate(t *testing.T) {
	m := customLaneModel(t, config.Lane{Name: "GOIAS", Rule: `label:"Solicitante: SEAD-GO"`})

	if got := laneNames(m); !sameStrings(got, []string{"GOIAS", model.OtherLaneName}) {
		t.Fatalf("board lanes = %v, want the lane plus %s", got, model.OtherLaneName)
	}
	if len(m.lanes[1]) != m.countAll() {
		t.Errorf("no pull request carries that label, all %d belong in %s, got %d",
			m.countAll(), model.OtherLaneName, len(m.lanes[1]))
	}
}

func TestInvalidLaneRuleIsShownButNeverMatches(t *testing.T) {
	m := customLaneModel(t,
		config.Lane{Name: "BROKEN", Rule: "state:blocked"},
		config.Lane{Name: "REST", Rule: model.CatchAllRule},
	)
	if len(m.lanes[0]) != 0 {
		t.Error("a rule that cannot be evaluated must not claim pull requests")
	}

	m.push(ovSettings)
	m.panel.idx = fieldIndex(m, "BROKEN")
	if !strings.Contains(stripANSI(m.renderSettings()), "state:blocked") {
		t.Error("the panel must still show the rule so it can be fixed")
	}
}

func TestLaneColorOverridesThePalette(t *testing.T) {
	m := customLaneModel(t,
		config.Lane{Name: "FIRST", Rule: "is:draft", Color: "red"},
		config.Lane{Name: "SECOND", Rule: model.CatchAllRule},
	)
	_ = m

	if got := laneTone(0); got != toneDanger {
		t.Errorf("an explicit colour must win, got %v", got)
	}
	if got := laneTone(1); got != lanePalette[1] {
		t.Errorf("a lane with no colour falls back to the palette, got %v", got)
	}
}

func TestLaneColorFallsBackWhenUnknown(t *testing.T) {
	customLaneModel(t, config.Lane{Name: "ONLY", Rule: model.CatchAllRule, Color: "chartreuse"})
	if got := laneTone(0); got != lanePalette[0] {
		t.Errorf("an unknown colour must fall back to the palette, got %v", got)
	}
}

func TestLaneSortOverridesTheBoardSort(t *testing.T) {
	m := customLaneModel(t,
		config.Lane{Name: "BY NUMBER", Rule: model.CatchAllRule, Sort: model.SortNumber.String()},
	)
	lane := m.lanes[0]
	if len(lane) < 3 {
		t.Fatalf("need a few pull requests to compare orderings, got %d", len(lane))
	}

	byNumber := append([]model.PullRequest(nil), lane...)
	model.Sort(byNumber, model.SortNumber, m.viewer, m.policy.RequiredApprovals)
	byUrgency := append([]model.PullRequest(nil), lane...)
	model.Sort(byUrgency, model.SortUrgency, m.viewer, m.policy.RequiredApprovals)

	if sameOrder(byNumber, byUrgency) {
		t.Skip("the mock data sorts the same either way, nothing to prove")
	}
	if !sameOrder(lane, byNumber) {
		t.Error("the lane must use its own sort, not the board sort")
	}
}

func TestLaneWithoutASortFollowsTheBoard(t *testing.T) {
	m := customLaneModel(t, config.Lane{Name: "EVERYTHING", Rule: model.CatchAllRule})
	expected := append([]model.PullRequest(nil), m.lanes[0]...)
	model.Sort(expected, m.sortMode, m.viewer, m.policy.RequiredApprovals)
	if !sameOrder(m.lanes[0], expected) {
		t.Error("a lane with no sort of its own must follow the board sort")
	}
}

func sameOrder(a, b []model.PullRequest) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Key() != b[i].Key() {
			return false
		}
	}
	return true
}

func TestSettingsCyclesLaneColorAndSort(t *testing.T) {
	m := customLaneModel(t, config.Lane{Name: "ONLY", Rule: model.CatchAllRule})
	m.push(ovSettings)
	m.panel.idx = fieldIndex(m, "ONLY")

	out, _ := m.handleSettingsKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = out.(Model)
	if m.settings.Lanes[0].Color != laneColors[1] {
		t.Errorf("c must cycle the colour, got %q", m.settings.Lanes[0].Color)
	}

	out, _ = m.handleSettingsKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = out.(Model)
	if m.settings.Lanes[0].Sort != laneSorts[1] {
		t.Errorf("s must cycle the sort, got %q", m.settings.Lanes[0].Sort)
	}

	for range laneColors {
		out, _ = m.handleSettingsKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
		m = out.(Model)
	}
	if m.settings.Lanes[0].Color != laneColors[1] {
		t.Errorf("cycling the whole palette must come back around, got %q", m.settings.Lanes[0].Color)
	}
}

func TestColorAndSortKeysIgnoreNonLaneFields(t *testing.T) {
	m := customLaneModel(t, config.Lane{Name: "ONLY", Rule: model.CatchAllRule})
	m.push(ovSettings)
	m.panel.idx = fieldIndex(m, "Theme")

	before := m.settings
	out, _ := m.handleSettingsKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if got := out.(Model).settings.Theme; got != before.Theme {
		t.Errorf("c must do nothing outside a lane field, theme became %q", got)
	}
}

func TestBuiltinLanesSurviveALaneOrderRoundTrip(t *testing.T) {
	m := customLaneModel(t, config.Lane{Name: "EVERYTHING", Rule: model.CatchAllRule})
	before := len(m.lanes)

	m.settings.LaneOrder = config.LaneOrderReady
	m.rebuild()

	if got := laneNames(m); len(got) != len(model.ActionFirstColumns) || got[0] != "READY TO MERGE" {
		t.Fatalf("switching back to ready must restore the built-in lanes, got %v", got)
	}
	if before == len(m.lanes) {
		t.Error("the custom board and the built-in board should not have the same lane count here")
	}
}
