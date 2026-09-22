package ui

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

func Pill(gtx layout.Context, th *material.Theme, label string, fg, bg color.NRGBA) layout.Dimensions {
	macro := op.Record(gtx.Ops)
	dims := layout.Inset{Top: unit.Dp(5), Bottom: unit.Dp(5), Left: unit.Dp(10), Right: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		t := material.Caption(th, label)
		t.Color = fg
		t.Font.Weight = font.Medium
		return t.Layout(gtx)
	})
	call := macro.Stop()
	r := image.Rectangle{Max: dims.Size}
	defer clip.UniformRRect(r, dims.Size.Y/2).Push(gtx.Ops).Pop()
	paint.Fill(gtx.Ops, bg)
	call.Add(gtx.Ops)
	return dims
}

func Card(gtx layout.Context, w layout.Widget) layout.Dimensions {
	return CardTint(gtx, Panel, w)
}

func CardTint(gtx layout.Context, bg color.NRGBA, w layout.Widget) layout.Dimensions {
	macro := op.Record(gtx.Ops)
	dims := layout.UniformInset(unit.Dp(16)).Layout(gtx, w)
	call := macro.Stop()
	r := image.Rectangle{Max: dims.Size}
	rr := gtx.Dp(20)
	defer clip.UniformRRect(r, rr).Push(gtx.Ops).Pop()
	paint.Fill(gtx.Ops, bg)
	paint.FillShape(gtx.Ops, Line, clip.Stroke{
		Path:  clip.UniformRRect(r, rr).Path(gtx.Ops),
		Width: 1,
	}.Op())
	call.Add(gtx.Ops)
	return dims
}

func Input(gtx layout.Context, w layout.Widget) layout.Dimensions {
	macro := op.Record(gtx.Ops)
	dims := layout.Inset{Top: unit.Dp(11), Bottom: unit.Dp(11), Left: unit.Dp(14), Right: unit.Dp(14)}.Layout(gtx, w)
	call := macro.Stop()
	r := image.Rectangle{Max: dims.Size}
	rr := gtx.Dp(12)
	defer clip.UniformRRect(r, rr).Push(gtx.Ops).Pop()
	paint.Fill(gtx.Ops, Field)
	paint.FillShape(gtx.Ops, Line, clip.Stroke{
		Path:  clip.UniformRRect(r, rr).Path(gtx.Ops),
		Width: 1,
	}.Op())
	call.Add(gtx.Ops)
	return dims
}

func Hairline(gtx layout.Context) layout.Dimensions {
	h := gtx.Dp(1)
	w := gtx.Constraints.Max.X
	defer clip.Rect{Max: image.Pt(w, h)}.Push(gtx.Ops).Pop()
	paint.Fill(gtx.Ops, Line)
	return layout.Dimensions{Size: image.Pt(w, h+gtx.Dp(10))}
}

func Switch(gtx layout.Context, b *widget.Bool) layout.Dimensions {
	trackW, trackH := gtx.Dp(44), gtx.Dp(24)
	hit := image.Pt(gtx.Dp(48), gtx.Dp(32))
	gtx.Constraints = layout.Exact(hit)
	return b.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			sz := image.Pt(trackW, trackH)
			r := image.Rectangle{Max: sz}
			rr := trackH / 2
			bg, knob := Subtle, Fg
			if b.Value {
				bg, knob = Accent, Ink
			}
			defer clip.UniformRRect(r, rr).Push(gtx.Ops).Pop()
			paint.Fill(gtx.Ops, bg)
			paint.FillShape(gtx.Ops, Line, clip.Stroke{
				Path:  clip.UniformRRect(r, rr).Path(gtx.Ops),
				Width: 1,
			}.Op())
			kn := gtx.Dp(20)
			pad := (trackH - kn) / 2
			x := pad
			if b.Value {
				x = trackW - kn - pad
			}
			paint.FillShape(gtx.Ops, knob, clip.Ellipse{
				Min: image.Pt(x, pad),
				Max: image.Pt(x+kn, pad+kn),
			}.Op(gtx.Ops))
			return layout.Dimensions{Size: sz}
		})
	})
}

func SwitchLabel(gtx layout.Context, th *material.Theme, b *widget.Bool, label string) layout.Dimensions {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			t := material.Caption(th, label)
			t.Color = Fg
			return t.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return Switch(gtx, b)
		}),
	)
}

func SwitchRow(gtx layout.Context, th *material.Theme, b *widget.Bool, title, sub string) layout.Dimensions {
	return layout.Inset{Top: unit.Dp(2), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						t := material.Body1(th, title)
						t.Color = Fg
						return t.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if sub == "" {
							return layout.Dimensions{}
						}
						t := material.Caption(th, sub)
						t.Color = Dim
						return t.Layout(gtx)
					}),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return Switch(gtx, b)
			}),
		)
	})
}

func fillBtn(gtx layout.Context, th *material.Theme, click *widget.Clickable, label string, bg, fg color.NRGBA, rad, minH unit.Dp) layout.Dimensions {
	h := gtx.Dp(minH)
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	if gtx.Constraints.Min.Y < h {
		gtx.Constraints.Min.Y = h
	}
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		sz := image.Pt(gtx.Constraints.Min.X, gtx.Constraints.Min.Y)
		if sz.Y < h {
			sz.Y = h
		}
		rr := gtx.Dp(rad)
		col := bg
		if click.Pressed() && bg.A > 0 {
			col.A = 220
		}
		defer clip.UniformRRect(image.Rectangle{Max: sz}, rr).Push(gtx.Ops).Pop()
		if col.A > 0 {
			paint.Fill(gtx.Ops, col)
		}
		return layout.Stack{Alignment: layout.Center}.Layout(gtx,
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{Size: sz}
			}),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				t := material.Body1(th, label)
				t.Color = fg
				t.Font.Weight = font.Medium
				return t.Layout(gtx)
			}),
		)
	})
}

func Primary(gtx layout.Context, th *material.Theme, click *widget.Clickable, label string) layout.Dimensions {
	return fillBtn(gtx, th, click, label, Accent, Ink, 12, 44)
}

func Ghost(gtx layout.Context, th *material.Theme, click *widget.Clickable, label string) layout.Dimensions {
	return fillBtn(gtx, th, click, label, Field, Fg, 12, 40)
}

func Quiet(gtx layout.Context, th *material.Theme, click *widget.Clickable, label string) layout.Dimensions {
	return fillBtn(gtx, th, click, label, Transparent, Dim, 8, 32)
}

func Outline(gtx layout.Context, th *material.Theme, click *widget.Clickable, label string) layout.Dimensions {
	h := gtx.Dp(44)
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	if gtx.Constraints.Min.Y < h {
		gtx.Constraints.Min.Y = h
	}
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		sz := image.Pt(gtx.Constraints.Min.X, gtx.Constraints.Min.Y)
		r := image.Rectangle{Max: sz}
		rr := gtx.Dp(12)
		defer clip.UniformRRect(r, rr).Push(gtx.Ops).Pop()
		paint.Fill(gtx.Ops, Panel)
		paint.FillShape(gtx.Ops, Line2, clip.Stroke{
			Path:  clip.UniformRRect(r, rr).Path(gtx.Ops),
			Width: 1,
		}.Op())
		return layout.Stack{Alignment: layout.Center}.Layout(gtx,
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{Size: sz}
			}),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				t := material.Body1(th, label)
				t.Color = Fg
				t.Font.Weight = font.Medium
				return t.Layout(gtx)
			}),
		)
	})
}
