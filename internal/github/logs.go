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

const maxLogLines = 2000

var ErrLogsNotReady = errors.New("logs are available once the job finishes")

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

func tailLines(out []byte, n int) []string {
	trimmed := bytes.TrimRight(out, "\n")
	if len(trimmed) == 0 {
		return nil
	}
	lines := strings.Split(string(trimmed), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	for i, line := range lines {
		lines[i] = sanitiseLogLine(line)
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
