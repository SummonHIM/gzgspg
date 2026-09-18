package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/summonhim/gzgspg/internal/controller"
)

// setupTray 在支持托盘的平台上安装托盘图标与菜单。
func setupTray(a fyne.App, win fyne.Window, ctrl *controller.Controller,
	status *widget.Label, startBtn, stopBtn *widget.Button) {

	d, ok := a.(desktop.App)
	if !ok {
		// 平台不支持托盘（如缺少 libayatana-appindicator 的 Linux），
		// 退化为纯窗口模式：关窗即退出。
		win.SetCloseIntercept(func() { a.Quit() })
		return
	}

	d.SetSystemTrayWindow(win)
	d.SetSystemTrayIcon(theme.InfoIcon())
	d.SetSystemTrayMenu(fyne.NewMenu("gzgspg",
		fyne.NewMenuItem("显示主窗口", func() {
			win.Show()
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("启动", func() {
			startBtn.OnTapped()
		}),
		fyne.NewMenuItem("停止", func() {
			stopBtn.OnTapped()
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("退出", func() {
			ctrl.Stop()
			a.Quit()
		}),
	))
}
