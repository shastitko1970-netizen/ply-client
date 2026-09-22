package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"image"
	"image/color"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	_ "embed"

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

//go:embed payload.zip
var payload []byte

var (
	colBg     = color.NRGBA{R: 10, G: 10, B: 11, A: 255}
	colPanel  = color.NRGBA{R: 18, G: 18, B: 20, A: 255}
	colFg     = color.NRGBA{R: 244, G: 244, B: 245, A: 255}
	colMuted  = color.NRGBA{R: 161, G: 161, B: 170, A: 255}
	colSubtle = color.NRGBA{R: 113, G: 113, B: 122, A: 255}
	colLine   = color.NRGBA{R: 244, G: 244, B: 245, A: 28}
	colAccent = color.NRGBA{R: 200, G: 204, B: 212, A: 255}
	colInk    = color.NRGBA{R: 10, G: 10, B: 11, A: 255}
	colErr    = color.NRGBA{R: 193, G: 123, B: 123, A: 255}
)

type step int

const (
	stepWelcome step = iota
	stepWork
	stepDone
)

type ui struct {
	w        *app.Window
	th       *material.Theme
	install  widget.Clickable
	launch   widget.Clickable
	desk     widget.Bool
	auto     widget.Bool
	menu     widget.Bool
	stage    step
	progress float32
	status   string
	err      string
	dest     string
}

func main() {
	go func() {
		w := new(app.Window)
		w.Option(
			app.Title("Ply — установка"),
			app.Size(unit.Dp(460), unit.Dp(620)),
			app.MinSize(unit.Dp(400), unit.Dp(540)),
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

	u := &ui{
		w:      w,
		th:     th,
		dest:   core.DefaultInstallDir(),
		status: "Готов поставить Ply",
	}
	u.desk.Value = true
	u.auto.Value = true
	u.menu.Value = true

	var ops op.Ops
	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			u.update(gtx)
			paint.Fill(gtx.Ops, colBg)
			u.layout(gtx)
			e.Frame(gtx.Ops)
		}
	}
}

func (u *ui) update(gtx layout.Context) {
	_ = u.desk.Update(gtx)
	_ = u.auto.Update(gtx)
	_ = u.menu.Update(gtx)
	if u.install.Clicked(gtx) && u.stage == stepWelcome {
		u.stage = stepWork
		u.status = "Распаковка"
		u.err = ""
		go u.doInstall()
	}
	if u.launch.Clicked(gtx) && u.stage == stepDone {
		exe := filepath.Join(u.dest, "Ply.exe")
		cmd := exec.Command(exe)
		cmd.Dir = u.dest
		_ = cmd.Start()
		os.Exit(0)
	}
}

func (u *ui) doInstall() {
	if len(payload) < 64 {
		u.fail("в установщике нет пакета — собери через GitHub Actions")
		return
	}
	zr, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		u.fail("архив: " + err.Error())
		return
	}
	u.status = "Распаковка"
	u.w.Invalidate()
	err = core.ExtractZip(zr, u.dest, func(done, total uint64) {
		if total > 0 {
			u.progress = float32(done) / float32(total)
			u.w.Invalidate()
		}
	})
	if err != nil {
		u.fail(err.Error())
		return
	}
	ply := filepath.Join(u.dest, "Ply.exe")
	xray := filepath.Join(u.dest, "xray.exe")
	if _, err := os.Stat(ply); err != nil {
		u.fail("в пакете нет Ply.exe")
		return
	}
	if _, err := os.Stat(xray); err != nil {
		u.fail("в пакете нет xray.exe")
		return
	}

	readme := "Ply " + core.Version + "\r\n\r\n" +
		"Вставь ссылку из кабинета Paper.\r\n" +
		"Ключ с двумя # чистится сам. Mux выключен.\r\n" +
		"Системный прокси 127.0.0.1:10808.\r\n\r\n" +
		"Happ и приложение Paper выключи — один слой.\r\n\r\n" +
		"Папка: " + u.dest + "\r\n"
	_ = os.WriteFile(filepath.Join(u.dest, "README.txt"), []byte(readme), 0644)

	u.status = "Ярлыки"
	u.progress = 0.92
	u.w.Invalidate()

	if u.desk.Value {
		link := filepath.Join(core.DesktopDir(), "Ply.lnk")
		if err := core.CreateShortcut(link, ply, u.dest, "Ply — клиент Paper"); err != nil {
			u.fail(err.Error())
			return
		}
	}
	if u.menu.Value {
		link := filepath.Join(core.StartMenuDir(), "Ply.lnk")
		if err := core.CreateShortcut(link, ply, u.dest, "Ply — клиент Paper"); err != nil {
			u.fail(err.Error())
			return
		}
	}
	if u.auto.Value {
		if err := core.SetAutoStart(true, ply); err != nil {
			u.fail(err.Error())
			return
		}
	}

	u.progress = 1
	u.stage = stepDone
	u.status = "Готово"
	u.w.Invalidate()
}

