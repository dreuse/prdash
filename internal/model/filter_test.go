package model

import (
	"testing"
	"time"
)

func daysAgo(n int) time.Time { return time.Now().Add(-time.Duration(n) * 24 * time.Hour) }

func samplePR() PullRequest {
	return PullRequest{
		Repo:               "acme/payments-api",
		Number:             12012,
		Title:              "Make migrations idempotent",
		Author:             "octodev",
		HeadRef:            "feature/sql-idempotent",
		CreatedAt:          daysAgo(6),
		UpdatedAt:          daysAgo(1),
		BehindBy:           19,
		Labels:             []string{"database"},
		RequestedReviewers: []string{"jchen"},
		Checks:             []Check{{State: CheckPassed}, {State: CheckPassed}},
	}
}

func TestParseFilterMarksUnknownTokens(t *testing.T) {
	tests := []struct {
		raw   string
		valid bool
	}{
		{"author:@me", true},
		{"reviewer:@me", true},
		{"repo:payments-api", true},
		{"label:bug", true},
		{"state:blocked", true},
		{"is:draft", true},
		{"is:stale", true},
		{"behind:>50", true},
		{"age:>30d", true},
		{"free text", true},
		{"state:nonsense", false},
		{"is:purple", false},
		{"behind:>abc", false},
		{"age:>xd", false},
		{"nonsense:1", false},
		{"author:", false},
	}
	for _, tc := range tests {
		f := ParseFilter(tc.raw)
		if got := f.Valid(); got != tc.valid {
			t.Errorf("ParseFilter(%q).Valid() = %v, want %v", tc.raw, got, tc.valid)
		}
	}
}

func TestFilterMatch(t *testing.T) {
	pr := samplePR()
	ctx := FilterContext{Viewer: "jchen", Column: ColNeedsReview}

	tests := []struct {
		raw  string
		want bool
	}{
		{"", true},
		{"author:octodev", true},
		{"author:@me", false},
		{"author:any", true},
		{"reviewer:@me", true},
		{"reviewer:none", false},
		{"repo:payments", true},
		{"repo:other", false},
		{"label:database", true},
		{"label:bug", false},
		{"state:needs-review", true},
		{"state:blocked", false},
		{"is:draft", false},
		{"is:stale", false},
		{"behind:>10", true},
		{"behind:>50", false},
		{"behind:<20", true},
		{"age:>3d", true},
		{"age:>30d", false},
		{"idempotent", true},
		{"feature/sql", true},
		{"nothing-like-this", false},
		{"author:octodev behind:>10", true},
		{"author:octodev behind:>50", false},
		{"state:nonsense author:octodev", true},
	}
	for _, tc := range tests {
		if got := ParseFilter(tc.raw).Match(pr, ctx); got != tc.want {
			t.Errorf("filter %q matched = %v, want %v", tc.raw, got, tc.want)
		}
	}
}

