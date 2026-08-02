package github

import (
	"bytes"
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/dreuse/prdash/internal/model"
)

const maxLogLines = 2000

var ErrLogsNotReady = errors.New("logs are available once the job finishes")

func logArgs(run model.WorkflowRun) []string {
	mode := "--log"
	if run.Failed() {
		mode = "--log-failed"
	}
	return []string{"run", "view", strconv.FormatInt(run.ID, 10), "--repo", run.Repo, mode}
}

func (c *CLI) RunLog(ctx context.Context, run model.WorkflowRun) ([]string, error) {
	if run.InProgress() {
		return nil, ErrLogsNotReady
	}
	out, err := c.run(ctx, run.Repo, "read run log", logArgs(run)...)
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
	return lines
}
