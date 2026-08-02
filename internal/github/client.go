package github

import (
	"context"
	"fmt"
	"strings"

	"github.com/dreuse/prdash/internal/model"
)

type Snapshot struct {
	PullRequests []model.PullRequest
	Runs         []model.WorkflowRun
}

type Fetcher interface {
	Fetch(ctx context.Context) (Snapshot, error)
}

type Repo struct {
	Owner string
	Name  string
}

func (r Repo) String() string { return r.Owner + "/" + r.Name }

func ParseRepo(s string) (Repo, error) {
	owner, name, ok := strings.Cut(strings.TrimSpace(s), "/")
	if !ok || owner == "" || name == "" {
		return Repo{}, fmt.Errorf("invalid repository %q: want owner/name", s)
	}
	return Repo{Owner: owner, Name: name}, nil
}

func ParseRepos(args []string) ([]Repo, error) {
	repos := make([]Repo, 0, len(args))
	for _, a := range args {
		r, err := ParseRepo(a)
		if err != nil {
			return nil, err
		}
		repos = append(repos, r)
	}
	return repos, nil
}

type Error struct {
	Repo      string
	Op        string
	Stderr    string
	RateLimit bool
	Auth      bool
	Err       error
}

func (e *Error) Error() string {
	msg := e.Op
	if e.Repo != "" {
		msg += " " + e.Repo
	}
	if e.Stderr != "" {
		return fmt.Sprintf("%s: %s", msg, firstLine(e.Stderr))
	}
	return fmt.Sprintf("%s: %v", msg, e.Err)
}

func (e *Error) Unwrap() error { return e.Err }

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func classifyStderr(stderr string) (rateLimit, auth bool) {
	low := strings.ToLower(stderr)
	rateLimit = strings.Contains(low, "rate limit") || strings.Contains(low, "secondary rate") || strings.Contains(low, "was submitted too quickly")
	auth = strings.Contains(low, "gh auth login") ||
		strings.Contains(low, "authentication") ||
		strings.Contains(low, "bad credentials") ||
		strings.Contains(low, "not logged in")
	return rateLimit, auth
}