func TestCycleFilterValue(t *testing.T) {
	tests := []struct{ in, want string }{
		{"", "author:@me"},
		{"author:@me", "author:any"},
		{"author:any", "author:@me"},
		{"repo:x is:draft", "repo:x is:stale"},
		{"free", "free"},
	}
	for _, tc := range tests {
		if got := CycleFilterValue(tc.in); got != tc.want {
			t.Errorf("CycleFilterValue(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestStakeFor(t *testing.T) {
	pr := samplePR()
	pr.Reviews = []Review{{Login: "tokoro", State: ReviewApproved, SubmittedAt: daysAgo(2)}}

	tests := []struct {
		viewer string
		want   Stake
	}{
		{"", StakeNone},
		{"octodev", StakeAuthor},
		{"JCHEN", StakeRequested},
		{"tokoro", StakeApproved},
		{"nobody", StakeNone},
	}
	for _, tc := range tests {
		if got := pr.StakeFor(tc.viewer); got != tc.want {
			t.Errorf("StakeFor(%q) = %v, want %v", tc.viewer, got, tc.want)
		}
	}
}

func TestReviewStale(t *testing.T) {
	pr := samplePR()
	pr.UpdatedAt = daysAgo(1)
	pr.Reviews = []Review{{Login: "tokoro", State: ReviewApproved, SubmittedAt: daysAgo(3)}}

	if !pr.ReviewStale("tokoro") {
		t.Error("a review older than the last update must be stale")
	}
	pr.Reviews[0].SubmittedAt = time.Now()
	if pr.ReviewStale("tokoro") {
		t.Error("a review newer than the last update must not be stale")
	}
	if pr.ReviewStale("someone-else") {
		t.Error("a reviewer who never reviewed cannot be stale")
	}
}

func TestFuzzyFreeText(t *testing.T) {
	pr := samplePR()
	pr.Assignees = []string{"tokoro"}
	pr.Labels = []string{"database"}
	ctx := FilterContext{Viewer: "jchen", Column: ColNeedsReview}

	tests := []struct {
		query string
		want  bool
	}{
		{"idempotent", true},
		{"sql", true},
		{"SQL", true},
		{"sqlidem", true},
		{"sqidm", true},
		{"12012", true},
		{"octodev", true},
		{"database", true},
		{"tokoro", true},
		{"payments", true},
		{"feature/sql", true},
		{"zzzqqq", false},
		{"nonexistentword", false},
	}
	for _, tc := range tests {
		if got := ParseFilter(tc.query).Match(pr, ctx); got != tc.want {
			t.Errorf("free text %q matched = %v, want %v", tc.query, got, tc.want)
		}
	}
}

func TestFuzzyIsASubsequence(t *testing.T) {
	tests := []struct {
		hay, needle string
		want        bool
	}{
		{"sql idempotent", "sqi", true},
		{"sql idempotent", "sqlidem", true},
		{"sql idempotent", "idem", true},
		{"sql idempotent", "qs", false},
		{"sql idempotent", "", true},
		{"", "x", false},
	}
	for _, tc := range tests {
		if got := Fuzzy(tc.hay, tc.needle); got != tc.want {
			t.Errorf("Fuzzy(%q, %q) = %v, want %v", tc.hay, tc.needle, got, tc.want)
		}
	}
}

func TestAssigneeFilter(t *testing.T) {
	ctx := FilterContext{Viewer: "tokoro", Column: ColNeedsReview}

	assigned := samplePR()
	assigned.Assignees = []string{"tokoro", "jchen"}

	unassigned := samplePR()

	tests := []struct {
		query string
		pr    PullRequest
		want  bool
	}{
		{"assignee:tokoro", assigned, true},
		{"assignee:@me", assigned, true},
		{"assignee:@me", unassigned, false},
		{"assignee:any", assigned, true},
		{"assignee:any", unassigned, false},
		{"assignee:none", unassigned, true},
		{"assignee:none", assigned, false},
		{"assignee:nobody", assigned, false},
	}
	for _, tc := range tests {
		if got := ParseFilter(tc.query).Match(tc.pr, ctx); got != tc.want {
			t.Errorf("%q matched = %v, want %v", tc.query, got, tc.want)
		}
	}
	if !ParseFilter("assignee:tokoro").Valid() {
		t.Error("assignee should be a known key")
	}
}

func TestFuzzyHandlesAccentedText(t *testing.T) {
	pr := samplePR()
	pr.Title = "Café résumé blocklist for São Paulo"
	ctx := FilterContext{}

	for _, q := range []string{"café", "Café", "blocklist", "cabl", "paulo"} {
		if !ParseFilter(q).Match(pr, ctx) {
			t.Errorf("%q should match %q", q, pr.Title)
		}
	}
	if ParseFilter("zzz").Match(pr, ctx) {
		t.Error("unrelated text must not match")
	}
}

func TestFuzzyMatchesPerFieldNotAcrossThem(t *testing.T) {
	unrelated := samplePR()
	unrelated.Title = "Add category field to the ledger"
	unrelated.HeadRef = "feat/category-field"
	unrelated.Author = "rmartins"
	unrelated.Labels = nil
	unrelated.RequestedReviewers = nil

	target := samplePR()

	ctx := FilterContext{}
	if !ParseFilter("idempo").Match(target, ctx) {
		t.Fatal("idempo should match Make migrations idempotent")
	}
	if ParseFilter("idempo").Match(unrelated, ctx) {
		t.Fatal("idempo must not match by stitching letters across separate fields")
	}
}

func TestNegatedTokens(t *testing.T) {
	ctx := FilterContext{Viewer: "octodev", Column: ColNeedsReview}

	draft := samplePR()
	draft.IsDraft = true
	normal := samplePR()

	tests := []struct {
		query string
		pr    PullRequest
		want  bool
	}{
		{"is:draft", draft, true},
		{"-is:draft", draft, false},
		{"-is:draft", normal, true},
		{"-author:octodev", normal, false},
		{"-author:someone", normal, true},
		{"-label:database", normal, false},
		{"-label:nothing", normal, true},
		{"-idempotent", normal, false},
		{"-zzzz", normal, true},
		{"-state:needs-review", normal, false},
		{"is:draft -author:someone", draft, true},
		{"-is:draft -author:someone", draft, false},
	}
	for _, tc := range tests {
		if got := ParseFilter(tc.query).Match(tc.pr, ctx); got != tc.want {
			t.Errorf("%q matched = %v, want %v", tc.query, got, tc.want)
		}
	}
}

func TestNegationIsParsed(t *testing.T) {
	f := ParseFilter("-is:draft author:me")
	if len(f.Tokens) != 2 {
		t.Fatalf("got %d tokens", len(f.Tokens))
	}
	if !f.Tokens[0].Negate || f.Tokens[0].Key != "is" {
		t.Fatalf("first token: %+v", f.Tokens[0])
	}
	if f.Tokens[1].Negate {
		t.Fatal("the second token must not be negated")
	}
	if !f.Valid() {
		t.Fatal("a negated known key stays valid")
	}
	if ParseFilter("-nonsense:x").Valid() {
		t.Fatal("a negated unknown key is still invalid")
	}
}

func TestCommaSeparatedValuesAreOr(t *testing.T) {
	ctx := FilterContext{Viewer: "octodev", Column: ColNeedsReview}
	pr := samplePR()
	pr.Labels = []string{"database"}

	tests := []struct {
		query string
		want  bool
	}{
		{"label:database", true},
		{"label:bug,database", true},
		{"label:bug,perf", false},
		{"-label:bug,database", false},
		{"is:draft,approved", false},
		{"author:octodev,someone", true},
	}
	for _, tc := range tests {
		if got := ParseFilter(tc.query).Match(pr, ctx); got != tc.want {
			t.Errorf("%q matched = %v, want %v", tc.query, got, tc.want)
		}
	}
}

func TestQuotedPhrases(t *testing.T) {
	ctx := FilterContext{}
	pr := samplePR()
	pr.Title = "Make migrations idempotent for payments"

	if !ParseFilter(`"idempotent for"`).Match(pr, ctx) {
		t.Error("a quoted phrase should match across the space")
	}
	if f := ParseFilter(`"idempotent for"`); len(f.Tokens) != 1 {
		t.Errorf("a quoted phrase is one token, got %d", len(f.Tokens))
	}
	if ParseFilter(`"payments idempotent"`).Match(pr, ctx) {
		t.Error("a quoted phrase must keep its word order")
	}
	if !ParseFilter(`-"nothing here"`).Match(pr, ctx) {
		t.Error("a negated quoted phrase should work")
	}
}

func TestNoKey(t *testing.T) {
	ctx := FilterContext{}

	bare := samplePR()
	bare.Assignees = nil
	bare.Labels = nil
	bare.RequestedReviewers = nil

	full := samplePR()
	full.Assignees = []string{"someone"}

	tests := []struct {
		query string
		pr    PullRequest
		want  bool
	}{
		{"no:assignee", bare, true},
		{"no:assignee", full, false},
		{"no:label", bare, true},
		{"no:reviewer", bare, true},
		{"-no:assignee", full, true},
	}
	for _, tc := range tests {
		if got := ParseFilter(tc.query).Match(tc.pr, ctx); got != tc.want {
			t.Errorf("%q matched = %v, want %v", tc.query, got, tc.want)
		}
	}
	if ParseFilter("no:nonsense").Valid() {
		t.Error("no: only takes assignee, reviewer or label")
	}
}

func TestLaneRuleVocabulary(t *testing.T) {
	parse := []struct {
		raw   string
		valid bool
	}{
		{"is:ready", true},
		{"is:running", true},
		{"is:changes-requested", true},
		{"is:blocked", true},
		{"is:behind", true},
		{"approvals:>=2", true},
		{"approvals:0", true},
		{"approvals:>abc", false},
	}
	for _, tc := range parse {
		if got := ParseFilter(tc.raw).Valid(); got != tc.valid {
			t.Errorf("ParseFilter(%q).Valid() = %v, want %v", tc.raw, got, tc.valid)
		}
	}

	running := samplePR()
	running.Checks = []Check{{State: CheckPassed}, {State: CheckRunning}}

	changes := samplePR()
	changes.ChangesRequested = 2

	protected := samplePR()
	protected.MergeStateStatus = "BLOCKED"

	approved := samplePR()
	approved.Approvals = 2

	match := []struct {
		query string
		pr    PullRequest
		ctx   FilterContext
		want  bool
	}{
		{"is:ready", samplePR(), FilterContext{Ready: true}, true},
		{"is:ready", samplePR(), FilterContext{}, false},
		{"-is:ready", samplePR(), FilterContext{}, true},
		{"is:running", running, FilterContext{}, true},
		{"is:running", samplePR(), FilterContext{}, false},
		{"is:changes-requested", changes, FilterContext{}, true},
		{"is:changes-requested", samplePR(), FilterContext{}, false},
		{"is:blocked", protected, FilterContext{}, true},
		{"is:blocked", samplePR(), FilterContext{}, false},
		{"is:behind", samplePR(), FilterContext{}, true}, // sample is 19 behind
		{"approvals:>=2", approved, FilterContext{}, true},
		{"approvals:>=2", samplePR(), FilterContext{}, false},
		{"approvals:0", samplePR(), FilterContext{}, true},
		{"is:conflict,failing", samplePR(), FilterContext{}, false},
	}
	for _, tc := range match {
		if got := ParseFilter(tc.query).Match(tc.pr, tc.ctx); got != tc.want {
			t.Errorf("%q matched = %v, want %v", tc.query, got, tc.want)
		}
	}
}
