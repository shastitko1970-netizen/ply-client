package core

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestSubMetaBodyWins(t *testing.T) {
	h := make(http.Header)
	h.Set("new-url", "https://header.example/old")
	h.Set("fallback-url", "https://fb.example/a")
	body := "#new-url https://body.example/new\n#new-domain: ignored.example\n" + paper + "\n"
	m := parseSubMeta(h, body)
	if m.newURL != "https://body.example/new" {
		t.Fatalf("url %q", m.newURL)
	}
	if m.fallback != "https://fb.example/a" {
		t.Fatalf("fb %q", m.fallback)
	}
	h.Set("new-url", "http://127.0.0.1/nope")
	if parseSubMeta(h).newURL != "" {
		t.Fatal("loopback new-url must be dropped")
	}
}

func TestSubMetaDomainAndAlias(t *testing.T) {
	body := "#subscription-url: https://next.example/sub/1\n" + paper + "\n"
	nu, nd, fb := metaFromBody(body)
	if nu != "https://next.example/sub/1" || nd != "" || fb != "" {
		t.Fatalf("%q %q %q", nu, nd, fb)
	}
	got, err := swapHost("https://old.example:8443/key/abc", "cdn.example")
	if err != nil || got != "https://cdn.example:8443/key/abc" {
		t.Fatalf("%s %v", got, err)
	}
	blob := base64.StdEncoding.EncodeToString([]byte(paper + "\n"))
	mixed := "#new-domain cdn.example\n" + blob
	exp := expandSubPayload(mixed)
	if !strings.Contains(exp, "vless://") || !strings.Contains(exp, "#new-domain cdn.example") {
		t.Fatalf("payload %q", exp)
	}
	_, domain, _ := metaFromBody(exp)
	if domain != "cdn.example" {
		t.Fatalf("domain %q", domain)
	}
}

func TestFetchRotateNewURL(t *testing.T) {
	t.Setenv("PLY_DATA", t.TempDir())
	t.Setenv("PLY_ALLOW_LOCAL_SUB", "1")
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.URL.Path == "/old" {
			w.Header().Set("new-url", "https://should-not-fetch.example/new")
			_, _ = w.Write([]byte(paper + "\n"))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	body, stored, err := fetchSubRotate(srv.URL + "/old")
	if err != nil {
		t.Fatal(err)
	}
	if hits != 1 {
		t.Fatalf("hops %d", hits)
	}
	if !strings.Contains(body, "vless://") {
		t.Fatalf("body %q", body)
	}
	if stored != "https://should-not-fetch.example/new" {
		t.Fatalf("stored %q", stored)
	}
}

func TestFetchFallback(t *testing.T) {
	t.Setenv("PLY_DATA", t.TempDir())
	t.Setenv("PLY_ALLOW_LOCAL_SUB", "1")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/dead" {
			http.Error(w, "gone", http.StatusGone)
			return
		}
		w.Header().Set("subscription-domain", "alive.example")
		_, _ = w.Write([]byte(paper + "\n"))
	}))
	defer srv.Close()
	if err := writeFallback(srv.URL + "/fb"); err != nil {
		t.Fatal(err)
	}
	_, stored, err := fetchSubRotate(srv.URL + "/dead")
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(stored)
	if err != nil || u.Hostname() != "alive.example" || !strings.HasSuffix(u.Path, "/fb") {
		t.Fatalf("stored %q", stored)
	}
}
