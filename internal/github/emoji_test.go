package github

import "testing"

func TestGlyphFromAssetURL(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://github.githubassets.com/images/icons/emoji/unicode/1f680.png?v8", "🚀"},
		{"https://github.githubassets.com/images/icons/emoji/unicode/1f44d.png?v8", "👍"},
		{"https://github.githubassets.com/images/icons/emoji/unicode/1f1f2-1f1e6.png?v8", "🇲🇦"},
		{"https://github.githubassets.com/images/icons/emoji/unicode/2764-fe0f.png?v8", "❤️"},
		{"https://github.githubassets.com/images/icons/emoji/unicode/1f680.png", "🚀"},
		{"https://github.githubassets.com/images/icons/emoji/shipit.png?v8", ""},
		{"https://github.githubassets.com/images/icons/emoji/octocat.png?v8", ""},
		{"", ""},
		{"https://example.com/unicode/notahexnumber.png", ""},
	}
	for _, tc := range tests {
		if got := glyphFromAssetURL(tc.url); got != tc.want {
			t.Errorf("glyphFromAssetURL(%q) = %q, want %q", tc.url, got, tc.want)
		}
	}
}
