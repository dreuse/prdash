package ui

import (
	"context"
	"errors"
	"math/rand"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dreuse/prdash/internal/config"
	"github.com/dreuse/prdash/internal/github"
	"github.com/dreuse/prdash/internal/model"
	"github.com/dreuse/prdash/internal/readiness"
	"github.com/dreuse/prdash/internal/update"
)

const (
	DefaultRefreshInterval = 30 * time.Second
	unfocusedInterval      = 5 * time.Minute
	rateLimitBackoff       = 60 * time.Second
	fetchTimeout           = 45 * time.Second
	actionTimeout          = 90 * time.Second
	spinnerInterval        = 120 * time.Millisecond
	toastLife              = 4 * time.Second
	saveDebounce           = 300 * time.Millisecond
	tickJitter             = 3 * time.Second
)

type View int

const (
	ViewBoard View = iota
	ViewCI
)

var Views = []View{ViewBoard, ViewCI}

func (v View) String() string {
	if v == ViewCI {
		return "ci"
	}
	return "board"
}

func (v View) Label() string {
	if v == ViewCI {
		return "CI"
	}
	return "Board"
}

func ViewBySlug(s string) View {
	if strings.ToLower(s) == "ci" {
		return ViewCI
	}
	return ViewBoard
}

type overlayKind int

const (
	ovSettings overlayKind = iota
	ovHelp
	ovConfirm
	ovRepo
)

type toastKind int

const (
	toastInfo toastKind = iota
	toastGood
	toastBad
)

type toast struct {
	text string
	kind toastKind
	gen  int
}

type confirmState struct {
	title  string
	body   string
	verb   string
	danger bool
	run    func(Model) tea.Cmd
}

type Source func(repos []string) (github.Fetcher, github.Actor, error)

type Options struct {
	Fetcher   github.Fetcher
	Actor     github.Actor
	NewSource Source
	Settings  config.Settings
	State     config.State
	Repos     []string
	View      string
	Cache     config.Cache
	HasCache  bool
}

type Model struct {
	fetcher github.Fetcher
	actor   github.Actor
	source  Source
	policy  readiness.Policy
	theme   Theme
	keys    KeyMap

	settings config.Settings
	state    config.State
	repos    []string

	width, height int
	view          View
	stack         []overlayKind

	prs      []model.PullRequest
	byBranch map[string]model.PullRequest
	runs     []model.WorkflowRun
	issues   []model.Issue
	people   []model.User
	viewer   string
	emoji    EmojiSet

	version    string
	newVersion string

	order   []model.Column
	lanes   map[model.Column][]model.PullRequest
	laneIdx int
	sel     model.Key
	split   bool
	multi   bool

	scope     string
	sortMode  model.SortMode
	filter    model.Filter
	filterBar filterBar
	comment   commentBar

	expanded map[string]bool

	ciRow          int
	ciSel          int64
	ciFailuresOnly bool
	ciCache        []model.WorkflowRun
	logs           logPane

	confirm  confirmState
	panel    settingsUI
	repoPick repoPicker

	pending     map[model.Key]string
	toast       toast
	toastGen    int
	spinnerStep int
	spinnerOn   bool
	loading     bool
	loadedOnce  bool
	stale       bool
	focused     bool
	err         error
	rateLimited bool
	lastUpdate  time.Time
	tickGen     int
	saveGen     int
	emojiFresh  bool
}