func (u *ui) fail(msg string) {
	u.err = msg
	u.stage = stepWelcome
	u.status = "ошибка"
	u.progress = 0
	u.w.Invalidate()
}

func (u *ui) layout(gtx layout.Context) layout.Dimensions {
	return layout.UniformInset(unit.Dp(32)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				t := material.H3(u.th, "Ply")
				t.Color = colFg
				t.Font.Style = font.Italic
				return t.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				b := material.Body2(u.th, "Установщик Windows. Ядро Xray внутри, без v2rayN.")
				b.Color = colMuted
				return b.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(28)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return roundPanel(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							l := material.Caption(u.th, "ПАПКА")
							l.Color = colSubtle
							return l.Layout(gtx)
						}),
						layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							t := material.Body1(u.th, u.dest)
							t.Color = colFg
							return t.Layout(gtx)
						}),
					)
				})
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(18)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if u.stage != stepWelcome {
					return layout.Dimensions{}
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						cb := material.CheckBox(u.th, &u.desk, "Ярлык на рабочем столе")
						cb.Color = colMuted
						cb.IconColor = colAccent
						return cb.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						cb := material.CheckBox(u.th, &u.menu, "Меню Пуск")
						cb.Color = colMuted
						cb.IconColor = colAccent
						return cb.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						cb := material.CheckBox(u.th, &u.auto, "Автозапуск с Windows")
						cb.Color = colMuted
						cb.IconColor = colAccent
						return cb.Layout(gtx)
					}),
				)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if u.stage != stepWork {
					return layout.Dimensions{}
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						t := material.Body2(u.th, u.status)
						t.Color = colMuted
						return t.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return progressBar(gtx, u.progress)
					}),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if u.err == "" {
					return layout.Dimensions{}
				}
				t := material.Body2(u.th, u.err)
				t.Color = colErr
				return t.Layout(gtx)
			}),
			layout.Flexed(1, layout.Spacer{}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				switch u.stage {
				case stepWork:
					t := material.Body2(u.th, "Не закрывай окно")
					t.Color = colSubtle
					t.Alignment = text.Middle
					return t.Layout(gtx)
				case stepDone:
					return primaryBtn(gtx, u.th, &u.launch, "Запустить Ply")
				default:
					return primaryBtn(gtx, u.th, &u.install, "Установить")
				}
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				t := material.Caption(u.th, fmt.Sprintf("Ply  ·  v%s  ·  Paper  ·  Xray", core.Version))
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

func progressBar(gtx layout.Context, p float32) layout.Dimensions {
	if p < 0 {
		p = 0
	}
	if p > 1 {
		p = 1
	}
	h := gtx.Dp(6)
	w := gtx.Constraints.Max.X
	rr := clip.UniformRRect(image.Rect(0, 0, w, h), h/2)
	paint.FillShape(gtx.Ops, colLine, rr.Op(gtx.Ops))
	fw := int(float32(w) * p)
	if fw > 0 {
		fr := clip.UniformRRect(image.Rect(0, 0, fw, h), h/2)
		paint.FillShape(gtx.Ops, colAccent, fr.Op(gtx.Ops))
	}
	return layout.Dimensions{Size: image.Pt(w, h)}
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
