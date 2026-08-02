package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dreuse/prdash/internal/model"
)

func selectFailing(t *testing.T, m Model) Model {
	t.Helper()
	for _, col := range m.order {
		for _, pr := range m.lanes[col] {
			if pr.CheckCounts().Failed > 0 {
				m.sel = pr.Key()
				m.syncLaneToSelection()
				return m
			}
		}
	}
	t.Fatal("the mock has no failing pull request")
	return m
}

func TestEveryOutwardActionConfirms(t *testing.T) {
	tests := []struct {
		key  rune
		want string
	}{
		{'a', "Approve"},
		{'m', "Merge"},
		{'X', "Close"},
		{'b', "Update"},
		{'r', "Re-run"},
	}
	for _, tc := range tests {
		m := testModel(t, 200, 40, ViewBoard)
		m = selectFailing(t, m)
		m.viewer = "nobody"

		out, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{tc.key}})
		m = out.(Model)

		kind, open := m.overlay()
		if !open || kind != ovConfirm {
			t.Errorf("%q ran without asking (cmd=%v)", tc.key, cmd != nil)
			continue
		}
		if !strings.HasPrefix(m.confirm.title, tc.want) {
			t.Errorf("%q asked %q, expected it to start with %q", tc.key, m.confirm.title, tc.want)
		}
		if m.confirm.run == nil {
			t.Errorf("%q has no action behind the confirm", tc.key)
		}
		if len(m.pending) != 0 {
			t.Errorf("%q marked the row pending before confirming", tc.key)
		}
	}
}

func TestConfirmCancelRunsNothing(t *testing.T) {
	for _, k := range []rune{'a', 'm', 'X', 'b', 'r'} {
		m := testModel(t, 200, 40, ViewBoard)
		m = selectFailing(t, m)
		m.viewer = "nobody"

		out, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{k}})
		m = out.(Model)
		out, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
		m = out.(Model)

		if _, open := m.overlay(); open {
			t.Errorf("%q: n did not dismiss the confirm", k)
		}
		if cmd != nil {
			t.Errorf("%q: n still ran the action", k)
		}
		if len(m.pending) != 0 {
			t.Errorf("%q: n left the row pending", k)
		}
	}
}

func TestConfirmingMarksTheRowPending(t *testing.T) {
	for _, tc := range []struct {
		key   rune
		label string
	}{
		{'b', "rebasing"},
		{'r', "re-running"},
	} {
		m := testModel(t, 200, 40, ViewBoard)
		m = selectFailing(t, m)
		pr, _ := m.selectedPR()

		out, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{tc.key}})
		m = out.(Model)
		out, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m = out.(Model)

		if cmd == nil {
			t.Fatalf("%q: confirming produced no command", tc.key)
		}
		if got := m.pending[pr.Key()]; got != tc.label {
			t.Errorf("%q: pending is %q, want %q", tc.key, got, tc.label)
		}
	}
}

func TestRerunRefusesWhenNothingIsFailing(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	for _, col := range m.order {
		for _, pr := range m.lanes[col] {
			if pr.CheckCounts().Failed == 0 {
				m.sel = pr.Key()
				m.syncLaneToSelection()
			}
		}
	}

	out, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = out.(Model)

	if _, open := m.overlay(); open {
		t.Fatal("re-run should not ask when nothing is failing")
	}
	if cmd == nil {
		t.Fatal("expected an explanation")
	}
	if msg, ok := cmd().(toastMsg); !ok || !strings.Contains(msg.text, "nothing is failing") {
		t.Fatalf("unexpected feedback: %#v", cmd())
	}
}

func TestCIRerunConfirms(t *testing.T) {
	m := testModel(t, 200, 40, ViewCI)

	out, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = out.(Model)

	kind, open := m.overlay()
	if !open || kind != ovConfirm {
		t.Fatalf("ci re-run ran without asking (cmd=%v)", cmd != nil)
	}
	if !strings.HasPrefix(m.confirm.title, "Re-run") {
		t.Fatalf("unexpected title %q", m.confirm.title)
	}

	out, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("confirming should run it")
	}
}

func TestHarmlessActionsDoNotConfirm(t *testing.T) {
	for _, k := range []rune{'y', 'Y', 'u'} {
		m := testModel(t, 200, 40, ViewBoard)
		out, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{k}})
		m = out.(Model)

		if _, open := m.overlay(); open {
			t.Errorf("%q should not need confirmation", k)
		}
	}
}

func TestConfirmBodiesFitThePanel(t *testing.T) {
	for _, k := range []rune{'a', 'm', 'X', 'b', 'r'} {
		m := testModel(t, 200, 40, ViewBoard)
		m = selectFailing(t, m)
		m.viewer = "nobody"

		out, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{k}})
		m = out.(Model)

		panel := m.renderConfirm()
		for _, line := range strings.Split(panel, "\n") {
			if w := lipglossWidth(line); w > confirmWidth {
				t.Errorf("%q: confirm line is %d wide:\n%q", k, w, line)
			}
		}
		if strings.Contains(stripANSI(panel), "%!") {
			t.Errorf("%q: confirm has a formatting bug:\n%s", k, stripANSI(panel))
		}
	}
}

var _ = model.PullRequest{}
