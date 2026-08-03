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

func TestPullRequestQueryAsksForCommentsAndRecentCommits(t *testing.T) {
	for _, want := range []string{"comments(last:", "recentCommits: commits(last:", "abbreviatedOid", "bodyText"} {
		if !strings.Contains(pullRequestQuery, want) {
			t.Errorf("the detail pane cannot show the discussion without %q", want)
		}
	}
	if !strings.Contains(pullRequestQuery, "commits(last: 1)") {
		t.Error("the check rollup must stay on a single commit, or every PR costs 5x the contexts")
	}
}

func TestCommentsAndCommitsDecodeFromTheResponse(t *testing.T) {
	var resp graphQLPRResponse
	body := `{"data":{"repository":{"nameWithOwner":"acme/api","pullRequests":{"nodes":[
		{"number":1,
		 "comments":{"totalCount":7,"nodes":[
			{"bodyText":"needs a test","createdAt":"2026-08-01T10:00:00Z","author":{"login":"rita"}}]},
		 "recentCommits":{"totalCount":12,"nodes":[
			{"commit":{"abbreviatedOid":"a3f91c2","messageHeadline":"fix retry on EOF",
			  "committedDate":"2026-08-01T09:00:00Z"}}]}}]}}}}`
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatal(err)
	}

	node := resp.Data.Repository.PullRequests.Nodes[0]
	if node.Comments.TotalCount != 7 || len(node.Comments.Nodes) != 1 {
		t.Fatalf("comments must decode, got %+v", node.Comments)
	}
	if got := node.Comments.Nodes[0].Author.Login; got != "rita" {
		t.Errorf("comment author must decode, got %q", got)
	}
	if node.RecentCommits.TotalCount != 12 || len(node.RecentCommits.Nodes) != 1 {
		t.Fatalf("commits must decode, got %+v", node.RecentCommits)
	}
	if got := node.RecentCommits.Nodes[0].Commit.AbbreviatedOID; got != "a3f91c2" {
		t.Errorf("commit oid must decode, got %q", got)
	}
}

func TestALongCommentIsFlattenedBeforeItReachesTheCache(t *testing.T) {
	body := "first line\n\n\nsecond line\n" + strings.Repeat("padding ", 700)
	got := flattenComment(body)

	if strings.ContainsAny(got, "\n\r") {
		t.Errorf("a comment must render as one logical line, got %q", got)
	}
	if len([]rune(got)) > maxCommentChars {
		t.Errorf("a comment must be capped at %d runes, got %d", maxCommentChars, len([]rune(got)))
	}
	if !strings.HasPrefix(got, "first line second line") {
		t.Errorf("flattening must keep the words, got %q", got)
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
