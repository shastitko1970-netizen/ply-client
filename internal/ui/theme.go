package ui

import (
	"image/color"

	"gioui.org/font"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

var (
	Bg      = color.NRGBA{R: 10, G: 10, B: 11, A: 255}
	Panel   = color.NRGBA{R: 18, G: 18, B: 20, A: 255}
	Field   = color.NRGBA{R: 12, G: 12, B: 14, A: 255}
	Subtle  = color.NRGBA{R: 26, G: 26, B: 30, A: 255}
	Fg      = color.NRGBA{R: 244, G: 244, B: 245, A: 255}
	Muted   = color.NRGBA{R: 161, G: 161, B: 170, A: 255}
	Dim     = color.NRGBA{R: 113, G: 113, B: 122, A: 255}
	Line    = color.NRGBA{R: 244, G: 244, B: 245, A: 28}
	Line2   = color.NRGBA{R: 244, G: 244, B: 245, A: 48}
	Accent  = color.NRGBA{R: 200, G: 204, B: 212, A: 255}
	Ink     = color.NRGBA{R: 10, G: 10, B: 11, A: 255}
	Ok      = color.NRGBA{R: 138, G: 163, B: 138, A: 255}
	OkDim   = color.NRGBA{R: 138, G: 163, B: 138, A: 40}
	Err     = color.NRGBA{R: 193, G: 123, B: 123, A: 255}
	ErrDim  = color.NRGBA{R: 193, G: 123, B: 123, A: 28}
	Transparent = color.NRGBA{}
)

func Title(th *material.Theme, s string) material.LabelStyle {
	l := material.Label(th, unit.Sp(34), s)
	l.Font.Typeface = FaceDisplay
	l.Font.Style = font.Italic
	l.Font.Weight = font.Medium
	l.Color = Fg
	return l
}

func Body(th *material.Theme, s string) material.LabelStyle {
	l := material.Body2(th, s)
	l.Color = Muted
	return l
}

func Small(th *material.Theme, s string) material.LabelStyle {
	l := material.Caption(th, s)
	l.Color = Dim
	return l
}
