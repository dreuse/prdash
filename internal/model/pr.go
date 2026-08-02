package model

import (
	"fmt"
	"strings"
	"time"
)

type Column int

const (
	ColReadyToMerge Column = iota
	ColNeedsReview
	ColChangesRequested
	ColCIRunning
	ColBlocked
	ColDraft
)

var ActionFirstColumns = []Column{
	ColReadyToMerge,
	ColNeedsReview,
	ColChangesRequested,
	ColCIRunning,
	ColBlocked,
	ColDraft,
}

var PipelineColumns = []Column{
	ColDraft,
	ColNeedsReview,
	ColChangesRequested,
	ColCIRunning,
	ColBlocked,
	ColReadyToMerge,
}

func (c Column) String() string {
	switch c {
	case ColReadyToMerge:
		return "READY TO MERGE"
	case ColNeedsReview:
		return "NEEDS REVIEW"
	case ColChangesRequested:
		return "CHANGES REQUESTED"
	case ColCIRunning:
		return "CI RUNNING"
	case ColBlocked:
		return "BLOCKED"
	case ColDraft:
		return "DRAFT"
	}
	return "UNKNOWN"
}

func (c Column) Slug() string {
	return strings.ReplaceAll(strings.ToLower(c.String()), " ", "-")
}

func ColumnBySlug(s string) (Column, bool) {
	for _, c := range ActionFirstColumns {
		if c.Slug() == s {
			return c, true
		}
	}
	return 0, false
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
	Name      string
	State     CheckState
	URL       string
	StartedAt time.Time
}

type ReviewState string

const (
	ReviewApproved         ReviewState = "approved"
	ReviewChangesRequested ReviewState = "changes_requested"
	ReviewCommented        ReviewState = "commented"
	ReviewPending          ReviewState = "pending"
)

type Review struct {
	Login       string
	State       ReviewState
	SubmittedAt time.Time
}

type PullRequest struct {
	Repo      string
	Number    int
	Title     string
	Author    string
	URL       string
	CreatedAt time.Time
	UpdatedAt time.Time

	IsDraft          bool
	Mergeable        Mergeable
	MergeStateStatus string
	BehindBy         int
	BaseRef          string
	HeadRef          string
	HeadOwner        string

	Approvals          int
	ChangesRequested   int
	RequestedReviewers []string
	Reviews            []Review
	Assignees          []string

	Labels    []string
	Additions int
	Deletions int
	Changed   int

	Checks []Check
}

func (p PullRequest) Age() time.Duration { return time.Since(p.CreatedAt) }

func (p PullRequest) Idle() time.Duration { return time.Since(p.UpdatedAt) }

func (p PullRequest) HasConflicts() bool { return p.Mergeable == MergeableConflict }

func (p PullRequest) Blocked() bool { return p.MergeStateStatus == "BLOCKED" }

func (p PullRequest) Key() Key { return Key{Repo: p.Repo, Number: p.Number} }

type Key struct {
	Repo   string
	Number int
}

func (k Key) Zero() bool { return k.Number == 0 }

type CheckCounts struct {
	Passed  int
	Failed  int
	Running int
	Neutral int
	Total   int
}

func (p PullRequest) CheckCounts() CheckCounts {
	var c CheckCounts
	for _, chk := range p.Checks {
		c.Total++
		switch chk.State {
		case CheckPassed:
			c.Passed++
		case CheckFailed:
			c.Failed++
		case CheckRunning:
			c.Running++
		default:
			c.Neutral++
		}
	}
	return c
}

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

func (p PullRequest) Green() bool {
	c := p.CheckCounts()
	return c.Failed == 0 && c.Running == 0
}

type Stake int

const (
	StakeNone Stake = iota
	StakeAuthor
	StakeRequested
	StakeApproved
	StakeChangesRequested
)

func (s Stake) String() string {
	switch s {
	case StakeAuthor:
		return "your PR"
	case StakeRequested:
		return "you were requested"
	case StakeApproved:
		return "you approved"
	case StakeChangesRequested:
		return "you requested changes"
	}
	return ""
}

func (p PullRequest) StakeFor(viewer string) Stake {
	if viewer == "" {
		return StakeNone
	}
	for _, r := range p.Reviews {
		if !strings.EqualFold(r.Login, viewer) {
			continue
		}
		switch r.State {
		case ReviewApproved:
			return StakeApproved
		case ReviewChangesRequested:
			return StakeChangesRequested
		}
	}
	for _, r := range p.RequestedReviewers {
		if strings.EqualFold(r, viewer) {
			return StakeRequested
		}
	}
	if strings.EqualFold(p.Author, viewer) {
		return StakeAuthor
	}
	return StakeNone
}

func (p PullRequest) ReviewedBy(viewer string) (Review, bool) {
	for _, r := range p.Reviews {
		if strings.EqualFold(r.Login, viewer) {
			return r, true
		}
	}
	return Review{}, false
}

func (p PullRequest) ReviewStale(viewer string) bool {
	r, ok := p.ReviewedBy(viewer)
	if !ok || r.SubmittedAt.IsZero() {
		return false
	}
	return p.UpdatedAt.After(r.SubmittedAt)
}

func (p PullRequest) AssignedTo(login string) bool {
	if login == "" {
		return false
	}
	for _, a := range p.Assignees {
		if strings.EqualFold(a, login) {
			return true
		}
	}
	return false
}

func (p PullRequest) RequestedFrom(viewer string) bool {
	if viewer == "" {
		return false
	}
	for _, r := range p.RequestedReviewers {
		if strings.EqualFold(r, viewer) {
			return true
		}
	}
	return false
}

func ShortAge(d time.Duration) string {
	switch {
	case d >= 24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	case d >= time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d >= time.Minute:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d >= time.Second:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	default:
		return "now"
	}
}
