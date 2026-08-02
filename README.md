# prdash

A terminal board of your open GitHub pull requests across one or more repositories, plus a
CI health screen.

![prdash](docs/demo.gif)

<sub>Static view: [docs/screenshot.svg](docs/screenshot.svg)</sub>

| Key | View | Answers |
|---|---|---|
| `1` | **Board** — kanban lanes ordered by what you can act on | what is the state of the whole queue? |
| `2` | **CI** — pass rate, medians, and the failure that matters | is CI healthy, and what broke? |

The CI screen is one table: every run still going, plus everything that finished inside the
recent window — 24 hours by default, and a setting. Runs still going sort first, the rest
newest after them. A row carries the workflow, the repository, the branch, the triggering
event, status, duration and age, and the pull request number when the branch has one open.
Columns drop as the terminal narrows, so the branch stays readable. Below the table, one row
per workflow tracks pass rate, median duration and a sparkline of its recent runs.

`L` splits the screen and loads the selected run's log into the bottom pane — only the
failing steps when the run failed, which is both faster and the part you wanted. `tab` moves
between the table and the log; arrows, `home`/`end` and `pgup`/`pgdn` act on whichever pane
has focus, and `esc` closes the split. Logs are cached per run and fetched only once the
cursor settles, so arrowing down the table does not hammer the API. A run that is still going
has no log yet — GitHub serves them once a job finishes — and the pane says so.

`v` splits the screen: the board keeps the top half and the selected pull request gets the
bottom — checks, reviewers, branch state and the actions that are legal right now. Moving
the selection updates it live; `v` again closes it. The pane lays itself out in two columns
on a wide terminal and stacks on a narrow one.

The board shows as many lanes as fit at 26 columns each and pages the rest with `←→`, so it
stays usable down to 60 columns.

`R` opens the repository switcher: every tracked repo with its open count, fuzzy filtering,
and `All repositories` to see them together. Type an `owner/name` it does not know yet and
it offers to start tracking it; `d` stops tracking the highlighted one. Repository names are
matched case-insensitively and corrected to GitHub's own spelling, so `owner/myrepo` and
`owner/MyRepo` never end up as two entries. The choice scopes the board, the CI screen and
the counts, and survives a restart.

Empty lanes are omitted rather than shown as empty columns; when nothing matches at all the
board says so and, if a filter caused it, offers a way out.

The board polls every 30s while the terminal is focused and backs off to 5m when it is not,
with jitter. `u` (also `ctrl-r` or `F5`) forces a refresh; the header shows how long ago the
last successful fetch landed.

Anything you can see you can act on: `a` approve, `m` merge, `X` close, `c` comment,
`r` re-run failed checks, `b` update the branch, `y` copy the branch name. Closing never
deletes the head branch — prdash does not pass `--delete-branch`, so the branch stays and
the pull request can be reopened on GitHub.

Everything that other people can see asks first — approve, merge, close, update branch and
re-run all show what they are about to do and wait for `y`. Only local actions (`y` copy,
`u` refresh, navigation) act immediately. The footer only offers verbs that
are legal for the selected pull request — `a` disappears once your approval is in, and
comes back if the branch moves and your review goes stale. Merging and approving confirm
first; every action shows pending, then a result toast.

`c` opens a one-line comment bar at the bottom with completion on three triggers, all
cycled with `tab`:

| Type | Completes |
|---|---|
| `:roc` | emoji — `:tada:` also becomes 🎉 on the closing colon |
| `#120` | open pull requests and issues, by number or title |
| `@alf` | people who can be assigned on the repo |

GitHub's image-only emoji (`:shipit:`, `:octocat:`) stay as shortcodes for GitHub to
render. `⏎` sends, `esc` cancels, and `ctrl-j` starts a new line — shown as `↵` in the bar
and sent as a real newline. `alt-enter` and `shift-enter` are bound too, but most terminals
send those identically to `⏎`, so `ctrl-j` is the one that always works.

The emoji list is downloaded once from your GitHub instance and cached in the config
folder, refreshed monthly — so custom and Enterprise emoji work too. A small built-in set
covers the gap before the first download lands.

`/` filters, with the same ghost-text completion as the comment bar: type `assi` and it
completes the key, then offers the logins it actually knows.

| Form | Meaning |
|---|---|
| `author: assignee: reviewer: repo: label: state: is: no: behind: age:` | the keys |
| `-is:draft` | negate any clause |
| `label:bug,perf` | comma separates alternatives |
| `"two words"` | quoted phrase |
| `idempo`, `sqidm` | plain words fuzzy match number, title, branch, author, labels, assignees |

Results update as you type, `esc` reverts to what was applied, and `F1`–`F4` save and recall
queries. `?` documents every key.

## Configuration

There is no config file to write. `,` opens a settings overlay from any view — default
view, theme, refresh interval, repositories, lane order, sort, startup filter.
Changes apply to the frame behind the overlay immediately and persist on their own to
`~/.config/pr-dashboard/` (`$XDG_CONFIG_HOME` respected). A missing or corrupt store falls
back to defaults instead of failing to start.

The last successful fetch is cached, so a cold start paints real data before the first
request returns, marked stale until it lands.

## Installation

Requires [`gh`](https://cli.github.com) installed and authenticated — `prdash` uses your
existing `gh` credentials and never handles tokens itself.

```sh
gh auth login
```

### Download a release

Grab the archive for your platform from the [releases page](../../releases), verify it
against `checksums.txt`, then put the binary on your `PATH`:

```sh
tar -xzf prdash_v0.1.0_darwin_arm64.tar.gz
sudo mv prdash_v0.1.0_darwin_arm64/prdash /usr/local/bin/
```

Builds are published for Linux, macOS and Windows on both amd64 and arm64.

### Install with Go

```sh
go install github.com/dreuse/prdash@latest
```

### Build from source

```sh
git clone https://github.com/dreuse/prdash.git
cd prdash
go build -o prdash .
```

### Run it

First run asks for a repository and a default view, then goes straight into the dashboard.

```sh
prdash
prdash acme/api acme/web acme/infra   # adds repositories to the stored list
prdash --view ci                      # override the default view for one run
prdash --ascii                        # ascii glyphs for one run
prdash --mock                         # explore the ui without credentials
```

`NO_COLOR=1` and light-background terminals are both supported; no information is carried
by colour alone.
