package readiness

import (
	"strconv"
	"strings"

	"github.com/dreuse/prdash/internal/model"
)

const DefaultRequiredApprovals = 1

type Policy struct {
	RequiredApprovals int
	BehindBlocks      bool
	Viewer            string
}

func DefaultPolicy() Policy {
	return Policy{RequiredApprovals: DefaultRequiredApprovals}
}

type Blocker string

const (
	BlockerDraft             Blocker = "is a draft"
	BlockerConflict          Blocker = "has merge conflicts"
	BlockerChangesRequested  Blocker = "has requested changes"
	BlockerChecksFailed      Blocker = "has failing checks"
	BlockerChecksRunning     Blocker = "has checks still running"
	BlockerApprovals         Blocker = "needs more approvals"
	BlockerBehind            Blocker = "is behind the base branch"
	BlockerProtection        Blocker = "is blocked by branch protection"
	BlockerMergeStatePending Blocker = "is still being checked by github"
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
	if pr.MergeStatePending() {
		out = append(out, BlockerMergeStatePending)
	}
	return out
}

func (p Policy) ReadyToMerge(pr model.PullRequest) bool {
	return len(p.Blockers(pr)) == 0
}

func ValidLaneRule(rule string) bool { return LaneRuleError(rule) == "" }

func LaneRuleError(rule string) string {
	rule = strings.TrimSpace(rule)
	if rule == "" {
		return "a lane needs a rule"
	}
	if rule == model.CatchAllRule {
		return ""
	}
	f := model.ParseFilter(rule)
	if f.Empty() {
		return "a lane needs a rule"
	}
	for _, t := range f.Tokens {
		if t.Key == "state" {
			return "state: is what this lane decides, use another key"
		}
		if !t.Valid {
			return "do not understand " + strconv.Quote(t.Text) + ", try quoting the value"
		}
	}
	return ""
}

func (p Policy) classifyByRules(pr model.PullRequest, lanes []model.LaneDef) model.Column {
	ctx := model.FilterContext{Viewer: p.Viewer, Ready: p.ReadyToMerge(pr)}
	for i, lane := range lanes {
		if strings.TrimSpace(lane.Rule) == model.CatchAllRule {
			return model.Column(i)
		}
		if !ValidLaneRule(lane.Rule) {
			continue
		}
		if model.ParseFilter(lane.Rule).Match(pr, ctx) {
			return model.Column(i)
		}
	}
	return model.Column(len(lanes) - 1)
}

func (p Policy) Classify(pr model.PullRequest) model.Column {
	if model.CustomLanes() {
		return p.classifyByRules(pr, model.Lanes())
	}
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

func (p Policy) AwaitingApproval(pr model.PullRequest) bool {
	protected := false
	for _, b := range p.Blockers(pr) {
		switch b {
		case BlockerProtection:
			protected = true
		case BlockerApprovals:
		default:
			return false
		}
	}
	return protected
}

func (p Policy) Group(prs []model.PullRequest) map[model.Column][]model.PullRequest {
	cols := model.AllColumns()
	out := make(map[model.Column][]model.PullRequest, len(cols))
	for _, col := range cols {
		out[col] = nil
	}
	for _, pr := range prs {
		col := p.Classify(pr)
		out[col] = append(out[col], pr)
	}
	return out
}
