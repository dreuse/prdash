package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dreuse/prdash/internal/model"
)

var testEmoji = NewEmojiSet(map[string]string{
	"tada": "🎉", "rocket": "🚀", "rock": "🪨", "+1": "👍",
	"shipit": "", "octocat": "", "sparkles": "✨", "morocco": "🇲🇦",
	"flag_Morocco": "🇲🇦", "smile": "😄", "smiley": "😃", "smile_cat": "😸",
})

var modelEmoji = NewEmojiSet(map[string]string{
	"morocco": "🇲🇦", "flag_Morocco": "🇲🇦",
})

func press(m Model, keys string) Model {
	for _, r := range keys {
		out, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = out.(Model)
	}
	return m
}

func acceptOnce(m Model, backwards bool) string {
	m.comment.accept(backwards)
	return m.comment.input.Value()
}

func acceptTwice(m Model, a, b bool) string {
	m.comment.accept(a)
	m.comment.accept(b)
	return m.comment.input.Value()
}

func pressKey(m Model, t tea.KeyType) Model {
	out, _ := m.Update(tea.KeyMsg{Type: t})
	return out.(Model)
}

func TestCommentBarOpensAndCancels(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	if m.comment.active {
		t.Fatal("the comment bar should start closed")
	}

	m = press(m, "c")
	if !m.comment.active {
		t.Fatal("c should open the comment bar")
	}
	pr, _ := m.selectedPR()
	if m.comment.pr.Key() != pr.Key() {
		t.Fatalf("the bar targets #%d, want #%d", m.comment.pr.Number, pr.Number)
	}
	if !strings.Contains(m.View(), "comment") {
		t.Fatal("the comment bar is not rendered")
	}

	m = press(m, "hello")
	if got := m.comment.input.Value(); got != "hello" {
		t.Fatalf("typing produced %q", got)
	}

	m = pressKey(m, tea.KeyEsc)
	if m.comment.active || m.comment.input.Value() != "" {
		t.Fatal("esc should close and clear the bar")
	}
}

func TestCommentBarSwallowsNavigationKeys(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	before := m.sel

	m = press(m, "c")
	m = press(m, "jkhlvq1234")

	if m.sel != before {
		t.Fatal("navigation keys must not move the selection while commenting")
	}
	if got := m.comment.input.Value(); got != "jkhlvq1234" {
		t.Fatalf("the bar swallowed characters: %q", got)
	}
}

