package ui

import (
	"reflect"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

func TestEveryBindingCanBeOverridden(t *testing.T) {
	k := DefaultKeyMap()
	named := k.index()

	v := reflect.ValueOf(k)
	for i := 0; i < v.NumField(); i++ {
		field := v.Type().Field(i)
		if field.Type != reflect.TypeOf(key.Binding{}) {
			continue
		}
		if _, ok := named[actionName(field.Name)]; !ok {
			t.Errorf("%s is not in the override index, so nobody can rebind it", field.Name)
		}
	}
}

func TestAnOverrideReplacesTheDefaultKey(t *testing.T) {
	k := DefaultKeyMap().Override(map[string]string{"diff": "D"})

	if !key.Matches(runeKey("D"), k.Diff) {
		t.Error("D should open the diff after the override")
	}
	if key.Matches(runeKey("d"), k.Diff) {
		t.Error("the default d should be gone once it is overridden")
	}
	if !key.Matches(runeKey("v"), k.Split) {
		t.Error("overriding one action must leave the others alone")
	}
}

func TestAnOverrideCanBindMoreThanOneKey(t *testing.T) {
	k := DefaultKeyMap().Override(map[string]string{"diff": "D  ctrl+g"})

	for _, want := range []string{"D", "ctrl+g"} {
		if !keyMatchesString(k.Diff, want) {
			t.Errorf("%q should be bound to the diff", want)
		}
	}
}

func TestAnOverrideKeepsTheHelpDescription(t *testing.T) {
	before := DefaultKeyMap().Diff.Help().Desc
	after := DefaultKeyMap().Override(map[string]string{"diff": "D"}).Diff.Help()

	if after.Desc != before {
		t.Errorf("the help text explains the action, not the key: got %q want %q", after.Desc, before)
	}
	if after.Key != "D" {
		t.Errorf("the help should show the new key, got %q", after.Key)
	}
}

func TestNonsenseOverridesAreIgnoredRatherThanFatal(t *testing.T) {
	k := DefaultKeyMap().Override(map[string]string{
		"frobnicate": "z",
		"diff":       "   ",
		"":           "x",
	})

	if !key.Matches(runeKey("d"), k.Diff) {
		t.Error("an empty override must leave the default alone")
	}
	if !key.Matches(runeKey("v"), k.Split) {
		t.Error("an unknown action name must not disturb anything else")
	}
}

func TestCtrlCAlwaysQuits(t *testing.T) {
	k := DefaultKeyMap().Override(map[string]string{"quit": "Q"})

	if !keyMatchesString(k.Quit, "ctrl+c") {
		t.Error("ctrl-c must survive any override, or a bad config traps the user")
	}
	if !keyMatchesString(k.Quit, "Q") {
		t.Error("the override should still apply")
	}
}

func TestTakingAKeyFromAnotherActionWins(t *testing.T) {
	k := DefaultKeyMap().Override(map[string]string{"split": "S"})

	if keyMatchesString(k.Settings, "S") {
		t.Error("S now belongs to the split, settings must let it go")
	}
	if !keyMatchesString(k.Settings, ",") {
		t.Error("settings keeps its other key")
	}
	if k.Settings.Help().Key != "," {
		t.Errorf("the help should advertise a key that still works, got %q", k.Settings.Help().Key)
	}
}

func TestTheHelpScreenShowsTheKeysThatActuallyWork(t *testing.T) {
	k := DefaultKeyMap().Override(map[string]string{"diff": "ctrl+g"})

	var line string
	for _, row := range k.HelpSections(unicodeGlyphs) {
		if strings.Contains(row[1], "read the diff") {
			line = row[0]
		}
	}
	if line == "" {
		t.Fatal("the diff row vanished from the help")
	}
	if line != "ctrl-g" {
		t.Errorf("help must advertise the rebound key, got %q", line)
	}
}

func TestOverridesReachTheRunningModel(t *testing.T) {
	m := testModel(t, 200, 40, ViewBoard)
	m.keys = DefaultKeyMap().Override(map[string]string{"split": "S"})

	m = selectPR(t, m, 12009)
	m = send(m, "v")
	if m.split {
		t.Error("v is no longer the split key")
	}
	m = send(m, "S")
	if !m.split {
		t.Error("S should open the split panel after the override")
	}
}

func keyMatchesString(b key.Binding, want string) bool {
	for _, k := range b.Keys() {
		if k == want {
			return true
		}
	}
	return false
}

func runeKey(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func actionName(field string) string { return strings.ToLower(field) }
