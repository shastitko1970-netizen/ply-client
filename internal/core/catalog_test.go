package core

import (
	"strings"
	"testing"
)

func TestAllShareLinesKeepsOrder(t *testing.T) {
	hy := "hy2://pass@h.example:443?sni=h.example"
	body := "hysteria://old@h:443\n" + hy + "\n" + paper + "\n" + hy + "\n"
	lines := AllShareLines(body)
	if len(lines) != 2 {
		t.Fatalf("lines %d %q", len(lines), lines)
	}
	if !strings.HasPrefix(lines[0], "hy2://") || !strings.HasPrefix(lines[1], "vless://") {
		t.Fatalf("%q", lines)
	}
}

func TestPickNodeByID(t *testing.T) {
	hy := "hy2://pass@h.example:443?sni=h.example"
	lines := AllShareLines(hy + "\n" + paper)
	want, err := ParseLink(paper)
	if err != nil {
		t.Fatal(err)
	}
	got, err := pickNode(lines, want.ID())
	if err != nil {
		t.Fatal(err)
	}
	if got.Host != want.Host || got.Proto != "vless" {
		t.Fatalf("picked %+v", got)
	}
	first, err := pickNode(lines, "")
	if err != nil || first.Proto != "hysteria" {
		t.Fatalf("first %+v %v", first, err)
	}
}

func TestNodeIDStable(t *testing.T) {
	a, err := ParseLink(paper)
	if err != nil {
		t.Fatal(err)
	}
	b, err := ParseLink(paper)
	if err != nil {
		t.Fatal(err)
	}
	if a.ID() == "" || a.ID() != b.ID() {
		t.Fatalf("id %s %s", a.ID(), b.ID())
	}
	if a.Card().Label == "" || a.Card().Host != a.Host {
		t.Fatalf("card %+v", a.Card())
	}
}

func TestSecretRoundtripPlain(t *testing.T) {
	in := []byte("vless://secret\n")
	out, err := seal(in)
	if err != nil {
		t.Fatal(err)
	}
	back, err := openSeal(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(back) != string(in) {
		t.Fatalf("got %q", back)
	}
}
