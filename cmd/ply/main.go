package main

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gioui.org/app"
	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/io/pointer"
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
	colField  = color.NRGBA{R: 12, G: 12, B: 14, A: 255}
	colFg     = color.NRGBA{R: 244, G: 244, B: 245, A: 255}
	colMuted  = color.NRGBA{R: 161, G: 161, B: 170, A: 255}
	colSubtle = color.NRGBA{R: 113, G: 113, B: 122, A: 255}
	colLine   = color.NRGBA{R: 244, G: 244, B: 245, A: 28}
	colLine2  = color.NRGBA{R: 244, G: 244, B: 245, A: 48}
	colAccent = color.NRGBA{R: 200, G: 204, B: 212, A: 255}
	colInk    = color.NRGBA{R: 10, G: 10, B: 11, A: 255}
	colOk     = color.NRGBA{R: 138, G: 163, B: 138, A: 255}
	colOkDim  = color.NRGBA{R: 138, G: 163, B: 138, A: 40}
	colErr    = color.NRGBA{R: 193, G: 123, B: 123, A: 255}
	colErrDim = color.NRGBA{R: 193, G: 123, B: 123, A: 28}
)

type ui struct {
	w        *app.Window
	th       *material.Theme
	url      widget.Editor
	power    widget.Clickable
	refresh  widget.Clickable
	elevate  widget.Clickable
	checkUpd widget.Clickable
	applyUpd widget.Clickable
	auto     widget.Bool
	split    widget.Bool
	busy     bool
	admin    bool
	trayOn   bool
	status   string
	detail   string
	err      string
	live     bool
	exitIP   string
	node     *core.Node
	upd      *core.Update
	updBusy  bool
	updNote  string
	t0       time.Time
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
			app.Size(unit.Dp(440), unit.Dp(780)),
			app.MinSize(unit.Dp(400), unit.Dp(640)),
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

	u := &ui{w: w, th: th, admin: core.IsAdmin(), status: "ожидание", t0: time.Now()}
	u.url.SingleLine = true
	u.url.Submit = true
	u.auto.Value = true
	u.split.Value = core.ReadSplit()
	if saved := core.ReadURL(); saved != "" {
		u.url.SetText(saved)
	}
	go func() {
		time.Sleep(1600 * time.Millisecond)
		u.checkUpdate(false)
	}()
	if u.admin {
		if saved := strings.TrimSpace(u.url.Text()); saved != "" {
			u.busy = true
			u.status = "включаю"
			go u.doConnect(saved, true)
		}
	} else {
		u.status = "нужны права"
		u.err = "Туннель без прав администратора не встанет."
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
			if u.live || u.busy {
				gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(80 * time.Millisecond)})
			}
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
			u.startConnect()
		}
	}
	if u.auto.Update(gtx) {
		exe, _ := os.Executable()
		_ = core.SetAutoStart(u.auto.Value, exe)
	}
	if u.split.Update(gtx) {
		_ = core.SaveSplit(u.split.Value)
		if u.live && u.admin && !u.busy {
			u.status = "маршрут"
			u.startConnect()
		}
	}
	if u.elevate.Clicked(gtx) && !u.admin {
		if err := core.RelaunchElevated(); err != nil {
			u.err = err.Error()
			u.w.Invalidate()
			return
		}
		os.Exit(0)
	}
	if u.power.Clicked(gtx) && !u.busy {
		if !u.admin {
			if err := core.RelaunchElevated(); err != nil {
				u.err = err.Error()
				u.w.Invalidate()
				return
			}
			os.Exit(0)
		}
		if u.live {
			u.busy = true
			go u.doDisconnect()
		} else {
			u.startConnect()
		}
	}
	if u.refresh.Clicked(gtx) && !u.busy && u.admin && u.live {
		u.startConnect()
	}
	if u.checkUpd.Clicked(gtx) && !u.updBusy {
		u.updBusy = true
		u.updNote = "ищу обновления…"
		go u.checkUpdate(true)
	}
	if u.applyUpd.Clicked(gtx) && !u.updBusy && u.upd != nil {
		u.updBusy = true
		u.updNote = "скачиваю установщик…"
		go u.applyUpdate()
	}
}

