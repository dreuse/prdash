package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
)

const maxCompletions = 6

type suggestBox struct {
	input      textinput.Model
	candidates []candidate
	total      int
	index      int
	head       string
	completing bool
}

func newSuggestBox(t Theme, placeholder string) suggestBox {
	in := textinput.New()
	in.Prompt = ""
	in.Placeholder = placeholder
	in.ShowSuggestions = true
	in.PlaceholderStyle = t.Faint
	in.KeyMap.AcceptSuggestion.SetEnabled(false)
	in.KeyMap.NextSuggestion.SetEnabled(false)
	in.KeyMap.PrevSuggestion.SetEnabled(false)
	return suggestBox{input: in}
}

func (s *suggestBox) reset() {
	s.clear()
	s.input.SetValue("")
}

func (s *suggestBox) clear() {
	s.candidates = nil
	s.total = 0
	s.index = 0
	s.head = ""
	s.completing = false
	s.input.SetSuggestions(nil)
}

func (s *suggestBox) offer(head string, candidates []candidate, total int) {
	s.head = head
	s.candidates = candidates
	s.total = total
	s.index = clamp(s.index, 0, maxInt(0, len(candidates)-1))
	s.completing = false
	s.showGhost()
}

func (s *suggestBox) showGhost() {
	if len(s.candidates) == 0 {
		s.input.SetSuggestions(nil)
		return
	}
	current := s.candidates[clamp(s.index, 0, len(s.candidates)-1)]
	s.input.SetSuggestions([]string{s.head + current.complete})
}

func (s *suggestBox) accept(backwards bool) {
	if len(s.candidates) == 0 {
		return
	}
	step := 1
	if backwards {
		step = -1
	}

	if s.completing && s.input.Value() == s.head+s.candidates[s.index].insert {
		s.index = wrapIndex(s.index+step, len(s.candidates))
	} else if backwards {
		s.index = len(s.candidates) - 1
	} else {
		s.index = 0
	}

	s.completing = true
	s.input.SetValue(s.head + s.candidates[s.index].insert)
	s.input.CursorEnd()
	s.input.SetSuggestions(nil)
}

func (s suggestBox) ghostWidth() int {
	if len(s.candidates) == 0 {
		return 0
	}
	full := s.head + s.candidates[clamp(s.index, 0, len(s.candidates)-1)].complete
	return maxInt(0, textWidth(full)-textWidth(s.input.Value()))
}

func (m Model) renderSuggestions(s suggestBox, detail bool) string {
	t := m.theme
	shown := s.candidates
	if len(shown) > maxCompletions {
		shown = shown[:maxCompletions]
	}

	rendered := make([]string, 0, len(shown))
	for i, c := range shown {
		text := c.label
		if detail && c.detail != "" {
			text += " " + truncate(c.detail, 20)
		}
		if i == s.index {
			rendered = append(rendered, t.Warn.Background(chromeBg(t)).Render(text))
			continue
		}
		rendered = append(rendered, t.ChromeFaint.Render(text))
	}

	tail := " tab"
	if s.total > len(shown) {
		tail = "  " + itoa(s.index+1) + "/" + itoa(s.total) + " tab"
	}
	return strings.Join(rendered, t.ChromeFaint.Render("  ")) + t.ChromeFaint.Render(tail)
}
