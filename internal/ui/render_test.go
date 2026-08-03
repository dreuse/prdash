package ui

import (
	"context"
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dreuse/prdash/internal/config"
	"github.com/dreuse/prdash/internal/github"
	"github.com/dreuse/prdash/internal/model"
)

func testModel(t *testing.T, width, height int, view View) Model {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	mock := github.NewMock()
	snap, err := mock.Fetch(context.Background())
	if err != nil {
		t.Fatalf("mock fetch: %v", err)
	}

	settings := config.DefaultSettings()
	settings.Repos = []string{"acme/payments-api", "acme/device-gateway"}
	settings.DefaultView = view.String()

	m := New(Options{Fetcher: mock, Actor: mock, Settings: settings, Repos: settings.Repos})
	m.emoji = modelEmoji
	updated, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	m = updated.(Model)
	updated, _ = m.Update(dataMsg{snapshot: snap})
	m = updated.(Model)
	m.view = view
	m.rebuild()
	return m
}

func TestNoLineWrapsAtAnyWidth(t *testing.T) {
	for _, view := range Views {
		for width := 60; width <= 300; width += 7 {
			for _, height := range []int{12, 24, 50} {
				m := testModel(t, width, height, view)
				lines := strings.Split(m.View(), "\n")
				if len(lines) != height {
					t.Fatalf("%s at %dx%d: got %d lines, want %d", view, width, height, len(lines), height)
				}
				for i, line := range lines {
					if w := lipgloss.Width(line); w > width {
						t.Fatalf("%s at %dx%d: line %d is %d columns wide:\n%q",
							view, width, height, i, w, line)
					}
				}
			}
		}
	}
}

func TestOverlaysStayInsideTheScreen(t *testing.T) {
	for _, kind := range []overlayKind{ovSettings, ovHelp, ovConfirm, ovRepo} {
		for _, width := range []int{62, 100, 180, 260} {
			m := testModel(t, width, 30, ViewBoard)
			m.confirm = confirmState{title: "Merge #12012?", body: "squash into master", verb: "merge"}
			m.repoPick = newRepoPicker()
			m.push(kind)

			lines := strings.Split(m.View(), "\n")
			if len(lines) != 30 {
				t.Fatalf("overlay %d at %d cols: got %d lines, want 30", kind, width, len(lines))
			}
			for i, line := range lines {
				if w := lipgloss.Width(line); w > width {
					t.Fatalf("overlay %d at %d cols: line %d is %d wide:\n%q", kind, width, i, w, line)
				}
			}
		}
	}
}

func TestResizeKeepsSelection(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m.laneIdx = 1
	m = m.moveLane(1)
	want := m.sel
	if want.Zero() {
		t.Fatal("no selection to preserve")
	}

	for _, size := range []int{120, 80, 300, 100} {
		updated, _ := m.Update(tea.WindowSizeMsg{Width: size, Height: 20})
		m = updated.(Model)
		if m.sel != want {
			t.Fatalf("resize to %d lost the selection: got %v want %v", size, m.sel, want)
		}
	}
}

func TestCardsNeverWrapInsideTheirLane(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)

	for _, col := range m.order {
		for _, pr := range m.lanes[col] {
			for width := 10; width <= 60; width++ {
				card := m.renderCard(pr, col, width, false)
				lines := strings.Split(card, "\n")
				if len(lines) > 3 {
					t.Fatalf("#%d at lane width %d rendered %d lines:\n%s",
						pr.Number, width, len(lines), card)
				}
				for _, line := range lines {
					if w := lipgloss.Width(line); w > width+cardRulePad {
						t.Fatalf("#%d at lane width %d produced a %d column line:\n%q",
							pr.Number, width, w, line)
					}
				}
			}
		}
	}
}

func TestSplitViewFollowsTheSelection(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m.split = true

	first, ok := m.selectedPR()
	if !ok {
		t.Fatal("nothing selected")
	}
	if !strings.Contains(m.View(), "#"+itoa(first.Number)) {
		t.Fatalf("the split pane does not show #%d", first.Number)
	}

	m = m.moveLane(1)
	second, ok := m.selectedPR()
	if !ok || second.Key() == first.Key() {
		t.Fatal("selection did not move")
	}
	out := m.View()
	if !strings.Contains(out, "CHECKS") || !strings.Contains(out, "BRANCH") {
		t.Fatal("the split pane lost its sections after moving")
	}
	if !strings.Contains(out, "#"+itoa(second.Number)) {
		t.Fatalf("the split pane did not follow the selection to #%d", second.Number)
	}
}