func (u *ui) startConnect() {
	src := strings.TrimSpace(u.url.Text())
	auto := u.auto.Value
	u.busy = true
	u.err = ""
	u.status = "включаю"
	go u.doConnect(src, auto)
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
	if s.Split {
		u.status = "Россия мимо"
	} else {
		u.status = "полный туннель"
	}
	u.detail = fmt.Sprintf("%s:%d", s.Node.Host, s.Node.Port)
	u.err = ""
	u.busy = false
	if auto {
		exe, _ := os.Executable()
		_ = core.SetAutoStart(true, exe)
	}
	u.w.Invalidate()
	go u.checkUpdate(false)
}

func (u *ui) doDisconnect() {
	_ = core.Disconnect()
	u.live = false
	u.status = "ожидание"
	u.detail = ""
	u.exitIP = ""
	u.err = ""
	u.busy = false
	u.w.Invalidate()
}

func (u *ui) checkUpdate(manual bool) {
	upd, err := core.CheckLatest()
	u.updBusy = false
	if err != nil {
		if manual {
			u.updNote = err.Error()
		}
		u.w.Invalidate()
		return
	}
	if upd == nil {
		u.upd = nil
		if manual {
			u.updNote = "уже свежая v" + core.Version
		}
		u.w.Invalidate()
		return
	}
	u.upd = upd
	u.updNote = ""
	u.w.Invalidate()
}

func (u *ui) applyUpdate() {
	if u.upd == nil || u.upd.SetupURL == "" {
		u.updBusy = false
		u.updNote = "нет ссылки на установщик"
		u.w.Invalidate()
		return
	}
	dest := filepath.Join(os.TempDir(), "PlySetup-"+u.upd.Tag+".exe")
	if err := core.DownloadSetup(u.upd.SetupURL, dest); err != nil {
		u.updBusy = false
		u.updNote = err.Error()
		u.w.Invalidate()
		return
	}
	u.updNote = "запускаю установщик — Ply закроется"
	u.w.Invalidate()
	_ = core.Disconnect()
	if err := core.RunInstaller(dest); err != nil {
		u.updBusy = false
		u.updNote = err.Error()
		u.w.Invalidate()
		return
	}
	core.RemoveTray()
	os.Exit(0)
}

func (u *ui) layout(gtx layout.Context) layout.Dimensions {
	return layout.UniformInset(unit.Dp(24)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(u.layoutHeader),
			layout.Rigid(layout.Spacer{Height: unit.Dp(28)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Center.Layout(gtx, u.layoutPower)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(14)}.Layout),
			layout.Rigid(u.layoutPowerCaption),
			layout.Rigid(layout.Spacer{Height: unit.Dp(22)}.Layout),
			layout.Rigid(u.layoutURL),
			layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
			layout.Rigid(u.layoutOptions),
			layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
			layout.Rigid(u.layoutMeta),
			layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
			layout.Rigid(u.layoutUpdate),
			layout.Flexed(1, layout.Spacer{}.Layout),
			layout.Rigid(u.layoutFooter),
		)
	})
}

func (u *ui) layoutHeader(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			t := material.H3(u.th, "Ply")
			t.Color = colFg
			t.Font.Style = font.Italic
			t.Font.Weight = font.Medium
			return t.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := u.status
			fg, bg := colMuted, colPanel
			switch {
			case u.err != "":
				fg, bg, label = colErr, colErrDim, "ошибка"
			case u.busy:
				fg, bg, label = colMuted, colPanel, "подключаю"
			case u.live:
				fg, bg = colOk, colOkDim
			}
			return pill(gtx, u.th, label, fg, bg)
		}),
	)
}

