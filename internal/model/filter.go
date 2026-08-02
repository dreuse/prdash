package model

import (
	"strconv"
	"strings"
	"time"
)

const StaleAfter = 30 * 24 * time.Hour

type FilterContext struct {
	Viewer string
	Column Column
}

type FilterToken struct {
	Text   string
	Key    string
	Op     string
	Value  string
	Values []string
	Negate bool
	Valid  bool
}

type Filter struct {
	Raw    string
	Tokens []FilterToken
}

var FilterKeys = []string{"author", "assignee", "reviewer", "repo", "label", "state", "is", "no", "behind", "age"}

var filterValues = map[string][]string{
	"author":   {"@me", "any"},
	"assignee": {"@me", "any", "none"},
	"reviewer": {"@me", "any", "none"},
	"is":       {"draft", "stale", "conflict", "failing", "approved"},
	"no":       {"assignee", "reviewer", "label"},
	"state":    laneSlugs(),
	"behind":   {">10", ">50", ">100"},
	"age":      {">7d", ">30d", ">180d"},
}

func laneSlugs() []string {
	out := make([]string, 0, len(ActionFirstColumns))
	for _, c := range ActionFirstColumns {
		out = append(out, c.Slug())
	}
	return out
}

func ParseFilter(raw string) Filter {
	f := Filter{Raw: raw}
	for _, word := range splitTokens(raw) {
		f.Tokens = append(f.Tokens, parseToken(word))
	}
	return f
}

func splitTokens(raw string) []string {
	var out []string
	var current strings.Builder
	quoted := false

	flush := func() {
		if current.Len() > 0 {
			out = append(out, current.String())
			current.Reset()
		}
	}
	for _, r := range raw {
		switch {
		case r == '"':
			quoted = !quoted
		case (r == ' ' || r == '\t') && !quoted:
			flush()
		default:
			current.WriteRune(r)
		}
	}
	flush()
	return out
}

func parseToken(word string) FilterToken {
	t := FilterToken{Text: word}
	if strings.HasPrefix(word, "-") && len(word) > 1 {
		t.Negate = true
		word = word[1:]
	}

	key, rest, ok := strings.Cut(word, ":")
	if !ok {
		t.Value = word
		t.Values = []string{word}
		t.Valid = word != ""
		return t
	}

	t.Key = strings.ToLower(key)
	if !knownKey(t.Key) {
		return t
	}
	t.Op, t.Value = splitOp(rest)
	t.Values = splitValues(t.Value)
	t.Valid = len(t.Values) > 0
	for _, v := range t.Values {
		if !validValue(t.Key, t.Op, v) {
			t.Valid = false
		}
	}
	return t
}

