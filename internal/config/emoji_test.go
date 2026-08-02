package config

import (
	"testing"
	"time"
)

func TestEmojiCacheRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if _, ok := LoadEmoji(); ok {
		t.Fatal("a missing cache must report no emoji")
	}

	want := map[string]string{"rocket": "🚀", "shipit": ""}
	if err := SaveEmoji(EmojiSet{FetchedAt: time.Now(), Emoji: want}); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, ok := LoadEmoji()
	if !ok {
		t.Fatal("the cache did not load back")
	}
	if got.Emoji["rocket"] != "🚀" {
		t.Fatalf("glyph did not survive: %q", got.Emoji["rocket"])
	}
	if _, present := got.Emoji["shipit"]; !present {
		t.Fatal("image-only names must survive the round trip")
	}
	if got.Stale() {
		t.Fatal("a cache written just now must not be stale")
	}
}

func TestEmojiCacheGoesStale(t *testing.T) {
	fresh := EmojiSet{FetchedAt: time.Now(), Emoji: map[string]string{"a": "b"}}
	if fresh.Stale() {
		t.Error("a fresh cache is not stale")
	}

	old := EmojiSet{FetchedAt: time.Now().Add(-EmojiTTL - time.Hour), Emoji: map[string]string{"a": "b"}}
	if !old.Stale() {
		t.Error("a cache older than the ttl is stale")
	}

	if !(EmojiSet{FetchedAt: time.Now()}).Stale() {
		t.Error("an empty cache is stale")
	}
}

func TestCorruptEmojiCacheFallsBack(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := SaveEmoji(EmojiSet{FetchedAt: time.Now(), Emoji: map[string]string{"a": "b"}}); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(emojiFile, map[string]any{"schema": 99}); err != nil {
		t.Fatal(err)
	}
	if _, ok := LoadEmoji(); ok {
		t.Fatal("a cache from another schema must be ignored")
	}
}
