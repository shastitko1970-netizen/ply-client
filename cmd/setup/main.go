package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"image"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	_ "embed"

	"gioui.org/app"
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

//go:embed payload.zip
var payload []byte

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
	elevate  widget.Clickable
	desk     widget.Bool
	auto     widget.Bool
	menu     widget.Bool
	stage    step
	progress float32
	status   string
	err      string
	dest     string
	oldVer   string
	admin    bool
}

func main() {
	go func() {
		w := new(app.Window)
		w.Option(
			app.Title("Ply — установка"),
			app.Size(unit.Dp(620), unit.Dp(520)),
			app.MinSize(unit.Dp(400), unit.Dp(440)),
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

	old := core.InstalledVersion()
	u := &ui{
		w:      w,
		th:     th,
		dest:   core.DefaultInstallDir(),
		oldVer: old,
		admin:  core.IsAdmin(),
		status: "Готов поставить Ply " + core.Version,
	}
	if old != "" {
		u.status = "Обновлю " + old + " → " + core.Version
	}
	u.desk.Value = true
	u.auto.Value = true
	u.menu.Value = true
	if !u.admin {
		u.err = "Нужны права администратора: туннель, меню Пуск и автозапуск."
	}

	var ops op.Ops
	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			u.update(gtx)
			paint.Fill(gtx.Ops, plyui.Bg)
			u.layout(gtx)
			e.Frame(gtx.Ops)
		}
	}
}

