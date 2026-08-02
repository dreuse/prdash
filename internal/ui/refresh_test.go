package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTickKeepsRescheduling(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)

	for round := 0; round < 5; round++ {
		out, cmd := m.Update(tickMsg{gen: m.tickGen})
		m = out.(Model)
		if cmd == nil {
			t.Fatalf("round %d: a tick produced no follow-up, the refresh loop is dead", round)
		}
		if !m.loading {
			t.Fatalf("round %d: a tick did not start a fetch", round)
		}
	}
}

func TestStaleTickIsIgnored(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m.tickGen = 7

	out, cmd := m.Update(tickMsg{gen: 3})
	if cmd != nil {
		t.Fatal("a superseded tick must not schedule anything")
	}
	if out.(Model).loading {
		t.Fatal("a superseded tick must not start a fetch")
	}
}

func TestBlurLengthensAndFocusRestoresTheInterval(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m.settings.RefreshSeconds = 30

	if d := m.interval(); d < 30*time.Second || d > 30*time.Second+tickJitter {
		t.Fatalf("focused interval is %s, want about 30s", d)
	}

	out, cmd := m.Update(tea.BlurMsg{})
	m = out.(Model)
	if cmd == nil {
		t.Fatal("blur must reschedule, otherwise the old interval stands")
	}
	if d := m.interval(); d < unfocusedInterval {
		t.Fatalf("unfocused interval is %s, want at least %s", d, unfocusedInterval)
	}

	m.lastUpdate = time.Now()
	out, cmd = m.Update(tea.FocusMsg{})
	m = out.(Model)
	if cmd == nil {
		t.Fatal("regaining focus must reschedule, otherwise the five minute tick stands")
	}
	if d := m.interval(); d > 30*time.Second+tickJitter {
		t.Fatalf("interval stayed at %s after regaining focus", d)
	}
}

func TestRegainingFocusRefreshesStaleData(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m.settings.RefreshSeconds = 30
	m.loading = false

	out, _ := m.Update(tea.BlurMsg{})
	m = out.(Model)
	m.lastUpdate = time.Now().Add(-2 * time.Minute)

	out, cmd := m.Update(tea.FocusMsg{})
	m = out.(Model)
	if !m.loading || cmd == nil {
		t.Fatal("coming back to a stale board should fetch straight away")
	}
}

func TestRegainingFocusDoesNotRefetchFreshData(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m.settings.RefreshSeconds = 30
	m.loading = false

	out, _ := m.Update(tea.BlurMsg{})
	m = out.(Model)
	m.lastUpdate = time.Now()

	out, _ = m.Update(tea.FocusMsg{})
	if out.(Model).loading {
		t.Fatal("data fetched a moment ago should not be refetched on focus")
	}
}

func TestRepeatedFocusEventsDoNotStack(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m.lastUpdate = time.Now()

	out, _ := m.Update(tea.FocusMsg{})
	m = out.(Model)
	before := m.tickGen

	out, cmd := m.Update(tea.FocusMsg{})
	m = out.(Model)
	if cmd != nil || m.tickGen != before {
		t.Fatal("a duplicate focus event must be ignored")
	}
}
