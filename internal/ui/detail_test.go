package ui

import (
	"errors"
	"strings"
	"testing"

	"github.com/dreuse/prdash/internal/model"
)

func selectPR(t *testing.T, m Model, number int) Model {
	t.Helper()
	for i, col := range m.order {
		for _, pr := range m.lanes[col] {
			if pr.Number == number {
				m.sel = pr.Key()
				m.laneIdx = i
				return m
			}
		}
	}
	t.Fatalf("the mock has no visible pull request #%d", number)
	return m
}

func splitScreen(t *testing.T, width, height, number int) (Model, string) {
	t.Helper()
	m := testModel(t, width, height, ViewBoard)
	m = selectPR(t, m, number)
	m = send(m, "v")
	if !m.split {
		t.Fatal("v did not open the split panel")
	}
	return m, stripANSI(m.View())
}

func TestARuleSeparatesTheBoardFromTheSplitPane(t *testing.T) {
	m, out := splitScreen(t, 200, 40, 12009)
	rule := strings.Repeat(m.theme.Glyphs.HRule, 4)

	lines := strings.Split(out, "\n")
	head := -1
	for i, line := range lines {
		if strings.Contains(line, "DETAILS #12009") {
			head = i
		}
	}
	if head < 1 {
		t.Fatalf("the pane header is missing:\n%s", out)
	}
	if !strings.Contains(lines[head-1], rule) {
		t.Errorf("the board and the pane need a divider between them, got %q", lines[head-1])
	}
}

func TestSplitPaneShowsRecentCommentsAndCommits(t *testing.T) {
	_, out := splitScreen(t, 200, 50, 12009)

	for _, want := range []string{"COMMENTS", "COMMITS", "apatel", "a3f91c2", "Rebase on master"} {
		if !strings.Contains(out, want) {
			t.Errorf("the split panel should show %q:\n%s", want, out)
		}
	}
}

func TestTheCommitBlockCountsWhatItDoesNotShow(t *testing.T) {
	_, out := splitScreen(t, 200, 50, 12009)

	if !strings.Contains(out, "COMMITS 12") || !strings.Contains(out, "+7 more") {
		t.Errorf("a truncated commit list must say how many it left out:\n%s", out)
	}
}

func TestEveryCommentIsListedNotJustTheLastFew(t *testing.T) {
	m, _ := splitScreen(t, 200, 50, 12009)
	pr, ok := m.selectedPR()
	if !ok {
		t.Fatal("no pull request selected")
	}

	pr.Comments = nil
	for i := 0; i < 6; i++ {
		pr.Comments = append(pr.Comments, model.Comment{Author: "dev" + itoa(i), Body: "note " + itoa(i)})
	}

	block := stripANSI(m.detailComments(pr, 60))
	if !strings.Contains(block, "COMMENTS 6") {
		t.Errorf("the header must count the comments it lists:\n%s", block)
	}
	for i := 0; i < 6; i++ {
		if !strings.Contains(block, "dev"+itoa(i)) {
			t.Errorf("comment from dev%d is missing:\n%s", i, block)
		}
	}
}

func TestALongCommentKeepsItsWholeBody(t *testing.T) {
	m, _ := splitScreen(t, 200, 50, 12009)
	pr, ok := m.selectedPR()
	if !ok {
		t.Fatal("no pull request selected")
	}
	pr.Comments = []model.Comment{{Author: "rita", Body: strings.Repeat("word ", 60) + "tail"}}

	block := stripANSI(m.detailComments(pr, 40))
	if !strings.Contains(block, "tail") {
		t.Errorf("a comment must be readable in full, not clipped to a teaser:\n%s", block)
	}
}

func TestAPullRequestWithNoDiscussionSaysSo(t *testing.T) {
	_, out := splitScreen(t, 200, 50, 11959)

	if !strings.Contains(out, "no comments yet") {
		t.Errorf("an empty conversation needs a word, not a blank block:\n%s", out)
	}
}

func TestTheSplitPaneScrollsWhenItsContentDoesNotFit(t *testing.T) {
	m, before := splitScreen(t, 80, 24, 12009)

	m = send(m, "tab")
	if !m.detail.focus {
		t.Fatal("tab did not focus the split panel")
	}
	m = send(m, "end")
	after := stripANSI(m.View())

	if before == after {
		t.Fatalf("a pane too short for its content must scroll:\n%s", after)
	}
	if strings.Contains(before, "reconciliation") {
		t.Fatal("the fixture must be taller than the pane, or there is nothing to scroll to")
	}
	if !strings.Contains(after, "reconciliation") {
		t.Errorf("scrolling to the end should reach the oldest comment:\n%s", after)
	}
}

