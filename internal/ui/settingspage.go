package ui

import (
	"errors"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/summonhim/gzgspg/internal/autostart"
	"github.com/summonhim/gzgspg/internal/controller"
	"github.com/summonhim/gzgspg/internal/version"
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
	autoLogin  *widget.Check
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
		autostart:  widget.NewCheck("开启", nil),
		autoLogin:  widget.NewCheck("开启", nil),
		autoStatus: widget.NewLabel(""),
	}
	s.load()

	// 数值字段内联校验：规则与 engine 的 Validate 一致，输入即时提示。
	s.keepAlive.Validator = validatePositiveInt
	s.keepAlive.AlwaysShowValidationError = true
	s.retryMax.Validator = validateNonNegativeInt
	s.retryMax.AlwaysShowValidationError = true
	s.retryTime.Validator = validatePositiveInt
	s.retryTime.AlwaysShowValidationError = true

	prefs := fyne.CurrentApp().Preferences()

	// 自动登陆：只存 preferences，OnChanged 即时写入
	s.autoLogin.SetChecked(prefs.BoolWithFallback("auto_login", false))
	s.autoLogin.OnChanged = func(on bool) {
		prefs.SetBool("auto_login", on)
	}

	// 自启开关
	if !autostart.Supported() {
		s.autostart.Disable()
		s.autostart.SetChecked(false)
		s.autoStatus.SetText("当前平台不支持")
	} else {
		s.autostart.SetChecked(prefs.BoolWithFallback("autostart", false))
		// 具名处理器，便于回退时临时摘除 OnChanged，避免 SetChecked 再次触发本函数。
		var onAutostartChanged func(on bool)
		onAutostartChanged = func(on bool) {
			prefs.SetBool("autostart", on)
			var err error
			if on {
				err = autostart.Enable()
			} else {
				err = autostart.Disable()
			}
			if err != nil {
				s.autoStatus.SetText("设置失败: " + err.Error())
				// 回退到实际状态，并让 preferences 与实际一致。
				// 临时摘除回调，避免 SetChecked 重新进入本函数而清空刚设置的错误提示，
				// 同时避免重复调用一次 Enable/Disable。
				actual := autostart.Enabled()
				prefs.SetBool("autostart", actual)
				s.autostart.OnChanged = nil
				s.autostart.SetChecked(actual)
				s.autostart.OnChanged = onAutostartChanged
				return
			}
			s.autoStatus.SetText("")
		}
		s.autostart.OnChanged = onAutostartChanged
	}

	form := widget.NewForm(
		widget.NewFormItem("网卡名称", s.iface),
		widget.NewFormItem("User-Agent", s.userAgent),
		widget.NewFormItem("监测间隔 (秒)", s.keepAlive),
		widget.NewFormItem("监测在线链接", s.kAliveLink),
		widget.NewFormItem("最大重试次数", s.retryMax),
		widget.NewFormItem("重试间隔 (秒)", s.retryTime),
		widget.NewFormItem("自动登陆", s.autoLogin),
		widget.NewFormItem("开机自启", s.autostart),
	)

	// 自启状态提示单独放表单底部，避免空标签在勾选框下方占一行高度
	body := container.NewVBox(
		form,
		s.autoStatus,
		widget.NewSeparator(),
		widget.NewLabel(versionLabel()),
	)

	backBtn := widget.NewButton("保存并返回", onBack)
	backBtn.Importance = widget.HighImportance
	s.root = container.NewBorder(
		container.NewCenter(settingTitle()),
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

// collect 校验并把设置写回 controller；任一数值字段非法时返回 false 且不写回。
func (s *settingsPage) collect() bool {
	if s.keepAlive.Validate() != nil || s.retryMax.Validate() != nil || s.retryTime.Validate() != nil {
		return false
	}
	inst := s.ctrl.Instance()
	inst.Interface = s.iface.Text
	inst.UserAgent = s.userAgent.Text
	inst.KAliveLink = s.kAliveLink.Text
	inst.KeepAlive, _ = strconv.Atoi(s.keepAlive.Text)
	inst.RetryMax, _ = strconv.Atoi(s.retryMax.Text)
	inst.RetryTime, _ = strconv.Atoi(s.retryTime.Text)
	s.ctrl.UpdateInstance(inst)
	return true
}

// validatePositiveInt 校验输入为正整数（>0），对应 engine 的 keep_alive / retry_time。
func validatePositiveInt(s string) error {
	if n, err := strconv.Atoi(s); err != nil || n <= 0 {
		return errors.New("请输入正整数")
	}
	return nil
}

// validateNonNegativeInt 校验输入为非负整数（≥0），对应 engine 的 retry_max。
func validateNonNegativeInt(s string) error {
	if n, err := strconv.Atoi(s); err != nil || n < 0 {
		return errors.New("请输入非负整数")
	}
	return nil
}

// settingTitle 返回设置页面标题文案。
func settingTitle() fyne.CanvasObject {
	return &canvas.Text{
		Text:      "高级设置",
		TextSize:  titleSize,
		TextStyle: fyne.TextStyle{Bold: true},
		Alignment: fyne.TextAlignCenter,
	}
}

// versionLabel 返回设置页底部展示的版本文案。
func versionLabel() string {
	return "版本 " + version.String()
}
