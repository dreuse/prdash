package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

const chordFile = "c"

var chordStarters = map[string]bool{"g": true, "]": true, "[": true}

func (m Model) handleChord(msg tea.KeyMsg) (bool, tea.Model, tea.Cmd) {
	pending := m.chord
	if pending == "" {
		if !chordStarters[msg.String()] {
			return false, m, nil
		}
		m.chord = msg.String()
		return true, m, nil
	}

	m.chord = ""
	switch pending + msg.String() {
	case "gg":
		mm, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyHome})
		return true, mm, cmd
	case "]" + chordFile:
		mm, cmd := m.jumpFile(1)
		return true, mm, cmd
	case "[" + chordFile:
		mm, cmd := m.jumpFile(-1)
		return true, mm, cmd
	}
	return true, m, nil
}