func (u *ui) layoutPower(gtx layout.Context) layout.Dimensions {
	d := gtx.Dp(124)
	gtx.Constraints = layout.Exact(image.Pt(d, d))
	return u.power.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		defer clip.Ellipse{Max: image.Pt(d, d)}.Push(gtx.Ops).Pop()
		bg := colPanel
		ring := colLine2
		icon := colFg
		if u.live {
			bg = color.NRGBA{R: 16, G: 20, B: 16, A: 255}
			ring = colOk
			icon = colOk
			pulse := 0.5 + 0.5*math.Sin(time.Since(u.t0).Seconds()*2.2)
			ring.A = uint8(90 + 80*pulse)
		} else if u.err != "" {
			ring = colErr
			icon = colErr
		}
		paint.Fill(gtx.Ops, bg)
		paint.FillShape(gtx.Ops, ring, clip.Stroke{
			Path:  clip.Ellipse{Max: image.Pt(d, d)}.Path(gtx.Ops),
			Width: float32(gtx.Dp(1.5)),
		}.Op())
		if u.live {
			m := gtx.Dp(7)
			inner := image.Rect(m, m, d-m, d-m)
			paint.FillShape(gtx.Ops, color.NRGBA{R: 138, G: 163, B: 138, A: 70}, clip.Stroke{
				Path:  clip.Ellipse{Min: inner.Min, Max: inner.Max}.Path(gtx.Ops),
				Width: 1,
			}.Op())
		}
		drawPowerIcon(gtx.Ops, icon, d, u.live)
		return layout.Dimensions{Size: image.Pt(d, d)}
	})
}

func (u *ui) layoutPowerCaption(gtx layout.Context) layout.Dimensions {
	msg := "нажми, чтобы включить"
	c := colSubtle
	switch {
	case !u.admin:
		msg = "нужны права администратора"
		c = colErr
	case u.busy:
		msg = "поднимаю туннель…"
		c = colMuted
	case u.live:
		msg = "нажми, чтобы выключить"
		c = colMuted
	}
	t := material.Body2(u.th, msg)
	t.Color = c
	t.Alignment = text.Middle
	return t.Layout(gtx)
}

func (u *ui) layoutURL(gtx layout.Context) layout.Dimensions {
	return panel(gtx, 20, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				l := material.Caption(u.th, "ССЫЛКА PAPER")
				l.Color = colSubtle
				return l.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return field(gtx, 10, func(gtx layout.Context) layout.Dimensions {
					ed := material.Editor(u.th, &u.url, "https://…azure-api.net/…")
					ed.Color = colFg
					ed.HintColor = colSubtle
					return ed.Layout(gtx)
				})
			}),
		)
	})
}

func (u *ui) layoutOptions(gtx layout.Context) layout.Dimensions {
	return panel(gtx, 20, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return optionRow(gtx, u.th, &u.split, "Россия напрямую", ".ru  ·  .рф  ·  Яндекс  ·  VK")
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return hairline(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return optionRow(gtx, u.th, &u.auto, "Автозапуск", "вместе с Windows, с правами")
			}),
		)
	})
}

func (u *ui) layoutMeta(gtx layout.Context) layout.Dimensions {
	if u.err != "" {
		return panelTint(gtx, 20, colErrDim, func(gtx layout.Context) layout.Dimensions {
			t := material.Body2(u.th, u.err)
			t.Color = colErr
			return t.Layout(gtx)
		})
	}
	if !u.admin {
		return primaryBtn(gtx, u.th, &u.elevate, "Запросить права")
	}
	if u.live {
		ip := u.exitIP
		if ip == "" {
			ip = "проверяю…"
		}
		sni := ""
		if u.node != nil {
			sni = u.node.SNI
		}
		route := "полный туннель"
		if u.split.Value {
			route = "Россия мимо · остальное в туннель"
		}
		return panel(gtx, 20, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					l := material.Caption(u.th, "ВЫХОД")
					l.Color = colSubtle
					return l.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					t := material.H6(u.th, ip)
					t.Color = colOk
					t.Font.Weight = font.Medium
					return t.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					t := material.Caption(u.th, route)
					t.Color = colMuted
					return t.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if u.detail == "" {
						return layout.Dimensions{}
					}
					line := "узел  " + u.detail
					if sni != "" {
						line += "  ·  " + sni
					}
					t := material.Caption(u.th, line)
					t.Color = colSubtle
					return t.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ghostBtn(gtx, u.th, &u.refresh, "Обновить ключ")
				}),
			)
		})
	}
	return panel(gtx, 20, func(gtx layout.Context) layout.Dimensions {
		t := material.Body2(u.th, "Пока выключено Windows сидит на Wi‑Fi — так и должно. Happ и приложение Paper выключи.")
		t.Color = colSubtle
		return t.Layout(gtx)
	})
}

