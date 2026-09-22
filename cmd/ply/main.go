package main

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"os"
	"runtime"
	"strings"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"ply/internal/core"
)

var (
	colBg     = color.NRGBA{R: 10, G: 10, B: 11, A: 255}
	colPanel  = color.NRGBA{R: 18, G: 18, B: 20, A: 255}
	colFg     = color.NRGBA{R: 244, G: 244, B: 245, A: 255}
	colMuted  = color.NRGBA{R: 161, G: 161, B: 170, A: 255}
	colSubtle = color.NRGBA{R: 113, G: 113, B: 122, A: 255}
	colLine   = color.NRGBA{R: 244, G: 244, B: 245, A: 28}
	colAccent = color.NRGBA{R: 200, G: 204, B: 212, A: 255}
	colInk    = color.NRGBA{R: 10, G: 10, B: 11, A: 255}
	colOk     = color.NRGBA{R: 138, G: 163, B: 138, A: 255}
	colErr    = color.NRGBA{R: 193, G: 123, B: 123, A: 255}
)

type ui struct {
	w          *app.Window
	th         *material.Theme
	url        widget.Editor
	connect    widget.Clickable
	disconnect widget.Clickable
	elevate    widget.Clickable
	auto       widget.Bool
	busy       bool
	admin      bool
	trayOn     bool
	status     string
	detail     string
	err        string
	live       bool
	exitIP     string
	node       *core.Node
}

func main() {
	core.SetAppID()
	if !core.AcquireInstance() {
		core.ActivateExisting()
		os.Exit(0)
	}
	go func() {
		w := new(app.Window)
		w.Option(
			app.Title(core.WindowTitle),
			app.Size(unit.Dp(460), unit.Dp(720)),
			app.MinSize(unit.Dp(400), unit.Dp(560)),
		)
		if err := run(w); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func run(w *app.Window) error {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	th.Palette.Bg = colBg
	th.Palette.Fg = colFg
	th.Palette.ContrastBg = colAccent
	th.Palette.ContrastFg = colInk

	u := &ui{w: w, th: th, admin: core.IsAdmin(), status: "VPN выключен"}
	u.url.SingleLine = true
	u.url.Submit = true
	u.auto.Value = true
	if saved := core.ReadURL(); saved != "" {
		u.url.SetText(saved)
	}
	if u.admin {
		if saved := strings.TrimSpace(u.url.Text()); saved != "" {
			u.busy = true
			u.status = "включаю туннель"
			go u.doConnect(saved, true)
		}
	} else {
		u.status = "нужны права"
		u.err = "Туннель без прав администратора не встанет. Нажми «Запросить права»."
	}

	var ops op.Ops
	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			core.RemoveTray()
			_ = core.Disconnect()
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			if !u.trayOn {
				u.trayOn = core.AttachTray(core.TrayHooks{
					Invalidate: w.Invalidate,
					OnDisconnect: func() {
						go u.doDisconnect()
					},
					OnQuit: func() {
						go func() {
							_ = core.Disconnect()
							core.RequestQuit()
						}()
					},
				})
			}
			u.update(gtx)
			paint.Fill(gtx.Ops, colBg)
			u.layout(gtx)
			e.Frame(gtx.Ops)
		}
	}
}

func (u *ui) update(gtx layout.Context) {
	for {
		ev, ok := u.url.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.SubmitEvent); ok && !u.busy && u.admin {
			src := strings.TrimSpace(u.url.Text())
			auto := u.auto.Value
			u.busy = true
			u.err = ""
			u.status = "включаю туннель"
			go u.doConnect(src, auto)
		}
	}
	if u.auto.Update(gtx) {
		exe, _ := os.Executable()
		_ = core.SetAutoStart(u.auto.Value, exe)
	}
	if u.elevate.Clicked(gtx) && !u.admin {
		if err := core.RelaunchElevated(); err != nil {
			u.err = err.Error()
			u.w.Invalidate()
			return
		}
		os.Exit(0)
	}
	if u.connect.Clicked(gtx) && !u.busy && u.admin {
		src := strings.TrimSpace(u.url.Text())
		auto := u.auto.Value
		u.busy = true
		u.err = ""
		u.status = "включаю туннель"
		go u.doConnect(src, auto)
	}
	if u.disconnect.Clicked(gtx) && !u.busy {
		u.busy = true
		go u.doDisconnect()
	}
}

func (u *ui) doConnect(src string, auto bool) {
	s, err := core.Connect(src)
	if err != nil {
		u.live = false
		u.err = err.Error()
		u.status = "ошибка"
		u.detail = ""
		u.exitIP = ""
		u.busy = false
		u.w.Invalidate()
		return
	}
	u.node = s.Node
	u.exitIP = s.ExitIP
	u.live = true
	u.status = "VPN включён"
	u.detail = fmt.Sprintf("%s:%d   %s", s.Node.Host, s.Node.Port, s.Node.SNI)
	u.err = ""
	u.busy = false
	if auto {
		exe, _ := os.Executable()
		_ = core.SetAutoStart(true, exe)
	}
	u.w.Invalidate()
}

