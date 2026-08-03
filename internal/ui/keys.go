package ui

import (
	"slices"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
)

type KeyMap struct {
	Board key.Binding
	CI    key.Binding
	Split key.Binding

	Up           key.Binding
	Down         key.Binding
	Left         key.Binding
	Right        key.Binding
	Top          key.Binding
	End          key.Binding
	PageUp       key.Binding
	PageDown     key.Binding
	HalfPageUp   key.Binding
	HalfPageDown key.Binding
	NextHunk     key.Binding
	PrevHunk     key.Binding
	Focus        key.Binding

	Open    key.Binding
	Approve key.Binding
	Comment key.Binding
	Merge   key.Binding
	Close   key.Binding
	Rerun   key.Binding
	Logs    key.Binding
	Diff    key.Binding
	Rebase  key.Binding
	Copy    key.Binding
	Clone   key.Binding

	SplitGrow   key.Binding
	SplitShrink key.Binding

	Repo         key.Binding
	Filter       key.Binding
	Sort         key.Binding
	Expand       key.Binding
	FailuresOnly key.Binding
	Refresh      key.Binding
	Update       key.Binding
	Settings     key.Binding
	Help         key.Binding
	Back         key.Binding
	Quit         key.Binding

	Save1 key.Binding
	Save2 key.Binding
	Save3 key.Binding
	Save4 key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Board: key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "board")),
		CI:    key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "ci")),
		Split: key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "details")),

		Up:    key.NewBinding(key.WithKeys("k", "up", "ctrl+y"), key.WithHelp("k", "up")),
		Down:  key.NewBinding(key.WithKeys("j", "down", "ctrl+e"), key.WithHelp("j", "down")),
		Left:  key.NewBinding(key.WithKeys("h", "left"), key.WithHelp("h", "prev lane")),
		Right: key.NewBinding(key.WithKeys("l", "right"), key.WithHelp("l", "next lane")),
		Top:   key.NewBinding(key.WithKeys("home"), key.WithHelp("home", "first")),
		End:   key.NewBinding(key.WithKeys("end", "G"), key.WithHelp("end", "last")),
		PageUp: key.NewBinding(key.WithKeys("pgup", "ctrl+b"),
			key.WithHelp("pgup", "page up")),
		PageDown: key.NewBinding(key.WithKeys("pgdown", "ctrl+f"),
			key.WithHelp("pgdn", "page down")),
		HalfPageUp: key.NewBinding(key.WithKeys("ctrl+u"),
			key.WithHelp("ctrl-u", "half page up")),
		HalfPageDown: key.NewBinding(key.WithKeys("ctrl+d"),
			key.WithHelp("ctrl-d", "half page down")),
		NextHunk: key.NewBinding(key.WithKeys("}"), key.WithHelp("}", "next hunk")),
		PrevHunk: key.NewBinding(key.WithKeys("{"), key.WithHelp("{", "previous hunk")),
		Focus:    key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "switch pane")),

		Open:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
		Approve: key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "approve")),
		Comment: key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "comment")),
		Merge:   key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "merge")),
		Close:   key.NewBinding(key.WithKeys("X"), key.WithHelp("X", "close")),
		Rerun:   key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "re-run checks")),
		Logs:    key.NewBinding(key.WithKeys("L"), key.WithHelp("L", "logs")),
		Diff:    key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "diff")),
		Rebase:  key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "rebase")),
		Copy:    key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "copy branch")),
		Clone:   key.NewBinding(key.WithKeys("Y"), key.WithHelp("Y", "copy checkout")),

		SplitGrow:   key.NewBinding(key.WithKeys("+", "="), key.WithHelp("+", "grow pane")),
		SplitShrink: key.NewBinding(key.WithKeys("-", "_"), key.WithHelp("-", "shrink pane")),

		Repo:         key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "repo")),
		Filter:       key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		Sort:         key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "sort")),
		Expand:       key.NewBinding(key.WithKeys("x", " "), key.WithHelp("x", "expand")),
		FailuresOnly: key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "failures only")),
		Refresh:      key.NewBinding(key.WithKeys("u", "ctrl+r", "f5"), key.WithHelp("u", "refresh")),
		Update:       key.NewBinding(key.WithKeys("U"), key.WithHelp("U", "update")),
		Settings:     key.NewBinding(key.WithKeys(",", "S"), key.WithHelp(",", "settings")),
		Help:         key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "keys")),
		Back:         key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		Quit:         key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),

		Save1: key.NewBinding(key.WithKeys("f1"), key.WithHelp("F1", "filter 1")),
		Save2: key.NewBinding(key.WithKeys("f2"), key.WithHelp("F2", "filter 2")),
		Save3: key.NewBinding(key.WithKeys("f3"), key.WithHelp("F3", "filter 3")),
		Save4: key.NewBinding(key.WithKeys("f4"), key.WithHelp("F4", "filter 4")),
	}
}