func TestSplitViewLeavesRoomForBothHalves(t *testing.T) {
	for _, height := range []int{12, 24, 50} {
		m := testModel(t, 200, height, ViewBoard)
		m.split = true

		lines := strings.Split(m.View(), "\n")
		if len(lines) != height {
			t.Fatalf("at height %d the split screen produced %d lines", height, len(lines))
		}
		if !strings.Contains(m.View(), "READY TO MERGE") {
			t.Fatalf("at height %d the board half disappeared", height)
		}
	}
}

func TestHealthyCardIsQuietAndAbnormalCardIsNot(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	pr := model.PullRequest{
		Repo: "acme/payments-api", Number: 1, Title: "healthy", Author: "someone",
		CreatedAt: nowMinusDays(3), UpdatedAt: nowMinusDays(1),
		Mergeable: model.MergeableYes,
		Checks:    []model.Check{{Name: "build", State: model.CheckPassed}},
	}
	if signals := m.cardSignals(pr, model.ColNeedsReview, 40); signals != "" {
		t.Fatalf("a healthy card must render no signal line, got %q", signals)
	}
	card := m.renderCard(pr, model.ColNeedsReview, 40, false)
	if lines := strings.Split(card, "\n"); len(lines) != 2 {
		t.Fatalf("a healthy card must be two lines, got %d:\n%s", len(lines), card)
	}

	for _, tc := range []struct {
		name string
		pr   model.PullRequest
	}{
		{"failing checks", withChecks(pr, model.CheckFailed)},
		{"conflict", withConflict(pr)},
		{"behind", withBehind(pr, 19)},
		{"incomplete checks", withPartialChecks(pr)},
	} {
		if m.cardSignals(tc.pr, model.ColNeedsReview, 40) == "" {
			t.Errorf("%s must produce a signal line", tc.name)
		}
	}
}

func TestApprovalWaitCardSaysSoAndKeepsItsHeight(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	pr := model.PullRequest{
		Repo: "acme/payments-api", Number: 1, Title: "waiting", Author: "someone",
		CreatedAt: nowMinusDays(3), UpdatedAt: nowMinusDays(1),
		Mergeable: model.MergeableYes, MergeStateStatus: "BLOCKED",
		Approvals: m.policy.RequiredApprovals,
		Checks:    []model.Check{{Name: "build", State: model.CheckPassed}},
	}
	signals := stripANSI(m.cardSignals(pr, model.ColBlocked, 40))
	if !strings.Contains(signals, "awaiting approval") {
		t.Fatalf("a protection-blocked card must say it is waiting on reviews, got %q", signals)
	}
	card := m.renderCard(pr, model.ColBlocked, 40, false)
	if lines := strings.Split(card, "\n"); len(lines) != m.cardHeight(pr, model.ColBlocked) {
		t.Fatalf("card rendered %d lines but reserved %d", len(lines), m.cardHeight(pr, model.ColBlocked))
	}
}

func withChecks(pr model.PullRequest, state model.CheckState) model.PullRequest {
	pr.Checks = []model.Check{{Name: "build", State: state}}
	return pr
}

func withConflict(pr model.PullRequest) model.PullRequest {
	pr.Mergeable = model.MergeableConflict
	return pr
}

func withBehind(pr model.PullRequest, n int) model.PullRequest {
	pr.BehindBy = n
	return pr
}

func withPartialChecks(pr model.PullRequest) model.PullRequest {
	pr.Checks = []model.Check{{State: model.CheckPassed}, {State: model.CheckNeutral}}
	return pr
}

func TestEmptyLanesAreOmitted(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	for col := range m.lanes {
		m.lanes[col] = nil
	}
	m.lanes[model.ColNeedsReview] = []model.PullRequest{{
		Repo: "r", Number: 2, Title: "only one", Author: "a",
		CreatedAt: nowMinusDays(1), UpdatedAt: nowMinusDays(1),
	}}
	m.sel = model.Key{Repo: "r", Number: 2}

	out := stripANSI(m.View())
	if !strings.Contains(out, "NEEDS REVIEW") {
		t.Fatal("the lane with a card should render")
	}
	for _, gone := range []string{"READY TO MERGE", "CHANGES REQUESTED", "CI RUNNING", "BLOCKED", "DRAFT"} {
		if strings.Contains(out, gone) {
			t.Errorf("empty lane %q should be omitted entirely:\n%s", gone, out)
		}
	}
}