func (u *ui) doDisconnect() {
	_ = core.Disconnect()
	u.live = false
	u.status = "VPN выключен"
	u.detail = ""
	u.exitIP = ""
	u.err = ""
	u.busy = false
	u.w.Invalidate()
}

func (u *ui) layout(gtx layout.Context) layout.Dimensions {
	inset := layout.UniformInset(unit.Dp(28))
	return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						t := material.H3(u.th, "Ply")
						t.Color = colFg
						t.Font.Style = font.Italic
						return t.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						c := colMuted
						if u.live {
							c = colOk
						}
						if u.err != "" {
							c = colErr
						}
						b := material.Body2(u.th, u.status)
						b.Color = c
						return b.Layout(gtx)
					}),
				)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				b := material.Body2(u.th, "1. Вставь ссылку Paper\n2. Нажми «Включить VPN»\n3. Крестик сворачивает в трей — туннель не гаснет\n4. Выход только из значка у часов")
				b.Color = colMuted
				return b.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(20)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return roundPanel(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							l := material.Caption(u.th, "ССЫЛКА КАБИНЕТА")
							l.Color = colSubtle
							return l.Layout(gtx)
						}),
						layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							ed := material.Editor(u.th, &u.url, "https://…azure-api.net/…")
							ed.Color = colFg
							ed.HintColor = colSubtle
							return ed.Layout(gtx)
						}),
					)
				})
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if u.admin {
					return layout.Dimensions{}
				}
				return primaryBtn(gtx, u.th, &u.elevate, "Запросить права")
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if !u.admin {
					return layout.Dimensions{}
				}
				label := "Включить VPN"
				if u.busy {
					label = "Поднимаю туннель…"
				} else if u.live {
					label = "Обновить ключ"
				}
				return primaryBtn(gtx, u.th, &u.connect, label)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ghostBtn(gtx, u.th, &u.disconnect, "Выключить VPN")
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				cb := material.CheckBox(u.th, &u.auto, "Автозапуск с Windows")
				cb.Color = colMuted
				cb.IconColor = colAccent
				return cb.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(18)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if u.err != "" {
					t := material.Body2(u.th, u.err)
					t.Color = colErr
					return t.Layout(gtx)
				}
				if u.live {
					ip := u.exitIP
					if ip == "" {
						ip = "проверяю…"
					}
					msg := "Адаптер Ply Tunnel поднят. Значок у часов.\nВыход  " + ip
					if u.detail != "" {
						msg += "\nУзел   " + u.detail
					}
					t := material.Body2(u.th, msg)
					t.Color = colOk
					return t.Layout(gtx)
				}
				t := material.Body2(u.th, "Пока выключено — Windows сидит на Wi‑Fi, это нормально.\nHapp и приложение Paper выключи.")
				t.Color = colSubtle
				return t.Layout(gtx)
			}),
			layout.Flexed(1, layout.Spacer{}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				hint := "Ply  ·  v" + core.Version + "  ·  Program Files  ·  трей"
				if runtime.GOOS != "windows" {
					hint = "Ply  ·  v" + core.Version
				}
				t := material.Caption(u.th, hint)
				t.Color = colSubtle
				t.Alignment = text.Middle
				return t.Layout(gtx)
			}),
		)
	})
}

func primaryBtn(gtx layout.Context, th *material.Theme, click *widget.Clickable, label string) layout.Dimensions {
	btn := material.Button(th, click, label)
	btn.Background = colAccent
	btn.Color = colInk
	btn.CornerRadius = unit.Dp(10)
	btn.Inset = layout.UniformInset(unit.Dp(14))
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return btn.Layout(gtx)
}

func ghostBtn(gtx layout.Context, th *material.Theme, click *widget.Clickable, label string) layout.Dimensions {
	btn := material.Button(th, click, label)
	btn.Background = colPanel
	btn.Color = colFg
	btn.CornerRadius = unit.Dp(10)
	btn.Inset = layout.UniformInset(unit.Dp(12))
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return btn.Layout(gtx)
}

func roundPanel(gtx layout.Context, w layout.Widget) layout.Dimensions {
	macro := op.Record(gtx.Ops)
	dims := layout.UniformInset(unit.Dp(16)).Layout(gtx, w)
	call := macro.Stop()
	r := image.Rectangle{Max: dims.Size}
	defer clip.UniformRRect(r, 14).Push(gtx.Ops).Pop()
	paint.Fill(gtx.Ops, colPanel)
	paint.FillShape(gtx.Ops, colLine, clip.Stroke{
		Path:  clip.UniformRRect(r, 14).Path(gtx.Ops),
		Width: 1,
	}.Op())
	call.Add(gtx.Ops)
	return dims
}
