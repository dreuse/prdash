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

	if runtime.GOOS == "linux" {
		ctx, cancel := context.WithTimeout(context.Background(), sendTimeout)
		defer cancel()

		if bin, err := exec.LookPath("notify-send"); err == nil {
			if exec.CommandContext(ctx, bin, linuxArgs(title, body)...).Run() == nil {
				return
			}
		}
	}
	fmt.Fprint(os.Stderr, osc9(title, body))
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

func linuxArgs(title, body string) []string {
	args := []string{"--app-name=prdash"}
	if entry := terminalDesktopEntries[os.Getenv("TERM_PROGRAM")]; entry != "" {
		args = append(args, "--hint=string:desktop-entry:"+entry)
	}
	return append(args, title, body)
}

func osc9(title, body string) string {
	return fmt.Sprintf("\x1b]9;%s\x07", strings.TrimSpace(title+" "+body))
}

func sanitise(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
}