func New(o Options) Model {
	s := o.Settings
	m := Model{
		fetcher:   o.Fetcher,
		actor:     o.Actor,
		source:    o.NewSource,
		policy:    readiness.Policy{RequiredApprovals: s.RequiredApprovals, BehindBlocks: s.BehindBlocks},
		theme:     NewTheme(s.Theme, s.ASCII),
		keys:      DefaultKeyMap(),
		settings:  s,
		state:     o.State,
		repos:     o.Repos,
		expanded:  map[string]bool{},
		pending:   map[model.Key]string{},
		logs:      logPane{cache: map[logKey][]string{}, order: []logKey{}},
		loading:   true,
		spinnerOn: true,
		focused:   true,
		version:   update.Current(),
		lanes:     map[model.Column][]model.PullRequest{},
	}

	m.view = ViewBySlug(s.DefaultView)
	if s.RememberLastView && o.State.LastView != "" {
		m.view = ViewBySlug(o.State.LastView)
	}
	if o.View != "" {
		m.view = ViewBySlug(o.View)
	}

	m.ciFailuresOnly = s.CIFailuresOnly
	m.sortMode, _ = model.SortModeBySlug(s.Sort)
	if mode, ok := model.SortModeBySlug(o.State.Sort); ok {
		m.sortMode = mode
	}

	raw := s.StartupFilter
	if o.State.Filter != "" {
		raw = o.State.Filter
	}
	m.filter = model.ParseFilter(raw)

	m.filterBar = newFilterBar(m.theme)
	m.filterBar.input.SetValue(raw)

	m.scope = o.State.Scope
	if o.State.SelectPR != 0 {
		m.sel = model.Key{Repo: o.State.SelectRepo, Number: o.State.SelectPR}
	}

	if o.HasCache {
		m.prs = o.Cache.PullRequests
		m.runs = o.Cache.Runs
		m.issues = o.Cache.Issues
		m.people = o.Cache.People
		m.viewer = o.Cache.Viewer
		m.lastUpdate = o.Cache.FetchedAt
		m.loadedOnce = true
		m.stale = true
	}
	m.panel = newSettingsUI()
	m.comment = newCommentBar(m.theme)

	m.emoji = NewEmojiSet()
	if cached, ok := config.LoadEmoji(); ok {
		m.emoji = NewEmojiSet(cached.Emoji)
		m.emojiFresh = !cached.Stale()
	}
	m.rebuild()
	return m
}

type dataMsg struct {
	snapshot github.Snapshot
	err      error
}

type tickMsg struct{ gen int }

type spinnerMsg struct{}

type toastMsg struct {
	text string
	kind toastKind
}

type clearToastMsg struct{ gen int }

type actionMsg struct {
	key  model.Key
	verb string
	err  error
	pr   model.PullRequest
}

type persistMsg struct{ gen int }

type resetSettingsMsg struct{}

type emojiMsg struct{ set map[string]string }

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.fetchCmd(), m.scheduleTick(), spinnerTick(), checkUpdateCmd()}
	if !m.emojiFresh {
		cmds = append(cmds, m.fetchEmojiCmd())
	}
	return tea.Batch(cmds...)
}

func (m Model) fetchEmojiCmd() tea.Cmd {
	source, ok := m.fetcher.(interface {
		Emoji(context.Context) (map[string]string, error)
	})
	if !ok {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		set, err := source.Emoji(ctx)
		if err != nil {
			return nil
		}
		return emojiMsg{set: set}
	}
}

func (m Model) fetchCmd() tea.Cmd {
	f := m.fetcher
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		snap, err := f.Fetch(ctx)
		return dataMsg{snapshot: snap, err: err}
	}
}

func (m Model) baseInterval() time.Duration {
	d := m.settings.Interval()
	if d <= 0 {
		d = DefaultRefreshInterval
	}
	return d
}

func (m Model) interval() time.Duration {
	d := m.baseInterval()
	if !m.focused && !m.settings.NotifiesAnything() {
		d = unfocusedInterval
	}
	if m.rateLimited && d < rateLimitBackoff {
		d = rateLimitBackoff
	}
	return d + time.Duration(rand.Int63n(int64(tickJitter)))
}

func (m Model) dueForRefresh() bool {
	return m.lastUpdate.IsZero() || time.Since(m.lastUpdate) >= m.baseInterval()
}

func (m Model) refresh() (tea.Model, tea.Cmd) {
	m.loading = true
	m.tickGen++
	return m, tea.Batch(m.fetchCmd(), m.scheduleTick())
}

func (m Model) scheduleTick() tea.Cmd {
	gen := m.tickGen
	return tea.Tick(m.interval(), func(time.Time) tea.Msg { return tickMsg{gen: gen} })
}

func spinnerTick() tea.Cmd {
	return tea.Tick(spinnerInterval, func(time.Time) tea.Msg { return spinnerMsg{} })
}

func (m Model) needsSpinner() bool {
	return m.loading || m.logs.loading || len(m.pending) > 0
}

func (m *Model) ensureSpinner() tea.Cmd {
	if m.spinnerOn || !m.needsSpinner() {
		return nil
	}
	m.spinnerOn = true
	return spinnerTick()
}

