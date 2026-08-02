package ui

import (
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dreuse/prdash/internal/config"
)

type Setup struct {
	theme    Theme
	input    textinput.Model
	view     int
	step     int
	width    int
	height   int
	settings config.Settings
	Done     bool
	Aborted  bool
}

func NewSetup(s config.Settings) Setup {
	in := textinput.New()
	in.Prompt = ""
	in.Placeholder = "owner/name"
	in.SetValue(GitRemoteRepo())
	in.CursorEnd()
	in.Focus()
	return Setup{theme: NewTheme(s.Theme, s.ASCII), input: in, settings: s}
}

func (s Setup) Settings() config.Settings { return s.settings }

func (s Setup) Init() tea.Cmd { return textinput.Blink }

func (s Setup) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width, s.height = msg.Width, msg.Height
		return s, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			s.Aborted = true
			return s, tea.Quit

		case "enter":
			if s.step == 0 {
				repo := strings.TrimSpace(s.input.Value())
				if !strings.Contains(repo, "/") {
					return s, nil
				}
				s.settings.Repos = []string{repo}
				s.step = 1
				s.input.Blur()
				return s, nil
			}
			s.settings.DefaultView = Views[s.view].String()
			s.Done = true
			return s, tea.Quit

		case "left":
			if s.step == 1 {
				s.view = wrapIndex(s.view-1, len(Views))
			}
			return s, nil

		case "right", "tab":
			if s.step == 1 {
				s.view = wrapIndex(s.view+1, len(Views))
			}
			return s, nil

		case "h", "k", "up":
			if s.step == 1 {
				s.view = wrapIndex(s.view-1, len(Views))
				return s, nil
			}

		case "l", "j", "down":
			if s.step == 1 {
				s.view = wrapIndex(s.view+1, len(Views))
				return s, nil
			}

		case "1", "2", "3":
			if s.step == 1 {
				s.view = clamp(int(msg.String()[0]-'1'), 0, len(Views)-1)
				return s, nil
			}
		}
	}

	if s.step == 0 {
		var cmd tea.Cmd
		s.input, cmd = s.input.Update(msg)
		return s, cmd
	}
	return s, nil
}

func (s Setup) View() string {
	t := s.theme
	if s.width <= 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(t.Brand.Render(appName) + t.Dim.Render("  first run") + "\n\n")

	if s.step == 0 {
		b.WriteString(t.Strong.Render("Which repository?") + "\n")
		b.WriteString(t.Faint.Render("owner/name - you can add more later with , (settings)") + "\n\n")
		b.WriteString(t.Accent.Render("  ") + s.input.View())
	} else {
		b.WriteString(t.Strong.Render("Which view opens first?") + "\n")
		b.WriteString(t.Faint.Render(t.Glyphs.LeftRight+" to choose, 1 / 2 / 3 switch at any time") + "\n\n")
		tabs := make([]string, 0, len(Views))
		for i, v := range Views {
			label := " " + itoa(i+1) + " " + v.Label() + " "
			if i == s.view {
				tabs = append(tabs, t.ChipFilled.Render(strings.TrimSpace(label)))
				continue
			}
			tabs = append(tabs, t.Chip.Render(strings.TrimSpace(label)))
		}
		b.WriteString("  " + strings.Join(tabs, " "))
	}

	b.WriteString("\n\n" + t.Faint.Render(t.Glyphs.Enter+" continue   esc quit"))
	return lipgloss.Place(s.width, s.height, lipgloss.Center, lipgloss.Center, b.String())
}

func GitRemoteRepo() string {
	out, err := exec.Command("git", "remote", "get-url", "origin").Output()
	if err != nil {
		return ""
	}
	return parseRemoteURL(strings.TrimSpace(string(out)))
}

func parseRemoteURL(url string) string {
	url = strings.TrimSuffix(url, ".git")
	_, rest, ok := strings.Cut(url, "github.com")
	if !ok {
		return ""
	}
	repo := strings.Trim(strings.TrimLeft(rest, ":/"), "/")
	if strings.Count(repo, "/") != 1 {
		return ""
	}
	return repo
}
