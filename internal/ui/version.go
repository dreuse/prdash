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

type updateDoneMsg struct {
	version string
	err     error
}

func applyUpdateCmd(tag string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), update.ApplyTimeout)
		defer cancel()
		if err := update.Apply(ctx, tag); err != nil {
			return updateDoneMsg{err: err}
		}
		return updateDoneMsg{version: tag}
	}
}

func (m Model) startUpdate() (tea.Model, tea.Cmd) {
	if m.newVersion == "" {
		return m, m.notify("already on the latest release", toastInfo)
	}
	if m.updating {
		return m, m.notify("already installing", toastInfo)
	}
	return m.ask(confirmState{
		title: "Update to " + m.newVersion + "?",
		body: "Downloads the release and replaces this binary.\n" +
			"You will need to restart prdash to use it.",
		verb:    "update",
		updates: true,
		run:     func(mm Model) tea.Cmd { return applyUpdateCmd(mm.newVersion) },
	})
}

func (m Model) applyUpdateDone(msg updateDoneMsg) (tea.Model, tea.Cmd) {
	m.updating = false
	if msg.err != nil {
		return m, m.notify("update failed: "+msg.err.Error(), toastBad)
	}
	m.restartWanted = true
	m.newVersion = ""
	return m, m.notify("installed "+msg.version+" - restart prdash to use it", toastGood)
}

func (m Model) versionLabel() string {
	if m.restartWanted {
		return m.version + " " + m.theme.Glyphs.Arrow + " restart to finish"
	}
	if m.newVersion == "" {
		return m.version
	}
	return m.version + " " + m.theme.Glyphs.Arrow + " " + m.newVersion
}
