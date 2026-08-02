package model

import "testing"

func TestUrgencyScoring(t *testing.T) {
	base := PullRequest{CreatedAt: daysAgo(0), UpdatedAt: daysAgo(0)}

	failing := base
	failing.Checks = []Check{{State: CheckFailed}}

	requested := base
	requested.RequestedReviewers = []string{"dreuse"}

	conflict := base
	conflict.Mergeable = MergeableConflict

	behind := base
	behind.BehindBy = 300

	old := base
	old.CreatedAt = daysAgo(300)

	approvedGreen := base
	approvedGreen.Approvals = 1
	approvedGreen.Mergeable = MergeableYes

	tests := []struct {
		name string
		pr   PullRequest
		want int
	}{
		{"an unremarkable PR scores nothing", base, 0},
		{"failing checks dominate", failing, 40},
		{"review requested from the viewer", requested, 30},
		{"conflict", conflict, 20},
		{"behind is capped at 20", behind, 20},
		{"age is capped at 15", old, 15},
		{"approved and green", approvedGreen, 10},
	}
	for _, tc := range tests {
		if got := Urgency(tc.pr, "dreuse", 1); got != tc.want {
			t.Errorf("%s: Urgency = %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestUrgencyCombines(t *testing.T) {
	pr := PullRequest{
		CreatedAt:          daysAgo(60),
		Mergeable:          MergeableConflict,
		BehindBy:           40,
		RequestedReviewers: []string{"dreuse"},
		Checks:             []Check{{State: CheckFailed}},
	}
	want := 40 + 30 + 20 + 4 + 6
	if got := Urgency(pr, "dreuse", 1); got != want {
		t.Errorf("Urgency = %d, want %d", got, want)
	}
}

func TestSortModes(t *testing.T) {
	a := PullRequest{Number: 1, CreatedAt: daysAgo(10), UpdatedAt: daysAgo(1)}
	b := PullRequest{Number: 2, CreatedAt: daysAgo(2), UpdatedAt: daysAgo(5),
		Checks: []Check{{State: CheckFailed}}}

	tests := []struct {
		mode  SortMode
		first int
	}{
		{SortUrgency, 2},
		{SortUpdated, 1},
		{SortAge, 1},
		{SortNumber, 2},
	}
	for _, tc := range tests {
		prs := []PullRequest{a, b}
		Sort(prs, tc.mode, "dreuse", 1)
		if prs[0].Number != tc.first {
			t.Errorf("sort by %s put #%d first, want #%d", tc.mode, prs[0].Number, tc.first)
		}
	}
}

func TestSortModeCycles(t *testing.T) {
	mode := SortUrgency
	for _, want := range []SortMode{SortUpdated, SortAge, SortNumber, SortUrgency} {
		if mode = mode.Next(); mode != want {
			t.Fatalf("cycle reached %v, want %v", mode, want)
		}
	}
}

func TestCheckCounts(t *testing.T) {
	pr := PullRequest{Checks: []Check{
		{State: CheckPassed}, {State: CheckPassed},
		{State: CheckFailed},
		{State: CheckRunning},
		{State: CheckNeutral},
	}}
	c := pr.CheckCounts()
	if c.Total != 5 || c.Passed != 2 || c.Failed != 1 || c.Running != 1 || c.Neutral != 1 {
		t.Fatalf("unexpected counts: %+v", c)
	}
	if pr.Green() {
		t.Error("a PR with a failing check is not green")
	}
	if !(PullRequest{Checks: []Check{{State: CheckPassed}, {State: CheckNeutral}}}).Green() {
		t.Error("passed plus neutral is green")
	}
}
