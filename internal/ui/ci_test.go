package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dreuse/prdash/internal/model"
)

func hasRun(runs []model.WorkflowRun, id int64) bool {
	for _, r := range runs {
		if r.ID == id {
			return true
		}
	}
	return false
}

func send(m Model, keys ...string) Model {
	for _, k := range keys {
		var msg tea.KeyMsg
		switch k {
		case "tab":
			msg = tea.KeyMsg{Type: tea.KeyTab}
		case "down":
			msg = tea.KeyMsg{Type: tea.KeyDown}
		case "up":
			msg = tea.KeyMsg{Type: tea.KeyUp}
		case "end":
			msg = tea.KeyMsg{Type: tea.KeyEnd}
		case "home":
			msg = tea.KeyMsg{Type: tea.KeyHome}
		case "pgdown":
			msg = tea.KeyMsg{Type: tea.KeyPgDown}
		case "pgup":
			msg = tea.KeyMsg{Type: tea.KeyPgUp}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEscape}
		case "ctrl+d":
			msg = tea.KeyMsg{Type: tea.KeyCtrlD}
		case "ctrl+u":
			msg = tea.KeyMsg{Type: tea.KeyCtrlU}
		case "ctrl+f":
			msg = tea.KeyMsg{Type: tea.KeyCtrlF}
		case "ctrl+b":
			msg = tea.KeyMsg{Type: tea.KeyCtrlB}
		case "ctrl+e":
			msg = tea.KeyMsg{Type: tea.KeyCtrlE}
		case "ctrl+y":
			msg = tea.KeyMsg{Type: tea.KeyCtrlY}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		out, _ := m.Update(msg)
		m = out.(Model)
	}
	return m
}

func TestCITableKeepsRunningRunsHoweverOld(t *testing.T) {
	m := testModel(t, 200, 60, ViewCI)
	m.runs = append(m.runs, model.WorkflowRun{
		ID: 9100, Repo: "acme/payments-api", Name: "CI", Branch: "ancient",
		Status:    "in_progress",
		StartedAt: time.Now().Add(-90 * 24 * time.Hour),
		UpdatedAt: time.Now().Add(-90 * 24 * time.Hour),
	})
	m.rebuild()

	if !hasRun(m.ciRows(), 9100) {
		t.Fatal("a run still in progress must stay in the table however long it has been going")
	}
}

func TestCITableDropsFinishedRunsOutsideTheWindow(t *testing.T) {
	m := testModel(t, 200, 60, ViewCI)
	m.settings.CIRecentHours = 24
	m.runs = append(m.runs, model.WorkflowRun{
		ID: 9101, Repo: "acme/payments-api", Name: "CI", Branch: "stale",
		Status: "completed", Conclusion: "success",
		StartedAt: time.Now().Add(-50 * time.Hour),
		UpdatedAt: time.Now().Add(-48 * time.Hour),
	})
	m.rebuild()

	if hasRun(m.ciRows(), 9101) {
		t.Fatal("a run that finished two days ago is not recent")
	}
}

func TestCITableOrdersRunningBeforeFinished(t *testing.T) {
	m := testModel(t, 200, 60, ViewCI)
	m.runs = append(m.runs, model.WorkflowRun{
		ID: 9102, Repo: "acme/payments-api", Name: "CI", Branch: "slow",
		Status:    "in_progress",
		StartedAt: time.Now().Add(-6 * time.Hour),
		UpdatedAt: time.Now().Add(-6 * time.Hour),
	})
	m.rebuild()

	seenFinished := false
	for _, r := range m.ciRows() {
		if !r.InProgress() {
			seenFinished = true
			continue
		}
		if seenFinished {
			t.Fatalf("run %d is still going but sorted below a finished run", r.ID)
		}
	}
}

func TestCITableOrdersFinishedRunsNewestFirst(t *testing.T) {
	m := testModel(t, 200, 60, ViewCI)

	var last time.Time
	for _, r := range m.ciRows() {
		if r.InProgress() {
			continue
		}
		if !last.IsZero() && r.UpdatedAt.After(last) {
			t.Fatalf("run %d breaks newest-first ordering", r.ID)
		}
		last = r.UpdatedAt
	}
}

func TestCICursorStaysInsideTheTable(t *testing.T) {
	m := testModel(t, 200, 60, ViewCI)
	total := len(m.ciRows())
	if total == 0 {
		t.Fatal("the mock should produce runs")
	}

	for i := 0; i < total*2; i++ {
		m = send(m, "down")
		if m.ciRow < 0 || m.ciRow >= total {
			t.Fatalf("cursor left the table at %d of %d", m.ciRow, total)
		}
	}
	for i := 0; i < total*2; i++ {
		m = send(m, "up")
		if m.ciRow < 0 || m.ciRow >= total {
			t.Fatalf("cursor left the table at %d of %d", m.ciRow, total)
		}
	}
}

func TestCIArrowKeysMoveTheCursor(t *testing.T) {
	m := testModel(t, 200, 60, ViewCI)
	if len(m.ciRows()) < 2 {
		t.Skip("needs at least two runs")
	}

	before := m.ciRow
	m = send(m, "down")
	if m.ciRow == before {
		t.Error("the down arrow should move the cursor")
	}
	m = send(m, "up")
	if m.ciRow != before {
		t.Error("the up arrow should move the cursor back")
	}
}

