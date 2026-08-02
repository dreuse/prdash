package config

import (
	"time"

	"github.com/dreuse/prdash/internal/model"
)

const (
	cacheFile   = "cache.json"
	cacheSchema = 1
)

type Cache struct {
	Schema       int                 `json:"schema"`
	FetchedAt    time.Time           `json:"fetched_at"`
	Viewer       string              `json:"viewer"`
	PullRequests []model.PullRequest `json:"pull_requests"`
	Runs         []model.WorkflowRun `json:"runs"`
	Issues       []model.Issue       `json:"issues"`
	People       []model.User        `json:"people"`
}

func LoadCache() (Cache, bool) {
	var c Cache
	if !readJSON(cacheFile, &c) || c.Schema != cacheSchema {
		return Cache{}, false
	}
	return c, true
}

func SaveCache(c Cache) error {
	c.Schema = cacheSchema
	return writeJSON(cacheFile, c)
}