func splitValues(raw string) []string {
	var out []string
	for _, v := range strings.Split(raw, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func knownKey(k string) bool {
	for _, known := range FilterKeys {
		if k == known {
			return true
		}
	}
	return false
}

func splitOp(s string) (op, value string) {
	for _, candidate := range []string{">=", "<=", ">", "<"} {
		if strings.HasPrefix(s, candidate) {
			return candidate, s[len(candidate):]
		}
	}
	return "", s
}

func validValue(key, op, value string) bool {
	if value == "" {
		return false
	}
	switch key {
	case "behind":
		_, err := strconv.Atoi(value)
		return err == nil
	case "age":
		_, ok := parseDays(value)
		return ok
	case "is":
		return contains(filterValues["is"], strings.ToLower(value))
	case "state":
		_, ok := ColumnBySlug(strings.ToLower(value))
		return ok
	case "no":
		return contains(filterValues["no"], strings.ToLower(value))
	}
	return op == ""
}

func parseDays(s string) (int, bool) {
	s = strings.TrimSuffix(strings.ToLower(s), "d")
	n, err := strconv.Atoi(s)
	return n, err == nil && n >= 0
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func searchFields(p PullRequest) []string {
	fields := make([]string, 0, 6+len(p.Labels)+len(p.Assignees)+len(p.RequestedReviewers))
	fields = append(fields, itoa(p.Number), p.Title, p.HeadRef, p.Author, shortName(p.Repo))
	fields = append(fields, p.Labels...)
	fields = append(fields, p.Assignees...)
	fields = append(fields, p.RequestedReviewers...)
	return fields
}

func shortName(repo string) string {
	if _, name, ok := strings.Cut(repo, "/"); ok {
		return name
	}
	return repo
}

func FuzzyAny(fields []string, needle string) bool {
	if strings.TrimSpace(needle) == "" {
		return true
	}
	for _, f := range fields {
		if Fuzzy(strings.ToLower(f), needle) {
			return true
		}
	}
	return false
}

func itoa(n int) string { return strconv.Itoa(n) }

func Fuzzy(haystack, needle string) bool {
	needle = strings.ToLower(strings.TrimSpace(needle))
	if needle == "" {
		return true
	}
	if strings.Contains(haystack, needle) {
		return true
	}

	want := []rune(needle)
	i := 0
	for _, r := range haystack {
		if i < len(want) && want[i] == r {
			i++
		}
	}
	return i == len(want)
}

func (f Filter) Empty() bool { return len(f.Tokens) == 0 }

func (f Filter) Valid() bool {
	for _, t := range f.Tokens {
		if !t.Valid {
			return false
		}
	}
	return true
}

func (f Filter) Match(p PullRequest, ctx FilterContext) bool {
	for _, t := range f.Tokens {
		if !t.Valid {
			continue
		}
		if t.match(p, ctx) == t.Negate {
			return false
		}
	}
	return true
}

func (t FilterToken) match(p PullRequest, ctx FilterContext) bool {
	for _, v := range t.Values {
		if t.matchValue(p, ctx, v) {
			return true
		}
	}
	return false
}

func (t FilterToken) matchValue(p PullRequest, ctx FilterContext, raw string) bool {
	value := raw
	if strings.EqualFold(value, "@me") && ctx.Viewer != "" {
		value = ctx.Viewer
	}
	switch t.Key {
	case "":
		return FuzzyAny(searchFields(p), value)
	case "author":
		return strings.EqualFold(value, "any") || strings.EqualFold(p.Author, value)
	case "assignee":
		switch {
		case strings.EqualFold(value, "any"):
			return len(p.Assignees) > 0
		case strings.EqualFold(value, "none"):
			return len(p.Assignees) == 0
		}
		return p.AssignedTo(value)
	case "reviewer":
		switch {
		case strings.EqualFold(value, "any"):
			return true
		case strings.EqualFold(value, "none"):
			return len(p.RequestedReviewers) == 0
		}
		return p.RequestedFrom(value)
	case "repo":
		return strings.Contains(strings.ToLower(p.Repo), strings.ToLower(value))
	case "label":
		for _, l := range p.Labels {
			if strings.EqualFold(l, value) {
				return true
			}
		}
		return false
	case "state":
		col, ok := ColumnBySlug(strings.ToLower(value))
		return ok && col == ctx.Column
	case "is":
		return matchIs(p, value)
	case "no":
		switch strings.ToLower(value) {
		case "assignee":
			return len(p.Assignees) == 0
		case "reviewer":
			return len(p.RequestedReviewers) == 0
		case "label":
			return len(p.Labels) == 0
		}
		return false
	case "behind":
		return compare(p.BehindBy, t.Op, atoi(value))
	case "age":
		days, ok := parseDays(value)
		return ok && compare(int(p.Age().Hours()/24), t.Op, days)
	}
	return true
}

func matchIs(p PullRequest, value string) bool {
	switch strings.ToLower(value) {
	case "draft":
		return p.IsDraft
	case "stale":
		return p.Idle() > StaleAfter
	case "conflict":
		return p.HasConflicts()
	case "failing":
		return p.CheckCounts().Failed > 0
	case "approved":
		return p.Approvals > 0
	}
	return false
}

func compare(got int, op string, want int) bool {
	switch op {
	case ">":
		return got > want
	case ">=":
		return got >= want
	case "<":
		return got < want
	case "<=":
		return got <= want
	}
	return got == want
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func CycleFilterValue(raw string) string {
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return "author:@me"
	}
	last := fields[len(fields)-1]
	key, rest, ok := strings.Cut(last, ":")
	if !ok {
		return raw
	}
	values := filterValues[strings.ToLower(key)]
	if len(values) == 0 {
		return raw
	}
	next := values[0]
	for i, v := range values {
		if v == rest {
			next = values[(i+1)%len(values)]
			break
		}
	}
	fields[len(fields)-1] = key + ":" + next
	return strings.Join(fields, " ")
}
