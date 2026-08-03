package ui

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"

	"github.com/dreuse/prdash/internal/model"
)

type Glyphs struct {
	Pass      string
	Fail      string
	Running   string
	Pending   string
	Conflict  string
	Behind    string
	Selected  string
	Expand    string
	LaneRule  string
	HRule     string
	Dot       string
	Arrow     string
	Refresh   string
	Stale     string
	Enter     string
	UpDown    string
	LeftRight string
	Spinner   []string
	Sparkline []string
	ASCII     bool
}

var unicodeGlyphs = Glyphs{
	Pass:      "✓",
	Fail:      "✗",
	Running:   "◐",
	Pending:   "◌",
	Conflict:  "⚠",
	Behind:    "↓",
	Selected:  "▸",
	Expand:    "▾",
	LaneRule:  "▌",
	HRule:     "─",
	Dot:       "·",
	Arrow:     "→",
	Refresh:   "⟳",
	Stale:     "⚠",
	Enter:     "⏎",
	UpDown:    "↑↓",
	LeftRight: "←→",
	Spinner:   []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
	Sparkline: []string{"▁", "▂", "▃", "▅", "█"},
}

var asciiGlyphs = Glyphs{
	Pass:      "+",
	Fail:      "x",
	Running:   "*",
	Pending:   "o",
	Conflict:  "!",
	Behind:    "v",
	Selected:  ">",
	Expand:    "v",
	LaneRule:  "|",
	HRule:     "-",
	Dot:       "-",
	Arrow:     "->",
	Refresh:   "@",
	Stale:     "!",
	Enter:     "ret",
	UpDown:    "jk",
	LeftRight: "hl",
	Spinner:   []string{"|", "/", "-", "\\"},
	Sparkline: []string{".", ":", "-", "=", "#"},
	ASCII:     true,
}

type tone struct{ dark, light string }

func (t tone) adaptive() lipgloss.AdaptiveColor {
	return lipgloss.AdaptiveColor{Dark: t.dark, Light: t.light}
}

func (t tone) blend(bg tone, ratio float64) tone {
	return tone{dark: blendHex(t.dark, bg.dark, ratio), light: blendHex(t.light, bg.light, ratio)}
}

var (
	toneBg         = tone{"#0B0C0E", "#FBFBFA"}
	toneBgChrome   = tone{"#101216", "#F1F1EF"}
	toneBgSelected = tone{"#141A1D", "#E6EFF2"}
	toneTextStrong = tone{"#EDF1F6", "#16181C"}
	toneText       = tone{"#C7CDD6", "#33373E"}
	toneTextDim    = tone{"#8A929D", "#5C6570"}
	toneTextFaint  = tone{"#4E5561", "#8B939D"}
	toneRule       = tone{"#1E2126", "#DDDDD8"}
	toneAccent     = tone{"#59B6C8", "#2A7C8C"}
	toneOK         = tone{"#5EC98D", "#1E7F4E"}
	toneWarn       = tone{"#E2B15A", "#8A6314"}
	toneDanger     = tone{"#E4686B", "#B3373A"}
	toneReview     = tone{"#A98BE8", "#6B4BB0"}

	toneFailPanel = tone{"#16100F", "#FDF2F2"}
)

const laneRuleBlend = 0.45

type Theme struct {
	Glyphs  Glyphs
	NoColor bool

	Strong lipgloss.Style
	Body   lipgloss.Style
	Dim    lipgloss.Style
	Faint  lipgloss.Style
	Rule   lipgloss.Style

	Accent lipgloss.Style
	OK     lipgloss.Style
	Warn   lipgloss.Style
	Danger lipgloss.Style
	Review lipgloss.Style

	Chrome      lipgloss.Style
	ChromeDim   lipgloss.Style
	ChromeFaint lipgloss.Style
	Brand       lipgloss.Style
	TabActive   lipgloss.Style
	TabIdle     lipgloss.Style

	Selected      lipgloss.Style
	SelectedTitle lipgloss.Style
	Drafted       lipgloss.Style

	Panel      lipgloss.Style
	FailPanel  lipgloss.Style
	ChipFilled lipgloss.Style
	Chip       lipgloss.Style
	Overlay    lipgloss.Style
}

