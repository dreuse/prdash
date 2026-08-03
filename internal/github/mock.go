package github

import (
	"context"
	"strconv"
	"time"

	"github.com/dreuse/prdash/internal/model"
)

const (
	MockViewer = "octodev"
	mockRepo   = "acme/payments-api"
	mockRepo2  = "acme/device-gateway"
)

type Mock struct {
	Err error
}

func NewMock() *Mock { return &Mock{} }

func (m *Mock) Approve(context.Context, model.PullRequest) error         { return nil }
func (m *Mock) Comment(context.Context, model.PullRequest, string) error { return nil }
func (m *Mock) Merge(context.Context, model.PullRequest, string) error   { return nil }
func (m *Mock) Close(context.Context, model.PullRequest) error           { return nil }
func (m *Mock) Rerun(context.Context, model.PullRequest) error           { return nil }
func (m *Mock) RerunRun(context.Context, model.WorkflowRun) error        { return nil }
func (m *Mock) UpdateBranch(context.Context, model.PullRequest) error    { return nil }

func (m *Mock) Diff(_ context.Context, pr model.PullRequest) ([]string, error) {
	lines := []string{
		"diff --git a/internal/ledger/entry.go b/internal/ledger/entry.go",
		"index 9c2f1a0..3ab77e4 100644",
		"--- a/internal/ledger/entry.go",
		"+++ b/internal/ledger/entry.go",
		"@@ -18,7 +18,9 @@ type Entry struct {",
		" \tID       int64",
		" \tAmount   Money",
		"-\tKind     string",
		"+\tKind     string",
		"+\tCategory string",
		" }",
	}
	for i := 0; i < pr.Changed; i++ {
		lines = append(lines,
			"diff --git a/internal/ledger/file"+strconv.Itoa(i)+".go b/internal/ledger/file"+strconv.Itoa(i)+".go",
			"@@ -1,4 +1,4 @@",
			"-\told line "+strconv.Itoa(i),
			"+\tnew line "+strconv.Itoa(i),
			" \tcontext")
	}
	return lines, nil
}

func (m *Mock) RunLog(_ context.Context, run model.WorkflowRun, failedOnly bool) ([]string, error) {
	if run.InProgress() {
		return nil, ErrLogsNotReady
	}
	job := run.FailingJob
	if job == "" {
		job = "build"
	}
	step := run.FailingStep
	if step == "" {
		step = "Run tests"
	}

	tail := []string{job + "\t" + step + "\tBUILD SUCCESS"}
	if run.Failed() {
		tail = []string{
			job + "\t" + step + "\tERROR Migration V42__add_company_index.sql failed",
			job + "\t" + step + "\tERROR SQL State  : 42S01",
			job + "\t" + step + "\tERROR Message    : Table 'company_item' already exists",
			job + "\t" + step + "\tProcess completed with exit code 1",
		}
	}
	if failedOnly {
		if !run.Failed() {
			return nil, nil
		}
		return tail, nil
	}

	lines := make([]string, 0, 64)
	for i := 0; i < 48; i++ {
		lines = append(lines, job+"\t"+step+"\t"+mockLogNoise[i%len(mockLogNoise)])
	}
	return append(lines, tail...), nil
}

var mockLogNoise = []string{
	"Downloading from central: org/example/core/6.1.4/core.jar",
	"Compiling 312 source files with javac",
	"Tests run: 48, Failures: 0, Errors: 0, Skipped: 2",
	"Migrator 10.4.1",
	"Database: jdbc:postgresql://localhost:5432/appdb (PostgreSQL 16.1)",
	"Successfully validated 41 migrations",
}

func day(n int) time.Duration { return time.Duration(n) * 24 * time.Hour }

func checks(passed, failed, running, neutral int) []model.Check {
	out := make([]model.Check, 0, passed+failed+running+neutral)
	names := []string{"build", "unit-tests", "lint", "integration-tests", "sonar", "db:migrate",
		"contract-tests", "e2e", "docker", "helm-lint", "owasp", "javadoc", "coverage", "spotless", "license"}
	next := func(i int) string {
		if i < len(names) {
			return names[i]
		}
		return "check"
	}
	i := 0
	for ; i < failed; i++ {
		out = append(out, model.Check{Name: next(i), State: model.CheckFailed})
	}
	for j := 0; j < running; j++ {
		out = append(out, model.Check{Name: next(i), State: model.CheckRunning,
			StartedAt: time.Now().Add(-4 * time.Minute)})
		i++
	}
	for j := 0; j < passed; j++ {
		out = append(out, model.Check{Name: next(i), State: model.CheckPassed})
		i++
	}
	for j := 0; j < neutral; j++ {
		out = append(out, model.Check{Name: next(i), State: model.CheckNeutral})
		i++
	}
	return out
}

