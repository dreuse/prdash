package readiness

import "github.com/dreuse/prdash/internal/model"

const DefaultRequiredApprovals = 1

type Policy struct {
	RequiredApprovals int
	BehindBlocks      bool
}

func DefaultPolicy() Policy {
	return Policy{RequiredApprovals: DefaultRequiredApprovals}
}

type Blocker string

const (
	BlockerDraft            Blocker = "is a draft"
	BlockerConflict         Blocker = "has merge conflicts"
	BlockerChangesRequested Blocker = "has requested changes"
	BlockerChecksFailed     Blocker = "has failing checks"
	BlockerChecksRunning    Blocker = "has checks still running"
	BlockerApprovals        Blocker = "needs more approvals"
	BlockerBehind           Blocker = "is behind the base branch"
	BlockerProtection       Blocker = "is blocked by branch protection"
)

func (p Policy) Blockers(pr model.PullRequest) []Blocker {
	var out []Blocker
	if pr.IsDraft {
		out = append(out, BlockerDraft)
	}
	if pr.HasConflicts() {
		out = append(out, BlockerConflict)
	}
	if pr.ChangesRequested > 0 {
		out = append(out, BlockerChangesRequested)
	}
	switch pr.ChecksState() {
	case model.CheckFailed:
		out = append(out, BlockerChecksFailed)
	case model.CheckRunning:
		out = append(out, BlockerChecksRunning)
	}
	if pr.Approvals < p.RequiredApprovals {
		out = append(out, BlockerApprovals)
	}
	if p.BehindBlocks && pr.BehindBy > 0 {
		out = append(out, BlockerBehind)
	}
	if pr.Blocked() {
		out = append(out, BlockerProtection)
	}
	return out
}

func (p Policy) ReadyToMerge(pr model.PullRequest) bool {
	return len(p.Blockers(pr)) == 0
}

func (p Policy) Classify(pr model.PullRequest) model.Column {
	switch {
	case pr.IsDraft:
		return model.ColDraft
	case pr.HasConflicts():
		return model.ColBlocked
	case pr.ChangesRequested > 0:
		return model.ColChangesRequested
	case pr.ChecksState() == model.CheckFailed:
		return model.ColBlocked
	case pr.ChecksState() == model.CheckRunning:
		return model.ColCIRunning
	case pr.Blocked():
		return model.ColBlocked
	case p.BehindBlocks && pr.BehindBy > 0:
		return model.ColBlocked
	case p.ReadyToMerge(pr):
		return model.ColReadyToMerge
	default:
		return model.ColNeedsReview
	}
}

func (p Policy) Group(prs []model.PullRequest) map[model.Column][]model.PullRequest {
	out := make(map[model.Column][]model.PullRequest, len(model.ActionFirstColumns))
	for _, col := range model.ActionFirstColumns {
		out[col] = nil
	}
	for _, pr := range prs {
		col := p.Classify(pr)
		out[col] = append(out[col], pr)
	}
	return out
}
