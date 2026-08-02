package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dreuse/prdash/internal/model"
)

const (
	prsPerRepo         = 50
	runsPerRepo        = 20
	compareBatchSize   = 50
	maxConcurrentRepos = 4
)

type CLI struct {
	Repos []Repo
	Bin   string
}

func NewCLI(repos []Repo) *CLI { return &CLI{Repos: repos, Bin: "gh"} }

func (c *CLI) bin() string {
	if c.Bin == "" {
		return "gh"
	}
	return c.Bin
}

func (c *CLI) run(ctx context.Context, repo, op string, args ...string) ([]byte, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, c.bin(), args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		rate, auth := classifyStderr(stderr.String())
		return stdout.Bytes(), &Error{Repo: repo, Op: op, Stderr: stderr.String(), RateLimit: rate, Auth: auth, Err: err}
	}
	return stdout.Bytes(), nil
}

func (c *CLI) CheckAuth(ctx context.Context) error {
	if _, err := exec.LookPath(c.bin()); err != nil {
		return &Error{Op: "locate gh", Err: fmt.Errorf("gh cli not found in PATH: %w", err)}
	}
	_, err := c.run(ctx, "", "gh auth status", "auth", "status")
	if err != nil {
		if e, ok := err.(*Error); ok {
			e.Auth = true
		}
		return err
	}
	return nil
}

func (c *CLI) Fetch(ctx context.Context) (Snapshot, error) {
	type repoResult struct {
		prs  []model.PullRequest
		runs []model.WorkflowRun
		err  error
	}

	results := make([]repoResult, len(c.Repos))
	sem := make(chan struct{}, maxConcurrentRepos)
	var wg sync.WaitGroup

	for i, repo := range c.Repos {
		wg.Add(1)
		go func(i int, repo Repo) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			prs, err := c.fetchPRs(ctx, repo)
			if err != nil {
				results[i] = repoResult{err: err}
				return
			}
			runs, err := c.fetchRuns(ctx, repo)
			if err != nil {
				results[i] = repoResult{prs: prs, err: err}
				return
			}
			results[i] = repoResult{prs: prs, runs: runs}
		}(i, repo)
	}
	wg.Wait()

	var snap Snapshot
	var errs []error
	for _, r := range results {
		snap.PullRequests = append(snap.PullRequests, r.prs...)
		snap.Runs = append(snap.Runs, r.runs...)
		if r.err != nil {
			errs = append(errs, r.err)
		}
	}

	c.fillBehindBy(ctx, snap.PullRequests)

	sort.Slice(snap.PullRequests, func(i, j int) bool {
		return snap.PullRequests[i].CreatedAt.After(snap.PullRequests[j].CreatedAt)
	})
	sort.Slice(snap.Runs, func(i, j int) bool {
		return snap.Runs[i].StartedAt.After(snap.Runs[j].StartedAt)
	})

	if len(errs) > 0 && len(snap.PullRequests) == 0 && len(snap.Runs) == 0 {
		return snap, errs[0]
	}
	if len(errs) > 0 {
		return snap, errs[0]
	}
	return snap, nil
}

type graphQLPRResponse struct {
	Data struct {
		Repository struct {
			NameWithOwner string `json:"nameWithOwner"`
			PullRequests  struct {
				Nodes []struct {
					Number              int    `json:"number"`
					Title               string `json:"title"`
					URL                 string `json:"url"`
					IsDraft             bool   `json:"isDraft"`
					Mergeable           string `json:"mergeable"`
					CreatedAt           string `json:"createdAt"`
					UpdatedAt           string `json:"updatedAt"`
					BaseRefName         string `json:"baseRefName"`
					HeadRefName         string `json:"headRefName"`
					HeadRepositoryOwner struct {
						Login string `json:"login"`
					} `json:"headRepositoryOwner"`
					Author struct {
						Login string `json:"login"`
					} `json:"author"`
					ReviewRequests struct {
						Nodes []struct {
							RequestedReviewer struct {
								Login string `json:"login"`
								Name  string `json:"name"`
							} `json:"requestedReviewer"`
						} `json:"nodes"`
					} `json:"reviewRequests"`
					LatestOpinionatedReviews struct {
						Nodes []struct {
							State  string `json:"state"`
							Author struct {
								Login string `json:"login"`
							} `json:"author"`
						} `json:"nodes"`
					} `json:"latestOpinionatedReviews"`
					Commits struct {
						Nodes []struct {
							Commit struct {
								StatusCheckRollup *struct {
									Contexts struct {
										Nodes []struct {
											Name       string `json:"name"`
											Status     string `json:"status"`
											Conclusion string `json:"conclusion"`
											DetailsURL string `json:"detailsUrl"`
											Context    string `json:"context"`
											State      string `json:"state"`
											TargetURL  string `json:"targetUrl"`
										} `json:"nodes"`
									} `json:"contexts"`
								} `json:"statusCheckRollup"`
							} `json:"commit"`
						} `json:"nodes"`
					} `json:"commits"`
				} `json:"nodes"`
			} `json:"pullRequests"`
		} `json:"repository"`
	} `json:"data"`
}