const alwaysQuit = "ctrl+c"

func (k *KeyMap) index() map[string]*key.Binding {
	return map[string]*key.Binding{
		"board": &k.Board, "ci": &k.CI, "split": &k.Split,
		"up": &k.Up, "down": &k.Down, "left": &k.Left, "right": &k.Right,
		"top": &k.Top, "end": &k.End,
		"pageup": &k.PageUp, "pagedown": &k.PageDown,
		"halfpageup": &k.HalfPageUp, "halfpagedown": &k.HalfPageDown,
		"nexthunk": &k.NextHunk, "prevhunk": &k.PrevHunk,
		"splitgrow": &k.SplitGrow, "splitshrink": &k.SplitShrink,
		"focus": &k.Focus,
		"open":  &k.Open, "approve": &k.Approve, "comment": &k.Comment,
		"merge": &k.Merge, "close": &k.Close, "rerun": &k.Rerun,
		"logs": &k.Logs, "diff": &k.Diff, "rebase": &k.Rebase,
		"copy": &k.Copy, "clone": &k.Clone,
		"repo": &k.Repo, "filter": &k.Filter, "sort": &k.Sort,
		"expand": &k.Expand, "failuresonly": &k.FailuresOnly,
		"refresh": &k.Refresh, "update": &k.Update,
		"settings": &k.Settings, "help": &k.Help,
		"back": &k.Back, "quit": &k.Quit,
		"save1": &k.Save1, "save2": &k.Save2, "save3": &k.Save3, "save4": &k.Save4,
	}
}

