package ui

import (
	"sort"
	"strings"

	"github.com/dreuse/prdash/internal/model"
)

const (
	triggerEmoji     = ':'
	triggerReference = '#'
	triggerMention   = '@'
	maxReferences    = 30
	maxMentions      = 30
)

type candidate struct {
	insert   string
	complete string
	label    string
	detail   string
}

func openToken(text string) (trigger byte, prefix string, ok bool) {
	i := strings.LastIndexAny(text, ":#@")
	if i < 0 {
		return 0, "", false
	}
	tail := text[i+1:]
	if strings.ContainsAny(tail, " \t") || strings.Contains(tail, lineBreak) {
		return 0, "", false
	}
	if i > 0 {
		if strings.HasSuffix(text[:i], lineBreak) {
			return text[i], tail, true
		}
		switch text[i-1] {
		case ' ', '\t', '(', '[', '\n':
		default:
			return 0, "", false
		}
	}
	if text[i] == triggerEmoji && tail == "" {
		return 0, "", false
	}
	return text[i], tail, true
}

func (m Model) candidates(trigger byte, prefix string) ([]candidate, int) {
	switch trigger {
	case triggerEmoji:
		return m.emojiCandidates(prefix)
	case triggerReference:
		return m.referenceCandidates(prefix)
	case triggerMention:
		return m.mentionCandidates(prefix)
	}
	return nil, 0
}

func (m Model) emojiCandidates(prefix string) ([]candidate, int) {
	names, total := m.emoji.Complete(prefix)
	out := make([]candidate, 0, len(names))
	for _, name := range names {
		label := ":" + name + ":"
		glyph, ok := m.emoji.Glyph(name)
		if ok && !m.theme.Glyphs.ASCII {
			label = glyph + " " + label
		}
		out = append(out, candidate{
			insert:   m.emoji.Render(name, m.theme.Glyphs.ASCII),
			complete: ":" + name + ":",
			label:    label,
		})
	}
	return out, total
}

func (m Model) referenceCandidates(prefix string) ([]candidate, int) {
	prefix = strings.ToLower(prefix)
	type ref struct {
		number int
		title  string
	}

	refs := make([]ref, 0, len(m.prs)+len(m.issues))
	for _, pr := range m.prs {
		refs = append(refs, ref{pr.Number, pr.Title})
	}
	for _, is := range m.issues {
		refs = append(refs, ref{is.Number, is.Title})
	}

	matched := make([]ref, 0, len(refs))
	for _, r := range refs {
		number := itoa(r.number)
		if prefix == "" || strings.HasPrefix(number, prefix) ||
			strings.Contains(strings.ToLower(r.title), prefix) {
			matched = append(matched, r)
		}
	}
	sort.SliceStable(matched, func(i, j int) bool {
		a := strings.HasPrefix(itoa(matched[i].number), prefix)
		b := strings.HasPrefix(itoa(matched[j].number), prefix)
		if a != b {
			return a
		}
		return matched[i].number > matched[j].number
	})

	total := len(matched)
	if len(matched) > maxReferences {
		matched = matched[:maxReferences]
	}
	out := make([]candidate, 0, len(matched))
	for _, r := range matched {
		out = append(out, candidate{
			insert:   "#" + itoa(r.number),
			complete: "#" + itoa(r.number),
			label:    "#" + itoa(r.number),
			detail:   r.title,
		})
	}
	return out, total
}

func (m Model) mentionCandidates(prefix string) ([]candidate, int) {
	prefix = strings.ToLower(prefix)

	type person struct {
		login string
		name  string
	}
	seen := make(map[string]person, len(m.people))
	add := func(login, name string) {
		if login == "" {
			return
		}
		key := strings.ToLower(login)
		existing, ok := seen[key]
		if !ok {
			seen[key] = person{login: login, name: name}
			return
		}
		if existing.name == "" && name != "" {
			existing.name = name
			seen[key] = existing
		}
	}
	for _, u := range m.people {
		add(u.Login, u.Name)
	}
	for _, pr := range m.prs {
		add(pr.Author, "")
		for _, r := range pr.RequestedReviewers {
			add(r, "")
		}
		for _, r := range pr.Reviews {
			add(r.Login, "")
		}
	}

	keys := make([]string, 0, len(seen))
	for key := range seen {
		if prefix == "" || strings.Contains(key, prefix) {
			keys = append(keys, key)
		}
	}
	sort.SliceStable(keys, func(i, j int) bool {
		a := strings.HasPrefix(keys[i], prefix)
		b := strings.HasPrefix(keys[j], prefix)
		if a != b {
			return a
		}
		if len(keys[i]) != len(keys[j]) {
			return len(keys[i]) < len(keys[j])
		}
		return keys[i] < keys[j]
	})

	total := len(keys)
	if len(keys) > maxMentions {
		keys = keys[:maxMentions]
	}
	out := make([]candidate, 0, len(keys))
	for _, key := range keys {
		p := seen[key]
		out = append(out, candidate{
			insert:   "@" + p.login,
			complete: "@" + p.login,
			label:    "@" + p.login,
			detail:   p.name,
		})
	}
	return out, total
}

func peopleFrom(users []model.User) []model.User {
	seen := make(map[string]bool, len(users))
	out := make([]model.User, 0, len(users))
	for _, u := range users {
		key := strings.ToLower(u.Login)
		if u.Login == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, u)
	}
	return out
}
