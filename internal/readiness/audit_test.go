package readiness

import (
	"testing"

	"github.com/dreuse/prdash/internal/model"
)

func protectedBranch(p *model.PullRequest) { p.MergeStateStatus = "BLOCKED" }

func TestBranchProtectionKeepsAPullRequestOutOfReadyToMerge(t *testing.T) {
	green := pr()
	if got := DefaultPolicy().Classify(green); got != model.ColReadyToMerge {
		t.Fatalf("precondition: a green pull request is ready, got %v", got)
	}

	blocked := pr(protectedBranch)
	if got := DefaultPolicy().Classify(blocked); got != model.ColBlocked {
		t.Errorf("branch protection must block, got %v", got)
	}
	if DefaultPolicy().ReadyToMerge(blocked) {
		t.Error("a protected branch is not ready to merge")
	}
}

func TestBranchProtectionIsReportedAsABlocker(t *testing.T) {
	for _, b := range DefaultPolicy().Blockers(pr(protectedBranch)) {
		if b == BlockerProtection {
			return
		}
	}
	t.Error("the protection blocker must reach the detail pane")
}

func TestAnUncomputedMergeStateDoesNotClaimReady(t *testing.T) {
	unknown := pr(func(p *model.PullRequest) {
		p.Mergeable = model.MergeableUnknown
		p.MergeStateStatus = "UNKNOWN"
	})
	if DefaultPolicy().ReadyToMerge(unknown) {
		t.Error("github has not finished computing the merge, do not offer to merge it")
	}
}