func TestCommentBarSubstitutesEmojiShortcodes(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m = press(m, "c")
	m = press(m, "ship it :rocket: and :+1:")

	want := "ship it 🚀 and 👍"
	if got := m.comment.input.Value(); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestCommentBarLeavesUnknownShortcodesAlone(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m = press(m, "c")
	m = press(m, "see :not_a_real_emoji: here")

	if got := m.comment.input.Value(); got != "see :not_a_real_emoji: here" {
		t.Fatalf("an unknown shortcode was rewritten: %q", got)
	}
}

func TestCommentBarCompletesWithTab(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m = press(m, "c")
	m = press(m, "nice :rocke")

	if len(m.comment.candidates) == 0 {
		t.Fatal("an open shortcode should offer completions")
	}
	if !strings.Contains(m.View(), "rocket") {
		t.Fatal("completions are not shown in the bar")
	}

	m = pressKey(m, tea.KeyTab)
	if got := m.comment.input.Value(); got != "nice 🚀" {
		t.Fatalf("tab produced %q, want %q", got, "nice 🚀")
	}
}

func TestCommentBarTabCyclesThroughMatches(t *testing.T) {
	all, _ := testEmoji.Complete("b")
	if len(all) < 2 {
		t.Skip("need at least two candidates to cycle")
	}

	m := testModel(t, 200, 40, ViewBoard)
	m = press(m, "c")
	m = press(m, ":b")

	first := acceptOnce(m, false)
	second := acceptTwice(m, false, false)
	if first == second {
		t.Fatalf("tab did not cycle, both gave %q", first)
	}
}

func TestCommentBarRejectsAnEmptyBody(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m = press(m, "c")
	m = press(m, "   ")

	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = out.(Model)
	if m.comment.active {
		t.Fatal("enter on a blank comment should close the bar")
	}
	if len(m.pending) != 0 {
		t.Fatal("a blank comment must not be submitted")
	}
}

func TestCommentBarSubmitsAndMarksPending(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	pr, _ := m.selectedPR()

	m = press(m, "c")
	m = press(m, "looks good :+1:")
	out, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = out.(Model)

	if m.comment.active {
		t.Fatal("enter should close the bar")
	}
	if m.pending[pr.Key()] != "commenting" {
		t.Fatalf("the row is not marked pending: %v", m.pending)
	}
	if cmd == nil {
		t.Fatal("enter should produce a submit command")
	}
	msg, ok := cmd().(actionMsg)
	if !ok || msg.err != nil || msg.verb != "commented on" {
		t.Fatalf("unexpected submit result: %#v", cmd())
	}
}

func TestCommentBarKeepsShortcodesUnderASCII(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m.theme = NewTheme("auto", true)
	m = press(m, "c")
	m = press(m, ":rocke")
	m.comment.accept(false)

	if got := m.comment.input.Value(); got != ":rocket:" {
		t.Fatalf("ascii mode should keep the shortcode, got %q", got)
	}
}

func TestCommentBarStaysInsideTheScreen(t *testing.T) {
	for _, width := range []int{60, 100, 200} {
		m := testModel(t, width, 24, ViewBoard)
		m = press(m, "c")
		m = press(m, "a very long comment that goes on and on and on :roc")

		for i, line := range strings.Split(m.View(), "\n") {
			if w := lipgloss.Width(line); w > width {
				t.Fatalf("at %d cols line %d is %d wide:\n%q", width, i, w, line)
			}
		}
	}
}

func TestApproveIsOfferedOnlyWhenYourApprovalIsMissing(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m.viewer = "dreuse"

	base := model.PullRequest{
		Repo: "r", Number: 1, Author: "someone", Title: "t",
		CreatedAt: nowMinusDays(2), UpdatedAt: nowMinusDays(1),
	}

	approved := base
	approved.Reviews = []model.Review{{
		Login: "dreuse", State: model.ReviewApproved, SubmittedAt: time.Now(),
	}}

	staleApproval := base
	staleApproval.UpdatedAt = time.Now()
	staleApproval.Reviews = []model.Review{{
		Login: "dreuse", State: model.ReviewApproved, SubmittedAt: nowMinusDays(3),
	}}

	own := base
	own.Author = "dreuse"

	draft := base
	draft.IsDraft = true

	changesRequested := base
	changesRequested.Reviews = []model.Review{{
		Login: "dreuse", State: model.ReviewChangesRequested, SubmittedAt: time.Now(),
	}}

	tests := []struct {
		name string
		pr   model.PullRequest
		want bool
	}{
		{"not reviewed yet", base, true},
		{"already approved", approved, false},
		{"approval went stale", staleApproval, true},
		{"your own pull request", own, false},
		{"a draft", draft, false},
		{"you asked for changes", changesRequested, true},
	}
	for _, tc := range tests {
		if got := m.canApprove(tc.pr); got != tc.want {
			t.Errorf("%s: canApprove = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestApproveChipTracksCanApprove(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m.viewer = "dreuse"

	pr := model.PullRequest{
		Repo: "r", Number: 1, Author: "someone", Title: "t",
		CreatedAt: nowMinusDays(2), UpdatedAt: nowMinusDays(1),
	}
	if !strings.Contains(m.actionChips(pr, 120), "approve") {
		t.Error("approve should be offered when your approval is missing")
	}

	pr.Reviews = []model.Review{{Login: "dreuse", State: model.ReviewApproved, SubmittedAt: time.Now()}}
	if strings.Contains(m.actionChips(pr, 120), "approve") {
		t.Error("approve should disappear once you have approved")
	}
}

func TestApproveKeyIsRefusedWhenAlreadyApproved(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m.viewer = "dreuse"
	for i := range m.prs {
		m.prs[i].Author = "someone"
		m.prs[i].Reviews = []model.Review{{
			Login: "dreuse", State: model.ReviewApproved, SubmittedAt: time.Now(),
		}}
		m.prs[i].UpdatedAt = nowMinusDays(1)
	}
	m.rebuild()

	out, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = out.(Model)
	if _, open := m.overlay(); open {
		t.Fatal("approving an already approved PR must not open a confirm")
	}
	if cmd == nil {
		t.Fatal("expected a toast explaining why")
	}
	if msg, ok := cmd().(toastMsg); !ok || !strings.Contains(msg.text, "already approved") {
		t.Fatalf("unexpected feedback: %#v", cmd())
	}
}

func TestSeedEmojiIsAlwaysAvailable(t *testing.T) {
	bare := NewEmojiSet()
	for _, name := range []string{"rocket", "tada", "+1", "eyes", "bug", "shipit"} {
		if !bare.Known(name) {
			t.Errorf(":%s: should be available before any download", name)
		}
	}
}

func TestDownloadedEmojiSupersedesTheSeed(t *testing.T) {
	set := NewEmojiSet(map[string]string{"rocket": "R", "brand_new": "N"})
	if glyph, _ := set.Glyph("rocket"); glyph != "R" {
		t.Errorf("the downloaded glyph should win, got %q", glyph)
	}
	if !set.Known("brand_new") {
		t.Error("a downloaded name should be completable")
	}
	if !set.Known("tada") {
		t.Error("seed names must survive a merge")
	}
}

func TestGithubOnlyEmojiStayAsShortcodes(t *testing.T) {
	for _, name := range []string{"shipit", "octocat"} {
		if !testEmoji.ImageOnly(name) {
			t.Errorf(":%s: should be known but have no unicode glyph", name)
		}
		if got := testEmoji.Render(name, false); got != ":"+name+":" {
			t.Errorf("testEmoji.Render(%q) = %q, want the literal shortcode for github to render", name, got)
		}
	}
}

func TestMixedCaseEmojiNamesResolve(t *testing.T) {
	matches, _ := testEmoji.Complete("morocco")
	if len(matches) == 0 {
		t.Skip("no mixed case names in this emoji set")
	}
	for _, name := range matches {
		glyph, ok := testEmoji.Glyph(name)
		if !ok || glyph == "" {
			t.Errorf("%q completed but does not resolve to a glyph", name)
		}
		if strings.HasPrefix(testEmoji.Render(name, false), ":") {
			t.Errorf("%q rendered as a raw shortcode instead of its glyph", name)
		}
	}
}

func TestCompletionLabelsAreNotDoubled(t *testing.T) {
	m := testModel(t, 220, 40, ViewBoard)
	m.emoji = testEmoji
	m = press(m, "c")
	m = press(m, ":morocc")

	if len(m.comment.candidates) == 0 {
		t.Fatal("expected candidates for morocc")
	}

	out := m.renderSuggestions(m.comment.suggestBox, m.comment.trigger != triggerEmoji)
	for _, c := range m.comment.candidates {
		if strings.Count(out, c.label) > 1 {
			t.Fatalf("%q appears twice in the completion strip:\n%s", c.label, out)
		}
	}
}

func TestEmojiCompletionRanksExactThenPrefix(t *testing.T) {
	matches, total := testEmoji.Complete("smile")
	if len(matches) == 0 {
		t.Fatal("no matches for smile")
	}
	if matches[0] != "smile" {
		t.Errorf("an exact name should rank first, got %q", matches[0])
	}
	if total < len(matches) {
		t.Errorf("total %d is below the returned %d", total, len(matches))
	}

	for i := 1; i < len(matches); i++ {
		if strings.HasPrefix(matches[i], "smile") && !strings.HasPrefix(matches[i-1], "smile") {
			t.Fatalf("a prefix match (%q) ranked below a substring match (%q)", matches[i], matches[i-1])
		}
	}
}

func TestEmojiCompletionPrefersTheShortestName(t *testing.T) {
	matches, _ := testEmoji.Complete("roc")
	if len(matches) < 2 {
		t.Fatal("expected several matches for roc")
	}
	if matches[0] != "rock" {
		t.Errorf("the shortest prefix match should rank first, got %q", matches[0])
	}
	if indexOf(matches, "rocket") < 0 {
		t.Error("rocket should still be reachable with tab")
	}
}

func indexOf(list []string, want string) int {
	for i, v := range list {
		if v == want {
			return i
		}
	}
	return -1
}

func TestEmojiCompletionIsCapped(t *testing.T) {
	big := make(map[string]string, maxCandidates*3)
	for i := 0; i < maxCandidates*3; i++ {
		big["aaa_"+itoa(i)] = "x"
	}
	set := NewEmojiSet(big)

	matches, total := set.Complete("aaa_")
	if len(matches) > maxCandidates {
		t.Fatalf("returned %d candidates, cap is %d", len(matches), maxCandidates)
	}
	if total <= len(matches) {
		t.Fatalf("total %d should exceed the capped %d", total, len(matches))
	}
}

func TestCommentBarShowsTheMatchCount(t *testing.T) {
	m := testModel(t, 220, 40, ViewBoard)
	m = press(m, "c")
	m = press(m, "#1")

	if m.comment.total <= maxCompletions {
		t.Skip("not enough matches to show a count")
	}
	if !strings.Contains(m.View(), itoa(m.comment.total)) {
		t.Fatal("the bar does not show how many matches there are")
	}
}

func TestOpenTokenDetectsEachTrigger(t *testing.T) {
	tests := []struct {
		text    string
		trigger byte
		prefix  string
		ok      bool
	}{
		{"", 0, "", false},
		{"plain words", 0, "", false},
		{":roc", triggerEmoji, "roc", true},
		{"ship :roc", triggerEmoji, "roc", true},
		{"#12", triggerReference, "12", true},
		{"see #12", triggerReference, "12", true},
		{"@alf", triggerMention, "alf", true},
		{"cc @alf", triggerMention, "alf", true},
		{"@", triggerMention, "", true},
		{"a:b", 0, "", false},
		{"mail@example", 0, "", false},
		{"issue#7", 0, "", false},
		{"#12 done", 0, "", false},
	}
	for _, tc := range tests {
		trigger, prefix, ok := openToken(tc.text)
		if ok != tc.ok || (ok && (trigger != tc.trigger || prefix != tc.prefix)) {
			t.Errorf("openToken(%q) = (%q, %q, %v), want (%q, %q, %v)",
				tc.text, string(trigger), prefix, ok, string(tc.trigger), tc.prefix, tc.ok)
		}
	}
}

func TestReferenceCompletionOffersPullRequestsAndIssues(t *testing.T) {
	m := testModel(t, 220, 40, ViewBoard)
	m = press(m, "c")
	m = press(m, "duplicate of #120")

	if len(m.comment.candidates) == 0 {
		t.Fatal("#120 should offer pull requests")
	}
	for _, c := range m.comment.candidates {
		if !strings.HasPrefix(c.insert, "#120") {
			t.Fatalf("candidate %q does not match the prefix", c.insert)
		}
		if c.detail == "" {
			t.Fatalf("candidate %q has no title to identify it", c.insert)
		}
	}

	m = pressKey(m, tea.KeyTab)
	if !strings.HasPrefix(m.comment.input.Value(), "duplicate of #120") {
		t.Fatalf("tab produced %q", m.comment.input.Value())
	}
}

func TestReferenceCompletionIncludesIssues(t *testing.T) {
	m := testModel(t, 220, 40, ViewBoard)
	if len(m.issues) == 0 {
		t.Fatal("the mock should provide issues")
	}
	want := "#" + itoa(m.issues[0].Number)

	m = press(m, "c")
	m = press(m, want)

	found := false
	for _, c := range m.comment.candidates {
		if c.insert == want {
			found = true
		}
	}
	if !found {
		t.Fatalf("issue %s is not offered, got %v", want, m.comment.candidates)
	}
}

func TestReferenceCompletionMatchesOnTitle(t *testing.T) {
	m := testModel(t, 220, 40, ViewBoard)
	m = press(m, "c")
	m = press(m, "#idempotente")

	if len(m.comment.candidates) == 0 {
		t.Fatal("a title fragment should find the pull request")
	}
	if m.comment.candidates[0].detail != "SQL idempotente" {
		t.Fatalf("got %q", m.comment.candidates[0].detail)
	}
}

func TestMentionCompletionOffersRepoPeople(t *testing.T) {
	m := testModel(t, 220, 40, ViewBoard)
	m = press(m, "c")
	m = press(m, "cc @alf")

	if len(m.comment.candidates) == 0 {
		t.Fatal("@alf should offer people")
	}
	if m.comment.candidates[0].insert != "@alfoltran" {
		t.Fatalf("got %q, want @alfoltran", m.comment.candidates[0].insert)
	}
	if m.comment.candidates[0].detail != "Alexandre Foltran" {
		t.Fatalf("the real name should be shown, got %q", m.comment.candidates[0].detail)
	}

	m = pressKey(m, tea.KeyTab)
	if got := m.comment.input.Value(); got != "cc @alfoltran" {
		t.Fatalf("tab produced %q", got)
	}
}

func TestMentionCompletionFallsBackToPeopleInTheData(t *testing.T) {
	m := testModel(t, 220, 40, ViewBoard)
	m.people = nil

	m = press(m, "c")
	m = press(m, "@hyako")

	if len(m.comment.candidates) == 0 {
		t.Fatal("authors seen in the pull requests should still be mentionable")
	}
	if m.comment.candidates[0].insert != "@HyakoV3" {
		t.Fatalf("got %q", m.comment.candidates[0].insert)
	}
}

func TestMentionCompletionKeepsGithubCasing(t *testing.T) {
	m := testModel(t, 220, 40, ViewBoard)
	m = press(m, "c")
	m = press(m, "@alexandre")

	if len(m.comment.candidates) == 0 {
		t.Fatal("expected a match for alexandre")
	}
	if got := m.comment.candidates[0].insert; got != "@AlexandreLages" {
		t.Fatalf("got %q, want the login as github spells it", got)
	}
}

func TestMentionAndReferenceCompletionCycle(t *testing.T) {
	m := testModel(t, 220, 40, ViewBoard)
	m = press(m, "c")
	m = press(m, "@a")

	if len(m.comment.candidates) < 2 {
		t.Skip("need two people to cycle")
	}
	first := acceptOnce(m, false)
	second := acceptTwice(m, false, false)
	if first == second {
		t.Fatalf("tab did not cycle mentions, both gave %q", first)
	}
}

func ghostOf(m Model) string {
	return m.comment.input.View()
}

func TestGhostTextCompletesTheOpenToken(t *testing.T) {
	tests := []struct {
		name  string
		typed string
		ghost string
	}{
		{"emoji", "ship it :roc", "ket:"},
		{"mention", "cc @alfo", "ltran"},
		{"reference", "see #120", "38"},
	}
	for _, tc := range tests {
		m := testModel(t, 220, 40, ViewBoard)
		m = press(m, "c")
		m = press(m, tc.typed)

		if len(m.comment.candidates) == 0 {
			t.Fatalf("%s: no candidates for %q", tc.name, tc.typed)
		}
		view := ghostOf(m)
		plain := stripANSI(view)
		if !strings.Contains(plain, tc.typed+tc.ghost) {
			t.Errorf("%s: expected ghost %q after %q, rendered:\n%q", tc.name, tc.ghost, tc.typed, plain)
		}
	}
}

func TestGhostTextIsDimmedNotLiteral(t *testing.T) {
	m := testModel(t, 220, 40, ViewBoard)
	m = press(m, "c")
	m = press(m, "cc @alfo")

	if got := m.comment.input.Value(); got != "cc @alfo" {
		t.Fatalf("the ghost must not enter the value, got %q", got)
	}

	body := strings.TrimSpace(m.comment.input.Value())
	if strings.Contains(body, "ltran") {
		t.Fatal("submitting now would send the ghost text")
	}
}

func TestGhostTextFollowsTheSelectedCandidate(t *testing.T) {
	m := testModel(t, 220, 40, ViewBoard)
	m = press(m, "c")
	m = press(m, "cc @a")

	if len(m.comment.candidates) < 2 {
		t.Skip("need two candidates")
	}
	first := m.comment.input.AvailableSuggestions()

	m.comment.index = 1
	m.comment.showGhost()
	second := m.comment.input.AvailableSuggestions()

	if len(first) == 0 || len(second) == 0 || first[0] == second[0] {
		t.Fatalf("the ghost did not follow the selection: %v then %v", first, second)
	}
}

func TestGhostTextDisappearsAfterAccepting(t *testing.T) {
	m := testModel(t, 220, 40, ViewBoard)
	m = press(m, "c")
	m = press(m, "cc @alfo")

	m = pressKey(m, tea.KeyTab)
	if got := m.comment.input.Value(); got != "cc @alfoltran" {
		t.Fatalf("tab produced %q", got)
	}
	if plain := stripANSI(ghostOf(m)); strings.Contains(plain, "cc @alfoltranltran") {
		t.Fatalf("the ghost survived acceptance: %q", plain)
	}
}

func TestGhostTextAbsentWhenNothingMatches(t *testing.T) {
	m := testModel(t, 220, 40, ViewBoard)
	m = press(m, "c")
	m = press(m, "@zzzznobody")

	if len(m.comment.candidates) != 0 {
		t.Fatalf("expected no candidates, got %v", m.comment.candidates)
	}
	if sugg := m.comment.input.AvailableSuggestions(); len(sugg) != 0 {
		t.Fatalf("a stale ghost is still set: %v", sugg)
	}
}

func TestCommentLineBreakKeys(t *testing.T) {
	for _, tc := range []struct {
		name string
		msg  tea.KeyMsg
	}{
		{"ctrl+j", tea.KeyMsg{Type: tea.KeyCtrlJ}},
		{"alt+enter", tea.KeyMsg{Type: tea.KeyEnter, Alt: true}},
	} {
		name := tc.name
		m := testModel(t, 200, 40, ViewBoard)
		m = press(m, "c")
		m = press(m, "first")

		out, _ := m.Update(tc.msg)
		m = out.(Model)
		m = press(m, "second")

		if !m.comment.active {
			t.Fatalf("%s sent the comment instead of breaking the line", name)
		}
		if got := m.comment.input.Value(); got != "first"+lineBreak+"second" {
			t.Fatalf("%s produced %q", name, got)
		}
		if got := commentBody(m.comment.input.Value()); got != "first\nsecond" {
			t.Fatalf("%s submits %q, want a real newline", name, got)
		}
	}
}

func TestLineBreakIsVisibleAndSingleLine(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m = press(m, "c")
	m = press(m, "one")
	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlJ})
	m = out.(Model)
	m = press(m, "two")

	if strings.Contains(m.comment.input.Value(), "\n") {
		t.Fatal("a raw newline in the buffer would break the single line bar")
	}
	if !strings.Contains(stripANSI(m.comment.input.View()), lineBreak) {
		t.Fatal("the break is not visible in the bar")
	}
	if n := len(strings.Split(m.View(), "\n")); n != 40 {
		t.Fatalf("the screen grew to %d lines", n)
	}
}

func TestLineBreakInsertsAtTheCursor(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m = press(m, "c")
	m = press(m, "ab")

	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = out.(Model)
	out, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlJ})
	m = out.(Model)

	if got := m.comment.input.Value(); got != "a"+lineBreak+"b" {
		t.Fatalf("break went to the wrong place: %q", got)
	}
	if got := m.comment.input.Position(); got != 2 {
		t.Fatalf("cursor is at %d, want 2", got)
	}
}

