package notify

import (
	"os"
	"strings"
	"testing"
)

func TestTheMacNotificationIsAttributedToTheHostTerminal(t *testing.T) {
	t.Setenv("__CFBundleIdentifier", "com.googlecode.iterm2")
	scripts := darwinScripts("CI failed", "on feature/login")

	if len(scripts) != 2 {
		t.Fatalf("we need the attributed script and a plain fallback, got %d", len(scripts))
	}
	if !strings.HasPrefix(scripts[0], `tell application id "com.googlecode.iterm2" to `) {
		t.Errorf("the terminal must own the notification so clicking it comes back here, got %s", scripts[0])
	}
	if !strings.Contains(scripts[0], `display notification "on feature/login" with title "CI failed"`) {
		t.Errorf("the text must survive the wrapping, got %s", scripts[0])
	}
	if strings.Contains(scripts[1], "tell application") {
		t.Errorf("the fallback must not name an app, got %s", scripts[1])
	}
}

func TestTermProgramNamesTheBundleWhenLaunchServicesDidNot(t *testing.T) {
	t.Setenv("__CFBundleIdentifier", "")
	t.Setenv("TERM_PROGRAM", "Apple_Terminal")

	if got := hostBundleID(); got != "com.apple.Terminal" {
		t.Errorf("Terminal.app should be recognised, got %q", got)
	}
}

func TestAnUnknownTerminalStillGetsANotification(t *testing.T) {
	t.Setenv("__CFBundleIdentifier", "")
	t.Setenv("TERM_PROGRAM", "some-terminal-nobody-has-heard-of")

	scripts := darwinScripts("CI failed", "on main")
	if len(scripts) != 1 {
		t.Fatalf("with no app to name there is nothing to fall back from, got %d", len(scripts))
	}
	if strings.Contains(scripts[0], "tell application") {
		t.Errorf("an unknown terminal must not be guessed at, got %s", scripts[0])
	}
}

func TestABundleIdCannotBreakOutOfTheScript(t *testing.T) {
	t.Setenv("__CFBundleIdentifier", `x" to do shell script "touch /tmp/pwned`)
	scripts := darwinScripts("t", "b")

	if strings.Contains(scripts[0], `to do shell script "touch`) {
		t.Errorf("a hostile bundle id must stay inside its string literal, got %s", scripts[0])
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

func TestQuoteKeepsAppleScriptFromBreakingOut(t *testing.T) {
	got := quote(`say "hi" \ now`)
	want := `"say \"hi\" \\ now"`
	if got != want {
		t.Errorf("a run title with quotes must stay one argument, got %s", got)
	}
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
