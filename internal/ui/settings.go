package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/dreuse/prdash/internal/config"
	"github.com/dreuse/prdash/internal/github"
	"github.com/dreuse/prdash/internal/model"
	"github.com/dreuse/prdash/internal/readiness"
)

const (
	settingsWidth = 60
	labelWidth    = 24
)

type fieldKind int

const (
	fieldCycle fieldKind = iota
	fieldToggle
	fieldText
	fieldRepo
	fieldLane
)

const (
	sectionBoard  = "BOARD"
	sectionRepos  = "REPOSITORIES"
	laneSeparator = "|"
)

type settingsField struct {
	section string
	label   string
	desc    string
	kind    fieldKind
	values  []string
	get     func(config.Settings) string
	set     func(*config.Settings, string)
	repo    string
	lane    int
}

type settingsUI struct {
	idx     int
	editing bool
	input   textinput.Model
}

func newSettingsUI() settingsUI {
	in := textinput.New()
	in.Prompt = ""
	return settingsUI{input: in}
}

func (m Model) settingsFields() []settingsField {
	fields := []settingsField{
		{section: "GENERAL", label: "Default view", desc: "the view that opens on start",
			kind: fieldCycle, values: []string{"board", "ci"},
			get: func(s config.Settings) string { return s.DefaultView },
			set: func(s *config.Settings, v string) { s.DefaultView = v }},
		{section: "GENERAL", label: "Reopen last view", desc: "start in whatever view you quit from",
			kind: fieldToggle,
			get:  func(s config.Settings) string { return onOff(s.RememberLastView) },
			set:  func(s *config.Settings, v string) { s.RememberLastView = v == "on" }},
		{section: "GENERAL", label: "Refresh every", desc: "poll interval while the terminal is focused",
			kind: fieldCycle, values: []string{"15s", "30s", "60s", "300s"},
			get: func(s config.Settings) string { return itoa(s.RefreshSeconds) + "s" },
			set: func(s *config.Settings, v string) { s.RefreshSeconds = atoiSuffix(v) }},
		{section: "GENERAL", label: "Theme", desc: "auto follows the terminal background",
			kind: fieldCycle, values: []string{"auto", "dark", "light", "none"},
			get: func(s config.Settings) string { return s.Theme },
			set: func(s *config.Settings, v string) { s.Theme = v }},
		{section: "GENERAL", label: "ASCII glyphs", desc: "for terminals without box drawing characters",
			kind: fieldToggle,
			get:  func(s config.Settings) string { return onOff(s.ASCII) },
			set:  func(s *config.Settings, v string) { s.ASCII = v == "on" }},
	}

	for _, repo := range m.settings.Repos {
		fields = append(fields, settingsField{
			section: sectionRepos, label: repo, kind: fieldRepo, repo: repo,
			desc: "d removes it", get: func(config.Settings) string { return "" }})
	}
	fields = append(fields, settingsField{
		section: sectionRepos, label: "add repository…", kind: fieldText,
		desc: "owner/name",
		get:  func(config.Settings) string { return "" },
		set: func(s *config.Settings, v string) {
			v = strings.TrimSpace(v)
			if _, err := github.ParseRepo(v); err != nil {
				return
			}
			s.Repos = appendFold(s.Repos, v)
		}})

	fields = append(fields,
		settingsField{section: sectionBoard, label: "Lane order", desc: "ready first, the order work moves through, or your own lanes",
			kind: fieldCycle, values: []string{config.LaneOrderReady, config.LaneOrderPipeline, config.LaneOrderCustom},
			get: func(s config.Settings) string { return s.LaneOrder },
			set: func(s *config.Settings, v string) { s.LaneOrder = v }})

	if m.settings.LaneOrder == config.LaneOrderCustom {
		fields = append(fields, m.laneFields()...)
	} else {
		fields = append(fields, settingsField{
			section: sectionBoard, label: "Hidden lanes", desc: "comma separated: " + strings.Join(laneSlugList(), ", "),
			kind: fieldText,
			get:  func(s config.Settings) string { return joinOr(s.HiddenLanes, "none") },
			set:  func(s *config.Settings, v string) { s.HiddenLanes = splitList(v) }})
	}

	fields = append(fields,
		settingsField{section: "CI", label: "Runs window", desc: "how many runs the health strip summarises",
			kind: fieldCycle, values: []string{"10", "20", "50", "100"},
			get: func(s config.Settings) string { return itoa(s.CIRunsWindow) },
			set: func(s *config.Settings, v string) { s.CIRunsWindow = atoiSuffix(v) }},
		settingsField{section: "CI", label: "Recent window", desc: "how many hours a finished run stays in the table",
			kind: fieldText,
			get:  func(s config.Settings) string { return itoa(s.CIRecentHours) + "h" },
			set:  func(s *config.Settings, v string) { s.CIRecentHours = atoiSuffix(v) }},
		settingsField{section: "CI", label: "Failures only", desc: "hide successful runs by default",
			kind: fieldToggle,
			get:  func(s config.Settings) string { return onOff(s.CIFailuresOnly) },
			set:  func(s *config.Settings, v string) { s.CIFailuresOnly = v == "on" }},
		settingsField{section: "NOTIFICATIONS", label: "Scope", desc: "any pull request, mine adds the ones assigned to you, authored is only the ones you opened",
			kind: fieldCycle, values: []string{config.ScopeAny, config.ScopeMine, config.ScopeAuthored},
			get: func(s config.Settings) string { return s.NotifyScope },
			set: func(s *config.Settings, v string) { s.NotifyScope = v }},
		settingsField{section: "NOTIFICATIONS", label: "Runs that finish", desc: "desktop alert when a workflow run lands",
			kind: fieldCycle, values: []string{config.NotifyOff, config.NotifyFailures, config.NotifyAll},
			get: func(s config.Settings) string { return s.Notify },
			set: func(s *config.Settings, v string) { s.Notify = v }},
		settingsField{section: "NOTIFICATIONS", label: "Reviews", desc: "someone approves or requests changes",
			kind: fieldToggle,
			get:  func(s config.Settings) string { return onOff(s.NotifyReviews) },
			set:  func(s *config.Settings, v string) { s.NotifyReviews = v == "on" }},
		settingsField{section: "NOTIFICATIONS", label: "Ready to merge", desc: "a pull request clears every check and approval",
			kind: fieldToggle,
			get:  func(s config.Settings) string { return onOff(s.NotifyReady) },
			set:  func(s *config.Settings, v string) { s.NotifyReady = v == "on" }},
		settingsField{section: "NOTIFICATIONS", label: "Handed to you", desc: "you are assigned or your review is requested, whatever the scope",
			kind: fieldToggle,
			get:  func(s config.Settings) string { return onOff(s.NotifyAssigned) },
			set:  func(s *config.Settings, v string) { s.NotifyAssigned = v == "on" }},

		settingsField{section: "DEFAULTS", label: "Sort", desc: "default ordering inside a lane or group",
			kind: fieldCycle, values: []string{"urgency", "updated", "age", "number"},
			get: func(s config.Settings) string { return s.Sort },
			set: func(s *config.Settings, v string) { s.Sort = v }},
		settingsField{section: "DEFAULTS", label: "Startup filter", desc: "applied on every start",
			kind: fieldText,
			get:  func(s config.Settings) string { return joinOr(splitSpace(s.StartupFilter), "none") },
			set:  func(s *config.Settings, v string) { s.StartupFilter = strings.TrimSpace(v) }},
		settingsField{section: "DEFAULTS", label: "Required approvals", desc: "approvals before a PR counts as ready",
			kind: fieldCycle, values: []string{"0", "1", "2", "3"},
			get: func(s config.Settings) string { return itoa(s.RequiredApprovals) },
			set: func(s *config.Settings, v string) { s.RequiredApprovals = atoiSuffix(v) }},
		settingsField{section: "DEFAULTS", label: "Behind blocks", desc: "a PR behind its base branch is not ready to merge",
			kind: fieldToggle,
			get:  func(s config.Settings) string { return onOff(s.BehindBlocks) },
			set:  func(s *config.Settings, v string) { s.BehindBlocks = v == "on" }},
	)
	return fields
}

