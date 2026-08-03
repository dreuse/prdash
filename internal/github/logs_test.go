package github

import (
	"context"
	"errors"
	"strconv"
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

func TestDiffArgsAskGhForTheUnifiedDiff(t *testing.T) {
	pr := model.PullRequest{Repo: "acme/api", Number: 412}
	args := strings.Join(diffArgs(pr), " ")

	if !strings.Contains(args, "pr diff") {
		t.Errorf("the diff comes from gh pr diff, got %q", args)
	}
	if !strings.Contains(args, "412") || !strings.Contains(args, "acme/api") {
		t.Errorf("the number and repo must reach gh, got %q", args)
	}
	if !strings.Contains(args, "--color never") {
		t.Errorf("gh must not colour the patch, we style it ourselves, got %q", args)
	}
}

func TestDiffKeepsTheStartOfThePatchNotTheEnd(t *testing.T) {
	raw := "diff --git a/one.go b/one.go\n@@ -1 +1 @@\n-old\n+new\ntrailing\n"
	got := headLines([]byte(raw), 2)

	if len(got) != 2 {
		t.Fatalf("the cap must be honoured, got %d lines", len(got))
	}
	if got[0] != "diff --git a/one.go b/one.go" {
		t.Errorf("a truncated diff must keep its first hunks, got %q", got)
	}
}

func TestDiffStripsTerminalEscapes(t *testing.T) {
	got := headLines([]byte("\x1b[31m-gone\x1b[0m\n"), 10)

	if len(got) != 1 || got[0] != "-gone" {
		t.Errorf("escapes must not reach the terminal, got %q", got)
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
		b.WriteString(strconv.Itoa(i))
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
