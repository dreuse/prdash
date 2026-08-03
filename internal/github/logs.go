package github

import (
	"bytes"
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/dreuse/prdash/internal/model"
)

const (
	maxLogLines  = 2000
	maxDiffLines = 2000
)

var ErrLogsNotReady = errors.New("logs are available once the job finishes")

func diffArgs(pr model.PullRequest) []string {
	return []string{"pr", "diff", strconv.Itoa(pr.Number), "--repo", pr.Repo, "--color", "never"}
}

func (c *CLI) Diff(ctx context.Context, pr model.PullRequest) ([]string, error) {
	out, err := c.run(ctx, pr.Repo, "read pull request diff", diffArgs(pr)...)
	if err != nil {
		return nil, err
	}
	return headLines(out, maxDiffLines), nil
}

func logArgs(run model.WorkflowRun, failedOnly bool) []string {
	mode := "--log"
	if failedOnly {
		mode = "--log-failed"
	}
	return []string{"run", "view", strconv.FormatInt(run.ID, 10), "--repo", run.Repo, mode}
}

func (c *CLI) RunLog(ctx context.Context, run model.WorkflowRun, failedOnly bool) ([]string, error) {
	if run.InProgress() {
		return nil, ErrLogsNotReady
	}
	out, err := c.run(ctx, run.Repo, "read run log", logArgs(run, failedOnly)...)
	if err != nil {
		return nil, err
	}
	return tailLines(out, maxLogLines), nil
}

func splitLines(out []byte) []string {
	trimmed := bytes.TrimRight(out, "\n")
	if len(trimmed) == 0 {
		return nil
	}
	lines := strings.Split(string(trimmed), "\n")
	for i, line := range lines {
		lines[i] = sanitiseLogLine(line)
	}
	return lines
}

func tailLines(out []byte, n int) []string {
	lines := splitLines(out)
	if len(lines) > n {
		return lines[len(lines)-n:]
	}
	return lines
}

func headLines(out []byte, n int) []string {
	lines := splitLines(out)
	if len(lines) > n {
		return lines[:n]
	}
	return lines
}

func sanitiseLogLine(line string) string {
	line = ansi.Strip(line)
	return strings.Map(func(r rune) rune {
		if r == '\t' || (r >= 0x20 && r != 0x7f) {
			return r
		}
		return -1
	}, line)
}
