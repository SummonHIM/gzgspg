package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/summonhim/gzgspg/internal/controller"
)

// homePage 是登录首页：居中 Logo + 账号 + 密码 + 登陆按钮 + 右上角设置图标。
type homePage struct {
	ctrl     *controller.Controller
	username *widget.Entry
	password *widget.Entry
	status   *widget.Label
	root     fyne.CanvasObject
}

func newHomePage(ctrl *controller.Controller, onLogin func(), onSettings func()) *homePage {
	h := &homePage{
		ctrl:     ctrl,
		username: widget.NewEntry(),
		password: widget.NewPasswordEntry(),
		status:   widget.NewLabel(""),
	}

	// 载入已有配置
	inst := ctrl.Instance()
	h.username.SetText(inst.Username)
	h.password.SetText(inst.Password)

	h.username.SetPlaceHolder("账号")
	h.password.SetPlaceHolder("密码")

	logo := logoObject()

	loginBtn := widget.NewButton("登陆", onLogin)
	loginBtn.Importance = widget.HighImportance

	settingsBtn := widget.NewButtonWithIcon("", theme.SettingsIcon(), onSettings)
	settingsBtn.Importance = widget.LowImportance

	topRight := container.NewHBox(layout.NewSpacer(), settingsBtn)

	center := container.NewVBox(
		layout.NewSpacer(),
		container.NewCenter(logo),
		layout.NewSpacer(),
	)

	form := container.NewVBox(
		h.username,
		h.password,
		loginBtn,
		h.status,
	)

	content := container.NewBorder(
		topRight,
		nil, nil, nil,
		container.NewVBox(center, container.NewPadded(form)),
	)
	h.root = content
	return h
}

// logoObject 返回居中显示的 Logo 对象。
func logoObject() fyne.CanvasObject {
	img := canvas.NewImageFromResource(appIcon())
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(160, 160))
	return img
}

// collect 把首页字段写回 controller。
func (h *homePage) collect() {
	inst := h.ctrl.Instance()
	inst.Username = h.username.Text
	inst.Password = h.password.Text
	h.ctrl.UpdateInstance(inst)
}

// setStatus 更新提示文本。
func (h *homePage) setStatus(s string) {
	h.status.SetText(s)
}