func ActionNames() []string {
	k := DefaultKeyMap()
	names := make([]string, 0, len(k.index()))
	for name := range k.index() {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (k KeyMap) Override(overrides map[string]string) KeyMap {
	index := k.index()
	for name, spec := range overrides {
		binding, known := index[strings.ToLower(strings.TrimSpace(name))]
		if !known {
			continue
		}
		keys := strings.Fields(spec)
		if len(keys) == 0 {
			continue
		}
		*binding = key.NewBinding(
			key.WithKeys(keys...),
			key.WithHelp(keys[0], binding.Help().Desc))
		releaseKeys(index, binding, keys)
	}
	if !slices.Contains(k.Quit.Keys(), alwaysQuit) {
		k.Quit = key.NewBinding(
			key.WithKeys(append(k.Quit.Keys(), alwaysQuit)...),
			key.WithHelp(k.Quit.Help().Key, k.Quit.Help().Desc))
	}
	return k
}

func releaseKeys(index map[string]*key.Binding, owner *key.Binding, taken []string) {
	for _, other := range index {
		if other == owner {
			continue
		}
		kept := make([]string, 0, len(other.Keys()))
		for _, k := range other.Keys() {
			if !slices.Contains(taken, k) {
				kept = append(kept, k)
			}
		}
		if len(kept) == len(other.Keys()) {
			continue
		}
		help := other.Help()
		if len(kept) > 0 && slices.Contains(taken, help.Key) {
			help.Key = kept[0]
		}
		*other = key.NewBinding(key.WithKeys(kept...), key.WithHelp(help.Key, help.Desc))
	}
}

func shown(bindings ...key.Binding) string {
	var out []string
	for _, b := range bindings {
		if keys := b.Keys(); len(keys) > 0 {
			out = append(out, keyLabel(keys[0]))
		}
	}
	if len(out) == 0 {
		return "unbound"
	}
	return strings.Join(out, " ")
}

func keyLabel(k string) string {
	switch k {
	case " ":
		return "space"
	case "enter":
		return "⏎"
	}
	return strings.ReplaceAll(k, "ctrl+", "ctrl-")
}

func (k KeyMap) HelpSections(g Glyphs) [][2]string {
	return [][2]string{
		{"VIEWS", ""},
		{shown(k.Board, k.CI), "board / ci"},
		{shown(k.Split), "split the screen and show the selected pull request"},
		{shown(k.Repo), "switch repository, add one, or d to remove"},
		{shown(k.Settings), "settings"},
		{shown(k.Help), "this help"},
		{shown(k.Quit), "quit, ctrl-c always works"},

		{"MOVE", ""},
		{shown(k.Down, k.Up) + " " + g.UpDown, "move selection"},
		{shown(k.Left, k.Right) + " " + g.LeftRight, "previous / next lane, collapse / expand"},
		{"gg " + shown(k.End), "first / last"},
		{shown(k.HalfPageDown, k.HalfPageUp), "half a page down / up"},
		{shown(k.PageDown, k.PageUp), "a full page down / up"},
		{"ctrl-e ctrl-y", "one line down / up, the same as j and k here"},
		{shown(k.NextHunk, k.PrevHunk), "next / previous hunk in the diff, block in the overview"},
		{shown(k.Left, k.Right) + " " + g.LeftRight + " ]c [c", "next / previous file in the diff"},
		{shown(k.Focus), "move into the detail pane (board) or the log pane (ci), and back"},
		{shown(k.SplitGrow, k.SplitShrink), "grow or shrink the detail pane, the size is remembered"},
		{shown(k.Expand), "expand or collapse the section under the cursor (board)"},

		{"ACT", ""},
		{shown(k.Open), "open in browser (detail overlay on a narrow terminal)"},
		{shown(k.Approve), "approve, when your approval is still missing"},
		{shown(k.Comment), "comment inline, :tada: and tab complete emoji"},
		{"shift-⏎", "new line while commenting, once the terminal is set up"},
		{"ctrl-j", "new line, works in every terminal"},
		{shown(k.Merge), "merge (always confirms)"},
		{shown(k.Close), "close without merging, the branch is kept"},
		{shown(k.Rerun), "re-run failed checks (confirms)"},
		{shown(k.Diff), "read the diff in the detail pane, esc goes back to the overview"},
		{shown(k.Logs), "split the ci screen and read the run log, esc closes it"},
		{shown(k.Rebase), "update branch from base (confirms)"},
		{shown(k.Copy, k.Clone), "copy branch name / git checkout command"},
		{shown(k.Refresh), "force refresh"},
		{shown(k.Update), "install the new release when one is offered, then restart"},

		{"FILTER", ""},
		{shown(k.Filter), "filter, with completion on keys and values"},
		{"", "author: assignee: reviewer: repo: label: state: is: no: behind: age:"},
		{"", "-token negates, key:a,b is or, \"two words\" is a phrase"},
		{"", "or just type words - they fuzzy match number, title, branch, people"},
		{"tab", "accept the suggestion, press again to cycle"},
		{shown(k.Back), "clear the filter"},
		{shown(k.Sort), "cycle sort"},
		{shown(k.FailuresOnly), "failures only in the run table, or the log mode when the log has focus"},
		{"F1..F4", "save the current filter, press again to recall"},

		{"MOUSE", ""},
		{"click", "select a pull request, or focus the pane you clicked in"},
		{"double click", "open the pull request in the browser"},
		{"wheel", "scroll whichever half the pointer is over"},
		{"", "turn it off in settings to get terminal text selection back"},

		{"KEYS", ""},
		{"", "rebind anything: \"keys\": {\"diff\": \"D\", \"approve\": \"ctrl+a\"} in settings.json"},
		{"", strings.Join(ActionNames(), " ")},
	}
}