func TestNoPullRequestsShowsAMessage(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	for col := range m.lanes {
		m.lanes[col] = nil
	}

	out := stripANSI(m.View())
	if !strings.Contains(out, "No PRs") {
		t.Fatalf("an empty board should say No PRs:\n%s", out)
	}
	for _, gone := range []string{"READY TO MERGE", "DRAFT"} {
		if strings.Contains(out, gone) {
			t.Errorf("no lane headers should render on an empty board, saw %q", gone)
		}
	}
}

func TestEmptyBoardDistinguishesFilterFromNoData(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m = openFilterBar(m)
	m = typeFilter(m, "zzzznothing")
	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = out.(Model)

	view := stripANSI(m.View())
	if !strings.Contains(view, "No PRs match this filter") {
		t.Fatalf("a filter matching nothing should say so:\n%s", view)
	}
	if !strings.Contains(view, "esc to clear") {
		t.Fatal("it should offer a way out")
	}
}

func TestNoColorModeKeepsEveryView(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	for _, view := range Views {
		m := testModel(t, 160, 30, view)
		out := m.View()
		if strings.Contains(out, "\x1b[38;2;") {
			t.Fatalf("%s still emits truecolor under NO_COLOR", view)
		}
		if !strings.Contains(out, appName) {
			t.Fatalf("%s lost its header under NO_COLOR", view)
		}
	}
}