func (m Model) notify(text string, kind toastKind) tea.Cmd {
	return func() tea.Msg { return toastMsg{text: text, kind: kind} }
}

func (m *Model) persist() tea.Cmd {
	m.saveGen++
	gen := m.saveGen
	return tea.Tick(saveDebounce, func(time.Time) tea.Msg { return persistMsg{gen: gen} })
}

func (m Model) writeStores() {
	_ = config.SaveSettings(m.settings)
	st := m.state
	st.LastView = m.view.String()
	st.Filter = m.filter.Raw
	st.Sort = m.sortMode.String()
	st.Scope = m.scope
	st.SelectRepo = m.sel.Repo
	st.SelectPR = m.sel.Number
	_ = config.SaveState(st)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	out, cmd := m.update(msg)
	next, ok := out.(Model)
	if !ok {
		return out, cmd
	}
	if spin := (&next).ensureSpinner(); spin != nil {
		return next, tea.Batch(cmd, spin)
	}
	return next, cmd
}

func (m Model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m.resize()

	case tea.FocusMsg:
		if m.focused {
			return m, nil
		}
		m.focused = true
		if m.dueForRefresh() {
			return m.refresh()
		}
		m.tickGen++
		return m, m.scheduleTick()

	case tea.BlurMsg:
		if !m.focused {
			return m, nil
		}
		m.focused = false
		m.tickGen++
		return m, m.scheduleTick()

	case spinnerMsg:
		m.spinnerStep++
		m.spinnerOn = false
		return m, nil

	case tickMsg:
		if msg.gen != m.tickGen {
			return m, nil
		}
		m.loading = true
		return m, tea.Batch(m.fetchCmd(), m.scheduleTick())

	case persistMsg:
		if msg.gen == m.saveGen {
			m.writeStores()
		}
		return m, nil

	case toastMsg:
		m.toastGen++
		m.toast = toast{text: msg.text, kind: msg.kind, gen: m.toastGen}
		gen := m.toastGen
		return m, tea.Tick(toastLife, func(time.Time) tea.Msg { return clearToastMsg{gen: gen} })

	case clearToastMsg:
		if msg.gen == m.toast.gen {
			m.toast = toast{}
		}
		return m, nil

	case emojiMsg:
		if len(msg.set) == 0 {
			return m, nil
		}
		m.emoji = NewEmojiSet(msg.set)
		m.emojiFresh = true
		return m, saveEmojiCmd(config.EmojiSet{FetchedAt: time.Now(), Emoji: msg.set})

	case updateMsg:
		m.newVersion = msg.latest
		return m, m.notify(m.theme.Glyphs.Arrow+" prdash "+msg.latest+" is out, run `prdash --update`", toastInfo)

	case dataMsg:
		return m.applyData(msg)

	case logLoadMsg:
		return m.applyLogLoad(msg)

	case logsMsg:
		return m.applyLogs(msg)

	case actionMsg:
		return m.applyAction(msg)

	case resetSettingsMsg:
		repos := m.settings.Repos
		m.settings = config.DefaultSettings()
		m.settings.Repos = repos
		m.panel.idx = 0
		mm, cmd := m.applySettings()
		return mm, tea.Batch(cmd, m.notify("settings reset to defaults", toastGood))

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) applyData(msg dataMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	m.err = msg.err
	m.rateLimited = isRateLimit(msg.err)

	if msg.err == nil || len(msg.snapshot.PullRequests) > 0 {
		settledRuns, priorRuns := msg.snapshot.Runs, m.runs
		priorPRs := m.prs
		m.prs = msg.snapshot.PullRequests
		m.runs = settledRuns
		m.issues = msg.snapshot.Issues
		m.people = peopleFrom(msg.snapshot.People)
		if msg.snapshot.Viewer != "" {
			m.viewer = msg.snapshot.Viewer
		}
		m.loadedOnce = true
		m.stale = msg.err != nil
		m.lastUpdate = time.Now()

		if fixed := canonicalRepos(m.settings.Repos, m.prs); !sameStrings(fixed, m.settings.Repos) {
			m.settings.Repos = fixed
			m.repos = fixed
		}
		if scoped := canonicalRepos([]string{m.scope}, m.prs); m.scope != "" && scoped[0] != m.scope {
			m.scope = scoped[0]
		}
		m.rebuild()
		return m, tea.Batch(
			m.announceRuns(priorRuns, settledRuns),
			m.announcePullRequests(priorPRs, m.prs),
			saveCacheCmd(config.Cache{
				FetchedAt:    m.lastUpdate,
				Viewer:       m.viewer,
				PullRequests: append([]model.PullRequest(nil), m.prs...),
				Runs:         append([]model.WorkflowRun(nil), m.runs...),
				Issues:       append([]model.Issue(nil), m.issues...),
				People:       append([]model.User(nil), m.people...),
			}))
	}
	m.stale = true
	if msg.err != nil && m.loadedOnce {
		return m, m.notify(m.theme.Glyphs.Fail+" refresh failed: "+firstLine(msg.err.Error()), toastBad)
	}
	return m, nil
}

