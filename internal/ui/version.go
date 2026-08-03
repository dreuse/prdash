package ui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dreuse/prdash/internal/update"
)

const updateCheckTimeout = 20 * time.Second

type updateMsg struct{ latest string }

func checkUpdateCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), updateCheckTimeout)
		defer cancel()
		latest, err := update.Latest(ctx)
		if err != nil || !update.Newer(latest, update.Current()) {
			return nil
		}
		return updateMsg{latest: latest}
	}
}

func (m Model) versionLabel() string {
	if m.newVersion == "" {
		return m.version
	}
	return m.version + " " + m.theme.Glyphs.Arrow + " " + m.newVersion
}
