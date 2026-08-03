package ui

import (
	"strings"
	"testing"
)

func footerOf(t *testing.T, m Model) string {
	t.Helper()
	lines := strings.Split(strings.TrimRight(stripANSI(m.View()), "\n"), "\n")
	return lines[len(lines)-1]
}

func TestTheFooterCarriesTheRunningVersion(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m.version = "v0.2.0"

	if footer := footerOf(t, m); !strings.Contains(footer, "v0.2.0") {
		t.Errorf("you should be able to read the version without leaving the app, got %q", footer)
	}
}

func TestAnAvailableUpdateReachesTheFooterAndAToast(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m.version = "v0.2.0"

	out, cmd := m.Update(updateMsg{latest: "v0.3.0"})
	if cmd == nil {
		t.Fatal("a new release should announce itself while you are in the app")
	}
	toast, ok := cmd().(toastMsg)
	if !ok || !strings.Contains(toast.text, "v0.3.0") {
		t.Fatalf("the toast should name the new version, got %#v", cmd())
	}

	footer := footerOf(t, out.(Model))
	if !strings.Contains(footer, "v0.2.0") || !strings.Contains(footer, "v0.3.0") {
		t.Errorf("the footer should keep showing the update after the toast fades, got %q", footer)
	}
}

func TestTheFooterStaysQuietWhenTheBuildIsCurrent(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m.version = "v0.3.0"

	if got := m.versionLabel(); got != "v0.3.0" {
		t.Errorf("with nothing to update to the version stands alone, got %q", got)
	}
}
