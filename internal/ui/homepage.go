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

	// Logo 与标题、账号密码作为一整组垂直居中：上下各一个 Spacer 夹住
	center := container.NewVBox(
		layout.NewSpacer(),
		container.NewCenter(logo),
		container.NewCenter(appTitle()),
		h.username,
		h.password,
		layout.NewSpacer(),
	)

	content := container.NewBorder(
		topRight,
		container.NewPadded(container.NewVBox(h.status, loginBtn)),
		nil, nil,
		container.NewPadded(center),
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

// appTitle 返回界面标题文案，与窗口标题、FyneApp.toml 中的发布名一致。
func appTitle() fyne.CanvasObject {
	return widget.NewLabelWithStyle("广工商校园网登录器", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
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
