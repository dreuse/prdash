package github

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
)

const unicodeSegment = "/unicode/"

func (c *CLI) Emoji(ctx context.Context) (map[string]string, error) {
	out, err := c.run(ctx, "", "list emoji", "api", "emojis")
	if err != nil {
		return nil, err
	}

	var raw map[string]string
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, &Error{Op: "decode emoji", Err: err}
	}

	set := make(map[string]string, len(raw))
	for name, url := range raw {
		set[name] = glyphFromAssetURL(url)
	}
	return set, nil
}

func glyphFromAssetURL(url string) string {
	i := strings.Index(url, unicodeSegment)
	if i < 0 {
		return ""
	}
	tail := url[i+len(unicodeSegment):]
	if cut := strings.IndexByte(tail, '?'); cut >= 0 {
		tail = tail[:cut]
	}
	tail = strings.TrimSuffix(tail, ".png")
	if tail == "" {
		return ""
	}

	var b strings.Builder
	for _, part := range strings.Split(tail, "-") {
		point, err := strconv.ParseInt(part, 16, 32)
		if err != nil {
			return ""
		}
		b.WriteRune(rune(point))
	}
	return b.String()
}
