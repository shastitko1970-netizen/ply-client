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
	plyui "ply/internal/ui"
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
			app.Size(unit.Dp(440), unit.Dp(760)),
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
	th := plyui.NewTheme()

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
			paint.Fill(gtx.Ops, plyui.Bg)
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
			layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
			layout.Rigid(u.layoutPowerCaption),
			layout.Rigid(layout.Spacer{Height: unit.Dp(24)}.Layout),
			layout.Rigid(u.layoutURL),
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
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return plyui.Title(u.th, "Ply").Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					label := u.status
					fg, bg := plyui.Muted, plyui.Subtle
					switch {
					case u.err != "":
						fg, bg, label = plyui.Err, plyui.ErrDim, "ошибка"
					case u.busy:
						fg, bg, label = plyui.Muted, plyui.Subtle, "подключаю"
					case u.live:
						fg, bg = plyui.Ok, plyui.OkDim
					}
					return plyui.Pill(gtx, u.th, label, fg, bg)
				}),
			)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			t := material.Body2(u.th, "Windows-клиент Paper. Туннель, не системный прокси. Ссылка из кабинета — ключ чистится сам.")
			t.Color = plyui.Muted
			return t.Layout(gtx)
		}),
	)
}

func (u *ui) layoutPower(gtx layout.Context) layout.Dimensions {
	d := gtx.Dp(112)
	gtx.Constraints = layout.Exact(image.Pt(d, d))
	return u.power.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		defer clip.Ellipse{Max: image.Pt(d, d)}.Push(gtx.Ops).Pop()
		bg := plyui.Panel
		ring := plyui.Line2
		icon := plyui.Fg
		if u.live {
			bg = color.NRGBA{R: 16, G: 20, B: 16, A: 255}
			ring = plyui.Ok
			icon = plyui.Ok
			pulse := 0.5 + 0.5*math.Sin(time.Since(u.t0).Seconds()*2.2)
			ring.A = uint8(90 + 80*pulse)
		} else if u.err != "" {
			ring = plyui.Err
			icon = plyui.Err
		} else if u.busy {
			ring = plyui.Muted
			icon = plyui.Muted
		}
		paint.Fill(gtx.Ops, bg)
		paint.FillShape(gtx.Ops, ring, clip.Stroke{
			Path:  clip.Ellipse{Max: image.Pt(d, d)}.Path(gtx.Ops),
			Width: float32(gtx.Dp(1.5)),
		}.Op())
		if u.live {
			m := gtx.Dp(6)
			inner := image.Rect(m, m, d-m, d-m)
			paint.FillShape(gtx.Ops, color.NRGBA{R: 138, G: 163, B: 138, A: 90}, clip.Stroke{
				Path:  clip.Ellipse{Min: inner.Min, Max: inner.Max}.Path(gtx.Ops),
				Width: 1,
			}.Op())
		}
		if u.busy {
			drawBusyArc(gtx.Ops, d, time.Since(u.t0).Seconds())
		} else {
			drawPowerIcon(gtx.Ops, icon, d)
		}
		return layout.Dimensions{Size: image.Pt(d, d)}
	})
}

func (u *ui) layoutPowerCaption(gtx layout.Context) layout.Dimensions {
	msg := "Вставь ссылку, затем нажми"
	c := plyui.Muted
	switch {
	case !u.admin:
		msg = "нужны права администратора"
		c = plyui.Err
	case u.busy:
		msg = "поднимаю туннель…"
		c = plyui.Muted
	case u.live:
		if u.detail != "" {
			msg = u.detail
		} else {
			msg = "нажми, чтобы выключить"
		}
		c = plyui.Muted
	}
	t := material.Body2(u.th, msg)
	t.Color = c
	t.Alignment = text.Middle
	return t.Layout(gtx)
}

func (u *ui) layoutURL(gtx layout.Context) layout.Dimensions {
	return plyui.Card(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				t := material.Body2(u.th, "Ссылка Paper")
				t.Color = plyui.Fg
				return t.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return plyui.SwitchLabel(gtx, u.th, &u.split, "Россия мимо")
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(16)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return plyui.SwitchLabel(gtx, u.th, &u.auto, "Автозапуск")
					}),
				)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return plyui.Input(gtx, func(gtx layout.Context) layout.Dimensions {
					ed := material.Editor(u.th, &u.url, "https://…azure-api.net/…  или  vless://")
					ed.Color = plyui.Fg
					ed.HintColor = plyui.Dim
					ed.TextSize = 13
					return ed.Layout(gtx)
				})
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				t := material.Caption(u.th, "Хвост #PaperVPN и второй # отрежутся сами. Россия мимо — .ru, .рф, Яндекс, VK. Остальное через Paper.")
				t.Color = plyui.Dim
				return t.Layout(gtx)
			}),
		)
	})
}

