package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dreuse/prdash/internal/model"
)

const (
	doubleClickWindow = 400 * time.Millisecond
	wheelStep         = 3
)

type cardHit struct {
	key                      model.Key
	number                   int
	lane                     int
	top, bottom, left, right int
}

func (h cardHit) covers(x, y int) bool {
	return y >= h.top && y < h.bottom && x >= h.left && x < h.right
}

func (m Model) boardHits(boardHeight int) []cardHit {
	l := Layout{Width: m.width, Height: m.height}
	if len(m.order) == 0 || boardHeight <= laneHeaderRows {
		return nil
	}

	filled := make([]model.Column, 0, len(m.order))
	index := make([]int, 0, len(m.order))
	for i, col := range m.order {
		if len(m.lanes[col]) > 0 {
			filled = append(filled, col)
			index = append(index, i)
		}
	}
	if len(filled) == 0 {
		return nil
	}

	current := 0
	for i, col := range filled {
		if col == m.order[clamp(m.laneIdx, 0, len(m.order)-1)] {
			current = i
		}
	}

	offset, visible := laneWindow(len(filled), current, l.MaxVisibleLanes())
	window := filled[offset : offset+visible]

	counts := make([]int, len(window))
	for i, col := range window {
		counts[i] = len(m.lanes[col])
	}
	widths := laneWidths(counts, m.width)

	hits := []cardHit{}
	x := 0
	for i, col := range window {
		prs := m.lanes[col]
		heights := make([]int, len(prs))
		for j, pr := range prs {
			heights[j] = m.cardHeight(pr, col) + 1
		}

		active := col == m.order[clamp(m.laneIdx, 0, len(m.order)-1)]
		start, end := fitCards(heights, maxInt(1, boardHeight-laneHeaderRows), m.laneRow(), active)

		y := laneHeaderRows
		for j := start; j < end; j++ {
			hits = append(hits, cardHit{
				key:    prs[j].Key(),
				number: prs[j].Number,
				lane:   index[offset+i],
				top:    y,
				bottom: y + m.cardHeight(prs[j], col),
				left:   x,
				right:  x + widths[i],
			})
			y += heights[j]
		}
		x += widths[i] + laneGutter
	}
	return hits
}

func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if _, ok := m.overlay(); ok || m.comment.active || m.filterBar.on {
		return m, nil
	}
	if m.view != ViewBoard {
		return m, nil
	}

	y := msg.Y - m.chromeTop()
	if y < 0 {
		return m, nil
	}
	boardHeight, detailHeight := m.splitHeights(Layout{Width: m.width, Height: m.height}, m.bodyHeight())
	onPane := detailHeight > 0 && y >= boardHeight

	switch msg.Button {
	case tea.MouseButtonWheelUp, tea.MouseButtonWheelDown:
		if msg.Action != tea.MouseActionPress {
			return m, nil
		}
		return m.wheel(msg.Button == tea.MouseButtonWheelUp, onPane)
	case tea.MouseButtonLeft:
		if msg.Action != tea.MouseActionPress {
			return m, nil
		}
		if onPane {
			m.detail.focus = true
			return m, nil
		}
		return m.clickBoard(msg.X, y, boardHeight)
	}
	return m, nil
}

func (m Model) wheel(up, onPane bool) (tea.Model, tea.Cmd) {
	if onPane {
		pr, ok := m.selectedPR()
		if !ok {
			return m, nil
		}
		rows := m.detailRows()
		last := maxInt(0, m.detailTotal(pr, m.width)-rows)
		step := wheelStep
		if up {
			step = -step
		}
		m.detail.scroll = clamp(m.detail.scroll+step, 0, last)
		return m, nil
	}

	lane := m.currentLane()
	row := m.laneRow()
	switch {
	case up && row > 0:
		m.sel = lane[row-1].Key()
	case !up && row+1 < len(lane):
		m.sel = lane[row+1].Key()
	default:
		return m, nil
	}
	return m.selectionMoved()
}

func (m Model) clickBoard(x, y, boardHeight int) (tea.Model, tea.Cmd) {
	m.detail.focus = false

	for _, hit := range m.boardHits(boardHeight) {
		if !hit.covers(x, y) {
			continue
		}
		again := hit.key == m.lastClick && nowSince(m.lastClickAt) < doubleClickWindow
		m.lastClick, m.lastClickAt = hit.key, time.Now()

		if hit.key == m.sel {
			if again {
				return m, openURL(m.selectedURL())
			}
			return m, nil
		}
		m.laneIdx = hit.lane
		m.sel = hit.key
		return m.selectionMoved()
	}
	return m, nil
}

func (m Model) selectedURL() string {
	if pr, ok := m.selectedPR(); ok {
		return pr.URL
	}
	return ""
}