func TestCommentBodyTrimsAndKeepsInnerBreaks(t *testing.T) {
	tests := []struct{ raw, want string }{
		{"one" + lineBreak + "two", "one\ntwo"},
		{"one" + lineBreak + lineBreak + "two", "one\n\ntwo"},
		{lineBreak + "one" + lineBreak, "one"},
		{"one  " + lineBreak + "  two", "one\n  two"},
		{lineBreak + lineBreak, ""},
		{"   ", ""},
	}
	for _, tc := range tests {
		if got := commentBody(tc.raw); got != tc.want {
			t.Errorf("commentBody(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

func TestOnlyLineBreaksCountsAsEmpty(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m = press(m, "c")
	for i := 0; i < 3; i++ {
		out, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlJ})
		m = out.(Model)
	}

	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = out.(Model)
	if len(m.pending) != 0 {
		t.Fatal("a comment of only line breaks must not be sent")
	}
}

func TestCompletionWorksAfterALineBreak(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m = press(m, "c")
	m = press(m, "first")
	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlJ})
	m = out.(Model)
	m = press(m, "@alfo")

	if len(m.comment.candidates) == 0 {
		t.Fatal("a mention right after a break should complete")
	}
	m = pressKey(m, tea.KeyTab)
	if got := m.comment.input.Value(); got != "first"+lineBreak+"@alfoltran" {
		t.Fatalf("got %q", got)
	}
}

