package ui

import (
	"errors"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dreuse/prdash/internal/github"
)

const (
	helpWidth      = 72
	confirmWidth   = 54
	pickerWidth    = 46
	overlayPadding = 6
	overlayText    = 4
)

func panelInner(desired, screen int) int {
	return maxInt(16, minInt(desired, screen-4)-overlayPadding)
}

func nowSince(t time.Time) time.Duration {
	if t.IsZero() {
		return 0
	}
	return time.Since(t)
}

func (m Model) recovery() string {
	var ghErr *github.Error
	switch {
	case errors.As(m.err, &ghErr) && ghErr.Auth:
		return "run `gh auth login`, then press u to retry — settings live in " + configHint()
	case errors.As(m.err, &ghErr) && ghErr.RateLimit:
		return "github rate limit hit; the dashboard retries on its own"
	}
	return "press u to retry, q to quit"
}

func (m Model) renderOverlay(kind overlayKind) string {
	switch kind {
	case ovSettings:
		return m.renderSettings()
	case ovHelp:
		return m.renderHelp()
	case ovConfirm:
		return m.renderConfirm()
	case ovRepo:
		return m.renderRepoPicker()
	}
	return ""
}

func (m Model) renderHelp() string {
	t := m.theme
	width := panelInner(helpWidth, m.width)
	height := maxInt(6, m.height-6)

	var b strings.Builder
	b.WriteString(t.Strong.Render("Keys"))
	b.WriteString("\n")
	for _, row := range m.keys.HelpSections(t.Glyphs) {
		if row[1] == "" {
			b.WriteString("\n" + t.Faint.Render(row[0]) + "\n")
			continue
		}
		b.WriteString(t.Accent.Render(pad(row[0], 12)) + t.Dim.Render(truncate(row[1], maxInt(1, width-12))) + "\n")
	}
	b.WriteString("\n" + t.Faint.Render("any key closes"))
	return t.Overlay.Width(width + overlayText).MaxHeight(height).Render(b.String())
}

func (m Model) renderConfirm() string {
	t := m.theme
	width := panelInner(confirmWidth, m.width)
	c := m.confirm

	title := t.Strong.Render(c.title)
	if c.danger {
		title = t.Danger.Render(t.Glyphs.Conflict+" ") + t.Strong.Render(c.title)
	}
	chips := t.ChipFilled.Render("y "+c.verb) + " " + t.Chip.Render("n cancel")

	body := make([]string, 0, 2)
	for _, line := range strings.Split(c.body, "\n") {
		body = append(body, t.Dim.Render(truncate(line, width)))
	}
	return t.Overlay.Width(width + overlayText).Render(
		title + "\n" + strings.Join(body, "\n") + "\n\n" + chips)
}

type repoEntry struct {
	label  string
	repo   string
	detail string
	add    bool
}

type repoPicker struct {
	input textinput.Model
	idx   int
}

func newRepoPicker() repoPicker {
	in := textinput.New()
	in.Prompt = ""
	in.Placeholder = "filter, or type owner/name to add"
	in.Focus()
	return repoPicker{input: in}
}

func (m Model) repoEntries() []repoEntry {
	needle := strings.ToLower(strings.TrimSpace(m.repoPick.input.Value()))
	names := m.repoNames()

	counts := make(map[string]int, len(names))
	for _, pr := range m.prs {
		counts[pr.Repo]++
	}

	entries := make([]repoEntry, 0, len(names)+2)
	if needle == "" || strings.Contains("all repositories", needle) {
		entries = append(entries, repoEntry{
			label:  "All repositories",
			detail: itoa(len(m.prs)) + " open",
		})
	}
	for _, name := range names {
		if needle != "" && !strings.Contains(strings.ToLower(name), needle) {
			continue
		}
		entries = append(entries, repoEntry{
			label:  name,
			repo:   name,
			detail: itoa(counts[name]) + " open",
		})
	}

	if _, err := github.ParseRepo(needle); err == nil && !containsFold(names, needle) {
		entries = append(entries, repoEntry{
			label:  "add " + needle,
			repo:   needle,
			detail: "track this repository",
			add:    true,
		})
	}
	return entries
}

func containsFold(list []string, want string) bool {
	for _, v := range list {
		if strings.EqualFold(v, want) {
			return true
		}
	}
	return false
}