func TestUpAndDownScrollTheFocusedPaneOneLineAtATime(t *testing.T) {
	m, _ := splitScreen(t, 80, 24, 12009)
	m = send(m, "tab")

	m = send(m, "down")
	m = send(m, "down")
	if m.detail.scroll != 2 {
		t.Fatalf("down must scroll the pane, got %d", m.detail.scroll)
	}
	m = send(m, "up")
	if m.detail.scroll != 1 {
		t.Errorf("up must scroll the pane back, got %d", m.detail.scroll)
	}
}

func TestScrollingTheDetailPaneDoesNotMoveTheSelection(t *testing.T) {
	m, _ := splitScreen(t, 80, 24, 11959)
	m = send(m, "tab")

	want := m.sel
	m = send(m, "j", "j", "down", "pgdown")
	if m.sel != want {
		t.Errorf("j and pgdn belong to the focused pane, selection moved to %+v", m.sel)
	}
	if m.detail.scroll == 0 {
		t.Error("the pane should have scrolled instead")
	}

	m = send(m, "tab")
	if m.detail.focus {
		t.Fatal("tab did not hand focus back to the board")
	}
	m = send(m, "j")
	if m.sel == want {
		t.Error("with the board focused again, j must move the selection")
	}
}

func TestTabDoesNothingWhileTheBoardSplitIsClosed(t *testing.T) {
	m := testModel(t, 80, 24, ViewBoard)
	m = selectPR(t, m, 12009)

	m = send(m, "tab")
	if m.detail.focus {
		t.Error("there is no pane to focus while the split is closed")
	}
}

func TestMovingTheSelectionRewindsTheDetailPane(t *testing.T) {
	m, _ := splitScreen(t, 80, 24, 11959)
	m = send(m, "tab", "end")
	if m.detail.scroll == 0 {
		t.Fatal("the pane did not scroll")
	}

	m = send(m, "tab", "j")
	if m.detail.scroll != 0 {
		t.Errorf("a different pull request must start at the top, got scroll %d", m.detail.scroll)
	}
}

func TestPlusAndMinusResizeTheSplitPaneAndSurviveARestart(t *testing.T) {
	m, _ := splitScreen(t, 120, 40, 12009)
	start := m.detailRows()

	m = send(m, "+", "+", "+")
	grown := m.detailRows()
	if grown != start+3 {
		t.Errorf("three presses of + should add three rows, went from %d to %d", start, grown)
	}
	if m.state.SplitRows != grown+detailChromeRows {
		t.Errorf("the new size must be remembered, got %d", m.state.SplitRows)
	}

	m = send(m, "-", "-")
	if shrunk := m.detailRows(); shrunk != grown-2 {
		t.Errorf("- should give the rows back, got %d", shrunk)
	}
}

func TestNeitherHalfOfTheSplitCanBeSqueezedAway(t *testing.T) {
	m, _ := splitScreen(t, 120, 40, 12009)

	for i := 0; i < 60; i++ {
		m = send(m, "+")
	}
	if board := m.bodyHeight() - m.detailRows() - detailChromeRows; board < minSplitBoard {
		t.Errorf("the board must keep %d rows, got %d", minSplitBoard, board)
	}

	for i := 0; i < 60; i++ {
		m = send(m, "-")
	}
	if m.detailRows()+detailChromeRows < minSplitDetail {
		t.Errorf("the detail pane must keep %d rows, got %d", minSplitDetail, m.detailRows()+detailChromeRows)
	}
}

func TestResizingDoesNothingWhileTheSplitIsClosed(t *testing.T) {
	m := testModel(t, 120, 40, ViewBoard)
	m = selectPR(t, m, 12009)

	m = send(m, "+")
	if m.state.SplitRows != 0 {
		t.Errorf("there is no pane to resize while the split is closed, got %d", m.state.SplitRows)
	}
}

func loadedDiff(t *testing.T, m Model) Model {
	t.Helper()
	m = send(m, "d")
	if !m.detail.diff {
		t.Fatal("d did not switch the pane to the diff")
	}

	out, _ := m.Update(diffLoadMsg{gen: m.detail.gen})
	m = out.(Model)

	pr, ok := m.selectedPR()
	if !ok {
		t.Fatal("no pull request selected")
	}
	lines, err := m.actor.Diff(t.Context(), pr)
	if err != nil {
		t.Fatalf("the mock refused to produce a diff: %v", err)
	}
	out, _ = m.Update(diffMsg{key: pr.Key(), lines: lines})
	return out.(Model)
}

