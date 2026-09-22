package core

import (
	"strings"
	"testing"
)

func TestWebViewDirNotProgramFiles(t *testing.T) {
	d, err := WebViewDir()
	if err != nil {
		t.Fatal(err)
	}
	low := strings.ToLower(d)
	if strings.Contains(low, "program files") {
		t.Fatalf("webview cache in Program Files: %s", d)
	}
	if !strings.Contains(low, "webview") {
		t.Fatalf("expected webview in path, got %s", d)
	}
}
