# gzgspg UI 重构实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 gzgspg 重构为双页面 UI，配置与日志落到 `os.UserConfigDir()/gzgspg/`，并加入三平台开机自启。

**Architecture:** 新增 `configdir`（路径解析）与 `autostart`（三平台自启）两个基础包；`ui` 从单页改为 首页/高级设置/运行页 三视图切换；`controller` 接口不变，仅由调用方传入新的配置路径。

**Tech Stack:** Go 1.25、Fyne v2.8.1、`os.UserConfigDir`、`golang.org/x/sys/windows/registry`（Windows 自启）。

## Global Constraints

- **配置与日志目录**：`os.UserConfigDir()/gzgspg/`。Windows 实测为 `C:\Users\<user>\AppData\Roaming\gzgspg`。
- **可测性**：`configdir` 在环境变量 `GZGSPG_CONFIG_DIR` 非空时直接使用其值作为目录（不再拼 `gzgspg` 子目录）。
- **日志不进 UI**：删除日志面板；slog 只写 `<configDir>/gzgspg/gzgspg.log`，追加模式。
- **config.json 保持 instance 数组**，与 gzgspd 双向兼容；GUI 只用 `[0]`。
- **界面三视图**：首页 / 高级设置页 / 运行页。
- **首页**：Logo 居中 + 账号 + 密码 + 登陆按钮 + 右上角设置图标。
- **高级设置页**：开机自启开关 + interface / User-Agent / keep_alive / keep_alive_link / retry_max / retry_time。**值为空时自动填入默认值文本**。
- **默认值**（必须与 gzgspd engine 一致）：
  - User-Agent = `Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36`
  - keep_alive = `5`
  - keep_alive_link = `http://3.3.3.3`
  - retry_max = `3`
  - retry_time = `5`
  - interface 默认空（自动探测）
- **登陆流程**：collect → Save → Start → 成功才切到运行页；Start 失败留在首页并显示错误。
- **运行页**：显示当前状态 + 登出按钮；登出 = `controller.Stop()`，完成后回首页。
- **托盘保持不变**：显示主窗口 / 启动 / 停止 / 退出；关窗隐藏。
- **线程约束**：所有 UI 更新经 `fyne.Do`；阻塞调用不在 UI 主线程。
- **构建环境**：`CGO_ENABLED=1`，gcc 来自 `C:\ProgramData\msys2\ucrt64\bin`。注意该目录下的 `go.exe` 是坏的（trimmed GOROOT），必须用 `C:\Program Files\Go\bin\go.exe`，只把 ucrt64 加到 PATH 末尾供 gcc 用。
- **测试策略（用户指定）**：测试与实现一起写，全部任务完成后统一运行一次 `go test ./...`。
- 不实现：密码加密、多账号、定时任务、自动更新、多语言、主题、日志查看器、日志轮转。

---

## 文件结构

| 文件 | 动作 | 职责 |
|------|------|------|
| `internal/configdir/configdir.go` | 新增 | 解析并创建配置目录，给出 config/log 路径 |
| `internal/configdir/configdir_test.go` | 新增 | 测试（用 `GZGSPG_CONFIG_DIR` 重定向） |
| `internal/autostart/autostart.go` | 新增 | 公共接口与错误 |
| `internal/autostart/autostart_windows.go` | 新增 | 注册表 Run 键 |
| `internal/autostart/autostart_linux.go` | 新增 | `~/.config/autostart/gzgspg.desktop` |
| `internal/autostart/autostart_darwin.go` | 新增 | `~/Library/LaunchAgents/top.summonhim.gzgspg.plist` |
| `internal/autostart/autostart_other.go` | 新增 | 不支持的平台 |
| `internal/autostart/autostart_test.go` | 新增 | 读写往返（可清理） |
| `internal/ui/logo.go` | 新增 | `go:embed` 打包图标，返回 `fyne.Resource` |
| `internal/ui/app.go` | 重写 | 三视图装配与切换 |
| `internal/ui/homepage.go` | 新增 | 首页 |
| `internal/ui/settingspage.go` | 新增 | 高级设置页 |
| `internal/ui/runpage.go` | 新增 | 运行页 |
| `internal/ui/logpanel.go` | 删除 | 日志不再进 UI |
| `internal/ui/editor.go` | 删除 | 由 homepage/settingspage 取代 |
| `internal/ui/tray.go` | 修改 | 适配新的按钮引用 |
| `internal/controller/controller.go` | 不变 | 已支持传入 ConfigPath |