func (m Model) laneFields() []settingsField {
	fields := make([]settingsField, 0, len(m.settings.Lanes)+1)
	for i := range m.settings.Lanes {
		i := i
		fields = append(fields, settingsField{
			section: sectionBoard, label: m.settings.Lanes[i].Name, kind: fieldLane, lane: i,
			desc: laneDesc(m.settings.Lanes[i].Rule),
			get: func(s config.Settings) string {
				if i >= len(s.Lanes) {
					return ""
				}
				return s.Lanes[i].Rule
			},
			set: func(s *config.Settings, v string) {
				if i < len(s.Lanes) {
					s.Lanes[i].Rule = strings.TrimSpace(v)
				}
			}})
	}
	return append(fields, settingsField{
		section: sectionBoard, label: "add lane…", kind: fieldText,
		desc: "NAME " + laneSeparator + " rule, e.g. MERGE NOW " + laneSeparator + " is:ready",
		get:  func(config.Settings) string { return "" },
		set: func(s *config.Settings, v string) {
			name, rule, ok := strings.Cut(v, laneSeparator)
			name, rule = strings.TrimSpace(name), strings.TrimSpace(rule)
			if !ok || name == "" || rule == "" {
				return
			}
			s.Lanes = append(s.Lanes, config.Lane{Name: name, Rule: rule})
		}})
}

