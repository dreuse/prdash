package github

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/dreuse/prdash/internal/model"
)

func TestLogArgsIsolatesTheFailingSteps(t *testing.T) {
	run := model.WorkflowRun{ID: 42, Repo: "acme/api", Status: "completed", Conclusion: "failure"}
	args := strings.Join(logArgs(run, true), " ")

	if !strings.Contains(args, "--log-failed") {
		t.Errorf("failing-steps mode should ask gh for just those, got %q", args)
	}
	if !strings.Contains(args, "42") || !strings.Contains(args, "acme/api") {
		t.Errorf("the run id and repo must reach gh, got %q", args)
	}
}

func TestLogArgsCanAskForTheWholeLog(t *testing.T) {
	run := model.WorkflowRun{ID: 42, Repo: "acme/api", Status: "completed", Conclusion: "failure"}
	args := strings.Join(logArgs(run, false), " ")

	if strings.Contains(args, "--log-failed") {
		t.Errorf("full mode must not narrow to the failing steps, got %q", args)
	}
	if !strings.Contains(args, "--log") {
		t.Errorf("full mode should still fetch the log, got %q", args)
	}
}

func TestRunLogRefusesRunsStillInProgress(t *testing.T) {
	c := &CLI{}
	run := model.WorkflowRun{ID: 1, Repo: "acme/api", Status: "in_progress"}

	for _, failedOnly := range []bool{true, false} {
		if _, err := c.RunLog(context.Background(), run, failedOnly); !errors.Is(err, ErrLogsNotReady) {
			t.Fatalf("github serves no logs before a job finishes, want ErrLogsNotReady, got %v", err)
		}
	}
}

func TestTailLinesKeepsTheEndOfTheLog(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 50; i++ {
		b.WriteString("line")
		b.WriteString(itoa(i))
		b.WriteString("\n")
	}

	got := tailLines([]byte(b.String()), 10)
	if len(got) != 10 {
		t.Fatalf("want the last 10 lines, got %d", len(got))
	}
	if got[0] != "line40" || got[9] != "line49" {
		t.Errorf("the tail is where a failure lands, got %q..%q", got[0], got[9])
	}
}

func TestTailLinesKeepsEverythingBelowTheCap(t *testing.T) {
	got := tailLines([]byte("a\nb\nc\n"), 10)
	if len(got) != 3 {
		t.Fatalf("a short log should survive whole, got %d lines: %q", len(got), got)
	}
	if got[0] != "a" || got[2] != "c" {
		t.Errorf("short logs must keep their order, got %q", got)
	}
}

func TestTailLinesDropsTrailingBlank(t *testing.T) {
	got := tailLines([]byte("only\n"), 10)
	if len(got) != 1 || got[0] != "only" {
		t.Errorf("a trailing newline should not become an empty line, got %q", got)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [8]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