func TestCIEndAndHomeJumpToTheEnds(t *testing.T) {
	m := testModel(t, 200, 60, ViewCI)
	total := len(m.ciRows())

	m = send(m, "end")
	if m.ciRow != total-1 {
		t.Errorf("end should select the last run, got %d of %d", m.ciRow, total)
	}
	m = send(m, "home")
	if m.ciRow != 0 {
		t.Errorf("home should select the first run, got %d", m.ciRow)
	}
}

func TestCIFailuresOnlyFiltersTheTable(t *testing.T) {
	m := testModel(t, 200, 60, ViewCI)
	m.runs = append(m.runs, model.WorkflowRun{
		ID: 9003, Repo: "acme/payments-api", Name: "CI", Branch: "master",
		Status: "completed", Conclusion: "failure",
		StartedAt: time.Now().Add(-3 * time.Hour),
		UpdatedAt: time.Now().Add(-2 * time.Hour),
	})
	m.rebuild()

	m = send(m, "f")

	listed := m.ciRows()
	if !hasRun(listed, 9003) {
		t.Fatal("failures only hid a failing master run")
	}
	for _, r := range listed {
		if !r.Failed() {
			t.Fatalf("failures only kept a passing run: %s on %s", r.Name, r.Branch)
		}
	}
}

func TestCITableShowsRepoOnlyWhenUnscoped(t *testing.T) {
	m := testModel(t, 200, 60, ViewCI)

	if !strings.Contains(stripANSI(m.View()), "device-gateway") {
		t.Error("with several repos in view the table should say which repo a run belongs to")
	}

	m.scope = "acme/payments-api"
	m.rebuild()
	if strings.Contains(stripANSI(m.View()), "device-gateway") {
		t.Error("a scoped table should not repeat the repository on every row")
	}
}

func TestCITrendsNameEveryWorkflow(t *testing.T) {
	m := testModel(t, 200, 60, ViewCI)
	out := stripANSI(m.View())

	if !strings.Contains(out, "TRENDS") {
		t.Fatal("the trend section is missing")
	}
	for _, want := range []string{"Sync releases", "DB migration"} {
		if !strings.Contains(out, want) {
			t.Errorf("the trend section does not name %q", want)
		}
	}
}

func TestCIViewDropsTheOldSectionHeaders(t *testing.T) {
	m := testModel(t, 200, 60, ViewCI)
	out := stripANSI(m.View())

	for _, gone := range []string{"RUNS ON OPEN PRs", "RUNS ON OTHER BRANCHES"} {
		if strings.Contains(out, gone) {
			t.Errorf("%q should be gone, the table is flat now", gone)
		}
	}
	if !strings.Contains(out, "WORKFLOW") || !strings.Contains(out, "BRANCH") {
		t.Error("the table header is missing")
	}
}

func TestCIColumnsDropOnNarrowTerminals(t *testing.T) {
	wide := testModel(t, 200, 60, ViewCI).ciColumns()
	if !wide.event || !wide.dur || !wide.pr {
		t.Fatalf("a 200 column terminal should show every column, got %+v", wide)
	}

	if wide.branch > maxCIBranch {
		t.Errorf("the branch column must stay capped, got %d", wide.branch)
	}

	ultrawide := testModel(t, 400, 60, ViewCI)
	cols := ultrawide.ciColumns()
	if cols.branch > maxCIBranch {
		t.Errorf("a 400 column terminal must not stretch the branch column to %d", cols.branch)
	}
	for _, line := range strings.Split(stripANSI(ultrawide.View()), "\n") {
		if !strings.Contains(line, "BRANCH") {
			continue
		}
		if gap := strings.Index(line, "EVENT") - strings.Index(line, "BRANCH"); gap > maxCIBranch+2 {
			t.Errorf("branch and event drift %d columns apart on a wide terminal", gap)
		}
	}

	narrow := testModel(t, 70, 60, ViewCI).ciColumns()
	if narrow.event {
		t.Error("the event column should be the first to go")
	}
	if narrow.branch < minCIBranch {
		t.Errorf("the branch column fell below its minimum: %d", narrow.branch)
	}
}

func TestCIEmptyStateNamesTheWindow(t *testing.T) {
	m := testModel(t, 200, 60, ViewCI)
	m.runs = nil
	m.rebuild()

	out := stripANSI(m.View())
	if !strings.Contains(out, "No runs in the last") {
		t.Errorf("an empty table should explain itself:\n%s", out)
	}
}

func TestCICursorFollowsItsRunAcrossARefresh(t *testing.T) {
	m := testModel(t, 200, 60, ViewCI)
	if len(m.ciRows()) < 3 {
		t.Skip("needs a few runs")
	}

	m = send(m, "down", "down")
	want, ok := m.selectedRun()
	if !ok {
		t.Fatal("nothing selected")
	}

	m.runs = append([]model.WorkflowRun{{
		ID: 9200, Repo: "acme/payments-api", Name: "CI", Branch: "brand-new",
		Status:    "in_progress",
		StartedAt: time.Now(),
		UpdatedAt: time.Now(),
	}}, m.runs...)
	m.rebuild()

	got, ok := m.selectedRun()
	if !ok || got.ID != want.ID {
		t.Fatalf("a refresh moved the cursor off run %d onto %d", want.ID, got.ID)
	}
}

func TestCICursorClampsWhenItsRunDisappears(t *testing.T) {
	m := testModel(t, 200, 60, ViewCI)
	m = send(m, "end")

	m.runs = m.runs[:1]
	m.rebuild()

	rows := m.ciRows()
	if m.ciRow < 0 || m.ciRow >= len(rows) {
		t.Fatalf("cursor at %d is outside a table of %d", m.ciRow, len(rows))
	}
}
