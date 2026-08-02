package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dreuse/prdash/internal/config"
	"github.com/dreuse/prdash/internal/github"
	"github.com/dreuse/prdash/internal/ui"
)

const authTimeout = 15 * time.Second

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "prdash:", err)
		os.Exit(1)
	}
}

func run() error {
	mock := flag.Bool("mock", false, "use built-in sample data instead of the github api")
	view := flag.String("view", "", "open in board or ci for this run only")
	ascii := flag.Bool("ascii", false, "use ascii glyphs instead of unicode")
	flag.Usage = usage
	flag.Parse()

	settings := config.LoadSettings()
	if *ascii {
		settings.ASCII = true
	}
	for _, arg := range flag.Args() {
		if _, err := github.ParseRepo(arg); err != nil {
			return err
		}
		settings.Repos = appendUnique(settings.Repos, arg)
	}

	if *mock {
		mock := github.NewMock()
		source := func([]string) (github.Fetcher, github.Actor, error) { return mock, mock, nil }
		return start(settings, mock, mock, source, []string{"mock data"}, *view)
	}

	ctx, cancel := context.WithTimeout(context.Background(), authTimeout)
	defer cancel()
	if err := github.NewCLI(nil).CheckAuth(ctx); err != nil {
		return fmt.Errorf("%w\nrun `gh auth login` first, or pass --mock to explore the ui without credentials", err)
	}

	if len(settings.Repos) == 0 {
		s, err := firstRun(settings)
		if err != nil {
			return err
		}
		settings = s
	}

	source := func(names []string) (github.Fetcher, github.Actor, error) {
		repos, err := github.ParseRepos(names)
		if err != nil {
			return nil, nil, err
		}
		cli := github.NewCLI(repos)
		return cli, cli, nil
	}
	fetcher, actor, err := source(settings.Repos)
	if err != nil {
		return err
	}
	return start(settings, fetcher, actor, source, settings.Repos, *view)
}

func firstRun(settings config.Settings) (config.Settings, error) {
	setup := ui.NewSetup(settings)
	out, err := tea.NewProgram(setup, tea.WithAltScreen()).Run()
	if err != nil {
		return settings, err
	}
	done, ok := out.(ui.Setup)
	if !ok || !done.Done {
		return settings, fmt.Errorf("setup cancelled")
	}
	result := done.Settings()
	if err := config.SaveSettings(result); err != nil {
		return result, err
	}
	return result, nil
}

func start(settings config.Settings, fetcher github.Fetcher, actor github.Actor, source ui.Source, labels []string, view string) error {
	cache, hasCache := config.LoadCache()
	m := ui.New(ui.Options{
		Fetcher:   fetcher,
		Actor:     actor,
		NewSource: source,
		Settings:  settings,
		State:     config.LoadState(),
		Repos:     labels,
		View:      view,
		Cache:     cache,
		HasCache:  hasCache,
	})
	_, err := tea.NewProgram(m, tea.WithAltScreen(), tea.WithReportFocus()).Run()
	return err
}

func appendUnique(list []string, v string) []string {
	for _, existing := range list {
		if strings.EqualFold(existing, v) {
			return list
		}
	}
	return append(list, v)
}

func usage() {
	out := flag.CommandLine.Output()
	fmt.Fprintln(out, strings.TrimSpace(`
usage: prdash [flags] [owner/repo ...]

a board of your open pull requests and a ci health screen. 1 and 2 switch
between them, v splits the screen to show the selected pull request, and ,
opens settings.

repositories and every preference live in the settings overlay and persist in
`+config.Dir()+`; arguments only add repositories to that list.

examples:
  prdash
  prdash acme/api acme/web
  prdash --view ci
  prdash --mock

flags:`))
	flag.PrintDefaults()
}
