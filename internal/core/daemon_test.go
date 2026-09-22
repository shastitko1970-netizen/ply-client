package core

import (
	"fmt"
	"net/http"
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
