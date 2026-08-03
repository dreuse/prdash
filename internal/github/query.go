package github

import (
	"encoding/json"
	"fmt"
	"strings"
)

const pullRequestQuery = `
query($owner: String!, $name: String!, $limit: Int!) {
  viewer { login }
  repository(owner: $owner, name: $name) {
    nameWithOwner
    assignableUsers(first: 100) { nodes { login name } }
    issues(states: OPEN, first: 50, orderBy: {field: UPDATED_AT, direction: DESC}) {
      nodes { number title url }
    }
    pullRequests(states: OPEN, first: $limit, orderBy: {field: UPDATED_AT, direction: DESC}) {
      nodes {
        number
        title
        url
        isDraft
        mergeable
        mergeStateStatus
        createdAt
        updatedAt
        baseRefName
        headRefName
        additions
        deletions
        changedFiles
        labels(first: 20) { nodes { name } }
        assignees(first: 10) { nodes { login } }
        headRepositoryOwner { login }
        author { login }
        reviewRequests(first: 20) {
          nodes {
            requestedReviewer {
              ... on User { login }
              ... on Team { name }
            }
          }
        }
        latestOpinionatedReviews(first: 50) {
          nodes { state submittedAt author { login } }
        }
        comments(last: 30) {
          nodes { bodyText createdAt author { login __typename } }
        }
        recentCommits: commits(last: 5) {
          totalCount
          nodes { commit { abbreviatedOid messageHeadline committedDate } }
        }
        commits(last: 1) {
          nodes {
            commit {
              statusCheckRollup {
                contexts(first: 100) {
                  nodes {
                    ... on CheckRun { name status conclusion detailsUrl startedAt }
                    ... on StatusContext { context state targetUrl }
                  }
                }
              }
            }
          }
        }
      }
    }
  }
}`

type compareTarget struct {
	Alias   string
	Repo    Repo
	Base    string
	Head    string
	PRIndex int
}

func graphQLString(s string) string {
	quoted, err := json.Marshal(s)
	if err != nil {
		return `""`
	}
	return string(quoted)
}

func buildCompareQuery(targets []compareTarget) string {
	var b strings.Builder
	b.WriteString("query {\n")
	for _, t := range targets {
		fmt.Fprintf(&b,
			"  %s: repository(owner: %s, name: %s) { ref(qualifiedName: %s) { compare(headRef: %s) { behindBy } } }\n",
			t.Alias,
			graphQLString(t.Repo.Owner), graphQLString(t.Repo.Name),
			graphQLString(t.Base), graphQLString(t.Head))
	}
	b.WriteString("}")
	return b.String()
}
