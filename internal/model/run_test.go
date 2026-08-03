package model

import "testing"

func running(id int64, repo string) WorkflowRun {
	return WorkflowRun{ID: id, Repo: repo, Name: "ci", Branch: "main", Status: "in_progress"}
}

func done(id int64, repo, conclusion string) WorkflowRun {
	return WorkflowRun{ID: id, Repo: repo, Name: "ci", Branch: "main", Status: "completed", Conclusion: conclusion}
}

func TestFinishedReportsRunsThatJustLanded(t *testing.T) {
	before := []WorkflowRun{running(1, "acme/api"), running(2, "acme/api")}
	after := []WorkflowRun{done(1, "acme/api", "failure"), running(2, "acme/api")}

	got := Finished(before, after)
	if len(got) != 1 || got[0].ID != 1 {
		t.Fatalf("only the run that left in_progress should report, got %v", got)
	}
}

func TestFinishedStaysQuietOnTheFirstLoad(t *testing.T) {
	after := []WorkflowRun{done(1, "acme/api", "failure"), done(2, "acme/api", "success")}

	if got := Finished(nil, after); len(got) != 0 {
		t.Errorf("a cold start must not replay old runs, got %v", got)
	}
}

func TestFinishedDoesNotRepeatItselfOnTheNextRefresh(t *testing.T) {
	settled := []WorkflowRun{done(1, "acme/api", "failure")}

	if got := Finished(settled, settled); len(got) != 0 {
		t.Errorf("a run already reported must not report again, got %v", got)
	}
}

func TestFinishedKeepsRepositoriesApart(t *testing.T) {
	before := []WorkflowRun{running(1, "acme/api")}
	after := []WorkflowRun{done(1, "acme/web", "failure")}

	if got := Finished(before, after); len(got) != 0 {
		t.Errorf("a run id from another repository is a different run, got %v", got)
	}
}

func TestMineCoversAuthoredAndAssignedPullRequests(t *testing.T) {
	authored := PullRequest{Author: "Me"}
	assigned := PullRequest{Author: "someone", Assignees: []string{"other", "ME"}}
	theirs := PullRequest{Author: "someone", Assignees: []string{"other"}}

	if !authored.Mine("me") || !assigned.Mine("me") {
		t.Error("a pr you opened or were assigned is yours, whatever the login casing")
	}
	if theirs.Mine("me") {
		t.Error("a pr you are not on is not yours")
	}
	if authored.Mine("") {
		t.Error("without a viewer nothing can be claimed as yours")
	}
}

func TestAuthoredIsNarrowerThanMine(t *testing.T) {
	assigned := PullRequest{Author: "someone", Assignees: []string{"me"}}

	if !assigned.Mine("me") {
		t.Error("an assigned pr is still yours")
	}
	if assigned.Authored("me") {
		t.Error("being assigned is not the same as having opened it")
	}
	if (PullRequest{Author: "ME"}).Authored("me") == false {
		t.Error("logins compare case-insensitively")
	}
	if (PullRequest{Author: "me"}).Authored("") {
		t.Error("without a viewer nothing is authored by you")
	}
}

func TestRunsOnPullRequestsMatchesTheHeadBranch(t *testing.T) {
	prs := []PullRequest{
		{Repo: "acme/api", Author: "me", HeadRef: "feature/login"},
		{Repo: "acme/api", Author: "someone", Assignees: []string{"me"}, HeadRef: "feature/review"},
		{Repo: "acme/api", Author: "someone", HeadRef: "feature/theirs"},
	}
	runs := []WorkflowRun{
		{ID: 1, Repo: "acme/api", Branch: "feature/login"},
		{ID: 2, Repo: "acme/api", Branch: "feature/review"},
		{ID: 3, Repo: "acme/api", Branch: "feature/theirs"},
		{ID: 4, Repo: "acme/api", Branch: "main"},
	}

	mine := RunsOnPullRequests(runs, prs, func(p PullRequest) bool { return p.Mine("me") })
	if len(mine) != 2 {
		t.Errorf("mine covers the pr you opened and the one assigned to you, got %v", mine)
	}

	authored := RunsOnPullRequests(runs, prs, func(p PullRequest) bool { return p.Authored("me") })
	if len(authored) != 1 || authored[0].ID != 1 {
		t.Errorf("authored keeps only the run on the pr you opened, got %v", authored)
	}
}

func TestRunsOnPullRequestsKeepsRepositoriesApart(t *testing.T) {
	prs := []PullRequest{{Repo: "acme/api", Author: "me", HeadRef: "shared"}}
	runs := []WorkflowRun{{ID: 1, Repo: "acme/web", Branch: "shared"}}

	got := RunsOnPullRequests(runs, prs, func(p PullRequest) bool { return p.Mine("me") })
	if len(got) != 0 {
		t.Errorf("a same-named branch in another repository is not your pr, got %v", got)
	}
}

func TestRunsOnPullRequestsStaysQuietWithoutAViewer(t *testing.T) {
	prs := []PullRequest{{Repo: "acme/api", Author: "me", HeadRef: "feature/login"}}
	runs := []WorkflowRun{{ID: 1, Repo: "acme/api", Branch: "feature/login"}}

	got := RunsOnPullRequests(runs, prs, func(p PullRequest) bool { return p.Mine("") })
	if len(got) != 0 {
		t.Errorf("an unknown viewer must not be handed everyone's runs, got %v", got)
	}
}

func TestFinishedIgnoresRunsThatVanish(t *testing.T) {
	before := []WorkflowRun{running(1, "acme/api")}
	after := []WorkflowRun{}

	if got := Finished(before, after); len(got) != 0 {
		t.Errorf("a run that fell out of the window has no outcome to report, got %v", got)
	}
}
