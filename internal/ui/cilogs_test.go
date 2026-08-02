package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/dreuse/prdash/internal/model"
)

func loadedLogs(t *testing.T, m Model) Model {
	t.Helper()
	m = send(m, "L")
	if !m.logs.open {
		t.Fatal("L did not open the log pane")
	}
	out, _ := m.Update(logLoadMsg{gen: m.logs.gen})
	m = out.(Model)

	run := m.logs.run
	lines, err := m.actor.RunLog(t.Context(), run, m.logs.failedOnly)
	if err != nil {
		t.Fatalf("the mock refused to produce logs: %v", err)
	}
	out, _ = m.Update(logsMsg{key: m.logs.key(), lines: lines})
	return out.(Model)
}

func TestLogPaneOpensAndClosesOnL(t *testing.T) {
	m := testModel(t, 200, 60, ViewCI)

	m = send(m, "L")
	if !m.logs.open {
		t.Fatal("L should open the log pane")
	}
	m = send(m, "L")
	if m.logs.open {
		t.Fatal("L should close the log pane again")
	}
}

func TestEscapeClosesTheLogPaneFromTheTable(t *testing.T) {
	m := testModel(t, 200, 60, ViewCI)
	m = send(m, "L")
	if m.logs.focus {
		t.Fatal("the table keeps focus when the pane opens")
	}

	m = send(m, "esc")
	if m.logs.open {
		t.Fatal("esc should close the split even while the table is focused")
	}
}

func TestClosingTheLogPaneReturnsFocusToTheTable(t *testing.T) {
	m := testModel(t, 200, 60, ViewCI)
	m = send(m, "L", "tab")
	if !m.logs.focus {
		t.Fatal("tab should focus the log pane")
	}

	m = send(m, "esc")
	if m.logs.open || m.logs.focus {
		t.Fatal("focus must not be stranded on a hidden pane")
	}
}

func TestTabDoesNothingWhileTheSplitIsClosed(t *testing.T) {
	m := testModel(t, 200, 60, ViewCI)

	m = send(m, "tab")
	if m.logs.focus || m.logs.open {
		t.Fatal("tab should be inert with no log pane on screen")
	}
}

func TestTabTogglesFocusBothWays(t *testing.T) {
	m := testModel(t, 200, 60, ViewCI)

	m = send(m, "L", "tab")
	if !m.logs.focus {
		t.Fatal("tab should move focus to the log")
	}
	m = send(m, "tab")
	if m.logs.focus {
		t.Fatal("tab should move focus back to the table")
	}
}

func TestArrowsScrollTheLogAndLeaveTheCursorAlone(t *testing.T) {
	m := loadedLogs(t, testModel(t, 200, 60, ViewCI))
	m = send(m, "tab", "home")

	row, scroll := m.ciRow, m.logs.scroll
	m = send(m, "down")

	if m.ciRow != row {
		t.Errorf("scrolling the log moved the table cursor from %d to %d", row, m.ciRow)
	}
	if m.logs.scroll <= scroll {
		t.Errorf("the down arrow did not scroll the log: %d then %d", scroll, m.logs.scroll)
	}
}

func TestLogScrollClampsAtBothEnds(t *testing.T) {
	m := loadedLogs(t, testModel(t, 200, 60, ViewCI))
	m = send(m, "tab")

	for i := 0; i < 500; i++ {
		m = send(m, "up")
	}
	if m.logs.scroll != 0 {
		t.Errorf("scrolling up past the start should stop at 0, got %d", m.logs.scroll)
	}

	for i := 0; i < 500; i++ {
		m = send(m, "down")
	}
	if m.logs.scroll > len(m.logs.lines) {
		t.Errorf("scroll ran past the end: %d of %d", m.logs.scroll, len(m.logs.lines))
	}
}

func TestPageKeysScrollTheFocusedPaneOnly(t *testing.T) {
	m := loadedLogs(t, testModel(t, 200, 60, ViewCI))
	m = send(m, "tab", "home")

	row := m.ciRow
	m = send(m, "pgdown")
	if m.logs.scroll <= 1 {
		t.Errorf("pgdn should page the log, got scroll %d", m.logs.scroll)
	}
	if m.ciRow != row {
		t.Error("pgdn moved the table cursor while the log was focused")
	}
}