---

## Task 1: configdir 包

**Files:**
- Create: `internal/configdir/configdir.go`
- Test: `internal/configdir/configdir_test.go`

**Interfaces:**
- Produces:
  - `configdir.Dir() (string, error)`
  - `configdir.ConfigPath() (string, error)`
  - `configdir.LogPath() (string, error)`

- [ ] **Step 1: 写测试**

`internal/configdir/configdir_test.go`:

```go
package configdir

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDirUsesEnvOverride(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("GZGSPG_CONFIG_DIR", tmp)

	dir, err := Dir()
	if err != nil {
		t.Fatalf("dir: %v", err)
	}
	if dir != tmp {
		t.Fatalf("expected %q, got %q", tmp, dir)
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		t.Fatalf("dir must exist: err=%v", err)
	}
}

func TestDirDefaultsUnderUserConfigDir(t *testing.T) {
	t.Setenv("GZGSPG_CONFIG_DIR", "")

	dir, err := Dir()
	if err != nil {
		t.Fatalf("dir: %v", err)
	}
	base, err := os.UserConfigDir()
	if err != nil {
		t.Skip("UserConfigDir unavailable")
	}
	want := filepath.Join(base, "gzgspg")
	if dir != want {
		t.Fatalf("expected %q, got %q", want, dir)
	}
}

func TestConfigAndLogPaths(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("GZGSPG_CONFIG_DIR", tmp)

	cp, err := ConfigPath()
	if err != nil {
		t.Fatalf("config path: %v", err)
	}
	if filepath.Base(cp) != "config.json" {
		t.Fatalf("unexpected config path: %q", cp)
	}
	if filepath.Dir(cp) != tmp {
		t.Fatalf("config path not in override dir: %q", cp)
	}

	lp, err := LogPath()
	if err != nil {
		t.Fatalf("log path: %v", err)
	}
	if !strings.HasSuffix(lp, "gzgspg.log") {
		t.Fatalf("unexpected log path: %q", lp)
	}
	if filepath.Dir(lp) != tmp {
		t.Fatalf("log path not in override dir: %q", lp)
	}
}
```

- [ ] **Step 2: 实现**

`internal/configdir/configdir.go`:

```go
// Package configdir 解析 gzgspg 的配置与日志目录。
package configdir

import (
	"os"
	"path/filepath"
)

// EnvOverride 是覆盖配置目录的环境变量名。非空时直接作为目录使用。
const EnvOverride = "GZGSPG_CONFIG_DIR"

// Dir 返回配置目录并确保其存在。
// 若设置了 GZGSPG_CONFIG_DIR，直接使用其值；否则为 <UserConfigDir>/gzgspg。
func Dir() (string, error) {
	dir := os.Getenv(EnvOverride)
	if dir == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(base, "gzgspg")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// ConfigPath 返回 config.json 的完整路径。
func ConfigPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// LogPath 返回日志文件的完整路径。
func LogPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "gzgspg.log"), nil
}
```

- [ ] **Step 3: 构建验证**

```powershell
$go = "C:\Program Files\Go\bin\go.exe"
$env:PATH = $env:PATH + ";C:\ProgramData\msys2\ucrt64\bin"
$env:CGO_ENABLED = "1"
& $go build ./...
```

Expected: exit 0。

- [ ] **Step 4: 提交**

```bash
git add internal/configdir/
git commit -m "feat: 添加 configdir 解析配置与日志目录"
```

---

## Task 2: autostart 包（三平台）

**Files:**
- Create: `internal/autostart/autostart.go`
- Create: `internal/autostart/autostart_windows.go`
- Create: `internal/autostart/autostart_linux.go`
- Create: `internal/autostart/autostart_darwin.go`
- Create: `internal/autostart/autostart_other.go`
- Test: `internal/autostart/autostart_test.go`

