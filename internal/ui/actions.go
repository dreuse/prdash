package ui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/termenv"

	"github.com/dreuse/prdash/internal/github"
	"github.com/dreuse/prdash/internal/model"
)

func (m Model) canApprove(pr model.PullRequest) bool {
	if strings.EqualFold(pr.Author, m.viewer) || pr.IsDraft || m.viewer == "" {
		return false
	}
	return pr.StakeFor(m.viewer) != model.StakeApproved || pr.ReviewStale(m.viewer)
}

func (m Model) approveBlockedReason(pr model.PullRequest) string {
	switch {
	case m.viewer == "":
		return "cannot tell who you are yet"
	case strings.EqualFold(pr.Author, m.viewer):
		return "you cannot approve your own pull request"
	case pr.IsDraft:
		return "a draft cannot be approved"
	}
	return "you already approved this"
}

func (m Model) handlePRAction(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := m.keys
	pr, ok := m.selectedPR()
	if !ok {
		return m, nil
	}

	switch {
	case key.Matches(msg, k.Open):
		return m, openURL(pr.URL)

	case key.Matches(msg, k.Approve):
		if !m.canApprove(pr) {
			return m, m.notify(m.approveBlockedReason(pr), toastInfo)
		}
		body := truncate(pr.Title, 52)
		danger := pr.CheckCounts().Failed > 0
		if danger {
			body = fmt.Sprintf("%d checks are failing on %s.", pr.CheckCounts().Failed, pr.HeadRef)
		}
		return m.ask(confirmState{
			title:  fmt.Sprintf("Approve #%d?", pr.Number),
			body:   body,
			verb:   "approve",
			danger: danger,
			run:    func(mm Model) tea.Cmd { return mm.approveCmd(pr) },
		})

	case key.Matches(msg, k.Merge):
		return m.ask(confirmState{
			title:  fmt.Sprintf("Merge #%d?", pr.Number),
			body:   fmt.Sprintf("squash into %s\n%s is deleted if the repository says so", pr.BaseRef, pr.HeadRef),
			verb:   "merge",
			danger: true,
			run:    func(mm Model) tea.Cmd { return mm.mergeCmd(pr) },
		})

	case key.Matches(msg, k.Close):
		return m.ask(confirmState{
			title:  fmt.Sprintf("Close #%d without merging?", pr.Number),
			body:   "the branch is kept, reopen on github any time\n" + pr.HeadRef,
			verb:   "close",
			danger: true,
			run:    func(mm Model) tea.Cmd { return mm.closeCmd(pr) },
		})

	case key.Matches(msg, k.Comment):
		return m.openComment(pr)

	case key.Matches(msg, k.Rerun):
		failed := pr.CheckCounts().Failed
		if failed == 0 {
			return m, m.notify("nothing is failing on #"+itoa(pr.Number), toastInfo)
		}
		return m.ask(confirmState{
			title: fmt.Sprintf("Re-run %d failed %s on #%d?",
				failed, plural(failed, "check", "checks"), pr.Number),
			body: "starts a new workflow run on\n" + pr.HeadRef,
			verb: "re-run",
			run:  func(mm Model) tea.Cmd { return mm.rerunCmd(pr) },
		})

	case key.Matches(msg, k.Rebase):
		return m.ask(confirmState{
			title:  fmt.Sprintf("Update #%d from %s?", pr.Number, pr.BaseRef),
			body:   "pushes a merge commit to\n" + pr.HeadRef,
			verb:   "update",
			danger: true,
			run:    func(mm Model) tea.Cmd { return mm.rebaseCmd(pr) },
		})

	case key.Matches(msg, k.Copy):
		return m, copyToClipboard(pr.HeadRef, "branch name")

	case key.Matches(msg, k.Clone):
		return m, copyToClipboard("git checkout "+pr.HeadRef, "checkout command")
	}
	return m, nil
}

func (m Model) ask(c confirmState) (tea.Model, tea.Cmd) {
	m.confirm = c
	m.push(ovConfirm)
	return m, nil
}

func (m Model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "enter":
		c := m.confirm
		m.pop()
		m.confirm = confirmState{}
		if c.run == nil {
			return m, nil
		}
		if c.updates {
			m.updating = true
		}
		return m, c.run(m)
	case "n", "esc", "q":
		m.pop()
		m.confirm = confirmState{}
		return m, nil
	}
	return m, nil
}

func (m Model) approveCmd(pr model.PullRequest) tea.Cmd {
	m.pending[pr.Key()] = "approving"
	actor := m.actor
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), actionTimeout)
		defer cancel()
		return actionMsg{key: pr.Key(), verb: "approved", pr: pr, err: actor.Approve(ctx, pr)}
	}
}

