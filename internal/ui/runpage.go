package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/summonhim/gzgspd/engine"
)

// runPage 是运行页：显示当前状态 + 登出。
type runPage struct {
	state *widget.Label
	root  fyne.CanvasObject
}

func newRunPage(onLogout func()) *runPage {
	r := &runPage{
		state: widget.NewLabelWithStyle("已连接", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
	}

	logoutBtn := widget.NewButtonWithIcon("登出", theme.LogoutIcon(), onLogout)
	logoutBtn.Importance = widget.HighImportance

	r.root = container.NewVBox(
		layout.NewSpacer(),
		container.NewCenter(r.state),
		layout.NewSpacer(),
		container.NewCenter(logoutBtn),
	)
	return r
}

// setState 更新展示状态。
func (r *runPage) setState(s engine.State) {
	r.state.SetText(s.String())
}