func (m *Mock) Fetch(context.Context) (Snapshot, error) {
	if m.Err != nil {
		return Snapshot{}, m.Err
	}
	now := time.Now()
	ago := func(d time.Duration) time.Time { return now.Add(-d) }

	prs := []model.PullRequest{
		{
			Repo: mockRepo, Number: 12009, Title: "Add category field to the ledger",
			Author: "rmartins", CreatedAt: ago(day(8)), UpdatedAt: ago(3 * time.Hour),
			Mergeable: model.MergeableYes, BaseRef: "master", HeadRef: "feat/category-field",
			BehindBy: 19, Approvals: 1, Additions: 88, Deletions: 12, Changed: 5,
			Reviews: []model.Review{{Login: "jchen", State: model.ReviewApproved, SubmittedAt: ago(2 * time.Hour)}},
			Checks:  checks(15, 0, 0, 0),
			Comments: []model.Comment{
				{Author: "jchen", CreatedAt: ago(day(1)), Body: "The ledger already carries a kind column further down the pipeline, so this needs a migration that backfills both or the reconciliation job will disagree with the export."},
				{Author: "apatel", CreatedAt: ago(6 * time.Hour), Body: "Rebased on master, checks are green again."},
				{Author: "rmartins", CreatedAt: ago(3 * time.Hour), Body: "Backfill added, ready for another look."},
			},
			Commits: []model.Commit{
				{OID: "a3f91c2", Headline: "Add the category column", CommittedAt: ago(day(8))},
				{OID: "8b04e77", Headline: "Backfill existing ledger rows", CommittedAt: ago(day(2))},
				{OID: "1d7c9a0", Headline: "Rebase on master", CommittedAt: ago(day(1))},
				{OID: "c44e210", Headline: "Drop the redundant index", CommittedAt: ago(5 * time.Hour)},
				{OID: "90ffe83", Headline: "Reconcile the export fixture", CommittedAt: ago(3 * time.Hour)},
			},
			CommitCount: 12,
		},
		{
			Repo: mockRepo, Number: 12012, Title: "Make migrations idempotent",
			Author: MockViewer, CreatedAt: ago(day(6)), UpdatedAt: ago(30 * time.Minute),
			Mergeable: model.MergeableYes, BaseRef: "master", HeadRef: "feature/sql-idempotent",
			BehindBy: 19, Additions: 412, Deletions: 86, Changed: 14,
			RequestedReviewers: []string{MockViewer, "jchen", "apatel"},
			Assignees:          []string{MockViewer},
			Checks:             checks(11, 0, 0, 4),
			Labels:             []string{"database"},
			Comments: []model.Comment{
				{Author: "tokoro", CreatedAt: ago(45 * time.Minute), Body: "Second run is clean now."},
			},
			Commits: []model.Commit{
				{OID: "7e1b3d4", Headline: "Guard the migration with a version check", CommittedAt: ago(day(6))},
				{OID: "2fa8c51", Headline: "Make the down migration a no-op", CommittedAt: ago(30 * time.Minute)},
			},
			CommitCount: 2,
		},
		{
			Repo: mockRepo, Number: 11959, Title: "Salary band importer",
			Author: "jchen", CreatedAt: ago(day(15)), UpdatedAt: ago(day(2)),
			Mergeable: model.MergeableYes, BaseRef: "master", HeadRef: "feat/salary-band-import",
			BehindBy: 38, Additions: 640, Deletions: 40, Changed: 22,
			RequestedReviewers: []string{MockViewer},
			Checks:             checks(9, 0, 0, 4),
		},
		{
			Repo: mockRepo, Number: 12038, Title: "Inventory sync with the partner API",
			Author: "lnovak", CreatedAt: ago(day(2)), UpdatedAt: ago(5 * time.Hour),
			Mergeable: model.MergeableYes, BaseRef: "master", HeadRef: "release/inventory/partner-sync",
			BehindBy: 8, ChangesRequested: 1, Additions: 120, Deletions: 8, Changed: 6,
			Assignees: []string{"tokoro"}, Labels: []string{"integration", "bug"},
			Reviews: []model.Review{{Login: "tokoro", State: model.ReviewChangesRequested, SubmittedAt: ago(6 * time.Hour)}},
			Checks:  checks(15, 0, 0, 0),
		},
		{
			Repo: mockRepo, Number: 12022, Title: "Cascade delete for linked people",
			Author: "MDubois", CreatedAt: ago(day(4)), UpdatedAt: ago(day(1)),
			Mergeable: model.MergeableYes, BaseRef: "master", HeadRef: "feature/cascade-delete",
			BehindBy: 7, ChangesRequested: 1, Additions: 55, Deletions: 210, Changed: 9,
			Reviews: []model.Review{{Login: "jchen", State: model.ReviewChangesRequested, SubmittedAt: ago(day(2))}},
			Checks:  checks(15, 0, 0, 0),
		},
		{
			Repo: mockRepo, Number: 11925, Title: "Performance based progression",
			Author: "tokoro", CreatedAt: ago(day(18)), UpdatedAt: ago(day(3)),
			Mergeable: model.MergeableYes, BaseRef: "master", HeadRef: "feat/performance-progression",
			BehindBy: 3, ChangesRequested: 1, Additions: 300, Deletions: 20, Changed: 11,
			Checks: checks(15, 0, 0, 0),
		},
		{
			Repo: mockRepo, Number: 12023, Title: "Revert probation period rules",
			Author: "bkeller", CreatedAt: ago(day(4)), UpdatedAt: ago(2 * time.Hour),
			Mergeable: model.MergeableYes, BaseRef: "master", HeadRef: "revert/probation-rules",
			BehindBy: 7, Additions: 12, Deletions: 400, Changed: 7,
			Checks: checks(13, 2, 0, 0),
		},
		{
			Repo: mockRepo, Number: 11761, Title: "Per company pay items",
			Author: "jchen", CreatedAt: ago(day(50)), UpdatedAt: ago(day(4)),
			Mergeable: model.MergeableYes, BaseRef: "master", HeadRef: "feat/per-company-items",
			Additions: 210, Deletions: 30, Changed: 12,
			Checks: checks(14, 1, 0, 0),
		},
		{
			Repo: mockRepo, Number: 11731, Title: "chore: add status column",
			Author: "nreyes", CreatedAt: ago(day(54)), UpdatedAt: ago(day(6)),
			Mergeable: model.MergeableYes, BaseRef: "master", HeadRef: "chore/status-column",
			BehindBy: 178, Additions: 40, Deletions: 4, Changed: 3,
			Checks: checks(14, 1, 0, 0),
		},
		{
			Repo: mockRepo, Number: 10289, Title: "POC: court integration",
			Author: "jchen", CreatedAt: ago(day(267)), UpdatedAt: ago(day(190)),
			Mergeable: model.MergeableConflict, BaseRef: "master", HeadRef: "poc/court-integration",
			BehindBy: 720, Additions: 900, Deletions: 300, Changed: 60,
			Checks: checks(15, 0, 0, 0),
		},
		{
			Repo: mockRepo, Number: 9925, Title: "Release 2025/09-1",
			Author: "jchen", CreatedAt: ago(day(320)), UpdatedAt: ago(day(240)),
			Mergeable: model.MergeableConflict, BaseRef: "master", HeadRef: "release/2025-09-1",
			Additions: 30, Deletions: 10, Changed: 4,
			Checks: checks(14, 1, 0, 0),
		},
		{
			Repo: mockRepo, Number: 11952, Title: "WIP: frontend",
			Author: MockViewer, CreatedAt: ago(day(15)), UpdatedAt: ago(day(9)),
			IsDraft: true, Mergeable: model.MergeableYes, BaseRef: "master", HeadRef: "wip/frontend",
			BehindBy: 69, Additions: 1200, Deletions: 40, Changed: 80,
			Checks: checks(11, 4, 0, 0),
		},
		{
			Repo: mockRepo2, Number: 11758, Title: "Device authentication",
			Author: "wfoster", CreatedAt: ago(day(51)), UpdatedAt: ago(day(20)),
			IsDraft: true, Mergeable: model.MergeableYes, BaseRef: "main", HeadRef: "feat/device-auth",
			Additions: 220, Deletions: 15, Changed: 14,
			Checks: checks(15, 0, 0, 0),
		},
		{
			Repo: mockRepo2, Number: 11757, Title: "Device model",
			Author: "wfoster", CreatedAt: ago(day(51)), UpdatedAt: ago(day(22)),
			IsDraft: true, Mergeable: model.MergeableConflict, BaseRef: "main", HeadRef: "feat/device-model",
			Additions: 180, Deletions: 5, Changed: 10,
		},
		{
			Repo: mockRepo2, Number: 11748, Title: "feat: manager controller",
			Author: "dhalvorsen", CreatedAt: ago(day(52)), UpdatedAt: ago(day(30)),
			IsDraft: true, Mergeable: model.MergeableConflict, BaseRef: "main", HeadRef: "feat/manager-controller",
			Additions: 95, Deletions: 2, Changed: 6,
		},
	}

	runs := []model.WorkflowRun{
		{ID: 1, Repo: mockRepo, Name: "CI", Branch: "master", Event: "push", Actor: "jchen",
			Status: "completed", Conclusion: "failure", FailingJob: "build", FailingStep: "db:migrate",
			StartedAt: ago(40*time.Minute + 2*time.Minute + 29*time.Second), UpdatedAt: ago(40 * time.Minute),
			URL: "https://github.com/" + mockRepo + "/actions/runs/1"},
		{ID: 2, Repo: mockRepo, Name: "CI", Branch: "release/inventory/partner-sync", Event: "pull_request",
			Status: "completed", Conclusion: "success",
			StartedAt: ago(2*time.Hour + 32*time.Minute), UpdatedAt: ago(2 * time.Hour),
			URL: "https://github.com/" + mockRepo + "/actions/runs/2"},
		{ID: 3, Repo: mockRepo, Name: "CI", Branch: "feat/performance-progression", Event: "pull_request",
			Status: "completed", Conclusion: "success",
			StartedAt: ago(3*time.Hour + 23*time.Minute), UpdatedAt: ago(3 * time.Hour),
			URL: "https://github.com/" + mockRepo + "/actions/runs/3"},
		{ID: 4, Repo: mockRepo, Name: "CI", Branch: "feature/cascade-delete", Event: "pull_request",
			Status: "completed", Conclusion: "success",
			StartedAt: ago(4*time.Hour + 28*time.Minute), UpdatedAt: ago(4 * time.Hour),
			URL: "https://github.com/" + mockRepo + "/actions/runs/4"},
		{ID: 5, Repo: mockRepo, Name: "Copilot Code Review", Branch: "master", Event: "push",
			Status: "completed", Conclusion: "success",
			StartedAt: ago(2*time.Hour + 6*time.Minute), UpdatedAt: ago(2 * time.Hour),
			URL: "https://github.com/" + mockRepo + "/actions/runs/5"},
		{ID: 6, Repo: mockRepo, Name: "Sync releases", Branch: "master", Event: "schedule",
			Status: "completed", Conclusion: "success",
			StartedAt: ago(3*time.Hour + 40*time.Second), UpdatedAt: ago(3 * time.Hour),
			URL: "https://github.com/" + mockRepo + "/actions/runs/6"},
		{ID: 7, Repo: mockRepo, Name: "DB migration", Branch: "master", Event: "workflow_dispatch",
			Status: "completed", Conclusion: "success",
			StartedAt: ago(5*time.Hour + 8*time.Minute + 57*time.Second), UpdatedAt: ago(5 * time.Hour),
			URL: "https://github.com/" + mockRepo + "/actions/runs/7"},
		{ID: 8, Repo: mockRepo2, Name: "Nightly sync", Branch: "main", Event: "schedule",
			Status: "completed", Conclusion: "success",
			StartedAt: ago(6*time.Hour + 16*time.Minute + 22*time.Second), UpdatedAt: ago(6 * time.Hour),
			URL: "https://github.com/" + mockRepo2 + "/actions/runs/8"},
	}
	for i := 0; i < 12; i++ {
		runs = append(runs, model.WorkflowRun{
			ID: int64(100 + i), Repo: mockRepo, Name: "CI", Branch: "master", Event: "push",
			Status: "completed", Conclusion: "success",
			StartedAt: ago(time.Duration(7+i)*time.Hour + 21*time.Minute),
			UpdatedAt: ago(time.Duration(7+i) * time.Hour),
			URL:       "https://github.com/" + mockRepo + "/actions/runs/100",
		})
	}

	issues := []model.Issue{
		{Repo: mockRepo, Number: 11890, Title: "Importer fails on large files"},
		{Repo: mockRepo, Number: 11402, Title: "Document the billing flow"},
		{Repo: mockRepo2, Number: 902, Title: "Refatorar o cliente HTTP"},
	}
	people := []model.User{
		{Login: "jchen", Name: "Jordan Chen"},
		{Login: "apatel", Name: "Anika Patel"},
		{Login: "tokoro", Name: "Taro Okoro"},
		{Login: "wfoster", Name: "Wren Foster"},
		{Login: MockViewer, Name: "Octo Dev"},
	}
	return Snapshot{
		Viewer: MockViewer, PullRequests: prs, Runs: runs,
		Issues: issues, People: people,
	}, nil
}
