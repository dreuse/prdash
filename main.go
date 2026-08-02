package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dreuse/prdash/internal/github"
	"github.com/dreuse/prdash/internal/readiness"
	"github.com/dreuse/prdash/internal/ui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "prdash:", err)
		os.Exit(1)
	}
}

func run() error {
	mock := flag.Bool("mock", false, "use built-in sample data instead of the github api")
	interval := flag.Duration("interval", ui.DefaultRefreshInterval, "auto refresh interval")
	approvals := flag.Int("required-approvals", readiness.DefaultRequiredApprovals, "approvals required before a pr counts as ready to merge")
	behindBlocks := flag.Bool("behind-blocks", false, "treat a branch behind its base as a merge blocker")
	flag.Usage = usage
	flag.Parse()

	policy := readiness.Policy{RequiredApprovals: *approvals, BehindBlocks: *behindBlocks}

	var fetcher github.Fetcher
	var labels []string

	if *mock {
		fetcher = github.NewMock()
		labels = []string{"mock data"}
	} else {
		repos, err := github.ParseRepos(flag.Args())
		if err != nil {
			return err
		}
		if len(repos) == 0 {
			flag.Usage()
			return fmt.Errorf("no repositories given")
		}
		cli := github.NewCLI(repos)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := cli.CheckAuth(ctx); err != nil {
			return fmt.Errorf("%w\nrun `gh auth login` first, or pass --mock to explore the ui without credentials", err)
		}
		fetcher = cli
		for _, r := range repos {
			labels = append(labels, r.String())
		}
	}

	program := tea.NewProgram(
		ui.New(fetcher, policy, *interval, labels),
		tea.WithAltScreen(),
	)
	_, err := program.Run()
	return err
}

func usage() {
	out := flag.CommandLine.Output()
	fmt.Fprintln(out, strings.TrimSpace(`
usage: prdash [flags] owner/repo [owner/repo ...]

a live kanban board of open pull requests, backed by the gh cli.

examples:
  prdash acme/api acme/web
  prdash --required-approvals 2 acme/api
  prdash --interval 30s --behind-blocks acme/infra
  prdash --mock

flags:`))
	flag.PrintDefaults()
}
