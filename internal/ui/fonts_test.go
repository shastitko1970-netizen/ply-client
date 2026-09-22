package ui

import "testing"

func TestFontsParse(t *testing.T) {
	faces := loadFaces()
	if len(faces) < 3 {
		t.Fatalf("want >= 3 faces, got %d", len(faces))
	}
	var plex, news int
	for _, f := range faces {
		switch f.Font.Typeface {
		case FacePlex:
			plex++
		case FaceDisplay:
			news++
		}
	}
	if plex < 2 {
		t.Fatalf("plex faces: %d", plex)
	}
	if news < 1 {
		t.Fatalf("newsreader faces: %d", news)
	}
}
