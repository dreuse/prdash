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
	if len(groups) != len(model.Columns) {
		t.Fatalf("Group() returned %d columns, want %d", len(groups), len(model.Columns))
	}
	if len(groups[model.ColReadyToMerge]) != 1 || len(groups[model.ColDraft]) != 1 || len(groups[model.ColNeedsReview]) != 1 {
		t.Fatalf("unexpected grouping: %v", groups)
	}
	if len(groups[model.ColBlocked]) != 0 {
		t.Fatalf("blocked column should be empty, got %v", groups[model.ColBlocked])
	}
}