func (u *ui) layoutUpdate(gtx layout.Context) layout.Dimensions {
	if u.upd != nil {
		return panel(gtx, 20, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					t := material.Body2(u.th, "Вышла v"+u.upd.Tag)
					t.Color = colFg
					return t.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					t := material.Caption(u.th, "можно поставить из приложения, ссылка Paper останется")
					t.Color = colSubtle
					return t.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					label := "Обновить до " + u.upd.Tag
					if u.updBusy {
						label = "Скачиваю установщик…"
					}
					return primaryBtn(gtx, u.th, &u.applyUpd, label)
				}),
			)
		})
	}
	label := "Проверить обновления"
	if u.updBusy {
		label = "Ищу на GitHub…"
	}
	if u.updNote != "" {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return quietBtn(gtx, u.th, &u.checkUpd, label)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				t := material.Caption(u.th, u.updNote)
				t.Color = colSubtle
				t.Alignment = text.Middle
				return t.Layout(gtx)
			}),
		)
	}
	return quietBtn(gtx, u.th, &u.checkUpd, label)
}

func (u *ui) layoutFooter(gtx layout.Context) layout.Dimensions {
	hint := "Ply  ·  v" + core.Version + "  ·  крестик в трей"
	if runtime.GOOS != "windows" {
		hint = "Ply  ·  v" + core.Version
	}
	t := material.Caption(u.th, hint)
	t.Color = colSubtle
	t.Alignment = text.Middle
	return t.Layout(gtx)
}

func optionRow(gtx layout.Context, th *material.Theme, b *widget.Bool, title, sub string) layout.Dimensions {
	return layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						t := material.Body1(th, title)
						t.Color = colFg
						return t.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						t := material.Caption(th, sub)
						t.Color = colSubtle
						return t.Layout(gtx)
					}),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				sw := material.Switch(th, b, title)
				sw.Color.Enabled = colOk
				sw.Color.Disabled = colLine2
				return sw.Layout(gtx)
			}),
		)
	})
}

func pill(gtx layout.Context, th *material.Theme, label string, fg, bg color.NRGBA) layout.Dimensions {
	macro := op.Record(gtx.Ops)
	dims := layout.Inset{Top: unit.Dp(5), Bottom: unit.Dp(5), Left: unit.Dp(10), Right: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		t := material.Caption(th, label)
		t.Color = fg
		return t.Layout(gtx)
	})
	call := macro.Stop()
	r := image.Rectangle{Max: dims.Size}
	defer clip.UniformRRect(r, dims.Size.Y/2).Push(gtx.Ops).Pop()
	paint.Fill(gtx.Ops, bg)
	call.Add(gtx.Ops)
	return dims
}

func panel(gtx layout.Context, rad int, w layout.Widget) layout.Dimensions {
	return panelTint(gtx, rad, colPanel, w)
}

func panelTint(gtx layout.Context, rad int, bg color.NRGBA, w layout.Widget) layout.Dimensions {
	macro := op.Record(gtx.Ops)
	dims := layout.UniformInset(unit.Dp(16)).Layout(gtx, w)
	call := macro.Stop()
	r := image.Rectangle{Max: dims.Size}
	rr := gtx.Dp(unit.Dp(rad))
	defer clip.UniformRRect(r, rr).Push(gtx.Ops).Pop()
	paint.Fill(gtx.Ops, bg)
	paint.FillShape(gtx.Ops, colLine, clip.Stroke{
		Path:  clip.UniformRRect(r, rr).Path(gtx.Ops),
		Width: 1,
	}.Op())
	call.Add(gtx.Ops)
	return dims
}