func TestPageKeysMoveTheTableWhenItIsFocused(t *testing.T) {
	m := testModel(t, 200, 60, ViewCI)
	if len(m.ciRows()) < 3 {
		t.Skip("needs a few runs")
	}

	before := m.ciRow
	m = send(m, "pgdown")
	if m.ciRow <= before {
		t.Errorf("pgdn should page the table, got %d then %d", before, m.ciRow)
	}
}

func TestRunningRunsSayLogsAreNotReady(t *testing.T) {
	m := testModel(t, 200, 60, ViewCI)
	m.runs = []model.WorkflowRun{{
		ID: 9300, Repo: "acme/payments-api", Name: "CI", Branch: "wip",
		Status:    "in_progress",
		StartedAt: time.Now().Add(-2 * time.Minute),
		UpdatedAt: time.Now().Add(-2 * time.Minute),
	}}
	m.rebuild()

	m = send(m, "L")
	if m.logs.err == nil {
		t.Fatal("a run still going has no logs to show and must say so")
	}
	if m.logs.loading {
		t.Error("no fetch should be started for a run that cannot have logs yet")
	}
	if !strings.Contains(stripANSI(m.View()), "finishes") {
		t.Error("the pane should explain why there is nothing to read")
	}
}

func TestACachedRunIsRenderedWithoutFetching(t *testing.T) {
	m := loadedLogs(t, testModel(t, 200, 60, ViewCI))
	cached := m.logs.run

	m = send(m, "L")
	m = send(m, "L")

	if m.logs.run.ID != cached.ID {
		t.Fatalf("reopened on a different run: %d then %d", cached.ID, m.logs.run.ID)
	}
	if m.logs.loading {
		t.Error("a cached run should not trigger another fetch")
	}
	if len(m.logs.lines) == 0 {
		t.Error("the cached lines were not restored")
	}
}

func TestAStaleLoadTickIsIgnored(t *testing.T) {
	m := testModel(t, 200, 60, ViewCI)
	m = send(m, "L")

	out, cmd := m.Update(logLoadMsg{gen: m.logs.gen - 1})
	m = out.(Model)
	if cmd != nil {
		t.Error("a superseded debounce tick must not start a fetch")
	}
}

func TestLogsForAnotherRunAreCachedButNotShown(t *testing.T) {
	m := loadedLogs(t, testModel(t, 200, 60, ViewCI))
	shown := m.logs.run.ID

	out, _ := m.Update(logsMsg{key: logKey{run: shown + 12345}, lines: []string{"unrelated"}})
	m = out.(Model)

	if len(m.logs.lines) == 1 && m.logs.lines[0] == "unrelated" {
		t.Fatal("a late reply for another run overwrote the visible log")
	}
}

func TestTrendsAreHiddenWhileTheSplitIsOpen(t *testing.T) {
	m := testModel(t, 200, 60, ViewCI)
	if !strings.Contains(stripANSI(m.View()), "TRENDS") {
		t.Fatal("trends should be visible with the split closed")
	}

	m = loadedLogs(t, m)
	if strings.Contains(stripANSI(m.View()), "TRENDS") {
		t.Error("the split and the trend section should not share the screen")
	}
}

func TestLogPaneShowsTheRunItIsBoundTo(t *testing.T) {
	m := loadedLogs(t, testModel(t, 200, 60, ViewCI))
	out := stripANSI(m.View())

	if !strings.Contains(out, "LOGS") {
		t.Fatal("the log pane has no header")
	}
	if !strings.Contains(out, truncate(m.logs.run.Branch, logRefWidth)) {
		t.Errorf("the header does not name the branch %q:\n%s", m.logs.run.Branch, out)
	}
}

func TestMovingTheCursorRebindsTheLogPane(t *testing.T) {
	m := testModel(t, 200, 60, ViewCI)
	if len(m.ciRows()) < 2 {
		t.Skip("needs at least two runs")
	}

	m = send(m, "L")
	first := m.logs.run.ID

	m = send(m, "down")
	if m.logs.run.ID == first {
		t.Error("the log pane should follow the table cursor")
	}
	if m.logs.run.ID != m.ciRows()[m.ciRow].ID {
		t.Error("the pane is bound to a run that is not selected")
	}
}

func failingRunModel(t *testing.T) Model {
	t.Helper()
	m := testModel(t, 200, 60, ViewCI)
	for i, r := range m.ciRows() {
		if r.Failed() {
			m.ciRow = i
			m.ciSel = r.ID
			return m
		}
	}
	t.Fatal("the mock should contain a failed run")
	return m
}