func (c *CLI) fetchPRs(ctx context.Context, repo Repo) ([]model.PullRequest, error) {
	out, err := c.run(ctx, repo.String(), "list pull requests",
		"api", "graphql",
		"-f", "query="+pullRequestQuery,
		"-F", "owner="+repo.Owner,
		"-F", "name="+repo.Name,
		"-F", "limit="+strconv.Itoa(prsPerRepo),
	)
	if err != nil {
		return nil, err
	}

	var resp graphQLPRResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, &Error{Repo: repo.String(), Op: "decode pull requests", Err: err}
	}

	name := resp.Data.Repository.NameWithOwner
	if name == "" {
		name = repo.String()
	}

	prs := make([]model.PullRequest, 0, len(resp.Data.Repository.PullRequests.Nodes))
	for _, n := range resp.Data.Repository.PullRequests.Nodes {
		pr := model.PullRequest{
			Repo:      name,
			Number:    n.Number,
			Title:     n.Title,
			Author:    n.Author.Login,
			URL:       n.URL,
			CreatedAt: parseTime(n.CreatedAt),
			UpdatedAt: parseTime(n.UpdatedAt),
			IsDraft:   n.IsDraft,
			Mergeable: parseMergeable(n.Mergeable),
			BaseRef:   n.BaseRefName,
			HeadRef:   n.HeadRefName,
			HeadOwner: n.HeadRepositoryOwner.Login,
		}
		for _, rr := range n.ReviewRequests.Nodes {
			if login := rr.RequestedReviewer.Login; login != "" {
				pr.RequestedReviewers = append(pr.RequestedReviewers, login)
			} else if team := rr.RequestedReviewer.Name; team != "" {
				pr.RequestedReviewers = append(pr.RequestedReviewers, team)
			}
		}
		for _, rv := range n.LatestOpinionatedReviews.Nodes {
			switch rv.State {
			case "APPROVED":
				pr.Approvals++
			case "CHANGES_REQUESTED":
				pr.ChangesRequested++
			}
		}
		if len(n.Commits.Nodes) > 0 {
			rollup := n.Commits.Nodes[0].Commit.StatusCheckRollup
			if rollup != nil {
				for _, ctxNode := range rollup.Contexts.Nodes {
					pr.Checks = append(pr.Checks, normaliseCheck(
						ctxNode.Name, ctxNode.Context, ctxNode.Status, ctxNode.Conclusion, ctxNode.State,
						ctxNode.DetailsURL, ctxNode.TargetURL))
				}
			}
		}
		prs = append(prs, pr)
	}
	return prs, nil
}

func normaliseCheck(name, context, status, conclusion, state, detailsURL, targetURL string) model.Check {
	c := model.Check{Name: name, URL: detailsURL}
	if c.Name == "" {
		c.Name = context
	}
	if c.URL == "" {
		c.URL = targetURL
	}

	if state != "" {
		switch state {
		case "SUCCESS":
			c.State = model.CheckPassed
		case "FAILURE", "ERROR":
			c.State = model.CheckFailed
		default:
			c.State = model.CheckRunning
		}
		return c
	}

	if status != "COMPLETED" && status != "" {
		c.State = model.CheckRunning
		return c
	}
	switch conclusion {
	case "SUCCESS":
		c.State = model.CheckPassed
	case "FAILURE", "TIMED_OUT", "ACTION_REQUIRED", "STARTUP_FAILURE":
		c.State = model.CheckFailed
	case "NEUTRAL", "SKIPPED", "CANCELLED", "STALE":
		c.State = model.CheckNeutral
	default:
		c.State = model.CheckRunning
	}
	return c
}

