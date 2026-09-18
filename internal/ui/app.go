// Package ui 是 gzgspg 的 Fyne 界面层，只负责展示与交互。
package ui

import (
	"log/slog"
	"os"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"

	"github.com/summonhim/gzgspd/engine"
	"github.com/summonhim/gzgspg/internal/configdir"
	"github.com/summonhim/gzgspg/internal/controller"
)

// quitPause 是退出时用于让人看见「正在登出」的停顿。
const quitPause = 600 * time.Millisecond

// Run 启动 GUI，阻塞至退出。
func Run() error {
	a := app.NewWithID("top.summonhim.gzgspg")

	logPath, err := configdir.LogPath()
	if err != nil {
		return err
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer logFile.Close()

	logger := slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	cfgPath, err := configdir.ConfigPath()
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
	win.SetIcon(appIcon())
	win.Resize(fyne.NewSize(320, 480))

	// 三视图共用一个 Stack，切换时只替换内容
	stack := container.NewStack()

	// 闭包声明在前、赋值在后，方便视图之间互相引用
	var (
		home       *homePage
		run        *runPage
		tray       *trayController
		showHome   func()
		showRun    func()
		onLogin    func()
		onSettings func()
		onLogout   func()
		quit       func()
	)

	showHome = func() {
		home = newHomePage(ctrl, onLogin, onSettings)
		stack.Objects = []fyne.CanvasObject{home.root}
		stack.Refresh()
	}

	// showRun 切换到运行页。进入前先按当前状态设一次文案，
	// 之后由事件回调持续更新。
	showRun = func() {
		run = newRunPage(onLogout)
		run.setState(ctrl.State())
		stack.Objects = []fyne.CanvasObject{run.root}
		stack.Refresh()
	}

	onBackFromSettings := func(s *settingsPage) func() {
		return func() {
			// 离开设置页时收集，返回首页即生效
			s.collect()
			showHome()
		}
	}

	showSettings := func() {
		var s *settingsPage
		s = newSettingsPage(ctrl, func() { onBackFromSettings(s)() })
		stack.Objects = []fyne.CanvasObject{s.root}
		stack.Refresh()
	}

	onSettings = func() {
		// 先收集首页输入，避免切换时丢失
		if home != nil {
			home.collect()
		}
		showSettings()
	}

	onLogin = func() {
		if home == nil {
			return
		}
		home.collect()
		home.setStatus("")
		if err := ctrl.Save(); err != nil {
			home.setStatus("保存失败: " + err.Error())
			return
		}
		if err := ctrl.Start(); err != nil {
			home.setStatus("启动失败: " + err.Error())
			return
		}
		showRun()
	}

	onLogout = func() {
		go func() {
			// Stop 只发起取消；等引擎真正停稳（含登出）再切回首页，
			// 否则切页后残留的引擎事件无处可去。
			ctrl.Stop()
			ctrl.WaitStopped()
			fyne.Do(showHome)
		}()
	}

	// quit 处理托盘「退出」：运行中先显示运行页、完成登出再退出。
	quit = func() {
		if !ctrl.Running() {
			// 没有会话可登出，直接退出
			fyne.Do(func() { a.Quit() })
			return
		}
		go func() {
			fyne.Do(func() {
				win.Show()
				showRun()
			})
			ctrl.Stop()
			ctrl.WaitStopped()
			time.Sleep(quitPause)
			fyne.Do(func() { a.Quit() })
		}()
	}

	ctrl.Subscribe(func(ev engine.Event) {
		fyne.Do(func() {
			if run != nil {
				run.setState(ev.State)
			}
			tray.setState(ev.State)
		})
	})

	showHome()

	win.SetContent(stack)
	win.SetCloseIntercept(func() { win.Hide() })

	tray = setupTray(a, win,
		func() { fyne.Do(func() { win.Show() }) },
		quit,
	)

	win.ShowAndRun()
	return nil
}
