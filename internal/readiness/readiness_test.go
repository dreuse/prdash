package readiness

import (
	"testing"

	"github.com/dreuse/prdash/internal/model"
)

func pr(mutate ...func(*model.PullRequest)) model.PullRequest {
	p := model.PullRequest{
		Repo:      "owner/repo",
		Number:    1,
		Title:     "a change",
		Author:    "someone",
		Mergeable: model.MergeableYes,
		Approvals: 1,
		Checks:    []model.Check{{Name: "build", State: model.CheckPassed}},
	}
	for _, m := range mutate {
		m(&p)
	}
	return p
}

func draft(p *model.PullRequest)            { p.IsDraft = true }
func conflicting(p *model.PullRequest)      { p.Mergeable = model.MergeableConflict }
func unknownMerge(p *model.PullRequest)     { p.Mergeable = model.MergeableUnknown }
func noApprovals(p *model.PullRequest)      { p.Approvals = 0 }
func requestedChanges(p *model.PullRequest) { p.ChangesRequested = 1 }
func failingChecks(p *model.PullRequest) {
	p.Checks = []model.Check{{Name: "build", State: model.CheckPassed}, {Name: "test", State: model.CheckFailed}}
}
func runningChecks(p *model.PullRequest) {
	p.Checks = []model.Check{{Name: "build", State: model.CheckPassed}, {Name: "test", State: model.CheckRunning}}
}
func noChecks(p *model.PullRequest) { p.Checks = nil }
func neutralChecks(p *model.PullRequest) {
	p.Checks = []model.Check{{Name: "lint", State: model.CheckNeutral}}
}
func behind(p *model.PullRequest) { p.BehindBy = 12 }

func protectionBlocked(p *model.PullRequest) { p.MergeStateStatus = "BLOCKED" }

