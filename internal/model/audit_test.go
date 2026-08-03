package model

import "testing"

func TestReviewerAnyNeedsAnActualReviewer(t *testing.T) {
	bare := PullRequest{Repo: "acme/api", Number: 1}
	asked := PullRequest{Repo: "acme/api", Number: 2, RequestedReviewers: []string{"octodev"}}

	f := ParseFilter("reviewer:any")
	if f.Match(bare, FilterContext{}) {
		t.Error("reviewer:any must not match a pull request nobody was asked to review")
	}
	if !f.Match(asked, FilterContext{}) {
		t.Error("reviewer:any must match a pull request with a requested reviewer")
	}
}

func TestAnyIsConsistentAcrossPeopleFilters(t *testing.T) {
	bare := PullRequest{Repo: "acme/api", Number: 1}
	for _, key := range []string{"assignee:any", "reviewer:any"} {
		if ParseFilter(key).Match(bare, FilterContext{}) {
			t.Errorf("%s must not match an empty pull request", key)
		}
	}
}

func TestSortByUrgencyOrdersTheSameAsScoring(t *testing.T) {
	failing := PullRequest{Repo: "acme/api", Number: 1, Checks: []Check{{State: CheckFailed}}}
	quiet := PullRequest{Repo: "acme/api", Number: 2}
	conflicted := PullRequest{Repo: "acme/api", Number: 3, Mergeable: MergeableConflict}

	prs := []PullRequest{quiet, conflicted, failing}
	Sort(prs, SortUrgency, "octodev", 1)

	for i := 1; i < len(prs); i++ {
		a := Urgency(prs[i-1], "octodev", 1)
		b := Urgency(prs[i], "octodev", 1)
		if a < b {
			t.Fatalf("position %d scores %d but follows a %d", i, b, a)
		}
	}
	if prs[0].Number != failing.Number {
		t.Errorf("the failing pull request is the most urgent, got #%d", prs[0].Number)
	}
}