func (m Model) handleSettingsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	fields := m.settingsFields()
	if m.panel.editing {
		return m.handleSettingsEdit(msg, fields)
	}

	switch msg.String() {
	case "esc", ",", "S":
		m.pop()
		return m, m.persist()
	case "j", "down":
		m.panel.idx = clamp(m.panel.idx+1, 0, len(fields)-1)
		return m, nil
	case "k", "up":
		m.panel.idx = clamp(m.panel.idx-1, 0, len(fields)-1)
		return m, nil
	case "a":
		return m.focusAdd(fields)
	case "d":
		return m.removeField(fields)
	case "J":
		return m.moveLaneField(fields, 1)
	case "K":
		return m.moveLaneField(fields, -1)
	case "c":
		return m.cycleLaneAttr(fields, func(l *config.Lane) {
			l.Color = cycle(laneColors, l.Color, 1)
		})
	case "s":
		return m.cycleLaneAttr(fields, func(l *config.Lane) {
			l.Sort = cycle(laneSorts, l.Sort, 1)
		})
	case "r":
		return m.resetSection(fields)
	case "R":
		return m.ask(confirmState{
			title: "Reset every setting?", body: "all preferences go back to their defaults",
			verb: "reset", danger: true,
			run: func(Model) tea.Cmd {
				return func() tea.Msg { return resetSettingsMsg{} }
			},
		})
	case "enter", "right", "l", " ":
		return m.cycleField(fields, 1)
	case "left", "h":
		return m.cycleField(fields, -1)
	}
	return m, nil
}

func (m Model) handleSettingsEdit(msg tea.KeyMsg, fields []settingsField) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.panel.editing = false
		m.panel.input.Blur()
		return m, nil
	case "enter":
		f := fields[clamp(m.panel.idx, 0, len(fields)-1)]
		if f.set != nil {
			f.set(&m.settings, m.panel.input.Value())
		}
		m.panel.editing = false
		m.panel.input.Blur()
		return m.applySettings()
	}
	var cmd tea.Cmd
	m.panel.input, cmd = m.panel.input.Update(msg)
	return m, cmd
}

func (m Model) cycleField(fields []settingsField, delta int) (tea.Model, tea.Cmd) {
	if len(fields) == 0 {
		return m, nil
	}
	f := fields[clamp(m.panel.idx, 0, len(fields)-1)]
	switch f.kind {
	case fieldToggle:
		f.set(&m.settings, toggle(f.get(m.settings)))
		return m.applySettings()
	case fieldCycle:
		f.set(&m.settings, cycle(f.values, f.get(m.settings), delta))
		return m.applySettings()
	case fieldText, fieldLane:
		m.panel.editing = true
		m.panel.input.SetValue(rawValue(f, m.settings))
		m.panel.input.CursorEnd()
		return m, m.panel.input.Focus()
	}
	return m, nil
}

func rawValue(f settingsField, s config.Settings) string {
	if f.kind == fieldLane {
		return f.get(s)
	}
	switch f.label {
	case "Hidden lanes":
		return strings.Join(s.HiddenLanes, ",")
	case "Startup filter":
		return s.StartupFilter
	}
	return ""
}

func (m Model) focusAdd(fields []settingsField) (tea.Model, tea.Cmd) {
	section := fields[clamp(m.panel.idx, 0, len(fields)-1)].section
	if section != sectionBoard {
		section = sectionRepos
	}
	for i, f := range fields {
		if f.kind == fieldText && f.section == section {
			m.panel.idx = i
			return m.cycleField(fields, 1)
		}
	}
	return m, nil
}