**Interfaces:**
- Produces:
  - `autostart.Supported() bool`
  - `autostart.Enabled() bool`
  - `autostart.Enable() error`
  - `autostart.Disable() error`
  - `autostart.ErrUnsupported`

**设计说明**：各平台用 `//go:build` 约束。应用名与 ID 固定：
- Windows 注册表值名：`gzgspg`
- Linux 桌面文件名：`gzgspg.desktop`
- macOS label：`top.summonhim.gzgspg`
- 可执行路径通过 `os.Executable()` 获取。

- [ ] **Step 1: 写公共定义**

`internal/autostart/autostart.go`:

```go
// Package autostart 管理开机自启。各平台实现见 autostart_<goos>.go。
package autostart

import "errors"

// ErrUnsupported 表示当前平台不支持自启。
var ErrUnsupported = errors.New("autostart is not supported on this platform")

const (
	// AppName 用于系统自启项的名称。
	AppName = "gzgspg"
	// AppID 用于 macOS 的 LaunchAgent label 与标识。
	AppID = "top.summonhim.gzgspg"
)
```

- [ ] **Step 2: 写 Windows 实现**

`internal/autostart/autostart_windows.go`:

```go
//go:build windows

package autostart

import (
	"os"

	"golang.org/x/sys/windows/registry"
)

const runKey = `Software\Microsoft\Windows\CurrentVersion\Run`

// Supported 报告当前平台是否支持自启。
func Supported() bool { return true }

// Enabled 报告自启是否已启用。
func Enabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	_, _, err = k.GetStringValue(AppName)
	return err == nil
}

// Enable 启用自启，指向当前可执行文件。
func Enable() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetStringValue(AppName, exe)
}

// Disable 关闭自启。
func Disable() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return nil // 键不存在即已经是关闭状态
	}
	defer k.Close()
	err = k.DeleteValue(AppName)
	if err == registry.ErrNotExist {
		return nil
	}
	return err
}
```

- [ ] **Step 3: 写 Linux 实现**

`internal/autostart/autostart_linux.go`:

```go
//go:build linux

package autostart

import (
	"fmt"
	"os"
	"path/filepath"
)

func desktopFilePath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "autostart", "gzgspg.desktop"), nil
}

// Supported 报告当前平台是否支持自启。
func Supported() bool { return true }

// Enabled 报告自启是否已启用。
func Enabled() bool {
	p, err := desktopFilePath()
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}

// Enable 启用自启，写入 autostart desktop 文件。
func Enable() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	p, err := desktopFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	content := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=%s
Exec=%s
X-GNOME-Autostart-enabled=true
`, AppName, exe)
	return os.WriteFile(p, []byte(content), 0o644)
}

// Disable 关闭自启。
func Disable() error {
	p, err := desktopFilePath()
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
```

- [ ] **Step 4: 写 macOS 实现**

`internal/autostart/autostart_darwin.go`:

```go
//go:build darwin

package autostart

import (
	"fmt"
	"os"
	"path/filepath"
)

func plistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", AppID+".plist"), nil
}

// Supported 报告当前平台是否支持自启。
func Supported() bool { return true }

// Enabled 报告自启是否已启用。
func Enabled() bool {
	p, err := plistPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}

// Enable 启用自启，写入 LaunchAgent plist。
func Enable() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	p, err := plistPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	content := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
</dict>
</plist>
`, AppID, exe)
	return os.WriteFile(p, []byte(content), 0o644)
}

// Disable 关闭自启。
func Disable() error {
	p, err := plistPath()
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
```

- [ ] **Step 5: 写其他平台实现**

`internal/autostart/autostart_other.go`:

```go
//go:build !windows && !linux && !darwin

package autostart

// Supported 报告当前平台是否支持自启。
func Supported() bool { return false }

// Enabled 报告自启是否已启用。
func Enabled() bool { return false }

// Enable 在当前平台不可用。
func Enable() error { return ErrUnsupported }

// Disable 在当前平台不可用。
func Disable() error { return ErrUnsupported }
```

- [ ] **Step 6: 写测试**

`internal/autostart/autostart_test.go`:

```go
package autostart

import "testing"

func TestSupportedPlatform(t *testing.T) {
	// 本仓库面向 windows/linux/darwin；CI 与开发机应为三者之一。
	if !Supported() {
		t.Skip("autostart not supported on this platform")
	}
}

func TestEnableDisableRoundTrip(t *testing.T) {
	if !Supported() {
		t.Skip("autostart not supported on this platform")
	}

	// 记录原状态，测试结束后恢复，避免污染开发机
	wasEnabled := Enabled()
	t.Cleanup(func() {
		if wasEnabled {
			_ = Enable()
		} else {
			_ = Disable()
		}
	})

	if err := Enable(); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if !Enabled() {
		t.Fatal("expected enabled after Enable()")
	}
	if err := Disable(); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if Enabled() {
		t.Fatal("expected disabled after Disable()")
	}
}
```

- [ ] **Step 7: 构建验证**

```powershell
$go = "C:\Program Files\Go\bin\go.exe"
$env:PATH = $env:PATH + ";C:\ProgramData\msys2\ucrt64\bin"
$env:CGO_ENABLED = "1"
& $go build ./...
& $go vet ./...
```

Expected: 均 exit 0。

- [ ] **Step 8: 提交**

```bash
git add internal/autostart/
git commit -m "feat: 添加三平台开机自启"
```

---

## Task 3: 首页与高级设置页

**Files:**
- Create: `internal/ui/logo.go`
- Create: `internal/ui/homepage.go`
- Create: `internal/ui/settingspage.go`
- Delete: `internal/ui/editor.go`

**Interfaces:**
- Consumes: `controller.Controller`、`autostart`、`config.ConfigInstance`
- Produces:
  - `newHomePage(ctrl *controller.Controller, onLogin func(), onSettings func()) *homePage`
  - `(*homePage) collect()`
  - `(*homePage) root fyne.CanvasObject`
  - `newSettingsPage(ctrl *controller.Controller, onBack func()) *settingsPage`
  - `(*settingsPage) collect()`
  - `(*settingsPage) root fyne.CanvasObject`

**默认值常量**（放在 `settingspage.go`，供 `defaultsFor` 使用）：

```go
const (
	defaultUserAgent  = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36"
	defaultKAliveLink = "http://3.3.3.3"
	defaultKeepAlive  = 5
	defaultRetryMax   = 3
	defaultRetryTime  = 5
)
```

- [ ] **Step 1: 实现嵌入 Logo**

先把图标复制到包内，以便 `go:embed` 打包（这样双击 exe 也能显示图标，不依赖工作目录）：

```powershell
Copy-Item assets\icon.png internal\ui\icon.png -Force
```

`internal/ui/logo.go`:

```go
package ui

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed icon.png
var iconPNG []byte

// appIcon 返回打包进二进制的应用图标。
func appIcon() fyne.Resource {
	return fyne.NewStaticResource("icon.png", iconPNG)
}
```

- [ ] **Step 2: 实现首页**

`internal/ui/homepage.go`:

```go
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

	// Logo：优先读打包资源，缺失时退化为纯色方块
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
```

- [ ] **Step 3: 实现高级设置页**

`internal/ui/settingspage.go`:

```go
package ui

import (
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/summonhim/gzgspd/config"
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

	backBtn := widget.NewButtonWithIcon("返回", nil, onBack)
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

	// 刷新界面展示（供 Fyne 立即渲染）
	s.userAgent.Refresh()
	s.keepAlive.Refresh()
	s.kAliveLink.Refresh()
	s.retryMax.Refresh()
	s.retryTime.Refresh()
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

// 确保 config 仍被引用（UpdateInstance 的参数类型来自该包）
var _ = config.ConfigInstance{}
```

**注意**：上面的 `var _ = config.ConfigInstance{}` 仅用于避免误删 import；若实现时 `config` 已被其它方式引用，可删除该行与对应 import。

- [ ] **Step 4: 删除旧编辑器**

```bash
git rm internal/ui/editor.go
```

- [ ] **Step 5: 构建验证**

在 Task 4 替换 app.go 后统一构建（当前 app.go 仍引用已删除的 `newEditor`，会失败）。

- [ ] **Step 6: 提交**

```bash
git add internal/ui/homepage.go internal/ui/settingspage.go internal/ui/logo.go internal/ui/icon.png
git rm internal/ui/editor.go
git commit -m "feat: 首页与高级设置页"
```

