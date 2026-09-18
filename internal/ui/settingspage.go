package ui

import (
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/summonhim/gzgspg/internal/autostart"
	"github.com/summonhim/gzgspg/internal/controller"
)

const (
	defaultUserAgent  = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36"
	defaultKAliveLink = "http://3.3.3.3"
	defaultKeepAlive  = 5
	defaultRetryMax   = 3
	defaultRetryTime  = 5
)

// settingsPage 是高级设置页。
type settingsPage struct {
	ctrl *controller.Controller

	iface      *widget.Entry
	userAgent  *widget.Entry
	keepAlive  *widget.Entry
	kAliveLink *widget.Entry
	retryMax   *widget.Entry
	retryTime  *widget.Entry
	autostart  *widget.Check
	autoStatus *widget.Label

	root fyne.CanvasObject
}

func newSettingsPage(ctrl *controller.Controller, onBack func()) *settingsPage {
	s := &settingsPage{
		ctrl:       ctrl,
		iface:      widget.NewEntry(),
		userAgent:  widget.NewEntry(),
		keepAlive:  widget.NewEntry(),
		kAliveLink: widget.NewEntry(),
		retryMax:   widget.NewEntry(),
		retryTime:  widget.NewEntry(),
		autostart:  widget.NewCheck("开机自启", nil),
		autoStatus: widget.NewLabel(""),
	}
	s.load()

	// 自启开关
	if !autostart.Supported() {
		s.autostart.Disable()
		s.autostart.SetChecked(false)
		s.autoStatus.SetText("当前平台不支持")
	} else {
		s.autostart.SetChecked(autostart.Enabled())
		s.autostart.OnChanged = func(on bool) {
			var err error
			if on {
				err = autostart.Enable()
			} else {
				err = autostart.Disable()
			}
			if err != nil {
				s.autoStatus.SetText("设置失败: " + err.Error())
				// 回退到实际状态
				s.autostart.SetChecked(autostart.Enabled())
				return
			}
			s.autoStatus.SetText("")
		}
	}

	form := widget.NewForm(
		widget.NewFormItem("开机自启", container.NewVBox(s.autostart, s.autoStatus)),
		widget.NewFormItem("网卡 interface", s.iface),
		widget.NewFormItem("User-Agent", s.userAgent),
		widget.NewFormItem("keep_alive (秒)", s.keepAlive),
		widget.NewFormItem("keep_alive_link", s.kAliveLink),
		widget.NewFormItem("retry_max", s.retryMax),
		widget.NewFormItem("retry_time (秒)", s.retryTime),
	)

	backBtn := widget.NewButton("返回", onBack)
	s.root = container.NewBorder(
		container.NewHBox(backBtn),
		nil, nil, nil,
		container.NewVScroll(form),
	)
	return s
}

// load 从配置载入；值为空时填入默认值。
func (s *settingsPage) load() {
	inst := s.ctrl.Instance()

	s.iface.SetText(inst.Interface) // interface 默认空 = 自动探测

	s.userAgent.SetText(orDefault(inst.UserAgent, defaultUserAgent))
	s.keepAlive.SetText(orDefaultInt(inst.KeepAlive, defaultKeepAlive))
	s.kAliveLink.SetText(orDefault(inst.KAliveLink, defaultKAliveLink))
	s.retryMax.SetText(orDefaultInt(inst.RetryMax, defaultRetryMax))
	s.retryTime.SetText(orDefaultInt(inst.RetryTime, defaultRetryTime))
}

// collect 写回 controller。
func (s *settingsPage) collect() {
	inst := s.ctrl.Instance()
	inst.Interface = s.iface.Text
	inst.UserAgent = s.userAgent.Text
	inst.KAliveLink = s.kAliveLink.Text
	inst.KeepAlive = atoiOr(s.keepAlive.Text, defaultKeepAlive)
	inst.RetryMax = atoiOr(s.retryMax.Text, defaultRetryMax)
	inst.RetryTime = atoiOr(s.retryTime.Text, defaultRetryTime)
	s.ctrl.UpdateInstance(inst)
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func orDefaultInt(v, def int) string {
	if v == 0 {
		return strconv.Itoa(def)
	}
	return strconv.Itoa(v)
}

func atoiOr(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}