func (u *ui) layoutMeta(gtx layout.Context) layout.Dimensions {
	if u.err != "" {
		return plyui.CardTint(gtx, plyui.ErrDim, func(gtx layout.Context) layout.Dimensions {
			t := material.Body2(u.th, u.err)
			t.Color = plyui.Err
			return t.Layout(gtx)
		})
	}
	if !u.admin {
		return plyui.Primary(gtx, u.th, &u.elevate, "Запросить права")
	}
	if !u.live {
		return layout.Dimensions{}
	}
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
	return plyui.Card(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				l := material.Caption(u.th, "ВЫХОД")
				l.Color = plyui.Dim
				return l.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				t := material.H6(u.th, ip)
				t.Color = plyui.Ok
				return t.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				t := material.Caption(u.th, route)
				t.Color = plyui.Muted
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
				t.Color = plyui.Dim
				return t.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return plyui.Ghost(gtx, u.th, &u.refresh, "Обновить ключ")
			}),
		)
	})
}

func (u *ui) layoutUpdate(gtx layout.Context) layout.Dimensions {
	if u.upd != nil {
		return plyui.Card(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					t := material.Body2(u.th, "Вышла v"+u.upd.Tag)
					t.Color = plyui.Fg
					return t.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					t := material.Caption(u.th, "можно поставить из приложения, ссылка Paper останется")
					t.Color = plyui.Dim
					return t.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					label := "Обновить до " + u.upd.Tag
					if u.updBusy {
						label = "Скачиваю установщик…"
					}
					return plyui.Primary(gtx, u.th, &u.applyUpd, label)
				}),
			)
		})
	}
	return layout.Dimensions{}
}

func (u *ui) layoutFooter(gtx layout.Context) layout.Dimensions {
	hint := "Ply  ·  v" + core.Version + "  ·  крестик в трей"
	if runtime.GOOS != "windows" {
		hint = "Ply  ·  v" + core.Version
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			t := material.Caption(u.th, hint)
			t.Color = plyui.Dim
			t.Alignment = text.Middle
			return t.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if u.upd != nil {
				return layout.Dimensions{}
			}
			label := "проверить обновления"
			if u.updBusy {
				label = "ищу на GitHub…"
			} else if u.updNote != "" {
				label = u.updNote
			}
			return plyui.Quiet(gtx, u.th, &u.checkUpd, label)
		}),
	)
}

func drawPowerIcon(ops *op.Ops, col color.NRGBA, d int) {
	cx := float32(d) / 2
	cy := float32(d) / 2
	s := float32(d)
	w := s * 0.042
	var p clip.Path
	p.Begin(ops)
	p.MoveTo(f32.Pt(cx, cy-s*0.22))
	p.LineTo(f32.Pt(cx, cy-s*0.01))
	stem := p.End()
	paint.FillShape(ops, col, clip.Stroke{Path: stem, Width: w}.Op())

	p.Begin(ops)
	r := s * 0.18
	start, end := 128.0, 412.0
	steps := 32
	for i := 0; i <= steps; i++ {
		a := (start + (end-start)*float64(i)/float64(steps)) * math.Pi / 180
		x := cx + r*float32(math.Cos(a))
		y := cy + r*0.06 + r*float32(math.Sin(a))
		if i == 0 {
			p.MoveTo(f32.Pt(x, y))
		} else {
			p.LineTo(f32.Pt(x, y))
		}
	}
	arc := p.End()
	paint.FillShape(ops, col, clip.Stroke{Path: arc, Width: w}.Op())
}

func drawBusyArc(ops *op.Ops, d int, sec float64) {
	cx := float32(d) / 2
	cy := float32(d) / 2
	s := float32(d)
	r := s * 0.18
	w := s * 0.042
	rot := sec * 240
	var p clip.Path
	p.Begin(ops)
	steps := 20
	for i := 0; i <= steps; i++ {
		a := (rot + float64(i)*12) * math.Pi / 180
		x := cx + r*float32(math.Cos(a))
		y := cy + r*float32(math.Sin(a))
		if i == 0 {
			p.MoveTo(f32.Pt(x, y))
		} else {
			p.LineTo(f32.Pt(x, y))
		}
	}
	arc := p.End()
	paint.FillShape(ops, plyui.Muted, clip.Stroke{Path: arc, Width: w}.Op())
}
