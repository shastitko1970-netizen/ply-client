package core

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestDaemonStateRequiresAuth(t *testing.T) {
	if err := StartDaemon(); err != nil {
		t.Fatal(err)
	}
	defer StopDaemon()
	c, err := readCoreFile()
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/v1/state", c.Port))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status %d", res.StatusCode)
	}
	d := TryAttach()
	if d == nil {
		t.Fatal("attach")
	}
	st, err := d.State()
	if err != nil {
		t.Fatal(err)
	}
	if st.Version != Version {
		t.Fatalf("version %s", st.Version)
	}
	if st.Live {
		t.Fatal("should start idle")
	}
	if !strings.Contains(d.ShellURL(), "/ui?t=") {
		t.Fatalf("shell url %s", d.ShellURL())
	}
}

func TestDaemonSplitRoundtrip(t *testing.T) {
	if err := StartDaemon(); err != nil {
		t.Fatal(err)
	}
	defer StopDaemon()
	d := TryAttach()
	if d == nil {
		t.Fatal("attach")
	}
	st, err := d.SetSplit(false)
	if err != nil {
		t.Fatal(err)
	}
	if st.Split {
		t.Fatal("split still on")
	}
	st, err = d.SetSplit(true)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Split {
		t.Fatal("split off")
	}
}

func TestDaemonUINoAuth(t *testing.T) {
	if err := StartDaemon(); err != nil {
		t.Fatal(err)
	}
	defer StopDaemon()
	c, err := readCoreFile()
	if err != nil {
		t.Fatal(err)
	}
	base := fmt.Sprintf("http://127.0.0.1:%d", c.Port)
	res, err := http.Get(base + "/ui")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("ui status %d", res.StatusCode)
	}
	if !strings.Contains(string(body), "Newsreader") {
		t.Fatal("ui html missing Newsreader")
	}
	res, err = http.Get(base + "/fonts/Newsreader-Italic.ttf")
	if err != nil {
		t.Fatal(err)
	}
	font, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || len(font) < 1000 {
		t.Fatalf("font status %d size %d", res.StatusCode, len(font))
	}
	res, err = http.Get(base + "/fonts/../embed.go")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode == 200 {
		t.Fatal("traversal should fail")
	}
}
