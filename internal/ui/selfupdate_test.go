package ui

import (
	"errors"
	"strings"
	"testing"
)

func TestUpdateAsksBeforeReplacingTheBinary(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m.newVersion = "v9.9.9"

	m = send(m, "U")
	kind, ok := m.overlay()
	if !ok || kind != ovConfirm {
		t.Fatal("U should ask first, replacing a binary is not undoable")
	}
	if !strings.Contains(m.confirm.body+m.confirm.title, "v9.9.9") {
		t.Errorf("the prompt should name the version, got %q / %q", m.confirm.title, m.confirm.body)
	}
}

func TestUpdateSaysSoWhenThereIsNothingToDo(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m.newVersion = ""

	m = send(m, "U")
	if _, ok := m.overlay(); ok {
		t.Error("with nothing to install there is nothing to confirm")
	}
	if m.updating {
		t.Error("and nothing should start")
	}
}

func TestConfirmingTheUpdateStartsItAndKeepsTheSpinnerGoing(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m.newVersion = "v9.9.9"
	m = send(m, "U")

	m = send(m, "y")
	if !m.updating {
		t.Fatal("saying yes should start the install")
	}
	if !m.needsSpinner() {
		t.Error("a running install must keep the spinner animating")
	}
	if _, ok := m.overlay(); ok {
		t.Error("the confirm overlay should be gone")
	}
}

func TestDecliningTheUpdateChangesNothing(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m.newVersion = "v9.9.9"
	m = send(m, "U")

	m = send(m, "n")
	if m.updating {
		t.Error("saying no must not install anything")
	}
	if m.newVersion != "v9.9.9" {
		t.Error("and the notice should stay")
	}
}

func TestAFinishedUpdateTellsYouToRestart(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m.newVersion = "v9.9.9"
	m.updating = true

	out, _ := m.Update(updateDoneMsg{version: "v9.9.9"})
	m = out.(Model)

	if m.updating {
		t.Error("the install is over")
	}
	if !m.restartWanted {
		t.Error("the running process is still the old binary, the user needs to know")
	}
	if !strings.Contains(stripANSI(m.View()), "restart") {
		t.Errorf("the screen should say so:\n%s", stripANSI(m.View()))
	}
}

func TestAFailedUpdateExplainsWhy(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m.newVersion = "v9.9.9"
	m.updating = true

	out, cmd := m.Update(updateDoneMsg{err: errors.New("permission denied on /usr/local/bin/prdash")})
	m = out.(Model)
	if cmd == nil {
		t.Fatal("a failure must produce a message for the user")
	}
	out, _ = m.Update(cmd())
	m = out.(Model)

	if m.updating {
		t.Error("the attempt is over")
	}
	if m.restartWanted {
		t.Error("nothing was replaced, so there is nothing to restart into")
	}
	if !strings.Contains(stripANSI(m.View()), "permission denied") {
		t.Errorf("the reason must reach the user:\n%s", stripANSI(m.View()))
	}
}

func TestTheFooterOffersTheUpdateOnlyWhenThereIsOne(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	if strings.Contains(stripANSI(m.View()), "U update") {
		t.Error("no update, no offer")
	}

	m.newVersion = "v9.9.9"
	if !strings.Contains(stripANSI(m.View()), "U update") {
		t.Errorf("an available update should advertise the key:\n%s", stripANSI(m.View()))
	}
}
