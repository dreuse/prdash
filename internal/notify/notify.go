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
			for _, script := range darwinScripts(title, body) {
				if exec.CommandContext(ctx, bin, "-e", script).Run() == nil {
					return
				}
			}
		}
	case "linux":
		if bin, err := exec.LookPath("notify-send"); err == nil {
			if exec.CommandContext(ctx, bin, linuxArgs(title, body)...).Run() == nil {
				return
			}
		}
	}
	terminal(title, body)
}

var terminalBundles = map[string]string{
	"Apple_Terminal": "com.apple.Terminal",
	"iTerm.app":      "com.googlecode.iterm2",
	"ghostty":        "com.mitchellh.ghostty",
	"WezTerm":        "com.github.wez.wezterm",
	"Hyper":          "co.zeit.hyper",
	"Tabby":          "org.tabby",
	"vscode":         "com.microsoft.VSCode",
	"kitty":          "net.kovidgoyal.kitty",
	"alacritty":      "org.alacritty",
	"rio":            "com.raphaelamorim.rio",
}

var terminalDesktopEntries = map[string]string{
	"WezTerm":   "org.wezfurlong.wezterm",
	"ghostty":   "com.mitchellh.ghostty",
	"kitty":     "kitty",
	"alacritty": "Alacritty",
	"vscode":    "code",
	"Tabby":     "tabby",
	"rio":       "rio",
}

func hostBundleID() string {
	if id := strings.TrimSpace(os.Getenv("__CFBundleIdentifier")); id != "" {
		return id
	}
	return terminalBundles[os.Getenv("TERM_PROGRAM")]
}

func darwinScripts(title, body string) []string {
	plain := fmt.Sprintf("display notification %s with title %s", quote(body), quote(title))
	id := hostBundleID()
	if id == "" {
		return []string{plain}
	}
	return []string{fmt.Sprintf("tell application id %s to %s", quote(id), plain), plain}
}

func linuxArgs(title, body string) []string {
	args := []string{"--app-name=prdash"}
	if entry := terminalDesktopEntries[os.Getenv("TERM_PROGRAM")]; entry != "" {
		args = append(args, "--hint=string:desktop-entry:"+entry)
	}
	return append(args, title, body)
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
