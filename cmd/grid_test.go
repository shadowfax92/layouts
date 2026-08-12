package cmd

import "testing"

func TestDetectGridBackendPrefersHerdrOverInheritedTmux(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "w1:p1")
	t.Setenv("TMUX", "/tmp/tmux/default,1,0")

	if got := detectGridBackend(); got != gridBackendHerdr {
		t.Fatalf("detectGridBackend() = %v, want Herdr", got)
	}
}
