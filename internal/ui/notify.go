package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dreuse/prdash/internal/config"
	"github.com/dreuse/prdash/internal/model"
	"github.com/dreuse/prdash/internal/notify"
)

func (m Model) announceRuns(before, after []model.WorkflowRun) tea.Cmd {
	runs := worthNotifying(model.Finished(before, after), m.settings.Notify)
	if owned := m.notifyScope(); owned != nil {
		runs = model.RunsOnPullRequests(runs, m.prs, owned)
	}
	if len(runs) == 0 {
		return nil
	}
	title, body := notifyMessage(runs)
	kind := toastGood
	for _, r := range runs {
		if r.Failed() {
			kind = toastBad
			break
		}
	}
	return tea.Batch(
		func() tea.Msg { notify.Send(title, body); return nil },
		m.notify(m.glyphFor(kind)+" "+body, kind),
	)
}

func (m Model) announcePullRequests(before, after []model.PullRequest) tea.Cmd {
	events := m.wantedEvents(model.PullRequestEvents(before, after, m.viewer, m.policy.ReadyToMerge))
	if len(events) == 0 {
		return nil
	}
	title, body := eventMessage(events)
	kind := toastGood
	for _, e := range events {
		if e.Kind == model.EventChangesRequested {
			kind = toastBad
			break
		}
	}
	return tea.Batch(
		func() tea.Msg { notify.Send(title, body); return nil },
		m.notify(m.glyphFor(kind)+" "+body, kind),
	)
}

func (m Model) wantedEvents(events []model.Event) []model.Event {
	scope := m.notifyScope()
	var out []model.Event
	for _, e := range events {
		if !m.wantsEvent(e.Kind) {
			continue
		}
		if e.Kind != model.EventAssigned && scope != nil && !scope(e.PR) {
			continue
		}
		out = append(out, e)
	}
	return out
}

func (m Model) wantsEvent(kind model.EventKind) bool {
	switch kind {
	case model.EventApproved, model.EventChangesRequested:
		return m.settings.NotifyReviews
	case model.EventReadyToMerge:
		return m.settings.NotifyReady
	case model.EventAssigned:
		return m.settings.NotifyAssigned
	}
	return false
}

func eventMessage(events []model.Event) (title, body string) {
	if len(events) != 1 {
		return "prdash", fmt.Sprintf("%d pull request updates", len(events))
	}
	e := events[0]
	return fmt.Sprintf("%s#%d", e.PR.Repo, e.PR.Number), eventLine(e)
}

func eventLine(e model.Event) string {
	switch e.Kind {
	case model.EventApproved:
		return "approved by " + e.Actor
	case model.EventChangesRequested:
		return e.Actor + " requested changes"
	case model.EventReadyToMerge:
		return "ready to merge"
	case model.EventAssigned:
		return "handed to you"
	}
	return ""
}

func (m Model) notifyScope() func(model.PullRequest) bool {
	viewer := m.viewer
	switch m.settings.NotifyScope {
	case config.ScopeAuthored:
		return func(p model.PullRequest) bool { return p.Authored(viewer) }
	case config.ScopeMine:
		return func(p model.PullRequest) bool { return p.Mine(viewer) }
	}
	return nil
}

func (m Model) glyphFor(kind toastKind) string {
	if kind == toastBad {
		return m.theme.Glyphs.Fail
	}
	return m.theme.Glyphs.Pass
}

func worthNotifying(runs []model.WorkflowRun, mode string) []model.WorkflowRun {
	switch mode {
	case config.NotifyAll:
		return runs
	case config.NotifyFailures:
		var out []model.WorkflowRun
		for _, r := range runs {
			if r.Failed() {
				out = append(out, r)
			}
		}
		return out
	}
	return nil
}

func notifyMessage(runs []model.WorkflowRun) (title, body string) {
	switch len(runs) {
	case 0:
		return "", ""
	case 1:
		r := runs[0]
		outcome := "passed"
		if r.Failed() {
			outcome = "failed"
		}
		return r.Repo, fmt.Sprintf("%s %s on %s", r.Name, outcome, r.Branch)
	}

	failed := 0
	for _, r := range runs {
		if r.Failed() {
			failed++
		}
	}
	return "prdash", fmt.Sprintf("%d runs finished, %d failed", len(runs), failed)
}
