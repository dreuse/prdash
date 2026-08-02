package ui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dreuse/prdash/internal/github"
	"github.com/dreuse/prdash/internal/readiness"
)

func loaded(t *testing.T, width, height int) tea.Model {
	t.Helper()
	snap, err := github.NewMock().Fetch(context.Background())
	if err != nil {
		t.Fatalf("mock fetch: %v", err)
	}
	var m tea.Model = New(github.NewMock(), readiness.DefaultPolicy(), 0, []string{"acme/api"})
	m, _ = m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	m, _ = m.Update(dataMsg{snapshot: snap})
	return m
}

func press(m tea.Model, keys ...string) tea.Model {
	for _, k := range keys {
		switch k {
		case "enter":
			m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		case "tab":
			m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
		default:
			m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)})
		}
	}
	return m
}

func TestViewAlwaysFillsExactlyTheTerminal(t *testing.T) {
	sizes := [][2]int{{175, 40}, {120, 30}, {90, 24}, {60, 20}, {40, 12}, {240, 60}, {80, 5}}
	journeys := map[string][]string{
		"board":          nil,
		"board scrolled": {"j", "j", "j", "j", "j"},
		"last column":    {"h"},
		"detail":         {"enter"},
		"actions":        {"tab"},
		"actions bottom": {"tab", "G"},
	}

	for _, size := range sizes {
		w, h := size[0], size[1]
		for name, keys := range journeys {
			m := press(loaded(t, w, h), keys...)
			out := m.View()
			lines := strings.Split(out, "\n")

			if len(lines) != h {
				t.Errorf("%dx%d %s: rendered %d lines, want exactly %d", w, h, name, len(lines), h)
			}
			for i, line := range lines {
				if n := lipgloss.Width(line); n > w {
					t.Errorf("%dx%d %s: line %d is %d cells wide, want at most %d", w, h, name, i, n, w)
				}
			}
		}
	}
}

func TestNothingIsRenderedBeforeTheTerminalSizeIsKnown(t *testing.T) {
	m := New(github.NewMock(), readiness.DefaultPolicy(), 0, []string{"acme/api"})
	if out := m.View(); out != "" {
		t.Fatalf("an unsized model must render nothing, got %d lines:\n%s",
			len(strings.Split(out, "\n")), out)
	}
}

func TestViewFitsWithAMultiLineFooter(t *testing.T) {
	for _, size := range [][2]int{{80, 3}, {80, 4}, {60, 6}, {120, 24}} {
		w, h := size[0], size[1]
		var m tea.Model = loaded(t, w, h)
		m, _ = m.Update(noticeMsg{text: "opened https://github.com/acme/api/pull/412"})

		lines := strings.Split(m.View(), "\n")
		if len(lines) != h {
			t.Errorf("%dx%d with a notice: rendered %d lines, want exactly %d", w, h, len(lines), h)
		}
		for i, line := range lines {
			if n := lipgloss.Width(line); n > w {
				t.Errorf("%dx%d with a notice: line %d is %d cells wide, want at most %d", w, h, i, n, w)
			}
		}
	}
}

func TestSelectedCardStaysVisibleWhenScrolling(t *testing.T) {
	m := loaded(t, 100, 12)
	first := m.View()
	if !strings.Contains(first, "#412") {
		t.Fatalf("first draft card should start visible:\n%s", first)
	}
	if strings.Contains(first, "#408") {
		t.Fatalf("second draft card should not fit at this height:\n%s", first)
	}

	scrolled := press(m, "j").View()
	if !strings.Contains(scrolled, "#408") {
		t.Fatalf("selecting the second card should scroll it into view:\n%s", scrolled)
	}
}

func TestRepositoryColumnOnlyAppearsWithSeveralRepos(t *testing.T) {
	multi := press(loaded(t, 160, 30), "tab").View()
	if !strings.Contains(multi, "REPOSITORY") {
		t.Fatalf("several repos should keep the repository column:\n%s", multi)
	}

	snap, _ := github.NewMock().Fetch(context.Background())
	for i := range snap.Runs {
		snap.Runs[i].Repo = "acme/api"
	}
	for i := range snap.PullRequests {
		snap.PullRequests[i].Repo = "acme/api"
	}
	var single tea.Model = New(github.NewMock(), readiness.DefaultPolicy(), 0, []string{"acme/api"})
	single, _ = single.Update(tea.WindowSizeMsg{Width: 160, Height: 30})
	single, _ = single.Update(dataMsg{snapshot: snap})
	if out := press(single, "tab").View(); strings.Contains(out, "REPOSITORY") {
		t.Fatalf("a single repo should drop the repository column:\n%s", out)
	}
}
