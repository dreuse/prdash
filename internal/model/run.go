package model

import (
	"fmt"
	"time"
)

type WorkflowRun struct {
	Repo       string
	Name       string
	Branch     string
	Event      string
	Status     string
	Conclusion string
	URL        string
	StartedAt  time.Time
	UpdatedAt  time.Time
}

func (r WorkflowRun) InProgress() bool {
	switch r.Status {
	case "in_progress", "queued", "requested", "waiting", "pending":
		return true
	}
	return false
}

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
	return fmt.Sprintf("%dm%02ds", m, s)
}
