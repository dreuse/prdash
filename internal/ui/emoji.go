package ui

import (
	"sort"
	"strings"
)

const maxCandidates = 50

var seedEmoji = map[string]string{
	"+1":               "👍",
	"-1":               "👎",
	"100":              "💯",
	"art":              "🎨",
	"boom":             "💥",
	"bug":              "🐛",
	"bulb":             "💡",
	"clap":             "👏",
	"construction":     "🚧",
	"eyes":             "👀",
	"fire":             "🔥",
	"hammer":           "🔨",
	"heart":            "💖",
	"joy":              "😂",
	"lock":             "🔒",
	"memo":             "📝",
	"mag":              "🔍",
	"muscle":           "💪",
	"ok_hand":          "👌",
	"package":          "📦",
	"pray":             "🙏",
	"pushpin":          "📌",
	"recycle":          "♻",
	"robot":            "🤖",
	"rocket":           "🚀",
	"rotating_light":   "🚨",
	"seedling":         "🌱",
	"shipit":           "",
	"octocat":          "",
	"smile":            "😄",
	"sob":              "😭",
	"sparkles":         "✨",
	"tada":             "🎉",
	"thinking":         "🤔",
	"truck":            "🚚",
	"warning":          "⚠",
	"wave":             "👋",
	"white_check_mark": "✅",
	"wrench":           "🔧",
	"x":                "❌",
	"zap":              "⚡",
}

type emojiEntry struct {
	name  string
	lower string
	glyph string
}

type EmojiSet struct {
	entries []emojiEntry
	byName  map[string]int
}

func NewEmojiSet(sets ...map[string]string) EmojiSet {
	merged := make(map[string]string, len(seedEmoji))
	for name, glyph := range seedEmoji {
		merged[name] = glyph
	}
	for _, set := range sets {
		for name, glyph := range set {
			merged[name] = glyph
		}
	}

	entries := make([]emojiEntry, 0, len(merged))
	for name, glyph := range merged {
		if name == "" {
			continue
		}
		entries = append(entries, emojiEntry{
			name:  name,
			lower: strings.ToLower(name),
			glyph: strings.TrimSpace(glyph),
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].lower < entries[j].lower })

	byName := make(map[string]int, len(entries))
	for i, e := range entries {
		byName[e.lower] = i
	}
	return EmojiSet{entries: entries, byName: byName}
}

func (s EmojiSet) Count() int { return len(s.entries) }

func (s EmojiSet) Glyph(name string) (string, bool) {
	i, ok := s.byName[strings.ToLower(name)]
	if !ok {
		return "", false
	}
	return s.entries[i].glyph, s.entries[i].glyph != ""
}

func (s EmojiSet) Known(name string) bool {
	_, ok := s.byName[strings.ToLower(name)]
	return ok
}

func (s EmojiSet) ImageOnly(name string) bool {
	i, ok := s.byName[strings.ToLower(name)]
	return ok && s.entries[i].glyph == ""
}

func (s EmojiSet) canonical(name string) string {
	if i, ok := s.byName[strings.ToLower(name)]; ok {
		return s.entries[i].name
	}
	return name
}

func (s EmojiSet) Render(name string, ascii bool) string {
	glyph, ok := s.Glyph(name)
	if !ok || ascii {
		return ":" + s.canonical(name) + ":"
	}
	return glyph
}

func (s EmojiSet) Complete(prefix string) (matches []string, total int) {
	prefix = strings.ToLower(prefix)
	if prefix == "" {
		return nil, 0
	}

	var exact, prefixed, contained []string
	for _, e := range s.entries {
		switch {
		case e.lower == prefix:
			exact = append(exact, e.name)
		case strings.HasPrefix(e.lower, prefix):
			prefixed = append(prefixed, e.name)
		case strings.Contains(e.lower, prefix):
			contained = append(contained, e.name)
		}
	}

	sort.SliceStable(prefixed, func(i, j int) bool { return len(prefixed[i]) < len(prefixed[j]) })
	sort.SliceStable(contained, func(i, j int) bool { return len(contained[i]) < len(contained[j]) })

	ranked := append(append(exact, prefixed...), contained...)
	total = len(ranked)
	if len(ranked) > maxCandidates {
		ranked = ranked[:maxCandidates]
	}
	return ranked, total
}

func (s EmojiSet) replaceTrailingShortcode(text string, ascii bool) string {
	if !strings.HasSuffix(text, ":") || len(text) < 3 {
		return text
	}
	body := text[:len(text)-1]
	i := strings.LastIndexByte(body, ':')
	if i < 0 {
		return text
	}
	name := body[i+1:]
	if name == "" || strings.ContainsAny(name, " \t:") || !s.Known(name) {
		return text
	}
	return body[:i] + s.Render(name, ascii)
}
