package github

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseRepoRejectsAnythingThatCouldSteerTheAPIPath(t *testing.T) {
	for _, bad := range []string{
		"acme/api/../../users",
		"acme/api?per_page=1",
		"acme/../admin",
		"acme/api extra",
		"ac me/api",
		"acme//api",
	} {
		if got, err := ParseRepo(bad); err == nil {
			t.Errorf("%q must not parse, got %+v", bad, got)
		}
	}
}

func TestParseRepoAcceptsRealRepositories(t *testing.T) {
	for _, good := range []string{"acme/api", "acme-inc/device_gateway", "Acme/api.v2", " acme/api "} {
		if _, err := ParseRepo(good); err != nil {
			t.Errorf("%q is a valid repository, got %v", good, err)
		}
	}
}

func TestCompareQueryEmitsOnlyEscapesGraphQLUnderstands(t *testing.T) {
	targets := []compareTarget{{
		Alias: "c0",
		Repo:  Repo{Owner: "acme", Name: "api"},
		Base:  "main",
		Head:  "fork:feat/\a\v\"quoted\"",
	}}
	q := buildCompareQuery(targets)

	for _, bad := range []string{`\a`, `\v`, `\x`} {
		if strings.Contains(q, bad) {
			t.Errorf("query carries the invalid graphql escape %s:\n%s", bad, q)
		}
	}
	if !strings.Contains(q, `\u0007`) {
		t.Errorf("control characters should survive as \\uXXXX:\n%s", q)
	}
}

func TestCompareQueryKeepsABranchNameInsideItsStringLiteral(t *testing.T) {
	targets := []compareTarget{{
		Alias: "c0",
		Repo:  Repo{Owner: "acme", Name: "api"},
		Base:  "main",
		Head:  `") { x } evil: repository(owner: "`,
	}}
	q := buildCompareQuery(targets)
	if strings.Contains(q, "evil:") && !strings.Contains(q, `\"`) {
		t.Errorf("a branch name must stay inside its string literal:\n%s", q)
	}
}

func TestPullRequestQueryAsksForTheMergeState(t *testing.T) {
	if !strings.Contains(pullRequestQuery, "mergeStateStatus") {
		t.Error("without mergeStateStatus every protected branch looks ready to merge")
	}
}

func TestMergeStateStatusDecodesFromTheResponse(t *testing.T) {
	var resp graphQLPRResponse
	body := `{"data":{"repository":{"nameWithOwner":"acme/api","pullRequests":{"nodes":[
		{"number":1,"mergeStateStatus":"BLOCKED"}]}}}}`
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatal(err)
	}
	if got := resp.Data.Repository.PullRequests.Nodes[0].MergeStateStatus; got != "BLOCKED" {
		t.Fatalf("mergeStateStatus must decode, got %q", got)
	}
}

func TestTailLinesStripsTerminalEscapesFromWorkflowLogs(t *testing.T) {
	raw := "\x1b]52;c;cHduZWQ=\x07clipboard\n\x1b[31mred\x1b[0m\n\x1b]0;retitled\x07\n"
	got := tailLines([]byte(raw), 10)

	for _, line := range got {
		if strings.ContainsRune(line, 0x1b) {
			t.Errorf("escape sequence reached the terminal: %q", line)
		}
	}
	if len(got) != 3 || got[0] != "clipboard" || got[1] != "red" {
		t.Errorf("stripping must keep the text, got %q", got)
	}
}
