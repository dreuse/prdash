package model

import "strings"

type EventKind int

const (
	EventApproved EventKind = iota
	EventChangesRequested
	EventReadyToMerge
	EventAssigned
)

type Event struct {
	Kind  EventKind
	PR    PullRequest
	Actor string
}

func PullRequestEvents(before, after []PullRequest, viewer string, ready func(PullRequest) bool) []Event {
	if len(before) == 0 {
		return nil
	}
	prior := make(map[Key]PullRequest, len(before))
	for _, p := range before {
		prior[p.Key()] = p
	}

	var out []Event
	for _, p := range after {
		was, seen := prior[p.Key()]
		if !seen {
			if holds(p, viewer) {
				out = append(out, Event{Kind: EventAssigned, PR: p})
			}
			continue
		}
		out = append(out, reviewEvents(was, p, viewer)...)
		if ready != nil && !ready(was) && ready(p) {
			out = append(out, Event{Kind: EventReadyToMerge, PR: p})
		}
		if !holds(was, viewer) && holds(p, viewer) {
			out = append(out, Event{Kind: EventAssigned, PR: p})
		}
	}
	return out
}

func reviewEvents(was, now PullRequest, viewer string) []Event {
	previous := reviewStates(was)
	var out []Event
	for _, r := range now.Reviews {
		login := strings.ToLower(r.Login)
		if login == "" || strings.EqualFold(r.Login, viewer) || previous[login] == r.State {
			continue
		}
		switch r.State {
		case ReviewApproved:
			out = append(out, Event{Kind: EventApproved, PR: now, Actor: r.Login})
		case ReviewChangesRequested:
			out = append(out, Event{Kind: EventChangesRequested, PR: now, Actor: r.Login})
		}
	}
	return out
}

func reviewStates(p PullRequest) map[string]ReviewState {
	out := make(map[string]ReviewState, len(p.Reviews))
	for _, r := range p.Reviews {
		out[strings.ToLower(r.Login)] = r.State
	}
	return out
}

func holds(p PullRequest, viewer string) bool {
	if viewer == "" {
		return false
	}
	for _, login := range append(append([]string(nil), p.Assignees...), p.RequestedReviewers...) {
		if strings.EqualFold(login, viewer) {
			return true
		}
	}
	return false
}