func TestLogPaneOpensAFailedRunOnItsFailingSteps(t *testing.T) {
	m := loadedLogs(t, failingRunModel(t))

	if !m.logs.failedOnly {
		t.Fatal("a failed run should open showing only what failed")
	}
	if !strings.Contains(stripANSI(m.View()), "failing steps") {
		t.Error("the pane should name the mode it is in")
	}
}

func TestLogPaneOpensASuccessfulRunOnTheFullLog(t *testing.T) {
	m := testModel(t, 200, 60, ViewCI)
	for i, r := range m.ciRows() {
		if r.Succeeded() {
			m.ciRow, m.ciSel = i, r.ID
			break
		}
	}
	m = loadedLogs(t, m)

	if m.logs.failedOnly {
		t.Fatal("a run with no failing steps should open on the whole log")
	}
	if !strings.Contains(stripANSI(m.View()), "full log") {
		t.Error("the pane should name the mode it is in")
	}
}

func TestFSwitchesTheLogBetweenFailingStepsAndTheFullLog(t *testing.T) {
	m := loadedLogs(t, failingRunModel(t))
	m = send(m, "tab")

	narrow := len(m.logs.lines)
	if narrow == 0 {
		t.Fatal("the failing steps should have produced lines")
	}

	m = send(m, "f")
	m = pumpAfterModeSwitch(t, m)
	if !strings.Contains(stripANSI(m.View()), "full log") {
		t.Fatal("f should switch to the full log")
	}
	if len(m.logs.lines) <= narrow {
		t.Errorf("the full log should carry more than the failing steps: %d then %d",
			narrow, len(m.logs.lines))
	}

	m = send(m, "f")
	if !m.logs.failedOnly {
		t.Fatal("f should switch back to the failing steps")
	}
}

func TestFDoesNotFilterTheTableWhileTheLogIsFocused(t *testing.T) {
	m := loadedLogs(t, failingRunModel(t))
	before := m.ciFailuresOnly

	m = send(m, "tab", "f")

	if m.ciFailuresOnly != before {
		t.Error("f belongs to the log pane while the log has focus")
	}
}

func TestFStillFiltersTheTableWhileTheTableIsFocused(t *testing.T) {
	m := loadedLogs(t, failingRunModel(t))
	before := m.ciFailuresOnly

	m = send(m, "f")

	if m.ciFailuresOnly == before {
		t.Error("with the table focused f should still filter the table")
	}
}

func TestEachLogModeIsCachedSeparately(t *testing.T) {
	m := loadedLogs(t, failingRunModel(t))
	m = send(m, "tab", "f")
	m = pumpAfterModeSwitch(t, m)
	full := len(m.logs.lines)

	m = send(m, "f")
	if m.logs.loading {
		t.Error("switching back to a mode already fetched should not refetch")
	}
	m = send(m, "f")
	if m.logs.loading {
		t.Error("switching forward again should come from the cache")
	}
	if len(m.logs.lines) != full {
		t.Errorf("the cached full log changed size: %d then %d", full, len(m.logs.lines))
	}
}

func pumpAfterModeSwitch(t *testing.T, m Model) Model {
	t.Helper()
	if !m.logs.loading {
		return m
	}
	out, _ := m.Update(logLoadMsg{gen: m.logs.gen})
	m = out.(Model)
	lines, err := m.actor.RunLog(t.Context(), m.logs.run, m.logs.failedOnly)
	if err != nil {
		t.Fatalf("mock logs: %v", err)
	}
	out, _ = m.Update(logsMsg{key: m.logs.key(), lines: lines})
	return out.(Model)
}

func TestTheFooterOffersFOnlyOnceAndForTheLivePane(t *testing.T) {
	m := loadedLogs(t, failingRunModel(t))

	tableFocused := stripANSI(m.renderFooter())
	if strings.Count(tableFocused, "f failures only") != 1 {
		t.Errorf("with the table focused f should filter the table:\n%s", tableFocused)
	}
	if strings.Contains(tableFocused, "f failing steps") || strings.Contains(tableFocused, "f full log") {
		t.Errorf("the log mode is not what f does while the table has focus:\n%s", tableFocused)
	}

	logFocused := stripANSI(send(m, "tab").renderFooter())
	if strings.Contains(logFocused, "failures only") {
		t.Errorf("f no longer filters the table once the log has focus:\n%s", logFocused)
	}
	if strings.Count(logFocused, "f failing steps") != 1 {
		t.Errorf("the footer should name the log mode exactly once:\n%s", logFocused)
	}
}