func TestAwaitingApproval(t *testing.T) {
	tests := []struct {
		name   string
		policy Policy
		pr     model.PullRequest
		want   bool
	}{
		{"protection only is just waiting on reviews", DefaultPolicy(), pr(protectionBlocked), true},
		{"missing approvals under protection still waits", Policy{RequiredApprovals: 2}, pr(protectionBlocked), true},
		{"conflicts are a real block", DefaultPolicy(), pr(protectionBlocked, conflicting), false},
		{"failing checks are a real block", DefaultPolicy(), pr(protectionBlocked, failingChecks), false},
		{"behind under strict policy is a real block", Policy{RequiredApprovals: 1, BehindBlocks: true}, pr(protectionBlocked, behind), false},
		{"a ready pr is not waiting", DefaultPolicy(), pr(), false},
		{"a needs-review pr is not in the blocked lane", DefaultPolicy(), pr(noApprovals), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.policy.AwaitingApproval(tt.pr); got != tt.want {
				t.Fatalf("AwaitingApproval = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClassify(t *testing.T) {
	tests := []struct {
		name   string
		policy Policy
		pr     model.PullRequest
		want   model.Column
	}{
		{"clean pr with one approval is ready", DefaultPolicy(), pr(), model.ColReadyToMerge},
		{"draft wins over everything", DefaultPolicy(), pr(draft, conflicting, failingChecks), model.ColDraft},
		{"conflict wins over requested changes", DefaultPolicy(), pr(conflicting, requestedChanges), model.ColBlocked},
		{"requested changes wins over failing checks", DefaultPolicy(), pr(requestedChanges, failingChecks), model.ColChangesRequested},
		{"failing check beats approvals", DefaultPolicy(), pr(failingChecks), model.ColBlocked},
		{"running checks", DefaultPolicy(), pr(runningChecks), model.ColCIRunning},
		{"failing beats running", DefaultPolicy(), pr(func(p *model.PullRequest) {
			p.Checks = []model.Check{{State: model.CheckRunning}, {State: model.CheckFailed}}
		}), model.ColBlocked},
		{"no approvals needs review", DefaultPolicy(), pr(noApprovals), model.ColNeedsReview},
		{"no checks at all is still ready", DefaultPolicy(), pr(noChecks), model.ColReadyToMerge},
		{"neutral checks do not block", DefaultPolicy(), pr(neutralChecks), model.ColReadyToMerge},
		{"unknown mergeability is not a conflict", DefaultPolicy(), pr(unknownMerge), model.ColReadyToMerge},
		{"behind base does not block by default", DefaultPolicy(), pr(behind), model.ColReadyToMerge},
		{"behind base blocks under strict policy", Policy{RequiredApprovals: 1, BehindBlocks: true}, pr(behind), model.ColBlocked},
		{"exactly the required approvals is enough", Policy{RequiredApprovals: 2}, pr(func(p *model.PullRequest) { p.Approvals = 2 }), model.ColReadyToMerge},
		{"one short of required approvals is not", Policy{RequiredApprovals: 2}, pr(func(p *model.PullRequest) { p.Approvals = 1 }), model.ColNeedsReview},
		{"zero required approvals never blocks on reviews", Policy{}, pr(noApprovals), model.ColReadyToMerge},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.policy.Classify(tt.pr); got != tt.want {
				t.Fatalf("Classify() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReadyToMergeRequiresEveryCondition(t *testing.T) {
	p := DefaultPolicy()
	if !p.ReadyToMerge(pr()) {
		t.Fatalf("a clean approved pr with passing checks should be ready: %v", p.Blockers(pr()))
	}
	for _, tt := range []struct {
		name   string
		mutate func(*model.PullRequest)
	}{
		{"draft", draft},
		{"conflicts", conflicting},
		{"requested changes", requestedChanges},
		{"failing checks", failingChecks},
		{"running checks", runningChecks},
		{"missing approvals", noApprovals},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if p.ReadyToMerge(pr(tt.mutate)) {
				t.Fatalf("%s must not be ready to merge", tt.name)
			}
		})
	}
}

func TestBlockersListsEveryReason(t *testing.T) {
	got := DefaultPolicy().Blockers(pr(draft, conflicting, requestedChanges, failingChecks, noApprovals))
	want := []Blocker{BlockerDraft, BlockerConflict, BlockerChangesRequested, BlockerChecksFailed, BlockerApprovals}
	if len(got) != len(want) {
		t.Fatalf("Blockers() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Blockers()[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestGroupCoversEveryColumn(t *testing.T) {
	groups := DefaultPolicy().Group([]model.PullRequest{pr(), pr(draft), pr(noApprovals)})
	if len(groups) != len(model.ActionFirstColumns) {
		t.Fatalf("Group() returned %d columns, want %d", len(groups), len(model.ActionFirstColumns))
	}
	if len(groups[model.ColReadyToMerge]) != 1 || len(groups[model.ColDraft]) != 1 || len(groups[model.ColNeedsReview]) != 1 {
		t.Fatalf("unexpected grouping: %v", groups)
	}
	if len(groups[model.ColBlocked]) != 0 {
		t.Fatalf("blocked column should be empty, got %v", groups[model.ColBlocked])
	}
}

func withLanes(t *testing.T, defs []model.LaneDef) {
	t.Helper()
	model.SetLanes(defs)
	t.Cleanup(func() { model.SetLanes(nil) })
}

func TestClassifyCustomLanesFirstMatchWins(t *testing.T) {
	withLanes(t, []model.LaneDef{
		{Name: "BROKEN", Rule: "is:conflict,failing"},
		{Name: "DRAFTS", Rule: "is:draft"},
		{Name: "MERGE NOW", Rule: "is:ready"},
		{Name: "EVERYTHING ELSE", Rule: model.CatchAllRule},
	})

	p := DefaultPolicy()
	tests := []struct {
		name string
		in   model.PullRequest
		want model.Column
	}{
		{"conflict beats draft", pr(draft, conflicting), 0},
		{"draft", pr(draft), 1},
		{"ready", pr(), 2},
		{"unmatched falls to the last lane", pr(noApprovals), 3},
	}
	for _, tc := range tests {
		if got := p.Classify(tc.in); got != tc.want {
			t.Errorf("%s: Classify() = %d (%s), want %d", tc.name, got, got, tc.want)
		}
	}
}

func TestUnmatchedPullRequestsGetTheirOwnLane(t *testing.T) {
	withLanes(t, []model.LaneDef{
		{Name: "DRAFTS", Rule: "is:draft"},
		{Name: "BROKEN", Rule: "is:conflict"},
	})
	lanes := model.Lanes()
	if len(lanes) != 3 || lanes[2].Name != model.OtherLaneName {
		t.Fatalf("lanes = %v, want an appended %s lane", lanes, model.OtherLaneName)
	}
	if got := DefaultPolicy().Classify(pr()); got != 2 {
		t.Fatalf("Classify() = %d, want the %s lane (2)", got, model.OtherLaneName)
	}
	if got := DefaultPolicy().Classify(pr(draft)); got != 0 {
		t.Fatalf("Classify() = %d, want DRAFTS (0)", got)
	}
}

func TestAnExplicitCatchAllSuppressesTheOtherLane(t *testing.T) {
	withLanes(t, []model.LaneDef{
		{Name: "DRAFTS", Rule: "is:draft"},
		{Name: "REST", Rule: model.CatchAllRule},
	})
	if lanes := model.Lanes(); len(lanes) != 2 {
		t.Fatalf("lanes = %v, want no extra lane when the user wrote their own catch-all", lanes)
	}
	if got := DefaultPolicy().Classify(pr()); got != 1 {
		t.Fatalf("Classify() = %d, want REST (1)", got)
	}
}

func TestASingleNarrowLaneDoesNotSwallowEverything(t *testing.T) {
	withLanes(t, []model.LaneDef{{Name: "GOIAS", Rule: `label:"Solicitante: SEAD-GO"`}})
	tagged := pr(func(p *model.PullRequest) { p.Labels = []string{"Solicitante: SEAD-GO"} })

	if got := DefaultPolicy().Classify(tagged); got != 0 {
		t.Errorf("a labelled pull request belongs in GOIAS, got lane %d", got)
	}
	if got := DefaultPolicy().Classify(pr()); got != 1 {
		t.Errorf("everything else belongs in %s, got lane %d", model.OtherLaneName, got)
	}
}

func TestLaneRuleErrorExplainsItself(t *testing.T) {
	tests := []struct {
		rule string
		want string
	}{
		{"is:ready", ""},
		{model.CatchAllRule, ""},
		{`label:"Solicitante: SEAD-GO"`, ""},
		{"", "a lane needs a rule"},
		{"state:blocked", `state: is what this lane decides, use another key`},
		{"label: Solicitante: SEAD-GO", `do not understand "label:", try quoting the value`},
		{"is:purple", `do not understand "is:purple", try quoting the value`},
	}
	for _, tc := range tests {
		if got := LaneRuleError(tc.rule); got != tc.want {
			t.Errorf("LaneRuleError(%q) = %q, want %q", tc.rule, got, tc.want)
		}
	}
}

func TestCustomLaneRulesResolveTheViewer(t *testing.T) {
	withLanes(t, []model.LaneDef{
		{Name: "ON ME", Rule: "reviewer:@me"},
		{Name: "REST", Rule: model.CatchAllRule},
	})
	mine := pr(func(p *model.PullRequest) { p.RequestedReviewers = []string{"jchen"} })

	p := Policy{RequiredApprovals: 1, Viewer: "jchen"}
	if got := p.Classify(mine); got != 0 {
		t.Fatalf("Classify() = %d, want 0: @me must resolve to the viewer", got)
	}
	if got := DefaultPolicy().Classify(mine); got != 1 {
		t.Fatalf("Classify() = %d, want 1: with no viewer @me must not match", got)
	}
}

func TestGroupCoversEveryCustomLane(t *testing.T) {
	withLanes(t, []model.LaneDef{
		{Name: "DRAFTS", Rule: "is:draft"},
		{Name: "REST", Rule: model.CatchAllRule},
	})
	groups := DefaultPolicy().Group([]model.PullRequest{pr(), pr(draft)})
	if len(groups) != 2 {
		t.Fatalf("Group() returned %d lanes, want 2", len(groups))
	}
	if len(groups[0]) != 1 || len(groups[1]) != 1 {
		t.Fatalf("unexpected grouping: %v", groups)
	}
}

func TestValidLaneRuleRejectsState(t *testing.T) {
	tests := []struct {
		rule string
		ok   bool
	}{
		{"is:ready", true},
		{model.CatchAllRule, true},
		{"reviewer:@me -is:draft", true},
		{"state:blocked", false},
		{"-state:draft", false},
		{"is:purple", false},
		{"", false},
	}
	for _, tc := range tests {
		if got := ValidLaneRule(tc.rule); got != tc.ok {
			t.Errorf("ValidLaneRule(%q) = %v, want %v", tc.rule, got, tc.ok)
		}
	}
}