func (m Model) removeField(fields []settingsField) (tea.Model, tea.Cmd) {
	f := fields[clamp(m.panel.idx, 0, len(fields)-1)]
	switch f.kind {
	case fieldRepo:
		kept := make([]string, 0, len(m.settings.Repos))
		for _, r := range m.settings.Repos {
			if !strings.EqualFold(r, f.repo) {
				kept = append(kept, r)
			}
		}
		m.settings.Repos = kept
	case fieldLane:
		if f.lane >= len(m.settings.Lanes) {
			return m, nil
		}
		kept := make([]config.Lane, 0, len(m.settings.Lanes)-1)
		kept = append(kept, m.settings.Lanes[:f.lane]...)
		kept = append(kept, m.settings.Lanes[f.lane+1:]...)
		m.settings.Lanes = kept
		m.panel.idx = maxInt(0, m.panel.idx-1)
	default:
		return m, nil
	}
	return m.applySettings()
}

func (m Model) cycleLaneAttr(fields []settingsField, apply func(*config.Lane)) (tea.Model, tea.Cmd) {
	f := fields[clamp(m.panel.idx, 0, len(fields)-1)]
	if f.kind != fieldLane || f.lane >= len(m.settings.Lanes) {
		return m, nil
	}
	lanes := make([]config.Lane, len(m.settings.Lanes))
	copy(lanes, m.settings.Lanes)
	apply(&lanes[f.lane])
	m.settings.Lanes = lanes
	return m.applySettings()
}

func (m Model) moveLaneField(fields []settingsField, delta int) (tea.Model, tea.Cmd) {
	f := fields[clamp(m.panel.idx, 0, len(fields)-1)]
	to := f.lane + delta
	if f.kind != fieldLane || to < 0 || to >= len(m.settings.Lanes) {
		return m, nil
	}
	lanes := make([]config.Lane, len(m.settings.Lanes))
	copy(lanes, m.settings.Lanes)
	lanes[f.lane], lanes[to] = lanes[to], lanes[f.lane]
	m.settings.Lanes = lanes
	m.panel.idx += delta
	return m.applySettings()
}

func (m Model) resetSection(fields []settingsField) (tea.Model, tea.Cmd) {
	if len(fields) == 0 {
		return m, nil
	}
	section := fields[clamp(m.panel.idx, 0, len(fields)-1)].section
	defaults := config.DefaultSettings()
	for _, f := range fields {
		if f.section == section && f.set != nil && f.kind != fieldRepo && f.kind != fieldText && f.kind != fieldLane {
			f.set(&m.settings, f.get(defaults))
		}
	}
	return m.applySettings()
}

