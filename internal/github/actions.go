package github

import (
	"context"
	"fmt"
	"strconv"

	"github.com/dreuse/prdash/internal/model"
)

func (c *CLI) Approve(ctx context.Context, pr model.PullRequest) error {
	_, err := c.run(ctx, pr.Repo, "approve pull request",
		"pr", "review", strconv.Itoa(pr.Number), "--repo", pr.Repo, "--approve")
	return err
}

func (c *CLI) Comment(ctx context.Context, pr model.PullRequest, body string) error {
	_, err := c.run(ctx, pr.Repo, "comment on pull request",
		"pr", "comment", strconv.Itoa(pr.Number), "--repo", pr.Repo, "--body", body)
	return err
}

func (c *CLI) Merge(ctx context.Context, pr model.PullRequest, method string) error {
	flag := "--squash"
	switch method {
	case MergeMerge:
		flag = "--merge"
	case MergeRebase:
		flag = "--rebase"
	}
	_, err := c.run(ctx, pr.Repo, "merge pull request",
		"pr", "merge", strconv.Itoa(pr.Number), "--repo", pr.Repo, flag)
	return err
}

func (c *CLI) Close(ctx context.Context, pr model.PullRequest) error {
	_, err := c.run(ctx, pr.Repo, "close pull request",
		"pr", "close", strconv.Itoa(pr.Number), "--repo", pr.Repo)
	return err
}

func (c *CLI) Rerun(ctx context.Context, pr model.PullRequest) error {
	out, err := c.run(ctx, pr.Repo, "list runs for branch",
		"run", "list", "--repo", pr.Repo, "--branch", pr.HeadRef, "--limit", "1", "--json", "databaseId")
	if err != nil {
		return err
	}
	var runs []struct {
		DatabaseID int64 `json:"databaseId"`
	}
	if err := jsonUnmarshal(out, &runs); err != nil || len(runs) == 0 {
		return &Error{Repo: pr.Repo, Op: "re-run checks", Err: fmt.Errorf("no workflow run found for %s", pr.HeadRef)}
	}
	return c.rerunID(ctx, pr.Repo, runs[0].DatabaseID)
}

func (c *CLI) RerunRun(ctx context.Context, run model.WorkflowRun) error {
	return c.rerunID(ctx, run.Repo, run.ID)
}

func (c *CLI) rerunID(ctx context.Context, repo string, id int64) error {
	_, err := c.run(ctx, repo, "re-run workflow",
		"run", "rerun", strconv.FormatInt(id, 10), "--repo", repo, "--failed")
	return err
}

func (c *CLI) UpdateBranch(ctx context.Context, pr model.PullRequest) error {
	repo, err := ParseRepo(pr.Repo)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("repos/%s/%s/pulls/%d/update-branch", repo.Owner, repo.Name, pr.Number)
	_, err = c.run(ctx, pr.Repo, "update branch", "api", "--method", "PUT", path)
	return err
}
