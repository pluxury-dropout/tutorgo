package handlers

import "testing"

// Locks the WS allowlist tolerance to gin-cors's: stray whitespace / trailing
// slash / casing in ALLOWED_ORIGIN must not silently 403 the WS upgrade.
func TestNormalizeOrigin(t *testing.T) {
	want := "https://tutorgo-henna.vercel.app"
	for _, in := range []string{
		"https://tutorgo-henna.vercel.app",
		"https://tutorgo-henna.vercel.app/",
		" https://tutorgo-henna.vercel.app\n",
		"https://Tutorgo-Henna.vercel.app",
	} {
		if got := normalizeOrigin(in); got != want {
			t.Errorf("normalizeOrigin(%q) = %q, want %q", in, got, want)
		}
	}
}
