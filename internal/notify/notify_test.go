package notify

import (
	"os"
	"strings"
	"testing"
)

func TestTheMacNotificationComesFromTheTerminalItself(t *testing.T) {
	got := osc9("CI failed", "on feature/login")

	if got != "\x1b]9;CI failed on feature/login\x07" {
		t.Errorf("clicking it must focus the terminal that posted it, got %q", got)
	}
}

func TestTheLinuxNotificationNamesTheTerminalWhenItCan(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "WezTerm")
	args := linuxArgs("CI failed", "on main")

	if !contains(args, "--hint=string:desktop-entry:org.wezfurlong.wezterm") {
		t.Errorf("the terminal should own the notification, got %v", args)
	}
	if !contains(args, "CI failed") || !contains(args, "on main") {
		t.Errorf("the text must still be passed, got %v", args)
	}
}

func TestTheLinuxNotificationSkipsTheHintWhenTheTerminalIsUnknown(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("TERM", "xterm-256color")

	for _, arg := range linuxArgs("CI failed", "on main") {
		if strings.HasPrefix(arg, "--hint=string:desktop-entry:") {
			t.Errorf("we must not invent a desktop entry, got %q", arg)
		}
	}
}

func contains(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

func TestSanitiseDropsControlCharacters(t *testing.T) {
	if got := sanitise("build\x1b]9;evil\x07 done\n"); got != "build]9;evil done" {
		t.Errorf("escape sequences from a branch name must not reach the terminal, got %q", got)
	}
}

func TestSanitiseKeepsOrdinaryText(t *testing.T) {
	if got := sanitise("ci failed on feature/login"); got != "ci failed on feature/login" {
		t.Errorf("normal text must survive untouched, got %q", got)
	}
}

func TestManualSend(t *testing.T) {
	if os.Getenv("PRDASH_NOTIFY_MANUAL") == "" {
		t.Skip("set PRDASH_NOTIFY_MANUAL=1 to fire a real notification")
	}
	Send("prdash · CI failed", "click me — I should focus your terminal")
}
