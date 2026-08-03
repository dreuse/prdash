package notify

import "testing"

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
