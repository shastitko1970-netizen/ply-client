package shell

import (
	"strings"
	"testing"
)

func TestHTMLLooksLikeSite(t *testing.T) {
	s := string(HTML)
	for _, want := range []string{
		"Newsreader",
		"IBM Plex Sans",
		"--bg:#0a0a0b",
		"--panel:#121214",
		"--radius:24px",
		"Россия мимо",
		"Ключ",
		"/v1/state",
		"class=\"chrome\"",
		"plyWin",
		`plyWin("drag")`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("html missing %q", want)
		}
	}
}

func TestFontEmbed(t *testing.T) {
	for _, name := range []string{
		"Newsreader-Italic.ttf",
		"IBMPlexSans-Regular.ttf",
		"IBMPlexSans-Medium.ttf",
	} {
		b, ok := Font(name)
		if !ok || len(b) < 1000 {
			t.Fatalf("font %s missing", name)
		}
	}
	if _, ok := Font("../embed.go"); ok {
		t.Fatal("path traversal")
	}
	if _, ok := Font("fonts/Newsreader-Italic.ttf"); ok {
		t.Fatal("slash should reject")
	}
}
