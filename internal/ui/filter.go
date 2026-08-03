package ui

import (
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dreuse/prdash/internal/model"
)

type filterBar struct {
	suggestBox
	on    bool
	saved string
}

func newFilterBar(t Theme) filterBar {
	return filterBar{suggestBox: newSuggestBox(t, "author: assignee: reviewer: label: state: is: or just type")}
}

func (m Model) openFilter() (tea.Model, tea.Cmd) {
	m.filterBar.on = true
	m.filterBar.saved = m.filter.Raw
	m.filterBar.clear()
	m.filterBar.input.SetValue(m.filter.Raw)
	m.filterBar.input.CursorEnd()
	return m, m.filterBar.input.Focus()
}

func (m Model) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.filterBar.on = false
		m.filterBar.clear()
		m.filterBar.input.Blur()
		m.filterBar.input.SetValue(m.filterBar.saved)
		return m.applyFilter()

	case "enter":
		m.filterBar.on = false
		m.filterBar.clear()
		m.filterBar.input.Blur()
		return m.applyFilter()

	case "tab", "shift+tab":
		m.filterBar.accept(msg.String() == "shift+tab")
		return m.applyFilter()
	}

	var cmd tea.Cmd
	m.filterBar.input, cmd = m.filterBar.input.Update(msg)
	m = m.refreshFilterCandidates()
	mm, apply := m.applyFilter()
	return mm, tea.Batch(cmd, apply)
}

func (m Model) applyFilter() (tea.Model, tea.Cmd) {
	m.filter = model.ParseFilter(m.filterBar.input.Value())
	m.rebuild()
	return m, m.persist()
}

func (m Model) refreshFilterCandidates() Model {
	value := m.filterBar.input.Value()
	word := lastWord(value)
	if word == "" {
		m.filterBar.clear()
		return m
	}

	candidates, total := m.filterCandidates(word)
	if len(candidates) == 0 {
		m.filterBar.clear()
		return m
	}
	m.filterBar.offer(value[:len(value)-len(word)], candidates, total)
	return m
}

func lastWord(s string) string {
	if i := strings.LastIndexAny(s, " \t"); i >= 0 {
		return s[i+1:]
	}
	return s
}

func (m Model) filterCandidates(word string) ([]candidate, int) {
	negate := strings.HasPrefix(word, "-")
	word = strings.TrimPrefix(word, "-")

	candidates, total := m.rawFilterCandidates(word)
	if !negate {
		return candidates, total
	}
	for i := range candidates {
		candidates[i].insert = "-" + candidates[i].insert
		candidates[i].complete = "-" + candidates[i].complete
	}
	return candidates, total
}

func (m Model) rawFilterCandidates(word string) ([]candidate, int) {
	key, value, hasKey := strings.Cut(word, ":")
	if !hasKey {
		return m.filterKeyCandidates(word)
	}
	return m.filterValueCandidates(strings.ToLower(key), value)
}

func (m Model) filterKeyCandidates(prefix string) ([]candidate, int) {
	prefix = strings.ToLower(prefix)
	out := make([]candidate, 0, len(model.FilterKeys))
	for _, key := range model.FilterKeys {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		out = append(out, candidate{
			insert:   key + ":",
			complete: key + ":",
			label:    key + ":",
			detail:   filterKeyHelp[key],
		})
	}
	return out, len(out)
}

var filterKeyHelp = map[string]string{
	"author":   "who opened it",
	"assignee": "who owns it",
	"reviewer": "who was asked",
	"repo":     "repository",
	"label":    "github label",
	"state":    "board lane",
	"is":       "draft, stale, failing…",
	"no":       "assignee, reviewer or label",
	"behind":   "commits behind base",
	"age":      "days since opened",
}

func (m Model) filterValueCandidates(key, prefix string) ([]candidate, int) {
	var values []string
	switch key {
	case "author", "assignee", "reviewer":
		values = append([]string{"@me"}, m.knownLogins()...)
		if key != "author" {
			values = append(values, "none", "any")
		}
	case "repo":
		for _, name := range m.repoNames() {
			values = append(values, shortRepo(name))
		}
	case "label":
		values = m.knownLabels()
	case "state", "is", "no", "behind", "age", "approvals":
		values = model.FilterValues(key)
	default:
		return nil, 0
	}

	prefix = strings.ToLower(prefix)
	out := make([]candidate, 0, len(values))
	for _, v := range values {
		if prefix != "" && !strings.Contains(strings.ToLower(v), prefix) {
			continue
		}
		token := key + ":" + v
		out = append(out, candidate{insert: token, complete: token, label: v})
	}

	sort.SliceStable(out, func(i, j int) bool {
		a := strings.HasPrefix(strings.ToLower(out[i].label), prefix)
		b := strings.HasPrefix(strings.ToLower(out[j].label), prefix)
		if a != b {
			return a
		}
		return len(out[i].label) < len(out[j].label)
	})

	total := len(out)
	if len(out) > maxCandidates {
		out = out[:maxCandidates]
	}
	return out, total
}

func (m Model) knownLogins() []string {
	seen := map[string]bool{}
	var out []string
	add := func(login string) {
		if login == "" || seen[strings.ToLower(login)] {
			return
		}
		seen[strings.ToLower(login)] = true
		out = append(out, login)
	}
	for _, pr := range m.prs {
		add(pr.Author)
		for _, a := range pr.Assignees {
			add(a)
		}
		for _, r := range pr.RequestedReviewers {
			add(r)
		}
	}
	for _, u := range m.people {
		add(u.Login)
	}
	sort.Strings(out)
	return out
}

func (m Model) knownLabels() []string {
	seen := map[string]bool{}
	var out []string
	for _, pr := range m.prs {
		for _, l := range pr.Labels {
			if !seen[strings.ToLower(l)] {
				seen[strings.ToLower(l)] = true
				out = append(out, l)
			}
		}
	}
	sort.Strings(out)
	return out
}
