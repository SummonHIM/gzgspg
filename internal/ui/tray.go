package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"

	"github.com/summonhim/gzgspg/internal/controller"
)

// setupTray 在支持托盘的平台上安装托盘图标与菜单。
// 启停由界面按钮驱动，托盘只负责显示主窗口与退出。
func setupTray(a fyne.App, win fyne.Window, ctrl *controller.Controller) {
	d, ok := a.(desktop.App)
	if !ok {
		// 平台不支持托盘：关窗即退出
		win.SetCloseIntercept(func() { a.Quit() })
		return
	}

	d.SetSystemTrayWindow(win)
	d.SetSystemTrayIcon(appIcon())
	d.SetSystemTrayMenu(fyne.NewMenu("gzgspg",
		fyne.NewMenuItem("显示主窗口", func() {
			win.Show()
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("退出", func() {
			ctrl.Stop()
			a.Quit()
		}),
	))
}
