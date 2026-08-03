package ui

import (
	"strings"
	"testing"

	"github.com/dreuse/prdash/internal/config"
	"github.com/dreuse/prdash/internal/model"
)

func finishedRun(id int64, conclusion string) model.WorkflowRun {
	return model.WorkflowRun{
		ID: id, Repo: "acme/api", Name: "ci", Branch: "main",
		Status: "completed", Conclusion: conclusion,
	}
}

func TestFailuresOnlyDropsTheGreenRuns(t *testing.T) {
	runs := []model.WorkflowRun{finishedRun(1, "success"), finishedRun(2, "failure")}

	got := worthNotifying(runs, config.NotifyFailures)
	if len(got) != 1 || !got[0].Failed() {
		t.Errorf("failures mode should keep only the broken run, got %v", got)
	}
	if len(worthNotifying(runs, config.NotifyAll)) != 2 {
		t.Error("all mode should keep both outcomes")
	}
	if len(worthNotifying(runs, config.NotifyOff)) != 0 {
		t.Error("off means off")
	}
}

func TestNotificationNamesTheRunAndItsOutcome(t *testing.T) {
	title, body := notifyMessage([]model.WorkflowRun{finishedRun(1, "failure")})

	if title != "acme/api" {
		t.Errorf("the repository belongs in the title, got %q", title)
	}
	if !strings.Contains(body, "ci") || !strings.Contains(body, "failed") || !strings.Contains(body, "main") {
		t.Errorf("the body should name the workflow, outcome and branch, got %q", body)
	}
}

func TestNotificationSummarisesABurst(t *testing.T) {
	runs := []model.WorkflowRun{finishedRun(1, "failure"), finishedRun(2, "success"), finishedRun(3, "failure")}

	_, body := notifyMessage(runs)
	if !strings.Contains(body, "3") || !strings.Contains(body, "2 failed") {
		t.Errorf("many runs at once should collapse into a count, got %q", body)
	}
}

func TestNotificationsKeepPollingWhileTheTerminalIsAway(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m.settings.RefreshSeconds = 30
	m.focused = false

	if d := m.interval(); d < unfocusedInterval {
		t.Fatalf("with notifications off the dashboard should back off, got %s", d)
	}

	m.settings.Notify = config.NotifyFailures
	if d := m.interval(); d >= unfocusedInterval {
		t.Errorf("a background alert is useless five minutes late, got %s", d)
	}
}

func TestEveryPullRequestEventIsIndividuallySwitchable(t *testing.T) {
	mine := model.PullRequest{Repo: "acme/api", Number: 1, Author: "me"}
	events := []model.Event{
		{Kind: model.EventApproved, PR: mine, Actor: "alice"},
		{Kind: model.EventReadyToMerge, PR: mine},
		{Kind: model.EventAssigned, PR: mine},
	}

	m := testModel(t, 200, 40, ViewBoard)
	m.viewer = "me"
	if got := m.wantedEvents(events); len(got) != 0 {
		t.Errorf("everything is off by default, got %v", got)
	}

	m.settings.NotifyReviews = true
	if got := m.wantedEvents(events); len(got) != 1 || got[0].Kind != model.EventApproved {
		t.Errorf("only the review should come through, got %v", got)
	}

	m.settings.NotifyReady = true
	m.settings.NotifyAssigned = true
	if got := m.wantedEvents(events); len(got) != 3 {
		t.Errorf("with all three on nothing should be dropped, got %v", got)
	}
}

func TestBeingHandedWorkIgnoresTheScope(t *testing.T) {
	theirs := model.PullRequest{Repo: "acme/api", Number: 2, Author: "someone", Assignees: []string{"me"}}
	events := []model.Event{
		{Kind: model.EventApproved, PR: theirs, Actor: "alice"},
		{Kind: model.EventAssigned, PR: theirs},
	}

	m := testModel(t, 200, 40, ViewBoard)
	m.viewer = "me"
	m.settings.NotifyReviews = true
	m.settings.NotifyAssigned = true
	m.settings.NotifyScope = config.ScopeAuthored

	got := m.wantedEvents(events)
	if len(got) != 1 || got[0].Kind != model.EventAssigned {
		t.Fatalf("authored scope hides someone else's review but must still say work landed on you, got %v", got)
	}
}

func TestPullRequestEventNamesTheReviewer(t *testing.T) {
	pr := model.PullRequest{Repo: "acme/api", Number: 42}
	title, body := eventMessage([]model.Event{{Kind: model.EventApproved, PR: pr, Actor: "alice"}})

	if title != "acme/api#42" {
		t.Errorf("the pull request belongs in the title, got %q", title)
	}
	if !strings.Contains(body, "alice") || !strings.Contains(body, "approved") {
		t.Errorf("the body should name the reviewer and what they did, got %q", body)
	}
}

func TestNotificationSaysNothingAboutNothing(t *testing.T) {
	if title, body := notifyMessage(nil); title != "" || body != "" {
		t.Errorf("no finished runs means no notification, got %q %q", title, body)
	}
}
