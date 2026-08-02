package github

import (
	"context"
	"time"

	"github.com/dreuse/prdash/internal/model"
)

type Mock struct {
	Err error
}

func NewMock() *Mock { return &Mock{} }

func (m *Mock) Fetch(ctx context.Context) (Snapshot, error) {
	if m.Err != nil {
		return Snapshot{}, m.Err
	}
	now := time.Now()
	ago := func(d time.Duration) time.Time { return now.Add(-d) }

	passing := []model.Check{
		{Name: "build", State: model.CheckPassed},
		{Name: "unit-tests", State: model.CheckPassed},
		{Name: "lint", State: model.CheckPassed},
	}
	running := []model.Check{
		{Name: "build", State: model.CheckPassed},
		{Name: "integration-tests", State: model.CheckRunning},
	}
	failing := []model.Check{
		{Name: "build", State: model.CheckPassed},
		{Name: "unit-tests", State: model.CheckFailed},
	}

	prs := []model.PullRequest{
		{
			Repo: "acme/api", Number: 412, Title: "wire the new billing webhook",
			Author: "dreuse", URL: "https://github.com/acme/api/pull/412",
			CreatedAt: ago(3 * time.Hour), UpdatedAt: ago(12 * time.Minute),
			IsDraft: true, Mergeable: model.MergeableYes,
			BaseRef: "main", HeadRef: "billing-webhook", Checks: running,
		},
		{
			Repo: "acme/api", Number: 408, Title: "drop the legacy v1 serializer",
			Author: "mgarrido", URL: "https://github.com/acme/api/pull/408",
			CreatedAt: ago(30 * time.Hour), UpdatedAt: ago(2 * time.Hour),
			IsDraft: true, Mergeable: model.MergeableUnknown,
			BaseRef: "main", HeadRef: "drop-v1-serializer", BehindBy: 7,
		},
		{
			Repo: "acme/api", Number: 401, Title: "add pagination to the audit log endpoint",
			Author: "jlin", URL: "https://github.com/acme/api/pull/401",
			CreatedAt: ago(20 * time.Hour), UpdatedAt: ago(45 * time.Minute),
			Mergeable: model.MergeableYes, BaseRef: "main", HeadRef: "audit-pagination",
			RequestedReviewers: []string{"dreuse", "platform-team"}, Checks: passing,
		},
		{
			Repo: "acme/web", Number: 1287, Title: "skeleton loaders for the dashboard cards",
			Author: "tsoares", URL: "https://github.com/acme/web/pull/1287",
			CreatedAt: ago(5 * time.Hour), UpdatedAt: ago(20 * time.Minute),
			Mergeable: model.MergeableYes, BaseRef: "main", HeadRef: "skeleton-loaders",
			RequestedReviewers: []string{"anap"}, Checks: passing,
		},
		{
			Repo: "acme/web", Number: 1281, Title: "replace moment with the native Intl API",
			Author: "anap", URL: "https://github.com/acme/web/pull/1281",
			CreatedAt: ago(4 * 24 * time.Hour), UpdatedAt: ago(6 * time.Hour),
			Mergeable: model.MergeableYes, BaseRef: "main", HeadRef: "kill-moment",
			Approvals: 1, ChangesRequested: 1, BehindBy: 3,
			RequestedReviewers: []string{"tsoares"}, Checks: passing,
		},
		{
			Repo: "acme/api", Number: 396, Title: "rate limit the public search endpoint",
			Author: "dreuse", URL: "https://github.com/acme/api/pull/396",
			CreatedAt: ago(2 * 24 * time.Hour), UpdatedAt: ago(90 * time.Minute),
			Mergeable: model.MergeableYes, BaseRef: "main", HeadRef: "search-rate-limit",
			ChangesRequested: 2, Checks: passing,
		},
		{
			Repo: "acme/infra", Number: 77, Title: "bump terraform provider to 5.x",
			Author: "kbecker", URL: "https://github.com/acme/infra/pull/77",
			CreatedAt: ago(90 * time.Minute), UpdatedAt: ago(3 * time.Minute),
			Mergeable: model.MergeableYes, BaseRef: "main", HeadRef: "tf-provider-5",
			Approvals: 2, Checks: running,
		},
		{
			Repo: "acme/web", Number: 1290, Title: "prefetch the org switcher payload",
			Author: "jlin", URL: "https://github.com/acme/web/pull/1290",
			CreatedAt: ago(25 * time.Minute), UpdatedAt: ago(1 * time.Minute),
			Mergeable: model.MergeableUnknown, BaseRef: "main", HeadRef: "prefetch-org-switcher",
			Approvals: 1, Checks: running,
		},
		{
			Repo: "acme/api", Number: 399, Title: "return idp claims from the whoami endpoint",
			Author: "mgarrido", URL: "https://github.com/acme/api/pull/399",
			CreatedAt: ago(28 * time.Hour), UpdatedAt: ago(35 * time.Minute),
			Mergeable: model.MergeableYes, BaseRef: "main", HeadRef: "whoami-idp-claims",
			Approvals: 2, Checks: passing,
		},
		{
			Repo: "acme/infra", Number: 75, Title: "move the staging cluster to graviton nodes",
			Author: "kbecker", URL: "https://github.com/acme/infra/pull/75",
			CreatedAt: ago(6 * 24 * time.Hour), UpdatedAt: ago(4 * time.Hour),
			Mergeable: model.MergeableYes, BaseRef: "main", HeadRef: "graviton-staging",
			Approvals: 1, BehindBy: 21, Checks: passing,
		},
		{
			Repo: "acme/web", Number: 1274, Title: "virtualise the notification list",
			Author: "tsoares", URL: "https://github.com/acme/web/pull/1274",
			CreatedAt: ago(9 * 24 * time.Hour), UpdatedAt: ago(11 * time.Hour),
			Mergeable: model.MergeableConflict, BaseRef: "main", HeadRef: "virtual-notifications",
			Approvals: 1, BehindBy: 48, Checks: passing,
		},
		{
			Repo: "acme/api", Number: 385, Title: "shard the events table by tenant",
			Author: "jlin", URL: "https://github.com/acme/api/pull/385",
			CreatedAt: ago(12 * 24 * time.Hour), UpdatedAt: ago(26 * time.Hour),
			Mergeable: model.MergeableYes, BaseRef: "main", HeadRef: "shard-events",
			Approvals: 2, RequestedReviewers: []string{"dba-team"}, Checks: failing,
		},
	}

	runs := []model.WorkflowRun{
		{Repo: "acme/api", Name: "ci", Branch: "billing-webhook", Event: "pull_request",
			Status: "in_progress", URL: "https://github.com/acme/api/actions/runs/9001",
			StartedAt: ago(4 * time.Minute), UpdatedAt: ago(10 * time.Second)},
		{Repo: "acme/infra", Name: "terraform-plan", Branch: "tf-provider-5", Event: "pull_request",
			Status: "in_progress", URL: "https://github.com/acme/infra/actions/runs/9000",
			StartedAt: ago(2 * time.Minute), UpdatedAt: ago(5 * time.Second)},
		{Repo: "acme/web", Name: "e2e", Branch: "prefetch-org-switcher", Event: "pull_request",
			Status: "queued", URL: "https://github.com/acme/web/actions/runs/8999",
			StartedAt: ago(40 * time.Second), UpdatedAt: ago(40 * time.Second)},
		{Repo: "acme/api", Name: "ci", Branch: "shard-events", Event: "pull_request",
			Status: "completed", Conclusion: "failure", URL: "https://github.com/acme/api/actions/runs/8990",
			StartedAt: ago(30 * time.Minute), UpdatedAt: ago(22 * time.Minute)},
		{Repo: "acme/web", Name: "ci", Branch: "main", Event: "push",
			Status: "completed", Conclusion: "success", URL: "https://github.com/acme/web/actions/runs/8985",
			StartedAt: ago(55 * time.Minute), UpdatedAt: ago(48 * time.Minute)},
		{Repo: "acme/api", Name: "nightly-migration-check", Branch: "main", Event: "schedule",
			Status: "completed", Conclusion: "success", URL: "https://github.com/acme/api/actions/runs/8970",
			StartedAt: ago(7 * time.Hour), UpdatedAt: ago(6*time.Hour - 40*time.Minute)},
		{Repo: "acme/infra", Name: "drift-detection", Branch: "main", Event: "schedule",
			Status: "completed", Conclusion: "cancelled", URL: "https://github.com/acme/infra/actions/runs/8965",
			StartedAt: ago(9 * time.Hour), UpdatedAt: ago(8*time.Hour - 50*time.Minute)},
		{Repo: "acme/web", Name: "release", Branch: "main", Event: "workflow_dispatch",
			Status: "completed", Conclusion: "success", URL: "https://github.com/acme/web/actions/runs/8950",
			StartedAt: ago(26 * time.Hour), UpdatedAt: ago(25 * time.Hour)},
	}

	return Snapshot{PullRequests: prs, Runs: runs}, nil
}