func saveCacheCmd(c config.Cache) tea.Cmd {
	return func() tea.Msg {
		_ = config.SaveCache(c)
		return nil
	}
}

func saveEmojiCmd(e config.EmojiSet) tea.Cmd {
	return func() tea.Msg {
		_ = config.SaveEmoji(e)
		return nil
	}
}

func (m Model) resize() (tea.Model, tea.Cmd) {
	m.rebuild()
	return m, nil
}

func (m *Model) rebuild() {
	m.policy = readiness.Policy{
		RequiredApprovals: m.settings.RequiredApprovals,
		BehindBlocks:      m.settings.BehindBlocks,
		Viewer:            m.viewer,
	}
	model.SetLanes(m.settings.LaneDefs())
	m.order = m.laneOrder()
	m.multi = m.countRepos() > 1
	m.ciCache = m.sortedVisibleRuns()
	m.indexBranches()

	visible := make([]model.PullRequest, 0, len(m.prs))
	for _, pr := range m.prs {
		if !m.inScope(pr.Repo) {
			continue
		}
		ctx := model.FilterContext{
			Viewer: m.viewer,
			Column: m.policy.Classify(pr),
			Ready:  m.policy.ReadyToMerge(pr),
		}
		if m.filter.Match(pr, ctx) {
			visible = append(visible, pr)
		}
	}

	m.lanes = m.policy.Group(visible)
	for col := range m.lanes {
		model.Sort(m.lanes[col], m.laneSort(col), m.viewer, m.policy.RequiredApprovals)
	}
	m.clampSelection()
}

func (m Model) scopeLabel() string {
	if m.scope == "" {
		if names := m.repoNames(); len(names) == 1 {
			return shortRepo(names[0])
		}
		return "all repos"
	}
	return shortRepo(m.scope)
}

func (m Model) laneSort(col model.Column) model.SortMode {
	if mode, ok := model.SortModeBySlug(col.Def().Sort); ok {
		return mode
	}
	return m.sortMode
}

