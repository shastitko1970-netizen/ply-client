package ui

import (
	_ "embed"
	"fmt"

	"gioui.org/font"
	"gioui.org/font/opentype"
	"gioui.org/text"
	"gioui.org/widget/material"
)

//go:embed fonts/IBMPlexSans-Regular.ttf
var plexRegular []byte

//go:embed fonts/IBMPlexSans-Medium.ttf
var plexMedium []byte

//go:embed fonts/Newsreader-Italic.ttf
var newsreaderItalic []byte

const (
	FacePlex    font.Typeface = "IBM Plex Sans"
	FaceDisplay font.Typeface = "Newsreader"
)

func NewTheme() *material.Theme {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(loadFaces()))
	th.Face = FacePlex
	th.Palette.Bg = Bg
	th.Palette.Fg = Fg
	th.Palette.ContrastBg = Accent
	th.Palette.ContrastFg = Ink
	return th
}

func loadFaces() []font.FontFace {
	var out []font.FontFace
	out = append(out, mustParse(plexRegular, FacePlex)...)
	out = append(out, mustParse(plexMedium, FacePlex)...)
	out = append(out, mustParse(newsreaderItalic, FaceDisplay)...)
	return out
}

func mustParse(src []byte, face font.Typeface) []font.FontFace {
	col, err := opentype.ParseCollection(src)
	if err != nil {
		panic(fmt.Sprintf("font %s: %v", face, err))
	}
	for i := range col {
		col[i].Font.Typeface = face
	}
	return col
}
