package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func assertPrivate(t *testing.T, base string) {
	t.Helper()

	info, err := os.Stat(base)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		t.Errorf("%s holds private repository data, want 0700, got %v", appDir, perm)
	}

	for _, name := range []string{cacheFile, settingsFile, stateFile} {
		info, err := os.Stat(filepath.Join(base, name))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm&0o077 != 0 {
			t.Errorf("%s caches private pull request titles, want 0600, got %v", name, perm)
		}
	}
}

func TestStoreIsNotReadableByOtherUsers(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	if err := SaveCache(Cache{Viewer: "me"}); err != nil {
		t.Fatal(err)
	}
	if err := SaveSettings(DefaultSettings()); err != nil {
		t.Fatal(err)
	}
	if err := SaveState(State{}); err != nil {
		t.Fatal(err)
	}

	assertPrivate(t, filepath.Join(dir, appDir))
}

func TestAWorldReadableStoreFromAnOlderBuildGetsTightened(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	base := filepath.Join(dir, appDir)
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, cacheFile), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := SaveCache(Cache{Viewer: "me"}); err != nil {
		t.Fatal(err)
	}
	assertPrivate(t, base)
}

func TestAnOldConfigKeepsTheMouseOn(t *testing.T) {
	var stored Settings
	body := `{"schema":1,"default_view":"board","refresh_seconds":30}`
	if err := json.Unmarshal([]byte(body), &stored); err != nil {
		t.Fatal(err)
	}
	if stored.DisableMouse {
		t.Error("a config written before the mouse existed must not switch it off")
	}
}
