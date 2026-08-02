package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dreuse/prdash/internal/model"
)

const lineBreak = "↵"

type commentBar struct {
	suggestBox
	pr      model.PullRequest
	active  bool
	trigger byte
}

func newCommentBar(t Theme) commentBar {
	return commentBar{suggestBox: newSuggestBox(t, "comment - :emoji  #pr  @user  shift-enter new line")}
}

func (m Model) openComment(pr model.PullRequest) (tea.Model, tea.Cmd) {
	m.comment.pr = pr
	m.comment.active = true
	m.comment.reset()
	return m, m.comment.input.Focus()
}

func (m Model) closeComment() Model {
	m.comment.active = false
	m.comment.reset()
	m.comment.input.Blur()
	return m
}

func (m Model) handleCommentKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		return m.closeComment(), nil

	case "shift+enter", "alt+enter", "ctrl+j":
		m.comment.insert(lineBreak)
		m = m.refreshCandidates()
		return m, nil

	case "enter":
		body := commentBody(m.comment.input.Value())
		if body == "" {
			return m.closeComment(), m.notify("comment cancelled (empty)", toastInfo)
		}
		pr := m.comment.pr
		m = m.closeComment()
		m.pending[pr.Key()] = "commenting"
		return m, m.submitCommentCmd(pr, body)

	case "tab", "shift+tab":
		m.comment.accept(msg.String() == "shift+tab")
		return m, nil
	}

	var cmd tea.Cmd
	m.comment.input, cmd = m.comment.input.Update(msg)
	m = m.refreshCandidates()
	return m, cmd
}

func commentBody(raw string) string {
	body := strings.ReplaceAll(raw, lineBreak, "\n")
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func (c *commentBar) insert(text string) {
	value := []rune(c.input.Value())
	pos := clamp(c.input.Position(), 0, len(value))

	next := string(value[:pos]) + text + string(value[pos:])
	c.input.SetValue(next)
	c.input.SetCursor(pos + len([]rune(text)))
}

func (m Model) refreshCandidates() Model {
	c := &m.comment

	value := c.input.Value()
	if replaced := m.emoji.replaceTrailingShortcode(value, m.theme.Glyphs.ASCII); replaced != value {
		c.input.SetValue(replaced)
		c.input.CursorEnd()
		c.clear()
		return m
	}

	trigger, prefix, ok := openToken(value)
	if !ok {
		c.clear()
		return m
	}
	candidates, total := m.candidates(trigger, prefix)
	c.trigger = trigger
	c.offer(value[:len(value)-len(prefix)-1], candidates, total)
	return m
}

func (m Model) renderCommentBar() string {
	t := m.theme
	label := t.Accent.Render("comment ") + t.Faint.Render("#"+itoa(m.comment.pr.Number)) +
		t.Accent.Render(" "+t.Glyphs.Selected+" ")

	hint := t.ChromeFaint.Render(t.Glyphs.Enter + " send " + t.Glyphs.Dot +
		" shift-" + t.Glyphs.Enter + " new line " + t.Glyphs.Dot + " esc cancel")
	if len(m.comment.candidates) > 0 {
		hint = m.renderSuggestions(m.comment.suggestBox, m.comment.trigger != triggerEmoji)
	}

	m.comment.input.Width = maxInt(1, m.width-lipgloss.Width(label)-
		lipgloss.Width(hint)-m.comment.ghostWidth()-2)
	return t.Chrome.Render(fillLine(spread(m.width, label+m.comment.input.View(), hint), m.width))
}
