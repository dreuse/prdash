package config

import (
	"strings"
	"time"
)

const (
	settingsFile   = "settings.json"
	settingsSchema = 1
	SavedFilterN   = 4

	NotifyOff      = "off"
	NotifyFailures = "failures"
	NotifyAll      = "all"

	ScopeAny      = "any"
	ScopeMine     = "mine"
	ScopeAuthored = "authored"
)

type Settings struct {
	Schema int `json:"schema"`

	DefaultView      string `json:"default_view"`
	RememberLastView bool   `json:"remember_last_view"`
	RefreshSeconds   int    `json:"refresh_seconds"`
	Theme            string `json:"theme"`
	ASCII            bool   `json:"ascii"`

	Repos []string `json:"repos"`

	LaneOrder   string   `json:"lane_order"`
	HiddenLanes []string `json:"hidden_lanes"`

	CIRunsWindow   int  `json:"ci_runs_window"`
	CIRecentHours  int  `json:"ci_recent_hours"`
	CIFailuresOnly bool `json:"ci_failures_only"`

	Notify         string `json:"notify"`
	NotifyScope    string `json:"notify_scope"`
	NotifyReviews  bool   `json:"notify_reviews"`
	NotifyReady    bool   `json:"notify_ready"`
	NotifyAssigned bool   `json:"notify_assigned"`

	Sort          string   `json:"sort"`
	StartupFilter string   `json:"startup_filter"`
	SavedFilters  []string `json:"saved_filters"`

	RequiredApprovals int  `json:"required_approvals"`
	BehindBlocks      bool `json:"behind_blocks"`
}

func DefaultSettings() Settings {
	return Settings{
		Schema:            settingsSchema,
		DefaultView:       "board",
		RememberLastView:  false,
		RefreshSeconds:    30,
		Theme:             "auto",
		ASCII:             false,
		LaneOrder:         "ready",
		CIRunsWindow:      20,
		CIRecentHours:     24,
		CIFailuresOnly:    false,
		Notify:            NotifyOff,
		NotifyScope:       ScopeAny,
		Sort:              "urgency",
		SavedFilters:      make([]string, SavedFilterN),
		RequiredApprovals: 1,
	}
}

func LoadSettings() Settings {
	s := DefaultSettings()
	var stored Settings
	if !readJSON(settingsFile, &stored) || stored.Schema != settingsSchema {
		return s
	}
	stored.normalise()
	return stored
}

func (s *Settings) normalise() {
	d := DefaultSettings()
	if s.RefreshSeconds < 5 {
		s.RefreshSeconds = d.RefreshSeconds
	}
	if s.CIRunsWindow < 1 {
		s.CIRunsWindow = d.CIRunsWindow
	}
	if s.CIRecentHours < 1 {
		s.CIRecentHours = d.CIRecentHours
	}
	if s.RequiredApprovals < 0 {
		s.RequiredApprovals = d.RequiredApprovals
	}
	switch s.Notify {
	case NotifyOff, NotifyFailures, NotifyAll:
	default:
		s.Notify = d.Notify
	}
	switch s.NotifyScope {
	case ScopeAny, ScopeMine, ScopeAuthored:
	default:
		s.NotifyScope = d.NotifyScope
	}
	for _, pair := range [][2]*string{
		{&s.DefaultView, &d.DefaultView},
		{&s.Theme, &d.Theme},
		{&s.LaneOrder, &d.LaneOrder},
		{&s.Sort, &d.Sort},
	} {
		if *pair[0] == "" {
			*pair[0] = *pair[1]
		}
	}
	for len(s.SavedFilters) < SavedFilterN {
		s.SavedFilters = append(s.SavedFilters, "")
	}
	s.Repos = dedupeFold(s.Repos)
}

func dedupeFold(list []string) []string {
	seen := make(map[string]bool, len(list))
	out := make([]string, 0, len(list))
	for _, v := range list {
		key := strings.ToLower(strings.TrimSpace(v))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, v)
	}
	return out
}

func (s Settings) NotifiesAnything() bool {
	return s.Notify != NotifyOff || s.NotifyReviews || s.NotifyReady || s.NotifyAssigned
}

func (s Settings) Interval() time.Duration {
	return time.Duration(s.RefreshSeconds) * time.Second
}

func SaveSettings(s Settings) error {
	s.Schema = settingsSchema
	return writeJSON(settingsFile, s)
}

const (
	stateFile   = "state.json"
	stateSchema = 1
)

type State struct {
	Schema     int    `json:"schema"`
	LastView   string `json:"last_view"`
	Filter     string `json:"filter"`
	Scope      string `json:"scope"`
	Sort       string `json:"sort"`
	SelectRepo string `json:"select_repo"`
	SelectPR   int    `json:"select_pr"`
}

func LoadState() State {
	var s State
	if !readJSON(stateFile, &s) || s.Schema != stateSchema {
		return State{Schema: stateSchema}
	}
	return s
}

func SaveState(s State) error {
	s.Schema = stateSchema
	return writeJSON(stateFile, s)
}