func (m Model) applySettings() (tea.Model, tea.Cmd) {
	m.theme = NewTheme(m.settings.Theme, m.settings.ASCII)
	if mode, ok := model.SortModeBySlug(m.settings.Sort); ok {
		m.sortMode = mode
	}
	m.rebuild()

	var cmds []tea.Cmd
	if !sameStrings(m.repos, m.settings.Repos) && m.source != nil {
		fetcher, actor, err := m.source(m.settings.Repos)
		if err != nil {
			m.settings.Repos = m.repos
			m.rebuild()
			return m, m.notify(firstLine(err.Error()), toastBad)
		}
		m.fetcher, m.actor, m.repos = fetcher, actor, m.settings.Repos
		m.loading = true
		m.tickGen++
		cmds = append(cmds, m.fetchCmd(), m.scheduleTick())
	}
	return m, tea.Batch(append(cmds, m.persist())...)
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (m Model) renderSettings() string {
	t := m.theme
	fields := m.settingsFields()
	width := panelInner(settingsWidth, m.width)

	var b strings.Builder
	b.WriteString(t.Strong.Render("Settings"))
	b.WriteString("\n")

	section := ""
	for i, f := range fields {
		if f.section != section {
			section = f.section
			b.WriteString("\n" + t.Faint.Render(section))
			if hint := m.sectionHint(section); hint != "" {
				b.WriteString(t.Faint.Render(padLeft(hint, maxInt(1, width-textWidth(section)-1))))
			}
			b.WriteString("\n")
		}

		focused := i == m.panel.idx
		text := f.label
		switch f.kind {
		case fieldRepo:
			text = t.Glyphs.Pass + " " + f.label
		case fieldLane:
			text = t.Glyphs.LaneRule + " " + f.label
		}
		label := pad("  "+text, labelWidth)
		labelStyle := t.Body
		if focused {
			labelStyle = t.SelectedTitle
			label = pad(" "+t.Glyphs.Selected+" "+text, labelWidth)
		}

		value := m.renderFieldValue(f, focused, width-labelWidth)
		line := labelStyle.Render(label) + value
		if focused {
			line = t.Selected.Render(fillLine(line, width))
		}
		b.WriteString(line + "\n")
		if focused && f.desc != "" {
			descStyle := t.Faint
			if f.kind == fieldLane && !readiness.ValidLaneRule(m.settings.Lanes[f.lane].Rule) {
				descStyle = t.Danger
			}
			b.WriteString(descStyle.Render(truncate("    "+f.desc, width)) + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(t.Faint.Render(truncate(t.Glyphs.Enter+" toggle/edit   "+t.Glyphs.LeftRight+" cycle   j/k move", width)) + "\n")
	b.WriteString(t.Faint.Render(truncate("r reset section   R reset all   esc close", width)))
	return t.Overlay.Width(width + overlayText).Render(b.String())
}

func (m Model) renderFieldValue(f settingsField, focused bool, width int) string {
	t := m.theme
	if focused && m.panel.editing {
		return m.panel.input.View()
	}
	switch f.kind {
	case fieldToggle:
		on := f.get(m.settings) == "on"
		box := "[ ] off"
		style := t.Faint
		if on {
			box, style = "[x] on", t.OK
		}
		return style.Render(box)
	case fieldCycle:
		return t.Faint.Render("‹ ") + t.Warn.Render(f.get(m.settings)) + t.Faint.Render(" ›")
	case fieldText:
		v := f.get(m.settings)
		if v == "" {
			return t.Faint.Render(t.Glyphs.Enter + " edit")
		}
		return t.Dim.Render(truncate(v, maxInt(1, width))) + t.Faint.Render("  "+t.Glyphs.Enter+" edit")
	case fieldLane:
		return m.renderLaneValue(f, width)
	}
	return ""
}

func (m Model) renderLaneValue(f settingsField, width int) string {
	t := m.theme
	lane := m.settings.Lanes[f.lane]

	swatch := " " + t.Glyphs.LaneRule
	plainTail, tail := swatch, t.LaneAccent(model.Column(f.lane)).Render(swatch)
	if lane.Sort != "" {
		plainTail += " " + lane.Sort
		tail += t.Warn.Render(" " + lane.Sort)
	}

	style := t.Dim
	if !readiness.ValidLaneRule(lane.Rule) {
		style = t.Danger
	}
	room := maxInt(1, width-textWidth(plainTail))
	return style.Render(truncate(lane.Rule, room)) + tail
}

func laneDesc(rule string) string {
	if err := readiness.LaneRuleError(rule); err != "" {
		return err
	}
	return "enter edits the rule"
}

var laneSorts = laneSortValues()

func laneSortValues() []string {
	out := make([]string, 0, len(model.SortModes)+1)
	out = append(out, "")
	for _, mode := range model.SortModes {
		out = append(out, mode.String())
	}
	return out
}

func (m Model) sectionHint(section string) string {
	switch section {
	case sectionRepos:
		return "a add " + m.theme.Glyphs.Dot + " d remove"
	case sectionBoard:
		if m.settings.LaneOrder == config.LaneOrderCustom {
			dot := " " + m.theme.Glyphs.Dot + " "
			return "c colour" + dot + "s sort" + dot + "J/K move" + dot + "d remove"
		}
	}
	return ""
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

func toggle(v string) string {
	if v == "on" {
		return "off"
	}
	return "on"
}

func cycle(values []string, current string, delta int) string {
	if len(values) == 0 {
		return current
	}
	for i, v := range values {
		if v == current {
			return values[wrapIndex(i+delta, len(values))]
		}
	}
	return values[0]
}

func atoiSuffix(v string) int {
	n := 0
	for _, r := range v {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
	}
	return n
}

func joinOr(list []string, empty string) string {
	if len(list) == 0 {
		return empty
	}
	return strings.Join(list, ", ")
}

func splitList(v string) []string {
	var out []string
	for _, part := range strings.Split(v, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func splitSpace(v string) []string { return strings.Fields(v) }

func laneSlugList() []string {
	cols := model.AllColumns()
	out := make([]string, 0, len(cols))
	for _, c := range cols {
		out = append(out, c.Slug())
	}
	return out
}