func TestDOpensTheDiffAndEscGoesBackToTheOverview(t *testing.T) {
	m, _ := splitScreen(t, 200, 40, 12009)
	m = loadedDiff(t, m)

	out := stripANSI(m.View())
	if !strings.Contains(out, "DIFF") {
		t.Errorf("the pane should say what it is showing:\n%s", out)
	}
	if !strings.Contains(out, "+  Category string") {
		t.Errorf("the patch should be on screen:\n%s", out)
	}
	if strings.Contains(out, "COMMENTS") {
		t.Errorf("the diff replaces the overview, it does not stack with it:\n%s", out)
	}

	m = send(m, "esc")
	if m.detail.diff {
		t.Fatal("esc did not go back to the overview")
	}
	if back := stripANSI(m.View()); !strings.Contains(back, "COMMENTS") {
		t.Errorf("the overview should be back:\n%s", back)
	}
	if !m.split {
		t.Error("esc must not close the whole split panel")
	}
}

func TestTheDiffPaneKeepsTheSpinnerTicking(t *testing.T) {
	m, _ := splitScreen(t, 120, 40, 12009)
	m = send(m, "d")

	if !m.detail.loading {
		t.Fatal("d should start a fetch")
	}
	if !m.needsSpinner() {
		t.Error("a loading pane must keep the spinner animating")
	}
}

func TestTheDiffIsDroppedWhenTheSelectionMoves(t *testing.T) {
	m, _ := splitScreen(t, 120, 40, 11959)
	m = loadedDiff(t, m)
	if len(m.detail.lines) == 0 {
		t.Fatal("the diff did not load")
	}

	m = send(m, "j")
	if len(m.detail.lines) != 0 {
		t.Errorf("a stale diff must not linger on another pull request, kept %d lines", len(m.detail.lines))
	}
	if !m.detail.diff {
		t.Error("moving the selection should keep showing diffs, not fall back to the overview")
	}
	if !m.detail.loading {
		t.Error("the pane should be fetching the diff of the pull request now selected")
	}
}

func focusedDiff(t *testing.T, width, height, number int) Model {
	t.Helper()
	m, _ := splitScreen(t, width, height, number)
	return send(loadedDiff(t, m), "tab")
}

func linesWithPrefix(lines []string, prefix string) []int {
	var at []int
	for i, line := range lines {
		if strings.HasPrefix(line, prefix) {
			at = append(at, i)
		}
	}
	return at
}

func TestHalfPageIsHalfOfAFullPage(t *testing.T) {
	m := focusedDiff(t, 200, 40, 12009)
	rows := m.detailRows()

	m = send(m, "ctrl+d")
	half := m.detail.scroll
	if half != rows/2 {
		t.Errorf("ctrl-d should move half a page (%d), moved %d", rows/2, half)
	}

	m = send(m, "ctrl+u")
	if m.detail.scroll != 0 {
		t.Errorf("ctrl-u should undo it, landed on %d", m.detail.scroll)
	}

	m = send(m, "ctrl+f")
	if m.detail.scroll <= half {
		t.Errorf("ctrl-f must move further than ctrl-d, got %d against %d", m.detail.scroll, half)
	}
	m = send(m, "ctrl+b")
	if m.detail.scroll != 0 {
		t.Errorf("ctrl-b should undo ctrl-f, landed on %d", m.detail.scroll)
	}
}

func TestCtrlEAndCtrlYScrollASingleLine(t *testing.T) {
	m := focusedDiff(t, 200, 40, 12009)

	m = send(m, "ctrl+e", "ctrl+e")
	if m.detail.scroll != 2 {
		t.Errorf("two ctrl-e should move two lines, moved %d", m.detail.scroll)
	}
	m = send(m, "ctrl+y")
	if m.detail.scroll != 1 {
		t.Errorf("ctrl-y should come back a line, landed on %d", m.detail.scroll)
	}
}

func TestGgGoesToTheTopOfThePane(t *testing.T) {
	m := focusedDiff(t, 200, 40, 12009)
	m = send(m, "ctrl+f", "ctrl+f")
	if m.detail.scroll == 0 {
		t.Fatal("the pane did not scroll")
	}

	m = send(m, "g", "g")
	if m.detail.scroll != 0 {
		t.Errorf("gg should go back to the top, landed on %d", m.detail.scroll)
	}
}

func TestAHalfTypedChordIsAbandonedNotRemembered(t *testing.T) {
	m := focusedDiff(t, 200, 40, 12009)
	m = send(m, "ctrl+f")
	scrolled := m.detail.scroll

	m = send(m, "g", "esc")
	if m.chord != "" {
		t.Errorf("esc should drop a pending chord, still holding %q", m.chord)
	}
	m = send(m, "g")
	if m.detail.scroll != scrolled {
		t.Errorf("a lone g must not move anything, scroll went to %d", m.detail.scroll)
	}
}

