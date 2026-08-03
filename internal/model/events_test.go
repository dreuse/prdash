package model

import "testing"

func openPR(number int, author string) PullRequest {
	return PullRequest{Repo: "acme/api", Number: number, Title: "add login", Author: author, HeadRef: "feature/login"}
}

func alwaysReady(PullRequest) bool { return true }
func neverReady(PullRequest) bool  { return false }

func withReview(p PullRequest, login string, state ReviewState) PullRequest {
	p.Reviews = append(append([]Review(nil), p.Reviews...), Review{Login: login, State: state})
	return p
}

func kinds(events []Event) []EventKind {
	out := make([]EventKind, 0, len(events))
	for _, e := range events {
		out = append(out, e.Kind)
	}
	return out
}

func TestEventsStayQuietOnTheFirstLoad(t *testing.T) {
	after := []PullRequest{withReview(openPR(1, "me"), "alice", ReviewApproved)}

	if got := PullRequestEvents(nil, after, "me", alwaysReady); len(got) != 0 {
		t.Errorf("a cold start has nothing to compare against, got %v", kinds(got))
	}
}

func TestEventsReportANewApproval(t *testing.T) {
	before := []PullRequest{openPR(1, "me")}
	after := []PullRequest{withReview(openPR(1, "me"), "alice", ReviewApproved)}

	got := PullRequestEvents(before, after, "me", neverReady)
	if len(got) != 1 || got[0].Kind != EventApproved || got[0].Actor != "alice" {
		t.Fatalf("an approval should name its reviewer, got %v", got)
	}
}

func TestEventsReportChangesRequestedAndTheFollowUpApproval(t *testing.T) {
	before := []PullRequest{withReview(openPR(1, "me"), "alice", ReviewChangesRequested)}
	after := []PullRequest{withReview(openPR(1, "me"), "alice", ReviewApproved)}

	got := PullRequestEvents(before, after, "me", neverReady)
	if len(got) != 1 || got[0].Kind != EventApproved {
		t.Fatalf("a reviewer flipping to approved is news, got %v", got)
	}
}

func TestEventsDoNotRepeatAStandingReview(t *testing.T) {
	settled := []PullRequest{withReview(openPR(1, "me"), "alice", ReviewApproved)}

	if got := PullRequestEvents(settled, settled, "me", neverReady); len(got) != 0 {
		t.Errorf("an approval already reported must not report again, got %v", kinds(got))
	}
}

func TestEventsIgnoreYourOwnReview(t *testing.T) {
	before := []PullRequest{openPR(1, "someone")}
	after := []PullRequest{withReview(openPR(1, "someone"), "ME", ReviewApproved)}

	if got := PullRequestEvents(before, after, "me", neverReady); len(got) != 0 {
		t.Errorf("you do not need telling what you just did, got %v", kinds(got))
	}
}

func TestEventsReportCrossingIntoReadyToMerge(t *testing.T) {
	mergeable := func(p PullRequest) bool { return p.Approvals >= 1 }

	waiting := openPR(1, "me")
	approved := openPR(1, "me")
	approved.Approvals = 1

	got := PullRequestEvents([]PullRequest{waiting}, []PullRequest{approved}, "me", mergeable)
	if len(got) != 1 || got[0].Kind != EventReadyToMerge {
		t.Fatalf("a pull request that just became mergeable is news, got %v", got)
	}

	again := PullRequestEvents([]PullRequest{approved}, []PullRequest{approved}, "me", mergeable)
	if len(again) != 0 {
		t.Errorf("staying ready is not news, got %v", kinds(again))
	}
}

func TestEventsReportBeingAssignedToAnExistingPullRequest(t *testing.T) {
	before := []PullRequest{openPR(1, "someone")}
	assigned := openPR(1, "someone")
	assigned.Assignees = []string{"me"}

	got := PullRequestEvents(before, []PullRequest{assigned}, "me", neverReady)
	if len(got) != 1 || got[0].Kind != EventAssigned {
		t.Fatalf("landing on someone else's pull request is news, got %v", got)
	}
}

func TestEventsReportAReviewRequest(t *testing.T) {
	before := []PullRequest{openPR(1, "someone")}
	asked := openPR(1, "someone")
	asked.RequestedReviewers = []string{"me"}

	got := PullRequestEvents(before, []PullRequest{asked}, "me", neverReady)
	if len(got) != 1 || got[0].Kind != EventAssigned {
		t.Fatalf("a review request is a way of being handed work, got %v", got)
	}
}

func TestEventsReportAPullRequestThatArrivesAlreadyAssigned(t *testing.T) {
	before := []PullRequest{openPR(1, "someone")}
	fresh := openPR(2, "someone")
	fresh.Assignees = []string{"me"}

	got := PullRequestEvents(before, []PullRequest{openPR(1, "someone"), fresh}, "me", neverReady)
	if len(got) != 1 || got[0].Kind != EventAssigned || got[0].PR.Number != 2 {
		t.Fatalf("a brand new pull request assigned to you is news, got %v", got)
	}
}

func TestEventsIgnoreAPullRequestThatArrivesForSomeoneElse(t *testing.T) {
	before := []PullRequest{openPR(1, "someone")}
	after := []PullRequest{openPR(1, "someone"), withReview(openPR(2, "someone"), "alice", ReviewApproved)}

	if got := PullRequestEvents(before, after, "me", alwaysReady); len(got) != 0 {
		t.Errorf("a pull request you have never seen is not a transition, got %v", kinds(got))
	}
}
