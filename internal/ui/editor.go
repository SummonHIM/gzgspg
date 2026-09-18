package ui

import (
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/summonhim/gzgspd/config"
	"github.com/summonhim/gzgspg/internal/controller"
)

// editor 是单账号的编辑表单。
type editor struct {
	ctrl *controller.Controller

	username   *widget.Entry
	password   *widget.Entry
	iface      *widget.Entry
	userAgent  *widget.Entry
	keepAlive  *widget.Entry
	kAliveLink *widget.Entry
	retryMax   *widget.Entry
	retryTime  *widget.Entry
	advanced   *widget.Accordion

	root fyne.CanvasObject
}

func newEditor(ctrl *controller.Controller) *editor {
	e := &editor{
		ctrl:       ctrl,
		username:   widget.NewEntry(),
		password:   widget.NewPasswordEntry(),
		iface:      widget.NewEntry(),
		userAgent:  widget.NewEntry(),
		keepAlive:  widget.NewEntry(),
		kAliveLink: widget.NewEntry(),
		retryMax:   widget.NewEntry(),
		retryTime:  widget.NewEntry(),
	}
	e.load()

	form := widget.NewForm(
		widget.NewFormItem("用户名", e.username),
		widget.NewFormItem("密码", e.password),
	)
	e.advanced = widget.NewAccordion(
		widget.NewAccordionItem("高级设置", widget.NewForm(
			widget.NewFormItem("网卡 interface", e.iface),
			widget.NewFormItem("User-Agent", e.userAgent),
			widget.NewFormItem("keep_alive (秒)", e.keepAlive),
			widget.NewFormItem("keep_alive_link", e.kAliveLink),
			widget.NewFormItem("retry_max", e.retryMax),
			widget.NewFormItem("retry_time (秒)", e.retryTime),
		)),
	)

	e.root = container.NewVBox(form, e.advanced)
	return e
}

func (e *editor) load() {
	inst := e.ctrl.Instance()
	e.username.SetText(inst.Username)
	e.password.SetText(inst.Password)
	e.iface.SetText(inst.Interface)
	e.userAgent.SetText(inst.UserAgent)
	e.keepAlive.SetText(strconv.Itoa(inst.KeepAlive))
	e.kAliveLink.SetText(inst.KAliveLink)
	e.retryMax.SetText(strconv.Itoa(inst.RetryMax))
	e.retryTime.SetText(strconv.Itoa(inst.RetryTime))
}

// collect 从表单读取并写回 controller。
func (e *editor) collect() {
	e.ctrl.UpdateInstance(config.ConfigInstance{
		Username:   e.username.Text,
		Password:   e.password.Text,
		Interface:  e.iface.Text,
		UserAgent:  e.userAgent.Text,
		KeepAlive:  atoiOr(e.keepAlive.Text, 5),
		KAliveLink: e.kAliveLink.Text,
		RetryMax:   atoiOr(e.retryMax.Text, 3),
		RetryTime:  atoiOr(e.retryTime.Text, 5),
	})
}

func atoiOr(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}