func NewTheme(mode string, ascii bool) Theme {
	t := Theme{Glyphs: unicodeGlyphs}
	if ascii || !unicodeCapable() {
		t.Glyphs = asciiGlyphs
	}

	switch mode {
	case "dark":
		lipgloss.SetHasDarkBackground(true)
	case "light":
		lipgloss.SetHasDarkBackground(false)
	}
	t.NoColor = mode == "none" || os.Getenv("NO_COLOR") != ""

	fg := func(c tone) lipgloss.Style {
		if t.NoColor {
			return lipgloss.NewStyle()
		}
		return lipgloss.NewStyle().Foreground(c.adaptive())
	}

	t.Strong = fg(toneTextStrong).Bold(true)
	t.Body = fg(toneText)
	t.Dim = fg(toneTextDim)
	t.Faint = fg(toneTextFaint)
	t.Rule = fg(toneRule)

	t.Accent = fg(toneAccent)
	t.OK = fg(toneOK)
	t.Warn = fg(toneWarn)
	t.Danger = fg(toneDanger)
	t.Review = fg(toneReview)

	if t.NoColor {
		t.Strong = lipgloss.NewStyle().Bold(true)
		t.Dim = lipgloss.NewStyle().Faint(true)
		t.Faint = lipgloss.NewStyle().Faint(true)
		t.OK = lipgloss.NewStyle().Bold(true)
		t.Warn = lipgloss.NewStyle().Bold(true)
		t.Danger = lipgloss.NewStyle().Bold(true).Underline(true)
		t.Review = lipgloss.NewStyle().Bold(true)
		t.Accent = lipgloss.NewStyle().Bold(true)
	}

	t.Chrome = t.Body
	t.ChromeDim = t.Dim
	t.ChromeFaint = t.Faint
	t.Brand = t.Accent.Bold(true)
	t.TabIdle = t.Faint
	t.TabActive = t.Strong
	if !t.NoColor {
		t.Chrome = t.Body.Background(toneBgChrome.adaptive())
		t.ChromeDim = t.Dim.Background(toneBgChrome.adaptive())
		t.ChromeFaint = t.Faint.Background(toneBgChrome.adaptive())
		t.Brand = t.Accent.Bold(true).Background(toneBgChrome.adaptive())
		t.TabIdle = t.Faint.Background(toneBgChrome.adaptive())
		t.TabActive = lipgloss.NewStyle().Bold(true).
			Foreground(toneTextStrong.adaptive()).
			Background(toneBgSelected.adaptive())
	}

	t.Selected = lipgloss.NewStyle()
	t.SelectedTitle = lipgloss.NewStyle().Bold(true)
	if t.NoColor {
		t.Selected = lipgloss.NewStyle().Reverse(true)
	} else {
		t.Selected = lipgloss.NewStyle().Background(toneBgSelected.adaptive())
		t.SelectedTitle = lipgloss.NewStyle().Bold(true).
			Foreground(lipgloss.AdaptiveColor{Dark: "#FFFFFF", Light: "#000000"}).
			Background(toneBgSelected.adaptive())
	}
	t.Drafted = t.Faint

	t.Panel = lipgloss.NewStyle()
	t.FailPanel = lipgloss.NewStyle()
	if !t.NoColor {
		t.FailPanel = lipgloss.NewStyle().Background(toneFailPanel.adaptive())
	}
	t.Chip = lipgloss.NewStyle().Padding(0, 1)
	t.ChipFilled = lipgloss.NewStyle().Padding(0, 1).Bold(true)
	if t.NoColor {
		t.Chip = t.Chip.Faint(true)
		t.ChipFilled = t.ChipFilled.Reverse(true)
	} else {
		t.Chip = t.Chip.Foreground(toneText.adaptive()).Background(toneBgSelected.adaptive())
		t.ChipFilled = t.ChipFilled.Foreground(toneBg.adaptive()).Background(toneOK.adaptive())
	}

	border := lipgloss.RoundedBorder()
	if t.Glyphs.ASCII {
		border = lipgloss.ASCIIBorder()
	}
	t.Overlay = lipgloss.NewStyle().Border(border).Padding(0, 2)
	if !t.NoColor {
		t.Overlay = t.Overlay.BorderForeground(toneRule.adaptive()).Background(toneBgChrome.adaptive())
	}

	return t
}