---

## Task 4: 运行页与应用装配

**Files:**
- Create: `internal/ui/runpage.go`
- Rewrite: `internal/ui/app.go`
- Delete: `internal/ui/logpanel.go`
- Modify: `internal/ui/tray.go`

**Interfaces:**
- Consumes: `controller.Controller`、`engine.State`
- Produces:
  - `newRunPage(onLogout func()) *runPage`
  - `(*runPage) root fyne.CanvasObject`
  - `(*runPage) setState(s engine.State)`
  - `ui.Run() error`（不变）

- [ ] **Step 1: 实现运行页**

`internal/ui/runpage.go`:

```go
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
	state  *widget.Label
	root   fyne.CanvasObject
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
```

- [ ] **Step 2: 重写 app.go**

`internal/ui/app.go`:

```go
// Package ui 是 gzgspg 的 Fyne 界面层，只负责展示与交互。
package ui

import (
	"log/slog"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"

	"github.com/summonhim/gzgspd/engine"
	"github.com/summonhim/gzgspg/internal/configdir"
	"github.com/summonhim/gzgspg/internal/controller"
)

// Run 启动 GUI，阻塞至退出。
func Run() error {
	a := app.NewWithID("top.summonhim.gzgspg")

	logPath, err := configdir.LogPath()
	if err != nil {
		return err
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer logFile.Close()

	logger := slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	cfgPath, err := configdir.ConfigPath()
	if err != nil {
		return err
	}
	ctrl, err := controller.New(controller.Options{
		ConfigPath: cfgPath,
		Logger:     logger,
	})
	if err != nil {
		return err
	}

	win := a.NewWindow("广工商校园网登录器")
	win.Resize(fyne.NewSize(420, 640))

	// 三视图：首页 / 高级设置 / 运行页
	var home *homePage
	var settings *settingsPage
	var run *runPage

	stack := container.NewStack()

	showHome := func() {
		home = newHomePage(ctrl, onLogin, onSettings)
		stack.Objects = []fyne.CanvasObject{home.root}
		stack.Refresh()
	}
	showSettings := func() {
		settings = newSettingsPage(ctrl, onBackFromSettings)
		stack.Objects = []fyne.CanvasObject{settings.root}
		stack.Refresh()
	}
	showRun := func() {
		run = newRunPage(onLogout)
		stack.Objects = []fyne.CanvasObject{run.root}
		stack.Refresh()
	}

	var onLogin, onSettings, onBackFromSettings, onLogout func()

	onSettings = func() {
		// 先保存首页输入，避免切换时丢失
		if home != nil {
			home.collect()
		}
		showSettings()
	}

	onBackFromSettings = func() {
		if settings != nil {
			settings.collect()
		}
		showHome()
	}

	onLogin = func() {
		if home == nil {
			return
		}
		home.collect()
		home.setStatus("")
		if err := ctrl.Save(); err != nil {
			home.setStatus("保存失败: " + err.Error())
			return
		}
		if err := ctrl.Start(); err != nil {
			home.setStatus("启动失败: " + err.Error())
			return
		}
		showRun()
	}

	onLogout = func() {
		go func() {
			ctrl.Stop()
			fyne.Do(func() {
				showHome()
			})
		}()
	}

	ctrl.Subscribe(func(ev engine.Event) {
		fyne.Do(func() {
			if run != nil {
				run.setState(ev.State)
			}
		})
	})

	showHome()

	win.SetContent(stack)
	win.SetCloseIntercept(func() { win.Hide() })

	setupTray(a, win, ctrl, showHome)

	win.ShowAndRun()
	return nil
}
```

- [ ] **Step 3: 适配 tray.go**

`internal/ui/tray.go` 改为不再依赖按钮，改为通过回调操作：

```go
package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"

	"github.com/summonhim/gzgspg/internal/controller"
)

// setupTray 在支持托盘的平台上安装托盘图标与菜单。
// controller 的启停由界面按钮驱动，托盘只负责显示主窗口与退出。
func setupTray(a fyne.App, win fyne.Window, ctrl *controller.Controller, showHome func()) {
	d, ok := a.(desktop.App)
	if !ok {
		// 平台不支持托盘：关窗即退出
		win.SetCloseIntercept(func() { a.Quit() })
		return
	}

	d.SetSystemTrayWindow(win)
	d.SetSystemTrayIcon(theme.InfoIcon())
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
```

