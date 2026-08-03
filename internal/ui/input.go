package ui

import (
	"strconv"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/dreuse/prdash/internal/model"
)

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.comment.active {
		return m.handleCommentKey(msg)
	}
	if m.filterBar.on {
		return m.handleFilterKey(msg)
	}
	if kind, ok := m.overlay(); ok {
		return m.handleOverlayKey(kind, msg)
	}
	if handled, mm, cmd := m.handleGlobalKey(msg); handled {
		return mm, cmd
	}
	if m.view == ViewCI {
		return m.handleCIKey(msg)
	}
	return m.handleBoardKey(msg)
}

func (m Model) handleGlobalKey(msg tea.KeyMsg) (bool, tea.Model, tea.Cmd) {
	k := m.keys
	switch {
	case key.Matches(msg, k.Quit):
		m.writeStores()
		return true, m, tea.Quit

	case key.Matches(msg, k.Board):
		mm, cmd := m.switchView(ViewBoard)
		return true, mm, cmd
	case key.Matches(msg, k.CI):
		mm, cmd := m.switchView(ViewCI)
		return true, mm, cmd
	case key.Matches(msg, k.Help):
		m.push(ovHelp)
		return true, m, nil

	case key.Matches(msg, k.Settings):
		m.push(ovSettings)
		return true, m, nil

	case key.Matches(msg, k.Repo):
		m.repoPick = newRepoPicker()
		m.push(ovRepo)
		return true, m, nil

	case key.Matches(msg, k.Filter):
		mm, cmd := m.openFilter()
		return true, mm, cmd

	case key.Matches(msg, k.Refresh):
		m.loading = true
		m.tickGen++
		return true, m, tea.Batch(m.fetchCmd(), m.scheduleTick(), m.notify("refreshing…", toastInfo))

	case key.Matches(msg, k.Sort):
		m.sortMode = m.sortMode.Next()
		m.rebuild()
		save := m.persist()
		return true, m, tea.Batch(save, m.notify("sort: "+m.sortMode.String()+"  ("+m.keys.Settings.Help().Key+" to make it permanent)", toastInfo))

	case key.Matches(msg, k.Back):
		if m.view == ViewCI && m.logs.open {
			m.logs.close()
			return true, m, nil
		}
		if !m.filter.Empty() {
			m.filter = model.ParseFilter("")
			m.filterBar.input.SetValue("")
			m.rebuild()
			return true, m, m.persist()
		}
		return true, m, nil

	case key.Matches(msg, k.Save1):
		mm, cmd := m.savedFilter(0)
		return true, mm, cmd
	case key.Matches(msg, k.Save2):
		mm, cmd := m.savedFilter(1)
		return true, mm, cmd
	case key.Matches(msg, k.Save3):
		mm, cmd := m.savedFilter(2)
		return true, mm, cmd
	case key.Matches(msg, k.Save4):
		mm, cmd := m.savedFilter(3)
		return true, mm, cmd
	}
	return false, m, nil
}

func (m Model) switchView(v View) (tea.Model, tea.Cmd) {
	m.view = v
	m.stack = nil
	m.rebuild()
	return m, m.persist()
}

func (m Model) savedFilter(slot int) (tea.Model, tea.Cmd) {
	if slot >= len(m.settings.SavedFilters) {
		return m, nil
	}
	stored := m.settings.SavedFilters[slot]
	if stored == "" || stored == m.filter.Raw {
		m.settings.SavedFilters[slot] = m.filter.Raw
		return m, tea.Batch(m.persist(), m.notify("saved filter to F"+itoa(slot+1), toastGood))
	}
	m.filter = model.ParseFilter(stored)
	m.filterBar.input.SetValue(stored)
	m.rebuild()
	return m, tea.Batch(m.persist(), m.notify("filter F"+itoa(slot+1)+": "+stored, toastInfo))
}

func (m Model) handleOverlayKey(kind overlayKind, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch kind {
	case ovSettings:
		return m.handleSettingsKey(msg)
	case ovConfirm:
		return m.handleConfirmKey(msg)
	case ovRepo:
		return m.handleRepoKey(msg)
	case ovHelp:
		m.pop()
		return m, nil
	}
	m.pop()
	return m, nil
}

func (m Model) handleBoardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := m.keys
	lane := m.currentLane()
	row := m.laneRow()

	switch {
	case key.Matches(msg, k.Down):
		if row+1 < len(lane) {
			m.sel = lane[row+1].Key()
		}
	case key.Matches(msg, k.Up):
		if row > 0 {
			m.sel = lane[row-1].Key()
		}
	case key.Matches(msg, k.Top):
		if len(lane) > 0 {
			m.sel = lane[0].Key()
		}
	case key.Matches(msg, k.End):
		if len(lane) > 0 {
			m.sel = lane[len(lane)-1].Key()
		}
	case key.Matches(msg, k.Left):
		return m.moveLane(-1), m.persist()
	case key.Matches(msg, k.Right):
		return m.moveLane(1), m.persist()
	case key.Matches(msg, k.Split):
		m.split = !m.split
		return m, nil
	case key.Matches(msg, k.Expand):
		m.expanded[passedChecks] = !m.expanded[passedChecks]
		return m, nil
	default:
		return m.handlePRAction(msg)
	}
	return m, m.persist()
}

func (m Model) moveLane(delta int) Model {
	if len(m.order) == 0 {
		return m
	}
	row := m.laneRow()
	for i := 1; i <= len(m.order); i++ {
		idx := wrapIndex(m.laneIdx+delta*i, len(m.order))
		prs := m.lanes[m.order[idx]]
		if len(prs) == 0 {
			continue
		}
		m.laneIdx = idx
		m.sel = prs[clamp(row, 0, len(prs)-1)].Key()
		return m
	}
	return m
}

func itoa(n int) string { return strconv.Itoa(n) }
