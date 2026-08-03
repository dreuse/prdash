package model

import "sort"

const (
	urgencyFailing         = 40
	urgencyReviewRequested = 30
	urgencyConflict        = 20
	urgencyApprovedGreen   = 10
	urgencyBehindCap       = 20
	urgencyBehindDivisor   = 10
	urgencyAgeCap          = 15
	urgencyAgeDivisor      = 10
)

func Urgency(p PullRequest, viewer string, requiredApprovals int) int {
	score := 0
	if p.CheckCounts().Failed > 0 {
		score += urgencyFailing
	}
	if p.RequestedFrom(viewer) {
		score += urgencyReviewRequested
	}
	if p.HasConflicts() {
		score += urgencyConflict
	}
	score += minInt(urgencyBehindCap, p.BehindBy/urgencyBehindDivisor)
	score += minInt(urgencyAgeCap, int(p.Age().Hours()/24)/urgencyAgeDivisor)
	if p.Approvals >= requiredApprovals && p.Green() && !p.HasConflicts() {
		score += urgencyApprovedGreen
	}
	return score
}

type SortMode int

const (
	SortUrgency SortMode = iota
	SortUpdated
	SortAge
	SortNumber
)

var SortModes = []SortMode{SortUrgency, SortUpdated, SortAge, SortNumber}

func (s SortMode) String() string {
	switch s {
	case SortUpdated:
		return "updated"
	case SortAge:
		return "age"
	case SortNumber:
		return "number"
	}
	return "urgency"
}

func SortModeBySlug(s string) (SortMode, bool) {
	for _, m := range SortModes {
		if m.String() == s {
			return m, true
		}
	}
	return SortUrgency, false
}

func (s SortMode) Next() SortMode { return SortMode((int(s) + 1) % len(SortModes)) }

func Sort(prs []PullRequest, mode SortMode, viewer string, requiredApprovals int) {
	if mode != SortUrgency {
		sort.SliceStable(prs, func(i, j int) bool {
			a, b := prs[i], prs[j]
			switch mode {
			case SortUpdated:
				return a.UpdatedAt.After(b.UpdatedAt)
			case SortAge:
				return a.CreatedAt.Before(b.CreatedAt)
			}
			return a.Number > b.Number
		})
		return
	}

	scores := make(map[Key]int, len(prs))
	for _, pr := range prs {
		scores[pr.Key()] = Urgency(pr, viewer, requiredApprovals)
	}
	sort.SliceStable(prs, func(i, j int) bool {
		a, b := prs[i], prs[j]
		if ua, ub := scores[a.Key()], scores[b.Key()]; ua != ub {
			return ua > ub
		}
		return a.UpdatedAt.After(b.UpdatedAt)
	})
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
