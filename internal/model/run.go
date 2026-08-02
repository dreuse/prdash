package model

import (
	"fmt"
	"sort"
	"time"
)

type WorkflowRun struct {
	ID          int64
	Repo        string
	Name        string
	Branch      string
	Event       string
	Status      string
	Conclusion  string
	URL         string
	Actor       string
	FailingJob  string
	FailingStep string
	StartedAt   time.Time
	UpdatedAt   time.Time
}

func (r WorkflowRun) InProgress() bool {
	switch r.Status {
	case "in_progress", "queued", "requested", "waiting", "pending":
		return true
	}
	return false
}

func (r WorkflowRun) Failed() bool {
	switch r.Conclusion {
	case "failure", "timed_out", "startup_failure", "action_required":
		return true
	}
	return false
}

func (r WorkflowRun) Succeeded() bool { return r.Conclusion == "success" }

func (r WorkflowRun) Duration() time.Duration {
	if r.StartedAt.IsZero() {
		return 0
	}
	if r.InProgress() {
		return time.Since(r.StartedAt)
	}
	if r.UpdatedAt.Before(r.StartedAt) {
		return 0
	}
	return r.UpdatedAt.Sub(r.StartedAt)
}

func FormatDuration(d time.Duration) string {
	if d <= 0 {
		return "-"
	}
	d = d.Round(time.Second)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%02dm", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%02ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

type CIHealth struct {
	Total     int
	Passed    int
	Running   int
	Failed    int
	Median    time.Duration
	Durations []time.Duration
	Green     []bool
}

func Health(runs []WorkflowRun, window int) CIHealth {
	var h CIHealth
	if window > 0 && len(runs) > window {
		runs = runs[:window]
	}
	completed := make([]time.Duration, 0, len(runs))
	for _, r := range runs {
		h.Total++
		switch {
		case r.InProgress():
			h.Running++
		case r.Failed():
			h.Failed++
		case r.Succeeded():
			h.Passed++
		}
		if !r.InProgress() {
			completed = append(completed, r.Duration())
		}
		h.Durations = append(h.Durations, r.Duration())
		h.Green = append(h.Green, !r.Failed())
	}
	h.Median = median(completed)
	return h
}

func (h CIHealth) PassRate() int {
	decided := h.Passed + h.Failed
	if decided == 0 {
		return 100
	}
	return h.Passed * 100 / decided
}

func median(ds []time.Duration) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	sorted := append([]time.Duration(nil), ds...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	return sorted[len(sorted)/2]
}

type WorkflowStats struct {
	Name   string
	Repo   string
	Runs   int
	Passed int
	Median time.Duration
	Trend  []WorkflowRun
	Last   WorkflowRun
}

func (w WorkflowStats) PassRate() int {
	if w.Runs == 0 {
		return 100
	}
	return w.Passed * 100 / w.Runs
}

func AggregateWorkflows(runs []WorkflowRun) []WorkflowStats {
	index := map[string]int{}
	var out []WorkflowStats
	for _, r := range runs {
		key := r.Repo + "\x00" + r.Name
		i, ok := index[key]
		if !ok {
			i = len(out)
			index[key] = i
			out = append(out, WorkflowStats{Name: r.Name, Repo: r.Repo, Last: r})
		}
		w := &out[i]
		w.Runs++
		if r.Succeeded() {
			w.Passed++
		}
		w.Trend = append(w.Trend, r)
		if r.StartedAt.After(w.Last.StartedAt) {
			w.Last = r
		}
	}
	for i := range out {
		durations := make([]time.Duration, 0, len(out[i].Trend))
		for _, r := range out[i].Trend {
			if !r.InProgress() {
				durations = append(durations, r.Duration())
			}
		}
		out[i].Median = median(durations)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if a, b := out[i].PassRate(), out[j].PassRate(); a != b {
			return a < b
		}
		return out[i].Runs > out[j].Runs
	})
	return out
}

func GreenRunsBefore(runs []WorkflowRun, target WorkflowRun) int {
	seen := false
	green := 0
	for _, r := range runs {
		if r.ID == target.ID && r.Repo == target.Repo {
			seen = true
			continue
		}
		if !seen || r.Name != target.Name || r.Branch != target.Branch || r.Repo != target.Repo {
			continue
		}
		if r.InProgress() {
			continue
		}
		if !r.Succeeded() {
			break
		}
		green++
	}
	return green
}
