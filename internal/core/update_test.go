package core

import "testing"

func TestVersionLess(t *testing.T) {
	if !VersionLess("1.4.0", "1.5.0") {
		t.Fatal("1.4 < 1.5")
	}
	if VersionLess("1.3.0", "1.3.0") {
		t.Fatal("equal")
	}
	if VersionLess("v1.3.1", "1.3.0") {
		t.Fatal("1.3.1 not less")
	}
	if !VersionLess("1.2.9", "v1.3.0") {
		t.Fatal("v prefix")
	}
}