func field(gtx layout.Context, rad int, w layout.Widget) layout.Dimensions {
	macro := op.Record(gtx.Ops)
	dims := layout.Inset{Top: unit.Dp(10), Bottom: unit.Dp(10), Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, w)
	call := macro.Stop()
	r := image.Rectangle{Max: dims.Size}
	rr := gtx.Dp(unit.Dp(rad))
	defer clip.UniformRRect(r, rr).Push(gtx.Ops).Pop()
	paint.Fill(gtx.Ops, colField)
	paint.FillShape(gtx.Ops, colLine, clip.Stroke{
		Path:  clip.UniformRRect(r, rr).Path(gtx.Ops),
		Width: 1,
	}.Op())
	call.Add(gtx.Ops)
	return dims
}

func hairline(gtx layout.Context) layout.Dimensions {
	h := gtx.Dp(1)
	w := gtx.Constraints.Max.X
	defer clip.Rect{Max: image.Pt(w, h)}.Push(gtx.Ops).Pop()
	paint.Fill(gtx.Ops, colLine)
	return layout.Dimensions{Size: image.Pt(w, h+gtx.Dp(8))}
}

func primaryBtn(gtx layout.Context, th *material.Theme, click *widget.Clickable, label string) layout.Dimensions {
	btn := material.Button(th, click, label)
	btn.Background = colAccent
	btn.Color = colInk
	btn.CornerRadius = unit.Dp(12)
	btn.Inset = layout.UniformInset(unit.Dp(12))
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return btn.Layout(gtx)
}

func ghostBtn(gtx layout.Context, th *material.Theme, click *widget.Clickable, label string) layout.Dimensions {
	btn := material.Button(th, click, label)
	btn.Background = colField
	btn.Color = colFg
	btn.CornerRadius = unit.Dp(12)
	btn.Inset = layout.UniformInset(unit.Dp(10))
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return btn.Layout(gtx)
}

func quietBtn(gtx layout.Context, th *material.Theme, click *widget.Clickable, label string) layout.Dimensions {
	btn := material.Button(th, click, label)
	btn.Background = color.NRGBA{}
	btn.Color = colSubtle
	btn.CornerRadius = unit.Dp(8)
	btn.Inset = layout.UniformInset(unit.Dp(8))
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return btn.Layout(gtx)
}

func drawPowerIcon(ops *op.Ops, col color.NRGBA, d int, live bool) {
	cx := float32(d) / 2
	cy := float32(d) / 2
	s := float32(d)
	w := s * 0.045
	if live {
		r := int(s * 0.07)
		min := image.Pt(int(cx)-r, int(cy)-r)
		max := image.Pt(int(cx)+r, int(cy)+r)
		defer clip.Ellipse{Min: min, Max: max}.Push(ops).Pop()
		paint.Fill(ops, col)
		return
	}
	var p clip.Path
	p.Begin(ops)
	p.MoveTo(f32.Pt(cx, cy-s*0.22))
	p.LineTo(f32.Pt(cx, cy-s*0.02))
	stem := p.End()
	paint.FillShape(ops, col, clip.Stroke{Path: stem, Width: w}.Op())

	p.Begin(ops)
	r := s * 0.18
	start, end := 130.0, 410.0
	steps := 28
	for i := 0; i <= steps; i++ {
		a := (start + (end-start)*float64(i)/float64(steps)) * math.Pi / 180
		x := cx + r*float32(math.Cos(a))
		y := cy + r*0.08 + r*float32(math.Sin(a))
		if i == 0 {
			p.MoveTo(f32.Pt(x, y))
		} else {
			p.LineTo(f32.Pt(x, y))
		}
	}
	arc := p.End()
	paint.FillShape(ops, col, clip.Stroke{Path: arc, Width: w}.Op())
}
