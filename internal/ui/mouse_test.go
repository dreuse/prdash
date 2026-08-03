package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func click(m Model, x, y int) Model {
	out, _ := m.Update(tea.MouseMsg{
		X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	})
	return out.(Model)
}

func wheel(m Model, x, y int, up bool) Model {
	button := tea.MouseButtonWheelDown
	if up {
		button = tea.MouseButtonWheelUp
	}
	out, _ := m.Update(tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionPress, Button: button})
	return out.(Model)
}

func screenRowOf(t *testing.T, m Model, want string) int {
	t.Helper()
	for i, line := range strings.Split(stripANSI(m.View()), "\n") {
		if strings.Contains(line, want) {
			return i
		}
	}
	t.Fatalf("%q is not on screen:\n%s", want, stripANSI(m.View()))
	return -1
}

func TestTheHitMapAgreesWithWhatIsOnScreen(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	boardHeight, _ := m.splitHeights(Layout{Width: m.width, Height: m.height}, m.bodyHeight())

	for _, hit := range m.boardHits(boardHeight) {
		label := "#" + itoa(hit.number)
		row := screenRowOf(t, m, label)

		if row < hit.top+m.chromeTop() || row >= hit.bottom+m.chromeTop() {
			t.Errorf("%s renders on row %d but the hit map claims rows %d..%d",
				label, row, hit.top+m.chromeTop(), hit.bottom+m.chromeTop())
		}
		line := strings.Split(stripANSI(m.View()), "\n")[row]
		if col := strings.Index(line, label); col < hit.left || col >= hit.right {
			t.Errorf("%s renders at column %d but the hit map claims %d..%d",
				label, col, hit.left, hit.right)
		}
	}
}

func TestClickingACardSelectsIt(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	boardHeight, _ := m.splitHeights(Layout{Width: m.width, Height: m.height}, m.bodyHeight())

	var target cardHit
	for _, hit := range m.boardHits(boardHeight) {
		if hit.key != m.sel {
			target = hit
			break
		}
	}
	if target.number == 0 {
		t.Fatal("the board needs a second card to click on")
	}

	m = click(m, target.left+1, target.top+m.chromeTop())
	if m.sel != target.key {
		t.Errorf("clicking #%d should select it, selection is %+v", target.number, m.sel)
	}
}

func TestClickingACardInAnotherLaneFollowsTheLane(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	boardHeight, _ := m.splitHeights(Layout{Width: m.width, Height: m.height}, m.bodyHeight())

	var target cardHit
	for _, hit := range m.boardHits(boardHeight) {
		if hit.lane != m.laneIdx {
			target = hit
			break
		}
	}
	if target.number == 0 {
		t.Skip("the fixture has a single visible lane")
	}

	m = click(m, target.left+1, target.top+m.chromeTop())
	if m.laneIdx != target.lane {
		t.Errorf("clicking into lane %d should move there, laneIdx is %d", target.lane, m.laneIdx)
	}
	if m.sel != target.key {
		t.Errorf("and it should select the card, selection is %+v", m.sel)
	}
}

func TestClickingEmptyBoardSpaceChangesNothing(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	before := m.sel

	m = click(m, m.width-1, m.chromeTop()+1)
	if m.sel != before {
		t.Errorf("a click on nothing must not move the selection, went to %+v", m.sel)
	}
}

func TestClickingThePaneFocusesItAndTheBoardTakesItBack(t *testing.T) {
	m, _ := splitScreen(t, 200, 40, 12009)
	boardHeight, detailHeight := m.splitHeights(Layout{Width: m.width, Height: m.height}, m.bodyHeight())
	if detailHeight == 0 {
		t.Fatal("the split pane is not open")
	}

	m = click(m, 5, m.chromeTop()+boardHeight+1)
	if !m.detail.focus {
		t.Error("clicking the pane should focus it")
	}

	m = click(m, 5, m.chromeTop()+1)
	if m.detail.focus {
		t.Error("clicking back on the board should hand focus back")
	}
}

