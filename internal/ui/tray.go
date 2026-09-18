package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"

	"github.com/summonhim/gzgspd/engine"
)

// trayController 持有托盘菜单中需要动态更新的项。
// 托盘不可用的平台上为 nil。
type trayController struct {
	menu   *fyne.Menu
	status *fyne.MenuItem
}

// setupTray 在支持托盘的平台上安装托盘图标与菜单。
// 启停由界面按钮驱动，托盘负责显示窗口、显示状态与退出。
func setupTray(a fyne.App, win fyne.Window, onShowWindow func(), onQuit func()) *trayController {
	d, ok := a.(desktop.App)
	if !ok {
		// 平台不支持托盘：关窗即退出
		win.SetCloseIntercept(func() { onQuit() })
		return nil
	}

	status := fyne.NewMenuItem("状态："+stateLabel(engine.StateStopped), nil)
	status.Disabled = true

	menu := fyne.NewMenu("gzgspg",
		status,
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("显示主窗口", onShowWindow),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("退出", onQuit),
	)

	d.SetSystemTrayWindow(win)
	d.SetSystemTrayIcon(appIcon())
	d.SetSystemTrayMenu(menu)

	return &trayController{menu: menu, status: status}
}

// setState 更新托盘首行的状态文本。
func (t *trayController) setState(s engine.State) {
	if t == nil {
		return
	}
	t.status.Label = "状态：" + stateLabel(s)
	t.menu.Refresh()
}
