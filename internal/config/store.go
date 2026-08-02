package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const appDir = "pr-dashboard"

func Dir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, appDir)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return appDir
	}
	return filepath.Join(home, ".config", appDir)
}

func path(name string) string { return filepath.Join(Dir(), name) }

func readJSON(name string, dst any) bool {
	data, err := os.ReadFile(path(name))
	if err != nil || len(data) == 0 {
		return false
	}
	return json.Unmarshal(data, dst) == nil
}

func writeJSON(name string, src any) error {
	dir := Dir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(src, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, name+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path(name))
}

func Exists() bool {
	_, err := os.Stat(path(settingsFile))
	return err == nil
}
