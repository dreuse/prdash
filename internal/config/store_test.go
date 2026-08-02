package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSettingsRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	s := DefaultSettings()
	s.DefaultView = "queue"
	s.RefreshSeconds = 60
	s.Theme = "dark"
	s.HiddenLanes = []string{"draft"}
	s.StartupFilter = "reviewer:@me"
	s.SavedFilters[1] = "is:stale"

	if err := SaveSettings(s); err != nil {
		t.Fatalf("save: %v", err)
	}
	got := LoadSettings()

	if got.DefaultView != "queue" || got.RefreshSeconds != 60 || got.Theme != "dark" {
		t.Fatalf("settings did not survive a round trip: %+v", got)
	}
	if len(got.HiddenLanes) != 1 || got.HiddenLanes[0] != "draft" {
		t.Fatalf("hidden lanes did not survive: %v", got.HiddenLanes)
	}
	if got.SavedFilters[1] != "is:stale" {
		t.Fatalf("saved filters did not survive: %v", got.SavedFilters)
	}
}

func TestCorruptStoreFallsBackToDefaults(t *testing.T) {
	for _, body := range []string{"", "{", "not json at all", `{"schema": 999}`} {
		dir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", dir)
		if err := os.MkdirAll(filepath.Join(dir, appDir), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, appDir, settingsFile), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}

		got := LoadSettings()
		if got.DefaultView != DefaultSettings().DefaultView || got.RefreshSeconds != DefaultSettings().RefreshSeconds {
			t.Fatalf("a store containing %q must fall back to defaults, got %+v", body, got)
		}
	}
}

func TestMissingStoreFallsBackToDefaults(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if LoadSettings().DefaultView != DefaultSettings().DefaultView {
		t.Fatal("a missing store must load defaults")
	}
	if LoadState().Schema != stateSchema {
		t.Fatal("a missing state file must load a fresh state")
	}
	if _, ok := LoadCache(); ok {
		t.Fatal("a missing cache must report no cache")
	}
}

func TestNormaliseRepairsNonsense(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	s := DefaultSettings()
	s.RefreshSeconds = 0
	s.CIRunsWindow = -5
	s.Theme = ""
	s.Sort = ""
	s.SavedFilters = nil
	if err := SaveSettings(s); err != nil {
		t.Fatal(err)
	}

	got := LoadSettings()
	d := DefaultSettings()
	if got.RefreshSeconds != d.RefreshSeconds || got.CIRunsWindow != d.CIRunsWindow {
		t.Fatalf("out of range numbers were not repaired: %+v", got)
	}
	if got.Theme != d.Theme || got.Sort != d.Sort {
		t.Fatalf("empty strings were not repaired: %+v", got)
	}
	if len(got.SavedFilters) != SavedFilterN {
		t.Fatalf("saved filter slots were not restored: %v", got.SavedFilters)
	}
}

func TestWriteIsAtomic(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	if err := SaveSettings(DefaultSettings()); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(dir, appDir))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != settingsFile {
			t.Fatalf("a temporary file was left behind: %s", e.Name())
		}
	}
}