func (u *ui) update(gtx layout.Context) {
	_ = u.desk.Update(gtx)
	_ = u.auto.Update(gtx)
	_ = u.menu.Update(gtx)
	if u.elevate.Clicked(gtx) && !u.admin {
		if err := core.RelaunchElevated(); err != nil {
			u.err = err.Error()
			return
		}
		os.Exit(0)
	}
	if u.install.Clicked(gtx) && u.stage == stepWelcome && u.admin {
		u.stage = stepWork
		u.status = "Останавливаю старую версию"
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
		u.fail("в установщике нет пакета")
		return
	}
	core.StopPlyProcesses()
	time.Sleep(400 * time.Millisecond)

	zr, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		u.fail("архив: " + err.Error())
		return
	}
	u.status = "Ставлю ядро Xray"
	u.w.Invalidate()
	err = core.ExtractZip(zr, u.dest, func(done, total uint64) {
		if total > 0 {
			u.progress = float32(done) / float32(total) * 0.8
			u.w.Invalidate()
		}
	})
	if err != nil {
		u.fail(err.Error())
		return
	}
	ply := filepath.Join(u.dest, "Ply.exe")
	xray := filepath.Join(u.dest, "xray.exe")
	wintun := filepath.Join(u.dest, "wintun.dll")
	geosite := filepath.Join(u.dest, "geosite.dat")
	if _, err := os.Stat(ply); err != nil {
		u.fail("в пакете нет Ply.exe")
		return
	}
	if _, err := os.Stat(xray); err != nil {
		u.fail("в пакете нет xray.exe")
		return
	}
	if _, err := os.Stat(wintun); err != nil {
		u.fail("в пакете нет wintun.dll — без него туннель не встанет")
		return
	}
	if _, err := os.Stat(geosite); err != nil {
		u.fail("в пакете нет geosite.dat — без него Россия не уйдёт в обход")
		return
	}

	_ = core.WriteVersionFile(u.dest)
	readme := "Ply " + core.Version + "\r\n\r\n" +
		"КАК ЗАПУСТИТЬ\r\n" +
		"1. Ply стоит в Program Files. Ищи «Ply» в меню Пуск или на рабочем столе.\r\n" +
		"2. Согласись на права администратора.\r\n" +
		"3. Вставь ключ (Paper / vless / hy2 / vmess / trojan / ss), нажми кнопку питания.\r\n" +
		"4. Крестик сворачивает в трей — туннель живой. Выход только из значка у часов.\r\n" +
		"5. «Россия мимо» — .ru и российские сервисы без VPN.\r\n" +
		"6. Новые версии Ply скачает сама.\r\n\r\n" +
		"Happ и приложение Paper выключи.\r\n" +
		"Папка: " + u.dest + "\r\n"
	_ = os.WriteFile(filepath.Join(u.dest, "README.txt"), []byte(readme), 0644)
	_ = core.WriteUninstall(u.dest)
	core.MigrateLegacyData(u.dest)

	u.status = "Регистрация в Windows"
	u.progress = 0.85
	u.w.Invalidate()
	if err := core.RegisterApp(u.dest); err != nil {
		u.fail(err.Error())
		return
	}
	core.AllowFirewall(ply)
	core.AllowFirewall(xray)
	core.RemoveLegacyStartFolder()

	u.status = "Ярлыки"
	u.progress = 0.92
	u.w.Invalidate()

	if u.desk.Value || u.menu.Value {
		if err := core.InstallShortcuts(ply, u.dest); err != nil {
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
	core.NotifyShell()

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
	return layout.UniformInset(unit.Dp(28)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return plyui.Title(u.th, "Ply").Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				line := "Ядро Xray ставится само. Go качать не надо."
				if u.oldVer != "" && u.oldVer != core.Version {
					line = "Обновление " + u.oldVer + " → " + core.Version + ". Ссылка Paper останется."
				}
				b := material.Body2(u.th, line)
				b.Color = plyui.Muted
				return b.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(20)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return plyui.Card(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							l := material.Caption(u.th, "ПАПКА")
							l.Color = plyui.Dim
							return l.Layout(gtx)
						}),
						layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							t := material.Body1(u.th, u.dest)
							t.Color = plyui.Fg
							return t.Layout(gtx)
						}),
					)
				})
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if u.stage != stepWelcome {
					return layout.Dimensions{}
				}
				return plyui.Card(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return plyui.SwitchRow(gtx, u.th, &u.desk, "Ярлык на рабочем столе", "")
						}),
						layout.Rigid(plyui.Hairline),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return plyui.SwitchRow(gtx, u.th, &u.menu, "Меню Пуск — плитка Ply", "")
						}),
						layout.Rigid(plyui.Hairline),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return plyui.SwitchRow(gtx, u.th, &u.auto, "Автозапуск с Windows", "с правами администратора")
						}),
					)
				})
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if u.stage != stepWork {
					return layout.Dimensions{}
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						t := material.Body2(u.th, u.status)
						t.Color = plyui.Muted
						return t.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return progressBar(gtx, u.progress)
					}),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if u.stage != stepDone {
					return layout.Dimensions{}
				}
				t := material.Body2(u.th, "Ply в Program Files и в меню Пуск.\nЗапусти, кнопка питания — VPN, крестик — в трей.")
				t.Color = plyui.Ok
				return t.Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if u.err == "" {
					return layout.Dimensions{}
				}
				t := material.Body2(u.th, u.err)
				t.Color = plyui.Err
				return t.Layout(gtx)
			}),
			layout.Flexed(1, layout.Spacer{}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				switch {
				case !u.admin && u.stage == stepWelcome:
					return plyui.Primary(gtx, u.th, &u.elevate, "Запросить права")
				case u.stage == stepWork:
					t := material.Body2(u.th, "Не закрывай окно")
					t.Color = plyui.Dim
					t.Alignment = text.Middle
					return t.Layout(gtx)
				case u.stage == stepDone:
					return plyui.Primary(gtx, u.th, &u.launch, "Запустить Ply")
				default:
					label := "Установить"
					if u.oldVer != "" {
						label = "Обновить до " + core.Version
					}
					return plyui.Primary(gtx, u.th, &u.install, label)
				}
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				t := material.Caption(u.th, fmt.Sprintf("Ply  ·  v%s  ·  ядро Xray внутри", core.Version))
				t.Color = plyui.Dim
				t.Alignment = text.Middle
				return t.Layout(gtx)
			}),
		)
	})
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
	paint.FillShape(gtx.Ops, plyui.Line, rr.Op(gtx.Ops))
	fw := int(float32(w) * p)
	if fw > 0 {
		fr := clip.UniformRRect(image.Rect(0, 0, fw, h), h/2)
		paint.FillShape(gtx.Ops, plyui.Accent, fr.Op(gtx.Ops))
	}
	return layout.Dimensions{Size: image.Pt(w, h)}
}