func TestBracketCJumpsFromFileToFile(t *testing.T) {
	m := focusedDiff(t, 200, 40, 12009)
	heads := linesWithPrefix(m.detail.lines, "diff --git")
	if len(heads) < 3 {
		t.Fatalf("the mock diff needs at least three files, got %d", len(heads))
	}

	m = send(m, "]", "c")
	if m.detail.scroll != heads[1] {
		t.Errorf("]c should land on the second file at %d, landed on %d", heads[1], m.detail.scroll)
	}
	m = send(m, "]", "c")
	if m.detail.scroll != heads[2] {
		t.Errorf("a second ]c should reach the third file at %d, landed on %d", heads[2], m.detail.scroll)
	}
	m = send(m, "[", "c")
	if m.detail.scroll != heads[1] {
		t.Errorf("[c should go back to %d, landed on %d", heads[1], m.detail.scroll)
	}
}

func TestBracesJumpBetweenHunks(t *testing.T) {
	m := focusedDiff(t, 200, 40, 12009)
	hunks := linesWithPrefix(m.detail.lines, "@@")
	if len(hunks) < 2 {
		t.Fatalf("the mock diff needs at least two hunks, got %d", len(hunks))
	}

	m = send(m, "}")
	if m.detail.scroll != hunks[0] {
		t.Errorf("} should reach the first hunk at %d, landed on %d", hunks[0], m.detail.scroll)
	}
	m = send(m, "}")
	if m.detail.scroll != hunks[1] {
		t.Errorf("a second } should reach %d, landed on %d", hunks[1], m.detail.scroll)
	}
	m = send(m, "{")
	if m.detail.scroll != hunks[0] {
		t.Errorf("{ should go back to %d, landed on %d", hunks[0], m.detail.scroll)
	}
}

func TestJumpingStopsAtTheEndsInsteadOfWrapping(t *testing.T) {
	m := focusedDiff(t, 200, 40, 12009)

	m = send(m, "[", "c")
	if m.detail.scroll != 0 {
		t.Errorf("there is no file before the first, scroll is %d", m.detail.scroll)
	}

	for i := 0; i < 40; i++ {
		m = send(m, "]", "c")
	}
	last := m.detail.scroll
	m = send(m, "]", "c")
	if m.detail.scroll != last {
		t.Errorf("]c past the last file must stay put, moved from %d to %d", last, m.detail.scroll)
	}
}

func TestBracesJumpBlockToBlockInTheOverview(t *testing.T) {
	m, _ := splitScreen(t, 80, 24, 12009)
	m = send(m, "tab", "}")

	if m.detail.scroll == 0 {
		t.Error("} should reach the next block, the overview is paragraph shaped")
	}
	first := m.detail.scroll
	m = send(m, "{")
	if m.detail.scroll >= first {
		t.Errorf("{ should go back above %d, landed on %d", first, m.detail.scroll)
	}
}

func TestAFailedDiffExplainsItselfAndOffersARetry(t *testing.T) {
	m, _ := splitScreen(t, 120, 40, 12009)
	m = send(m, "d")

	pr, _ := m.selectedPR()
	out, _ := m.Update(diffMsg{key: pr.Key(), err: errors.New("gh exploded")})
	m = out.(Model)

	if body := stripANSI(m.View()); !strings.Contains(body, "gh exploded") {
		t.Errorf("the reason must reach the user:\n%s", body)
	}
}

func TestDiffLinesAreColouredByWhatTheyDo(t *testing.T) {
	m, _ := splitScreen(t, 120, 40, 12009)
	t2 := m.theme

	cases := map[string]string{
		"+added":                   t2.OK.Render("x"),
		"-removed":                 t2.Danger.Render("x"),
		"@@ -1 +1 @@":              t2.Accent.Render("x"),
		"--- a/one.go":             t2.Faint.Render("x"),
		"+++ b/one.go":             t2.Faint.Render("x"),
		"diff --git a/x.go b/x.go": t2.Strong.Render("x"),
	}
	for line, want := range cases {
		if got := diffLineStyle(t2, line).Render("x"); got != want {
			t.Errorf("%q is styled wrong: got %q want %q", line, got, want)
		}
	}
}

func TestALongCommentIsWrappedNotSpilled(t *testing.T) {
	m, _ := splitScreen(t, 200, 50, 12009)

	pr, ok := m.selectedPR()
	if !ok {
		t.Fatal("no pull request selected")
	}
	var longest model.Comment
	for _, c := range pr.Comments {
		if len(c.Body) > len(longest.Body) {
			longest = c
		}
	}
	if len(longest.Body) < 80 {
		t.Fatalf("the fixture needs a comment long enough to wrap, got %d chars", len(longest.Body))
	}

	block := m.detailComments(pr, 40)
	for _, line := range strings.Split(stripANSI(block), "\n") {
		if textWidth(line) > 40 {
			t.Errorf("a wrapped comment must respect the column width, got %d:\n%q", textWidth(line), line)
		}
	}
}