func (m Model) laneOrder() []model.Column {
	base := model.ActionFirstColumns
	switch m.settings.LaneOrder {
	case config.LaneOrderPipeline:
		base = model.PipelineColumns
	case config.LaneOrderCustom:
		base = model.AllColumns()
	}
	hidden := map[model.Column]bool{}
	for _, slug := range m.settings.HiddenLanes {
		if c, ok := model.ColumnBySlug(slug); ok {
			hidden[c] = true
		}
	}
	out := make([]model.Column, 0, len(base))
	for _, c := range base {
		if !hidden[c] {
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		return base
	}
	return out
}

func (m *Model) clampSelection() {
	m.laneIdx = clamp(m.laneIdx, 0, maxInt(0, len(m.order)-1))
	if !m.selectionVisible() {
		m.selectFirst()
	}
	m.syncLaneToSelection()
	m.clampCIRow()
}

func (m *Model) clampCIRow() {
	rows := m.ciRows()
	if m.ciSel != 0 {
		for i, r := range rows {
			if r.ID == m.ciSel {
				m.ciRow = i
				break
			}
		}
	}
	m.ciRow = clamp(m.ciRow, 0, maxInt(0, len(rows)-1))
	if m.ciRow < len(rows) {
		m.ciSel = rows[m.ciRow].ID
	}
}

func (m Model) selectionVisible() bool {
	if m.sel.Zero() {
		return false
	}
	for _, prs := range m.lanes {
		for _, pr := range prs {
			if pr.Key() == m.sel {
				return true
			}
		}
	}
	return false
}

func (m *Model) selectFirst() {
	for _, col := range m.order {
		if prs := m.lanes[col]; len(prs) > 0 {
			m.sel = prs[0].Key()
			return
		}
	}
	m.sel = model.Key{}
}

func (m *Model) syncLaneToSelection() {
	if m.sel.Zero() {
		return
	}
	for i, col := range m.order {
		for _, pr := range m.lanes[col] {
			if pr.Key() == m.sel {
				m.laneIdx = i
				return
			}
		}
	}
}

func (m Model) currentLane() []model.PullRequest {
	if len(m.order) == 0 {
		return nil
	}
	return m.lanes[m.order[clamp(m.laneIdx, 0, len(m.order)-1)]]
}

func (m Model) laneRow() int {
	for i, pr := range m.currentLane() {
		if pr.Key() == m.sel {
			return i
		}
	}
	return 0
}

func (m Model) selectedPR() (model.PullRequest, bool) {
	if m.sel.Zero() {
		return model.PullRequest{}, false
	}
	for _, col := range m.order {
		for _, pr := range m.lanes[col] {
			if pr.Key() == m.sel {
				return pr, true
			}
		}
	}
	return model.PullRequest{}, false
}

func branchKey(repo, branch string) string { return repo + "\x00" + branch }

func (m *Model) indexBranches() {
	m.byBranch = make(map[string]model.PullRequest, len(m.prs))
	for _, pr := range m.prs {
		key := branchKey(pr.Repo, pr.HeadRef)
		if _, taken := m.byBranch[key]; !taken {
			m.byBranch[key] = pr
		}
	}
}

func (m Model) prByBranch(repo, branch string) (model.PullRequest, bool) {
	pr, ok := m.byBranch[branchKey(repo, branch)]
	return pr, ok
}

func (m Model) countRepos() int {
	seen := map[string]struct{}{}
	for _, pr := range m.prs {
		seen[pr.Repo] = struct{}{}
		if len(seen) > 1 {
			return len(seen)
		}
	}
	return len(seen)
}

func (m Model) repoNames() []string {
	seen := map[string]int{}
	var out []string
	add := func(name string) {
		if name == "" {
			return
		}
		key := strings.ToLower(name)
		if i, ok := seen[key]; ok {
			out[i] = name
			return
		}
		seen[key] = len(out)
		out = append(out, name)
	}
	for _, r := range m.repos {
		add(r)
	}
	for _, pr := range m.prs {
		add(pr.Repo)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i]) < strings.ToLower(out[j])
	})
	return out
}

func (m Model) inScope(repo string) bool {
	return m.scope == "" || strings.EqualFold(m.scope, repo)
}

func canonicalRepos(tracked []string, prs []model.PullRequest) []string {
	canonical := make(map[string]string, len(prs))
	for _, pr := range prs {
		canonical[strings.ToLower(pr.Repo)] = pr.Repo
	}

	seen := make(map[string]bool, len(tracked))
	out := make([]string, 0, len(tracked))
	for _, name := range tracked {
		key := strings.ToLower(name)
		if seen[key] {
			continue
		}
		seen[key] = true
		if fixed, ok := canonical[key]; ok {
			name = fixed
		}
		out = append(out, name)
	}
	return out
}

func (m Model) overlay() (overlayKind, bool) {
	if len(m.stack) == 0 {
		return 0, false
	}
	return m.stack[len(m.stack)-1], true
}

func (m *Model) push(o overlayKind) {
	for _, existing := range m.stack {
		if existing == o {
			return
		}
	}
	m.stack = append(m.stack, o)
}

func (m *Model) pop() {
	if len(m.stack) > 0 {
		m.stack = m.stack[:len(m.stack)-1]
	}
}

func (m Model) spinnerFrame() string {
	frames := m.theme.Glyphs.Spinner
	return frames[m.spinnerStep%len(frames)]
}

func isRateLimit(err error) bool {
	var ghErr *github.Error
	return errors.As(err, &ghErr) && ghErr.RateLimit
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
