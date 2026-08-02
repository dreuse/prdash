package ui

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	Board key.Binding
	CI    key.Binding
	Split key.Binding

	Up       key.Binding
	Down     key.Binding
	Left     key.Binding
	Right    key.Binding
	Top      key.Binding
	End      key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Focus    key.Binding

	Open    key.Binding
	Approve key.Binding
	Comment key.Binding
	Merge   key.Binding
	Close   key.Binding
	Rerun   key.Binding
	Logs    key.Binding
	Rebase  key.Binding
	Copy    key.Binding
	Clone   key.Binding

	Repo         key.Binding
	Filter       key.Binding
	Sort         key.Binding
	Expand       key.Binding
	FailuresOnly key.Binding
	Refresh      key.Binding
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

		Up:    key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k", "up")),
		Down:  key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j", "down")),
		Left:  key.NewBinding(key.WithKeys("h", "left"), key.WithHelp("h", "prev lane")),
		Right: key.NewBinding(key.WithKeys("l", "right"), key.WithHelp("l", "next lane")),
		Top:   key.NewBinding(key.WithKeys("home"), key.WithHelp("home", "first")),
		End:   key.NewBinding(key.WithKeys("end", "G"), key.WithHelp("end", "last")),
		PageUp: key.NewBinding(key.WithKeys("pgup", "ctrl+u"),
			key.WithHelp("pgup", "page up")),
		PageDown: key.NewBinding(key.WithKeys("pgdown", "ctrl+d"),
			key.WithHelp("pgdn", "page down")),
		Focus: key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "switch pane")),

		Open:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
		Approve: key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "approve")),
		Comment: key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "comment")),
		Merge:   key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "merge")),
		Close:   key.NewBinding(key.WithKeys("X"), key.WithHelp("X", "close")),
		Rerun:   key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "re-run checks")),
		Logs:    key.NewBinding(key.WithKeys("L"), key.WithHelp("L", "logs")),
		Rebase:  key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "rebase")),
		Copy:    key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "copy branch")),
		Clone:   key.NewBinding(key.WithKeys("Y"), key.WithHelp("Y", "copy checkout")),

		Repo:         key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "repo")),
		Filter:       key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		Sort:         key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "sort")),
		Expand:       key.NewBinding(key.WithKeys("x", " "), key.WithHelp("x", "expand")),
		FailuresOnly: key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "failures only")),
		Refresh:      key.NewBinding(key.WithKeys("u", "ctrl+r", "f5"), key.WithHelp("u", "refresh")),
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

func (k KeyMap) HelpSections(g Glyphs) [][2]string {
	return [][2]string{
		{"VIEWS", ""},
		{"1 / 2", "board / ci"},
		{"v", "split the screen and show the selected pull request"},
		{"R", "switch repository, add one, or d to remove"},
		{", or S", "settings"},
		{"?", "this help"},
		{"q ctrl-c", "quit"},

		{"MOVE", ""},
		{"j k " + g.UpDown, "move selection"},
		{"h l " + g.LeftRight, "previous / next lane, collapse / expand"},
		{"home end", "first / last"},
		{"pgup pgdn", "page the focused list or log"},
		{"tab", "move between the run table and the log pane (ci)"},
		{"x space", "expand or collapse the section under the cursor (board)"},

		{"ACT", ""},
		{g.Enter, "open in browser (detail overlay on a narrow terminal)"},
		{"a", "approve, when your approval is still missing"},
		{"c", "comment inline, :tada: and tab complete emoji"},
		{"shift-⏎", "new line while commenting, once the terminal is set up"},
		{"ctrl-j", "new line, works in every terminal"},
		{"m", "merge (always confirms)"},
		{"X", "close without merging, the branch is kept"},
		{"r", "re-run failed checks (confirms)"},
		{"L", "split the ci screen and read the run log, esc closes it"},
		{"b", "update branch from base (confirms)"},
		{"y Y", "copy branch name / git checkout command"},
		{"u ctrl-r F5", "force refresh"},

		{"FILTER", ""},
		{"/", "filter, with completion on keys and values"},
		{"", "author: assignee: reviewer: repo: label: state: is: no: behind: age:"},
		{"", "-token negates, key:a,b is or, \"two words\" is a phrase"},
		{"", "or just type words - they fuzzy match number, title, branch, people"},
		{"tab", "accept the suggestion, press again to cycle"},
		{"esc", "clear the filter"},
		{"s", "cycle sort"},
		{"f", "failures only in the run table, or the log mode when the log has focus"},
		{"F1..F4", "save the current filter, press again to recall"},
	}
}
