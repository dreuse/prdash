package github

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dreuse/prdash/internal/model"
)

func fakeGH(t *testing.T) (bin, log string) {
	t.Helper()
	dir := t.TempDir()
	log = filepath.Join(dir, "args.log")
	bin = filepath.Join(dir, "gh")

	script := "#!/bin/sh\necho \"$@\" >> " + log + "\nexit 0\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin, log
}

func TestCloseNeverDeletesTheBranch(t *testing.T) {
	bin, log := fakeGH(t)
	cli := &CLI{Bin: bin}

	pr := model.PullRequest{Repo: "acme/api", Number: 42, HeadRef: "feat/keep-me"}
	if err := cli.Close(context.Background(), pr); err != nil {
		t.Fatalf("close: %v", err)
	}

	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	args := string(data)

	if !strings.Contains(args, "pr close 42") {
		t.Fatalf("unexpected command: %q", args)
	}
	for _, forbidden := range []string{"--delete-branch", "-d"} {
		if strings.Contains(args, forbidden) {
			t.Fatalf("close must never delete the branch, got %q", args)
		}
	}
}

func TestMergeNeverPassesDeleteBranch(t *testing.T) {
	bin, log := fakeGH(t)
	cli := &CLI{Bin: bin}

	pr := model.PullRequest{Repo: "acme/api", Number: 42, HeadRef: "feat/x"}
	if err := cli.Merge(context.Background(), pr, MergeSquash); err != nil {
		t.Fatalf("merge: %v", err)
	}

	data, _ := os.ReadFile(log)
	if strings.Contains(string(data), "--delete-branch") {
		t.Fatalf("prdash must not ask github to delete branches: %q", data)
	}
}