var lanePalette = []tone{toneOK, toneAccent, toneReview, toneWarn, toneDanger, toneTextFaint}

var namedLaneTones = map[string]tone{
	"ready-to-merge":    toneOK,
	"needs-review":      toneAccent,
	"changes-requested": toneReview,
	"ci-running":        toneWarn,
	"blocked":           toneDanger,
	"draft":             toneTextFaint,
}

var laneColors = []string{"", "green", "cyan", "purple", "amber", "red", "grey"}

var laneColorTones = map[string]tone{
	"green":  toneOK,
	"cyan":   toneAccent,
	"purple": toneReview,
	"amber":  toneWarn,
	"red":    toneDanger,
	"grey":   toneTextFaint,
}

func laneTone(c model.Column) tone {
	if t, ok := laneColorTones[c.Def().Color]; ok {
		return t
	}
	if t, ok := namedLaneTones[c.Slug()]; ok {
		return t
	}
	if int(c) < 0 {
		return toneAccent
	}
	return lanePalette[int(c)%len(lanePalette)]
}

func (t Theme) LaneHeader(c model.Column) lipgloss.Style {
	if t.NoColor {
		return lipgloss.NewStyle().Bold(true)
	}
	return lipgloss.NewStyle().Bold(true).Foreground(laneTone(c).adaptive())
}

func (t Theme) LaneRule(c model.Column) lipgloss.Style {
	if t.NoColor {
		return lipgloss.NewStyle().Faint(true)
	}
	return lipgloss.NewStyle().Foreground(laneTone(c).blend(toneBg, laneRuleBlend).adaptive())
}

func (t Theme) LaneAccent(c model.Column) lipgloss.Style {
	if t.NoColor {
		return lipgloss.NewStyle()
	}
	return lipgloss.NewStyle().Foreground(laneTone(c).adaptive())
}

func (t Theme) CheckStyle(s model.CheckState) lipgloss.Style {
	switch s {
	case model.CheckFailed:
		return t.Danger
	case model.CheckRunning:
		return t.Warn
	case model.CheckPassed:
		return t.Faint
	}
	return t.Faint
}

func (t Theme) CheckGlyph(s model.CheckState) string {
	switch s {
	case model.CheckPassed:
		return t.Glyphs.Pass
	case model.CheckFailed:
		return t.Glyphs.Fail
	case model.CheckRunning:
		return t.Glyphs.Running
	}
	return t.Glyphs.Pending
}

func (t Theme) BehindStyle(n int) lipgloss.Style {
	switch {
	case n >= 100:
		return t.Danger
	case n >= 20:
		return t.Warn
	}
	return t.Faint
}

func (t Theme) AgeStyle(days int) lipgloss.Style {
	switch {
	case days > 180:
		return t.Danger
	case days >= 30:
		return t.Warn
	}
	return t.Dim
}

func (t Theme) HorizontalRule(width int) string {
	if width <= 0 {
		return ""
	}
	return t.Rule.Render(strings.Repeat(t.Glyphs.HRule, width))
}

func blendHex(fg, bg string, ratio float64) string {
	a, err1 := colorful.Hex(fg)
	b, err2 := colorful.Hex(bg)
	if err1 != nil || err2 != nil {
		return fg
	}
	return a.BlendRgb(b, ratio).Clamped().Hex()
}

func unicodeCapable() bool {
	for _, k := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		if v := os.Getenv(k); v != "" {
			return strings.Contains(strings.ToUpper(v), "UTF")
		}
	}
	return false
}