func TestMultilineCommentIsSubmitted(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	pr, _ := m.selectedPR()

	m = press(m, "c")
	m = press(m, "looks good")
	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlJ})
	m = out.(Model)
	m = press(m, "ship it")

	out, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = out.(Model)
	if m.pending[pr.Key()] != "commenting" {
		t.Fatal("the multi line comment was not submitted")
	}
	if msg, ok := cmd().(actionMsg); !ok || msg.err != nil {
		t.Fatalf("unexpected result: %#v", cmd())
	}
}

func TestShiftEnterArrivesAsTheTerminalSendsIt(t *testing.T) {
	tests := []struct {
		terminal string
		sends    string
		msg      tea.KeyMsg
	}{
		{"iTerm2 (claude /terminal-setup)", "LF 0x0a", tea.KeyMsg{Type: tea.KeyCtrlJ}},
		{"VS Code (claude /terminal-setup)", "ESC CR 0x1b 0x0d", tea.KeyMsg{Type: tea.KeyEnter, Alt: true}},
	}
	for _, tc := range tests {
		m := testModel(t, 200, 40, ViewBoard)
		m = press(m, "c")
		m = press(m, "before")

		out, _ := m.Update(tc.msg)
		m = out.(Model)
		m = press(m, "after")

		if !m.comment.active {
			t.Fatalf("%s sends %s and it submitted instead of breaking", tc.terminal, tc.sends)
		}
		if got := commentBody(m.comment.input.Value()); got != "before\nafter" {
			t.Fatalf("%s: got %q", tc.terminal, got)
		}
	}
}

func TestPlainEnterStillSends(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m = press(m, "c")
	m = press(m, "ship it")

	out, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = out.(Model)
	if m.comment.active {
		t.Fatal("a bare enter must still send")
	}
	if cmd == nil {
		t.Fatal("no submit command")
	}
}