func (m Model) mergeCmd(pr model.PullRequest) tea.Cmd {
	m.pending[pr.Key()] = "merging"
	actor := m.actor
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), actionTimeout)
		defer cancel()
		return actionMsg{key: pr.Key(), verb: "merged", pr: pr, err: actor.Merge(ctx, pr, github.MergeSquash)}
	}
}

func (m Model) closeCmd(pr model.PullRequest) tea.Cmd {
	m.pending[pr.Key()] = "closing"
	actor := m.actor
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), actionTimeout)
		defer cancel()
		return actionMsg{key: pr.Key(), verb: "closed", pr: pr, err: actor.Close(ctx, pr)}
	}
}

func (m Model) rerunCmd(pr model.PullRequest) tea.Cmd {
	m.pending[pr.Key()] = "re-running"
	actor := m.actor
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), actionTimeout)
		defer cancel()
		return actionMsg{key: pr.Key(), verb: "re-ran checks on", pr: pr, err: actor.Rerun(ctx, pr)}
	}
}

func (m Model) rebaseCmd(pr model.PullRequest) tea.Cmd {
	m.pending[pr.Key()] = "rebasing"
	actor := m.actor
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), actionTimeout)
		defer cancel()
		return actionMsg{key: pr.Key(), verb: "updated branch on", pr: pr, err: actor.UpdateBranch(ctx, pr)}
	}
}

func (m Model) rerunRunCmd(run model.WorkflowRun) tea.Cmd {
	actor := m.actor
	glyphs := m.theme.Glyphs
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), actionTimeout)
		defer cancel()
		if err := actor.RerunRun(ctx, run); err != nil {
			return toastMsg{text: glyphs.Fail + " re-run failed: " + firstLine(err.Error()), kind: toastBad}
		}
		return toastMsg{text: glyphs.Pass + " re-running " + run.Name, kind: toastGood}
	}
}

func (m Model) submitCommentCmd(pr model.PullRequest, body string) tea.Cmd {
	actor := m.actor
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), actionTimeout)
		defer cancel()
		return actionMsg{key: pr.Key(), verb: "commented on", pr: pr, err: actor.Comment(ctx, pr, body)}
	}
}

func (m Model) applyAction(msg actionMsg) (tea.Model, tea.Cmd) {
	delete(m.pending, msg.key)
	glyphs := m.theme.Glyphs
	if msg.err != nil {
		return m, m.notify(fmt.Sprintf("%s %s #%d failed: %s",
			glyphs.Fail, msg.verb, msg.key.Number, firstLine(msg.err.Error())), toastBad)
	}

	m.optimistic(msg)
	m.rebuild()
	m.loading = true
	m.tickGen++
	return m, tea.Batch(
		m.fetchCmd(),
		m.scheduleTick(),
		m.notify(fmt.Sprintf("%s %s #%d", glyphs.Pass, msg.verb, msg.key.Number), toastGood),
	)
}

func (m *Model) optimistic(msg actionMsg) {
	switch msg.verb {
	case "approved":
		for i := range m.prs {
			if m.prs[i].Key() != msg.key {
				continue
			}
			m.prs[i].Approvals++
			m.prs[i].Reviews = append(m.prs[i].Reviews, model.Review{
				Login: m.viewer, State: model.ReviewApproved, SubmittedAt: time.Now(),
			})
			m.prs[i].RequestedReviewers = without(m.prs[i].RequestedReviewers, m.viewer)
		}
	case "merged", "closed":
		kept := make([]model.PullRequest, 0, len(m.prs))
		for _, pr := range m.prs {
			if pr.Key() != msg.key {
				kept = append(kept, pr)
			}
		}
		m.prs = kept
	}
}

func without(list []string, name string) []string {
	out := make([]string, 0, len(list))
	for _, v := range list {
		if !strings.EqualFold(v, name) {
			out = append(out, v)
		}
	}
	return out
}

func copyToClipboard(text, label string) tea.Cmd {
	return func() tea.Msg {
		termenv.NewOutput(os.Stdout).Copy(text)
		return toastMsg{text: "copied " + label + ": " + text, kind: toastGood}
	}
}

func openURL(url string) tea.Cmd {
	if url == "" {
		return func() tea.Msg { return toastMsg{text: "nothing to open", kind: toastInfo} }
	}
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", url)
		case "windows":
			cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
		default:
			cmd = exec.Command("xdg-open", url)
		}
		if err := cmd.Start(); err != nil {
			return toastMsg{text: "could not open browser: " + err.Error(), kind: toastBad}
		}
		return toastMsg{text: "opened " + url, kind: toastInfo}
	}
}
