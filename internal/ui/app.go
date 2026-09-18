// Package ui 是 gzgspg 的 Fyne 界面层，只负责展示与交互。
package ui

import (
	"fmt"
	"log/slog"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/summonhim/gzgspd/engine"
	"github.com/summonhim/gzgspg/internal/controller"
	"github.com/summonhim/gzgspg/internal/logwriter"
)

// Run 启动 GUI，阻塞至退出。
func Run() error {
	a := app.NewWithID("top.summonhim.gzgspg")

	logBuf := logwriter.New(1000)
	logger := slog.New(slog.NewTextHandler(logBuf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	cfgPath, err := filepath.Abs("config.json")
	if err != nil {
		return err
	}
	ctrl, err := controller.New(controller.Options{
		ConfigPath: cfgPath,
		Logger:     logger,
	})
	if err != nil {
		return err
	}

	win := a.NewWindow("广工商校园网登录器")
	win.Resize(fyne.NewSize(560, 640))

	ed := newEditor(ctrl)
	status := widget.NewLabel("未运行")
	logs := newLogPanel(logBuf)

	var startBtn, stopBtn *widget.Button
	startBtn = widget.NewButton("启动", func() {
		ed.collect()
		if err := ctrl.Save(); err != nil {
			logger.Error("save config failed", "error", err)
		}
		startBtn.Disable()
		stopBtn.Enable()
		status.SetText("启动中…")
		// Start 非阻塞，可直接调用；失败时恢复按钮状态
		if err := ctrl.Start(); err != nil {
			status.SetText("启动失败: " + err.Error())
			startBtn.Enable()
			stopBtn.Disable()
		}
	})
	stopBtn = widget.NewButton("停止", func() {
		stopBtn.Disable()
		status.SetText("停止中…")
		go func() {
			ctrl.Stop()
			fyne.Do(func() {
				startBtn.Enable()
				stopBtn.Disable()
				status.SetText("已停止")
			})
		}()
	})
	stopBtn.Disable()

	saveBtn := widget.NewButton("保存", func() {
		ed.collect()
		if err := ctrl.Save(); err != nil {
			status.SetText("保存失败: " + err.Error())
			return
		}
		status.SetText("已保存")
	})

	ctrl.Subscribe(func(ev engine.Event) {
		fyne.Do(func() {
			status.SetText(statusText(ev))
		})
	})

	bottom := container.NewHBox(saveBtn, startBtn, stopBtn)
	content := container.NewBorder(
		nil,
		container.NewVBox(container.NewHBox(status), bottom),
		nil, nil,
		container.NewVSplit(ed.root, container.NewBorder(
			widget.NewLabel("日志"), nil, nil, nil, logs.box,
		)),
	)
	win.SetContent(content)

	// 关窗只隐藏（托盘继续运行）
	win.SetCloseIntercept(func() {
		win.Hide()
	})

	setupTray(a, win, ctrl, status, startBtn, stopBtn)

	win.ShowAndRun()
	return nil
}

func statusText(ev engine.Event) string {
	if ev.Err != nil {
		return fmt.Sprintf("%s: %v", ev.State.String(), ev.Err)
	}
	if ev.Message != "" {
		return fmt.Sprintf("%s - %s", ev.State.String(), ev.Message)
	}
	return ev.State.String()
}
