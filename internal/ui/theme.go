package ui

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/dreuse/prdash/internal/model"
)

type Icons struct {
	Passed    string
	Failed    string
	Running   string
	Neutral   string
	Conflict  string
	Behind    string
	Approved  string
	Reviewer  string
	Draft     string
	Selected  string
	Spinner   []string
	Separator string
	Dot       string
	Arrow     string
	ASCIIOnly bool
}

var unicodeIcons = Icons{
	Passed:    "✔",
	Failed:    "✘",
	Running:   "◐",
	Neutral:   "•",
	Conflict:  "⚠",
	Behind:    "↓",
	Approved:  "✓",
	Reviewer:  "@",
	Draft:     "✎",
	Selected:  "▌",
	Spinner:   []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
	Separator: "│",
	Dot:       "·",
	Arrow:     "→",
}

var asciiIcons = Icons{
	Passed:    "ok",
	Failed:    "x",
	Running:   "~",
	Neutral:   "-",
	Conflict:  "!",
	Behind:    "v",
	Approved:  "+",
	Reviewer:  "@",
	Draft:     "d",
	Selected:  ">",
	Spinner:   []string{"|", "/", "-", "\\"},
	Separator: "|",
	Dot:       "-",
	Arrow:     "->",
	ASCIIOnly: true,
}

type Theme struct {
	Icons Icons

	Title       lipgloss.Style
	Status      lipgloss.Style
	StatusError lipgloss.Style
	StatusWarn  lipgloss.Style
	Help        lipgloss.Style
	Dim         lipgloss.Style
	Empty       lipgloss.Style

	ColumnHeader   lipgloss.Style
	ColumnCount    lipgloss.Style
	Card           lipgloss.Style
	CardSelected   lipgloss.Style
	CardTitle      lipgloss.Style
	CardMeta       lipgloss.Style
	TableHeader    lipgloss.Style
	TableRow       lipgloss.Style
	TableRowActive lipgloss.Style

	Passed  lipgloss.Style
	Failed  lipgloss.Style
	Running lipgloss.Style
	Warn    lipgloss.Style

	columnColor map[model.Column]lipgloss.TerminalColor
}

func NewTheme() Theme {
	profile := lipgloss.ColorProfile()
	t := Theme{Icons: unicodeIcons}
	if profile == termenv.Ascii || !unicodeCapable() {
		t.Icons = asciiIcons
	}

	var (
		fg      = lipgloss.AdaptiveColor{Light: "236", Dark: "252"}
		dim     = lipgloss.AdaptiveColor{Light: "245", Dark: "243"}
		accent  = lipgloss.AdaptiveColor{Light: "27", Dark: "39"}
		green   = lipgloss.AdaptiveColor{Light: "28", Dark: "42"}
		red     = lipgloss.AdaptiveColor{Light: "160", Dark: "203"}
		yellow  = lipgloss.AdaptiveColor{Light: "136", Dark: "221"}
		magenta = lipgloss.AdaptiveColor{Light: "90", Dark: "177"}
		border  = lipgloss.AdaptiveColor{Light: "252", Dark: "238"}
	)

	t.Title = lipgloss.NewStyle().Bold(true).Foreground(accent)
	t.Status = lipgloss.NewStyle().Foreground(dim)
	t.StatusError = lipgloss.NewStyle().Bold(true).Foreground(red)
	t.StatusWarn = lipgloss.NewStyle().Bold(true).Foreground(yellow)
	t.Help = lipgloss.NewStyle().Foreground(dim)
	t.Dim = lipgloss.NewStyle().Foreground(dim)
	t.Empty = lipgloss.NewStyle().Foreground(dim).Italic(true)

	cardBorder := lipgloss.RoundedBorder()
	if t.Icons.ASCIIOnly {
		cardBorder = lipgloss.ASCIIBorder()
	}

	t.ColumnHeader = lipgloss.NewStyle().Bold(true).Padding(0, 1)
	t.ColumnCount = lipgloss.NewStyle().Foreground(dim)
	t.Card = lipgloss.NewStyle().
		Border(cardBorder).
		BorderForeground(border).
		Padding(0, 1).
		MarginBottom(1)
	t.CardSelected = t.Card.Copy().BorderForeground(accent).Bold(false)
	t.CardTitle = lipgloss.NewStyle().Foreground(fg).Bold(true)
	t.CardMeta = lipgloss.NewStyle().Foreground(dim)

	t.TableHeader = lipgloss.NewStyle().Bold(true).Foreground(accent)
	t.TableRow = lipgloss.NewStyle().Foreground(fg)
	t.TableRowActive = lipgloss.NewStyle().Foreground(fg).Bold(true).Background(lipgloss.AdaptiveColor{Light: "254", Dark: "237"})

	t.Passed = lipgloss.NewStyle().Foreground(green)
	t.Failed = lipgloss.NewStyle().Foreground(red)
	t.Running = lipgloss.NewStyle().Foreground(yellow)
	t.Warn = lipgloss.NewStyle().Foreground(yellow)

	t.columnColor = map[model.Column]lipgloss.TerminalColor{
		model.ColDraft:            dim,
		model.ColNeedsReview:      accent,
		model.ColChangesRequested: magenta,
		model.ColCIRunning:        yellow,
		model.ColReadyToMerge:     green,
		model.ColBlocked:          red,
	}
	return t
}

func (t Theme) ColumnStyle(c model.Column) lipgloss.Style {
	return t.ColumnHeader.Copy().Foreground(t.columnColor[c])
}

func (t Theme) CheckStyle(s model.CheckState) lipgloss.Style {
	switch s {
	case model.CheckPassed:
		return t.Passed
	case model.CheckFailed:
		return t.Failed
	case model.CheckRunning:
		return t.Running
	default:
		return t.Dim
	}
}

func (t Theme) CheckIcon(s model.CheckState) string {
	switch s {
	case model.CheckPassed:
		return t.Icons.Passed
	case model.CheckFailed:
		return t.Icons.Failed
	case model.CheckRunning:
		return t.Icons.Running
	default:
		return t.Icons.Neutral
	}
}

func unicodeCapable() bool {
	for _, k := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		if v := os.Getenv(k); v != "" {
			return strings.Contains(strings.ToUpper(v), "UTF")
		}
	}
	return false
}

func (m Model) emptyLabel() string {
	if m.theme.Icons.ASCIIOnly {
		return "-- none --"
	}
	return "— none —"
}

func (m Model) joinKeys(keys ...string) string {
	return strings.Join(keys, " "+m.theme.Icons.Dot+" ")
}
