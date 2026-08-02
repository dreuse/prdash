package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dreuse/prdash/internal/config"
)

func typeInto(s Setup, text string) Setup {
	for _, r := range text {
		out, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		s = out.(Setup)
	}
	return s
}

func TestSetupAcceptsEveryPrintableCharacter(t *testing.T) {
	settings := config.DefaultSettings()

	const typed = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_./"
	s := NewSetup(settings)
	s.input.SetValue("")
	s = typeInto(s, typed)

	if got := s.input.Value(); got != typed {
		t.Fatalf("the repository field swallowed characters:\n got %q\nwant %q", got, typed)
	}
}

func TestSetupAcceptsRepositoriesWithNavigationLetters(t *testing.T) {
	for _, repo := range []string{"dreuse/prdash", "helm/helm", "kubernetes/kubectl", "jj-vcs/jj"} {
		s := NewSetup(config.DefaultSettings())
		s.input.SetValue("")
		s = typeInto(s, repo)
		if got := s.input.Value(); got != repo {
			t.Errorf("typing %q produced %q", repo, got)
		}
	}
}

func TestSetupStepsThroughToDone(t *testing.T) {
	s := NewSetup(config.DefaultSettings())
	s.input.SetValue("")
	s = typeInto(s, "dreuse/prdash")

	out, _ := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	s = out.(Setup)
	if s.step != 1 {
		t.Fatalf("enter on a valid repository should advance, step = %d", s.step)
	}
	if len(s.settings.Repos) != 1 || s.settings.Repos[0] != "dreuse/prdash" {
		t.Fatalf("repository was not stored: %v", s.settings.Repos)
	}

	out, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	s = out.(Setup)
	if s.view != 1 {
		t.Fatalf("l should move the view picker on step 1, view = %d", s.view)
	}
	out, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	s = out.(Setup)
	if s.view != 0 {
		t.Fatalf("h should move back, view = %d", s.view)
	}
	out, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	s = out.(Setup)
	if s.view != 1 {
		t.Fatalf("2 should jump to the second view, view = %d", s.view)
	}

	out, _ = s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	s = out.(Setup)
	if !s.Done {
		t.Fatal("enter on step 1 should finish setup")
	}
	if s.Settings().DefaultView != Views[1].String() {
		t.Fatalf("default view not stored: %q", s.Settings().DefaultView)
	}
}

func TestSetupRejectsARepositoryWithoutASlash(t *testing.T) {
	s := NewSetup(config.DefaultSettings())
	s.input.SetValue("")
	s = typeInto(s, "prdash")

	out, _ := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	s = out.(Setup)
	if s.step != 0 || s.Done {
		t.Fatal("a repository without owner/name must not advance")
	}
}

func TestGitRemoteRepoParsesBothURLForms(t *testing.T) {
	for _, tc := range []struct{ url, want string }{
		{"git@github.com:dreuse/prdash.git", "dreuse/prdash"},
		{"https://github.com/dreuse/prdash.git", "dreuse/prdash"},
		{"https://github.com/dreuse/prdash", "dreuse/prdash"},
		{"https://gitlab.com/dreuse/prdash.git", ""},
	} {
		if got := parseRemoteURL(tc.url); got != tc.want {
			t.Errorf("parseRemoteURL(%q) = %q, want %q", tc.url, got, tc.want)
		}
	}
}

func TestSetupPreselectsTheGitRemote(t *testing.T) {
	s := NewSetup(config.DefaultSettings())
	if v := s.input.Value(); v != "" && !strings.Contains(v, "/") {
		t.Fatalf("the preselected repository should be owner/name or empty, got %q", v)
	}
}