func TestTheWheelScrollsWhicheverHalfIsUnderThePointer(t *testing.T) {
	m, _ := splitScreen(t, 80, 24, 12009)
	boardHeight, _ := m.splitHeights(Layout{Width: m.width, Height: m.height}, m.bodyHeight())

	m = wheel(m, 5, m.chromeTop()+boardHeight+1, false)
	if m.detail.scroll == 0 {
		t.Error("the wheel over the pane should scroll the pane")
	}
	scrolled := m.detail.scroll

	m = wheel(m, 5, m.chromeTop()+boardHeight+1, true)
	if m.detail.scroll >= scrolled {
		t.Errorf("wheeling up should come back, went from %d to %d", scrolled, m.detail.scroll)
	}
}

func TestTheWheelOverTheBoardMovesTheSelection(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m = selectPR(t, m, 11959)
	before := m.sel

	m = wheel(m, 5, m.chromeTop()+1, false)
	if m.sel == before {
		t.Error("the wheel over the board should move the selection")
	}
}

func TestADoubleClickOpensThePullRequest(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	boardHeight, _ := m.splitHeights(Layout{Width: m.width, Height: m.height}, m.bodyHeight())
	hits := m.boardHits(boardHeight)
	if len(hits) == 0 {
		t.Fatal("no cards on the board")
	}
	target := hits[0]

	m = click(m, target.left+1, target.top+m.chromeTop())
	out, cmd := m.Update(tea.MouseMsg{
		X: target.left + 1, Y: target.top + m.chromeTop(),
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	})
	m = out.(Model)

	if cmd == nil {
		t.Error("a second click on the same card should open it")
	}
	if m.sel != target.key {
		t.Errorf("and the selection should stay put, got %+v", m.sel)
	}
}

func TestTwoSlowClicksAreNotADoubleClick(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	boardHeight, _ := m.splitHeights(Layout{Width: m.width, Height: m.height}, m.bodyHeight())
	target := m.boardHits(boardHeight)[0]

	m = click(m, target.left+1, target.top+m.chromeTop())
	m.lastClickAt = m.lastClickAt.Add(-2 * doubleClickWindow)

	_, cmd := m.Update(tea.MouseMsg{
		X: target.left + 1, Y: target.top + m.chromeTop(),
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	})
	if cmd != nil {
		t.Error("clicks a long way apart must not open anything")
	}
}

func TestTheMouseIsIgnoredWhileAnOverlayIsOpen(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	before := m.sel
	m.push(ovHelp)

	boardHeight, _ := m.splitHeights(Layout{Width: m.width, Height: m.height}, m.bodyHeight())
	for _, hit := range m.boardHits(boardHeight) {
		if hit.key != before {
			m = click(m, hit.left+1, hit.top+m.chromeTop())
			break
		}
	}
	if m.sel != before {
		t.Errorf("the overlay owns the screen, selection moved to %+v", m.sel)
	}
}

func TestReleaseAndMotionAreNotClicks(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	boardHeight, _ := m.splitHeights(Layout{Width: m.width, Height: m.height}, m.bodyHeight())
	var target cardHit
	for _, hit := range m.boardHits(boardHeight) {
		if hit.key != m.sel {
			target = hit
			break
		}
	}
	before := m.sel

	for _, action := range []tea.MouseAction{tea.MouseActionRelease, tea.MouseActionMotion} {
		out, _ := m.Update(tea.MouseMsg{
			X: target.left + 1, Y: target.top + m.chromeTop(),
			Action: action, Button: tea.MouseButtonLeft,
		})
		if out.(Model).sel != before {
			t.Errorf("%v must not select anything", action)
		}
	}
}

func TestDoubleClickWindowIsAWholeNumberOfMilliseconds(t *testing.T) {
	if doubleClickWindow < 100*time.Millisecond || doubleClickWindow > time.Second {
		t.Errorf("a double click window of %v will feel wrong", doubleClickWindow)
	}
}
