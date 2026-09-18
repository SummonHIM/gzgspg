package ui

import (
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/summonhim/gzgspg/internal/autostart"
	"github.com/summonhim/gzgspg/internal/controller"
)

// 数值字段的兜底默认值，仅在输入框为空或内容非法时使用。
const (
	defaultKeepAlive = controller.DefaultKeepAlive
	defaultRetryMax  = controller.DefaultRetryMax
	defaultRetryTime = controller.DefaultRetryTime
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
		widget.NewFormItem("网卡名称", s.iface),
		widget.NewFormItem("User-Agent", s.userAgent),
		widget.NewFormItem("监测在线间隔 (秒)", s.keepAlive),
		widget.NewFormItem("监测在线链接", s.kAliveLink),
		widget.NewFormItem("最大重试次数", s.retryMax),
		widget.NewFormItem("重试间隔 (秒)", s.retryTime),
		widget.NewFormItem("开机自启", s.autostart),
	)

	// 自启状态提示单独放表单底部，避免空标签在勾选框下方占一行高度
	body := container.NewVBox(
		form,
		s.autoStatus,
	)

	backBtn := widget.NewButton("保存并返回", onBack)
	backBtn.Importance = widget.HighImportance
	s.root = container.NewBorder(
		nil,
		container.NewPadded(backBtn),
		nil, nil,
		container.NewVScroll(body),
	)
	return s
}

// load 从配置载入。配置在 controller 侧已带默认值，这里直读、不回填，
// 以便界面如实反映文件内容（用户清空的字段就显示为空）。
func (s *settingsPage) load() {
	inst := s.ctrl.Instance()

	s.iface.SetText(inst.Interface) // interface 默认空 = 自动探测
	s.userAgent.SetText(inst.UserAgent)
	s.keepAlive.SetText(strconv.Itoa(inst.KeepAlive))
	s.kAliveLink.SetText(inst.KAliveLink)
	s.retryMax.SetText(strconv.Itoa(inst.RetryMax))
	s.retryTime.SetText(strconv.Itoa(inst.RetryTime))
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

// atoiOr 把输入框文本转为整数；空串或非法输入回退到 def。
// 这是空输入的兜底：0 会让 keep_alive/retry_time 违反 engine 校验，启动失败。
func atoiOr(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}
