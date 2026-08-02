package config

import "time"

const (
	emojiFile   = "emojis.json"
	emojiSchema = 1
	EmojiTTL    = 30 * 24 * time.Hour
)

type EmojiSet struct {
	Schema    int               `json:"schema"`
	FetchedAt time.Time         `json:"fetched_at"`
	Emoji     map[string]string `json:"emoji"`
}

func (e EmojiSet) Stale() bool {
	return len(e.Emoji) == 0 || time.Since(e.FetchedAt) > EmojiTTL
}

func LoadEmoji() (EmojiSet, bool) {
	var e EmojiSet
	if !readJSON(emojiFile, &e) || e.Schema != emojiSchema || len(e.Emoji) == 0 {
		return EmojiSet{}, false
	}
	return e, true
}

func SaveEmoji(e EmojiSet) error {
	e.Schema = emojiSchema
	return writeJSON(emojiFile, e)
}
