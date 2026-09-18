package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/summonhim/gzgspd/engine"
)

// runPageStateSize 是运行页状态文字的字号。
// widget.Label 改字号需自定义 Theme，故这里用 canvas.Text。
const runPageStateSize float32 = 28

// runPage 是运行页：显示当前状态 + 登出。
type runPage struct {
	state *canvas.Text
	root  fyne.CanvasObject
}

func newRunPage(onLogout func()) *runPage {
	r := &runPage{
		state: &canvas.Text{
			Text:      stateLabel(engine.StateStopped),
			TextSize:  runPageStateSize,
			TextStyle: fyne.TextStyle{Bold: true},
			Alignment: fyne.TextAlignCenter,
		},
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
	r.state.Text = stateLabel(s)
	canvas.Refresh(r.state)
}