func (m Model) handleRepoKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	entries := m.repoEntries()

	switch msg.String() {
	case "esc":
		m.pop()
		return m, nil
	case "down", "ctrl+n":
		m.repoPick.idx = clamp(m.repoPick.idx+1, 0, maxInt(0, len(entries)-1))
		return m, nil
	case "up", "ctrl+p":
		m.repoPick.idx = clamp(m.repoPick.idx-1, 0, maxInt(0, len(entries)-1))
		return m, nil
	case "d", "delete", "backspace":
		if msg.String() == "backspace" && m.repoPick.input.Value() != "" {
			break
		}
		if len(entries) == 0 {
			return m, nil
		}
		return m.untrackRepo(entries[clamp(m.repoPick.idx, 0, len(entries)-1)])

	case "enter":
		if len(entries) == 0 {
			return m, nil
		}
		return m.applyRepo(entries[clamp(m.repoPick.idx, 0, len(entries)-1)])
	}

	var cmd tea.Cmd
	m.repoPick.input, cmd = m.repoPick.input.Update(msg)
	m.repoPick.idx = 0
	return m, cmd
}

func (m Model) applyRepo(entry repoEntry) (tea.Model, tea.Cmd) {
	m.pop()

	if entry.add {
		m.settings.Repos = appendFold(m.settings.Repos, entry.repo)
		m.scope = entry.repo
		out, cmd := m.applySettings()
		return out, tea.Batch(cmd, m.notify("tracking "+entry.repo, toastGood))
	}

	m.scope = entry.repo
	m.rebuild()
	if entry.repo == "" {
		return m, tea.Batch(m.persist(), m.notify("showing all repositories", toastInfo))
	}
	return m, tea.Batch(m.persist(), m.notify("showing "+entry.repo, toastInfo))
}

func (m Model) untrackRepo(entry repoEntry) (tea.Model, tea.Cmd) {
	if entry.add || entry.repo == "" {
		return m, nil
	}
	if len(m.settings.Repos) <= 1 {
		return m, m.notify("at least one repository must stay tracked", toastInfo)
	}

	kept := make([]string, 0, len(m.settings.Repos))
	for _, r := range m.settings.Repos {
		if !strings.EqualFold(r, entry.repo) {
			kept = append(kept, r)
		}
	}
	if len(kept) == len(m.settings.Repos) {
		return m, m.notify(entry.repo+" is not tracked", toastInfo)
	}

	m.settings.Repos = kept
	if strings.EqualFold(m.scope, entry.repo) {
		m.scope = ""
	}
	m.repoPick.idx = 0

	out, cmd := m.applySettings()
	return out, tea.Batch(cmd, m.notify("stopped tracking "+entry.repo, toastGood))
}

func appendFold(list []string, v string) []string {
	if containsFold(list, v) {
		return list
	}
	return append(list, v)
}

func (m Model) renderRepoPicker() string {
	t := m.theme
	width := panelInner(pickerWidth, m.width)
	entries := m.repoEntries()

	var b strings.Builder
	b.WriteString(t.Strong.Render("Repository") + "\n")
	b.WriteString(t.Accent.Render("/ ") + m.repoPick.input.View() + "\n\n")

	if len(entries) == 0 {
		b.WriteString(t.Faint.Render("no repository matches"))
	}
	for i, entry := range entries {
		if i >= 12 {
			b.WriteString(t.Faint.Render("+" + itoa(len(entries)-12) + " more"))
			break
		}

		selected := i == m.repoPick.idx
		active := !entry.add && entry.repo != "" && strings.EqualFold(entry.repo, m.scope)
		if entry.repo == "" && m.scope == "" {
			active = true
		}

		marker := "  "
		if selected {
			marker = t.Accent.Render(t.Glyphs.Selected) + " "
		}

		label := entry.label
		labelStyle := t.Dim
		if entry.add {
			label = "+ " + label
			labelStyle = t.Accent
		}
		if selected {
			labelStyle = t.SelectedTitle
		}

		detail := t.Faint.Render(entry.detail)
		if active {
			detail = t.OK.Render(t.Glyphs.Pass+" ") + detail
		}

		line := spread(width,
			marker+labelStyle.Render(truncate(label, maxInt(1, width-16))),
			detail)
		if selected {
			b.WriteString(t.Selected.Render(fillLine(line, width)) + "\n")
			continue
		}
		b.WriteString(line + "\n")
	}

	b.WriteString("\n" + t.Faint.Render(t.Glyphs.Enter+" switch   d remove   esc cancel"))
	return t.Overlay.Width(width + overlayText).Render(b.String())
}

func configHint() string { return "the settings overlay (,)" }

var _ = lipgloss.JoinVertical