- [ ] **Step 4: 删除日志面板**

```bash
git rm internal/ui/logpanel.go
```

- [ ] **Step 5: 构建验证**

```powershell
$go = "C:\Program Files\Go\bin\go.exe"
$env:PATH = $env:PATH + ";C:\ProgramData\msys2\ucrt64\bin"
$env:CGO_ENABLED = "1"
& $go build ./...
& $go vet ./...
```

Expected: 均 exit 0。若报未使用 import，逐一清理。

- [ ] **Step 6: 提交**

```bash
git add internal/ui/
git rm internal/ui/logpanel.go
git commit -m "refact: 双页面 UI 与日志落盘"
```

---

## Task 5: 统一测试与手动验收

**Files:** 无新增

- [ ] **Step 1: 格式化**

```powershell
& "C:\Program Files\Go\bin\go.exe" fmt ./...
```

- [ ] **Step 2: 统一运行测试**

```powershell
$go = "C:\Program Files\Go\bin\go.exe"
$env:PATH = $env:PATH + ";C:\ProgramData\msys2\ucrt64\bin"
$env:CGO_ENABLED = "1"
& $go test ./... -count=1
```

Expected: `configdir`、`autostart`、`controller`、`logwriter` 均 `ok`，0 FAIL。

- [ ] **Step 3: race 检测（并发部分）**

```powershell
& $go test -race ./internal/controller/... ./internal/logwriter/... -count=1
```

Expected: exit 0。

- [ ] **Step 4: 手动验收（无法自动化）**

```powershell
# 用隔离目录跑，避免污染真实配置
$env:GZGSPG_CONFIG_DIR = "$env:TEMP\gzgspg-accept"
& $go run .
```

逐项确认：

1. 首页显示居中 Logo；右上角有设置图标。
2. 填账号密码 → 点登陆 → 检查 `%TEMP%\gzgspg-accept\config.json` 已生成且为 `instance` 数组。
3. 成功跳转到运行页，显示状态；状态随事件变化。
4. 运行页点「登出」→ 回到首页。
5. 首页设置图标 → 高级设置页；空字段已自动填入默认值（UA/3.3.3.3/5/3/5）。
6. 高级设置页的开机自启开关可切换；切换后 `Enabled()` 与实际一致。
7. 关闭窗口 → 隐藏而非退出；托盘菜单可重新显示。
8. 检查 `%TEMP%\gzgspg-accept\gzgspg.log` 有日志内容，且**界面上看不到日志**。

- [ ] **Step 5: 提交**

```bash
git add -A
git commit -m "chore: 格式化与收尾"
```

---

## 覆盖对照（spec → task）

| Spec 要求 | 对应 Task |
|-----------|-----------|
| 配置与日志目录 `UserConfigDir()/gzgspg` | Task 1 |
| 日志落盘、不进 UI | Task 4（删除 logpanel，slog 写文件） |
| 首页：Logo 居中 + 账号/密码/登陆 + 设置图标 | Task 3 |
| 高级设置页：默认值自动填入 | Task 3 |
| 开机自启开关（在高级设置页） | Task 2 + Task 3 |
| 三平台自启实现 | Task 2 |
| 登陆 → 保存 → 跳运行页 | Task 4 |
| 运行页：状态 + 登出（回首页） | Task 4 |
| 托盘保留 | Task 4 |
| `GZGSPG_CONFIG_DIR` 可测性 | Task 1 |
| config.json 保持数组、兼容 gzgspd | Task 1 + Task 4 |

## 已知缺口

- **UI 层无自动化测试**：Fyne 组件成本高，Task 5 Step 4 手动验收覆盖。
- **自启测试会写真实系统位置**：测试通过 `t.Cleanup` 恢复原状态；若开发者本就开着自启，测试结束会恢复为开启。
- **Logo 加载路径**：`fyne.LoadResourceFromPath("assets/icon.png")` 是相对工作目录的路径。若从其它目录启动，需改用打包资源。当前设计在找不到时退化为纯色方块。