func TestPreview(t *testing.T) {
	dir := os.Getenv("PRDASH_PREVIEW")
	if dir == "" {
		t.Skip("set PRDASH_PREVIEW=<dir> to dump rendered screens")
	}
	for _, tc := range []struct {
		name          string
		view          View
		width, height int
	}{
		{"board", ViewBoard, 200, 40},
		{"board-narrow", ViewBoard, 120, 30},
		{"board-60", ViewBoard, 60, 24},
		{"ci", ViewCI, 200, 40},
	} {
		m := testModel(t, tc.width, tc.height, tc.view)
		if err := os.WriteFile(dir+"/"+tc.name+".txt", []byte(m.View()), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	m := testModel(t, 200, 40, ViewBoard)
	m.push(ovSettings)
	_ = os.WriteFile(dir+"/settings.txt", []byte(m.View()), 0o644)

	m = testModel(t, 200, 40, ViewBoard)
	m.split = true
	_ = os.WriteFile(dir+"/split.txt", []byte(m.View()), 0o644)

	m = testModel(t, 110, 34, ViewBoard)
	m.split = true
	_ = os.WriteFile(dir+"/split-narrow.txt", []byte(m.View()), 0o644)

	m = testModel(t, 200, 40, ViewBoard)
	m.confirm = confirmState{title: "Merge #12012?", body: "squash into master", verb: "merge", danger: true}
	m.push(ovConfirm)
	_ = os.WriteFile(dir+"/confirm.txt", []byte(m.View()), 0o644)
}

func TestPreviewComment(t *testing.T) {
	dir := os.Getenv("PRDASH_PREVIEW")
	if dir == "" {
		t.Skip("set PRDASH_PREVIEW=<dir> to dump rendered screens")
	}
	for _, tc := range []struct{ name, typed string }{
		{"comment-emoji", "nice work, shipping :roc"},
		{"comment-ref", "same root cause as #120"},
		{"comment-mention", "could you take a look @a"},
	} {
		m := testModel(t, 200, 26, ViewBoard)
		m.split = true
		m = press(m, "c")
		m = press(m, tc.typed)
		_ = os.WriteFile(dir+"/"+tc.name+".txt", []byte(m.View()), 0o644)
	}
}

func TestPreviewRepoPicker(t *testing.T) {
	dir := os.Getenv("PRDASH_PREVIEW")
	if dir == "" {
		t.Skip("set PRDASH_PREVIEW=<dir> to dump rendered screens")
	}
	m := testModel(t, 200, 30, ViewBoard)
	m.repoPick = newRepoPicker()
	m.push(ovRepo)
	_ = os.WriteFile(dir+"/repo-picker.txt", []byte(m.View()), 0o644)
}

func TestPreviewFilter(t *testing.T) {
	dir := os.Getenv("PRDASH_PREVIEW")
	if dir == "" {
		t.Skip("set PRDASH_PREVIEW=<dir> to dump rendered screens")
	}
	for _, tc := range []struct{ name, typed string }{
		{"filter-keys", "assi"},
		{"filter-values", "assignee:"},
		{"filter-free", "idempo"},
	} {
		m := testModel(t, 200, 26, ViewBoard)
		m = openFilterBar(m)
		m = typeFilter(m, tc.typed)
		_ = os.WriteFile(dir+"/"+tc.name+".txt", []byte(m.View()), 0o644)
	}
}

func TestPreviewEmpty(t *testing.T) {
	dir := os.Getenv("PRDASH_PREVIEW")
	if dir == "" {
		t.Skip("set PRDASH_PREVIEW=<dir> to dump rendered screens")
	}
	m := testModel(t, 120, 20, ViewBoard)
	for col := range m.lanes {
		m.lanes[col] = nil
	}
	_ = os.WriteFile(dir+"/empty.txt", []byte(m.View()), 0o644)

	m = testModel(t, 120, 20, ViewBoard)
	m = openFilterBar(m)
	m = typeFilter(m, "-is:draft is:failing")
	_ = os.WriteFile(dir+"/negated.txt", []byte(m.View()), 0o644)
}

func TestFooterOffersNoActionsWithoutASelection(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	for col := range m.lanes {
		m.lanes[col] = nil
	}
	m.clampSelection()

	if _, ok := m.selectedPR(); ok {
		t.Fatal("nothing is visible, so nothing can be selected")
	}
	footer := stripANSI(m.renderFooter())
	for _, verb := range []string{"approve", "merge", "rebase", "copy branch"} {
		if strings.Contains(footer, verb) {
			t.Errorf("the footer offers %q with nothing selected:\n%s", verb, footer)
		}
	}
}

func TestSelectionIgnoresFilteredOutPullRequests(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	pr, ok := m.selectedPR()
	if !ok {
		t.Fatal("expected a selection")
	}

	m = openFilterBar(m)
	m = typeFilter(m, "is:draft")

	if got, ok := m.selectedPR(); ok && got.Key() == pr.Key() && !got.IsDraft {
		t.Fatal("a filtered out pull request must not stay selected")
	}
	if got, ok := m.selectedPR(); ok && !got.IsDraft {
		t.Fatalf("#%d does not match the filter but is selected", got.Number)
	}
}

func TestPreviewClose(t *testing.T) {
	dir := os.Getenv("PRDASH_PREVIEW")
	if dir == "" {
		t.Skip("set PRDASH_PREVIEW=<dir> to dump rendered screens")
	}
	m := testModel(t, 200, 24, ViewBoard)
	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}})
	_ = os.WriteFile(dir+"/close.txt", []byte(out.(Model).View()), 0o644)
}

func TestPreviewConfirms(t *testing.T) {
	dir := os.Getenv("PRDASH_PREVIEW")
	if dir == "" {
		t.Skip("set PRDASH_PREVIEW=<dir> to dump rendered screens")
	}
	for _, k := range []rune{'m', 'b', 'r'} {
		m := testModel(t, 200, 24, ViewBoard)
		m = selectFailing(t, m)
		m.viewer = "nobody"
		out, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{k}})
		_ = os.WriteFile(dir+"/confirm-"+string(k)+".txt", []byte(out.(Model).renderConfirm()), 0o644)
	}
}

func TestPreviewMultiline(t *testing.T) {
	dir := os.Getenv("PRDASH_PREVIEW")
	if dir == "" {
		t.Skip("set PRDASH_PREVIEW=<dir> to dump rendered screens")
	}
	m := testModel(t, 200, 24, ViewBoard)
	m = press(m, "c")
	m = press(m, "looks good to me")
	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlJ})
	m = out.(Model)
	m = press(m, "one nit inline, otherwise ship it :roc")
	_ = os.WriteFile(dir+"/comment-multiline.txt", []byte(m.View()), 0o644)
}