func (c *CLI) fillBehindBy(ctx context.Context, prs []model.PullRequest) {
	targets := make([]compareTarget, 0, len(prs))
	for i, pr := range prs {
		if pr.BaseRef == "" || pr.HeadRef == "" {
			continue
		}
		repo, err := ParseRepo(pr.Repo)
		if err != nil {
			continue
		}
		head := pr.HeadRef
		if pr.HeadOwner != "" && pr.HeadOwner != repo.Owner {
			head = pr.HeadOwner + ":" + pr.HeadRef
		}
		if head == pr.BaseRef {
			continue
		}
		targets = append(targets, compareTarget{
			Alias:   "c" + strconv.Itoa(i),
			Repo:    repo,
			Base:    pr.BaseRef,
			Head:    head,
			PRIndex: i,
		})
	}

	for start := 0; start < len(targets); start += compareBatchSize {
		end := start + compareBatchSize
		if end > len(targets) {
			end = len(targets)
		}
		batch := targets[start:end]

		out, err := c.run(ctx, "", "compare branches", "api", "graphql", "-f", "query="+buildCompareQuery(batch))
		if len(out) == 0 && err != nil {
			continue
		}
		var resp struct {
			Data map[string]struct {
				Ref *struct {
					Compare *struct {
						BehindBy int `json:"behindBy"`
					} `json:"compare"`
				} `json:"ref"`
			} `json:"data"`
		}
		if err := json.Unmarshal(out, &resp); err != nil {
			continue
		}
		for _, t := range batch {
			node, ok := resp.Data[t.Alias]
			if !ok || node.Ref == nil || node.Ref.Compare == nil {
				continue
			}
			prs[t.PRIndex].BehindBy = node.Ref.Compare.BehindBy
		}
	}
}

func (c *CLI) fetchRuns(ctx context.Context, repo Repo) ([]model.WorkflowRun, error) {
	path := fmt.Sprintf("repos/%s/%s/actions/runs?per_page=%d", repo.Owner, repo.Name, runsPerRepo)
	out, err := c.run(ctx, repo.String(), "list workflow runs", "api", path)
	if err != nil {
		return nil, err
	}

	var resp struct {
		WorkflowRuns []struct {
			Name       string `json:"name"`
			HeadBranch string `json:"head_branch"`
			Event      string `json:"event"`
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
			HTMLURL    string `json:"html_url"`
			RunStarted string `json:"run_started_at"`
			CreatedAt  string `json:"created_at"`
			UpdatedAt  string `json:"updated_at"`
		} `json:"workflow_runs"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, &Error{Repo: repo.String(), Op: "decode workflow runs", Err: err}
	}

	runs := make([]model.WorkflowRun, 0, len(resp.WorkflowRuns))
	for _, r := range resp.WorkflowRuns {
		started := parseTime(r.RunStarted)
		if started.IsZero() {
			started = parseTime(r.CreatedAt)
		}
		runs = append(runs, model.WorkflowRun{
			Repo:       repo.String(),
			Name:       r.Name,
			Branch:     r.HeadBranch,
			Event:      r.Event,
			Status:     strings.ToLower(r.Status),
			Conclusion: strings.ToLower(r.Conclusion),
			URL:        r.HTMLURL,
			StartedAt:  started,
			UpdatedAt:  parseTime(r.UpdatedAt),
		})
	}
	return runs, nil
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func parseMergeable(s string) model.Mergeable {
	switch s {
	case "MERGEABLE":
		return model.MergeableYes
	case "CONFLICTING":
		return model.MergeableConflict
	default:
		return model.MergeableUnknown
	}
}
