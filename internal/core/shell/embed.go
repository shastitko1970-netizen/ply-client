package shell

import (
	"embed"
	"strings"
)

//go:embed index.html
var HTML []byte

//go:embed fonts/*.ttf
var fonts embed.FS

func Font(name string) ([]byte, bool) {
	name = strings.ReplaceAll(name, "\\", "/")
	if strings.Contains(name, "/") || strings.Contains(name, "..") {
		return nil, false
	}
	b, err := fonts.ReadFile("fonts/" + name)
	if err != nil || len(b) == 0 {
		return nil, false
	}
	return b, true
}
