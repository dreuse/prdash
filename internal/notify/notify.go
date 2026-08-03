package notify

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const sendTimeout = 5 * time.Second

func Send(title, body string) {
	title, body = sanitise(title), sanitise(body)
	if title == "" && body == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), sendTimeout)
	defer cancel()

	switch runtime.GOOS {
	case "darwin":
		if bin, err := exec.LookPath("osascript"); err == nil {
			script := fmt.Sprintf("display notification %s with title %s", quote(body), quote(title))
			if exec.CommandContext(ctx, bin, "-e", script).Run() == nil {
				return
			}
		}
	case "linux":
		if bin, err := exec.LookPath("notify-send"); err == nil {
			if exec.CommandContext(ctx, bin, "--app-name=prdash", title, body).Run() == nil {
				return
			}
		}
	}
	terminal(title, body)
}

func terminal(title, body string) {
	text := strings.TrimSpace(title + " " + body)
	fmt.Fprintf(os.Stderr, "\x1b]9;%s\x07", text)
}

func quote(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

func sanitise(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
}
