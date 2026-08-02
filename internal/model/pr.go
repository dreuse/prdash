package model

import (
	"fmt"
	"time"
)

type Column int

const (
	ColDraft Column = iota
	ColNeedsReview
	ColChangesRequested
	ColCIRunning
	ColReadyToMerge
	ColBlocked
)

var Columns = []Column{
	ColDraft,
	ColNeedsReview,
	ColChangesRequested,
	ColCIRunning,
	ColReadyToMerge,
	ColBlocked,
}

func (c Column) String() string {
	switch c {
	case ColDraft:
		return "Draft"
	case ColNeedsReview:
		return "Needs review"
	case ColChangesRequested:
		return "Changes requested"
	case ColCIRunning:
		return "CI running"
	case ColReadyToMerge:
		return "Ready to merge"
	case ColBlocked:
		return "Blocked"
	}
	return "Unknown"
}

type Mergeable string

const (
	MergeableYes      Mergeable = "MERGEABLE"
	MergeableConflict Mergeable = "CONFLICTING"
	MergeableUnknown  Mergeable = "UNKNOWN"
)

type CheckState string

const (
	CheckPassed  CheckState = "passed"
	CheckFailed  CheckState = "failed"
	CheckRunning CheckState = "running"
	CheckNeutral CheckState = "neutral"
)

type Check struct {
	Name  string
	State CheckState
	URL   string
}

type PullRequest struct {
	Repo      string
	Number    int
	Title     string
	Author    string
	URL       string
	CreatedAt time.Time
	UpdatedAt time.Time

	IsDraft   bool
	Mergeable Mergeable
	BehindBy  int
	BaseRef   string
	HeadRef   string
	HeadOwner string

	Approvals          int
	ChangesRequested   int
	RequestedReviewers []string

	Checks []Check
}

func (p PullRequest) Age() time.Duration { return time.Since(p.CreatedAt) }

func (p PullRequest) HasConflicts() bool { return p.Mergeable == MergeableConflict }

func (p PullRequest) ChecksState() CheckState {
	state := CheckPassed
	for _, c := range p.Checks {
		switch c.State {
		case CheckFailed:
			return CheckFailed
		case CheckRunning:
			state = CheckRunning
		}
	}
	return state
}

func ShortAge(d time.Duration) string {
	switch {
	case d >= 24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	case d >= time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d >= time.Minute:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	default:
		return "now"
	}
}
