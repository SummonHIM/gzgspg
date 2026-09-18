# gzgspg 托盘退出与状态中文化实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 托盘「退出」在运行中先显示「正在登出」、等登出完成再退出；`engine.State` 展示中文化、运行页字号加大；托盘菜单首行显示运行状态。

**Architecture:** 新增纯函数 `stateLabel` 承载状态→中文映射，供运行页与托盘共用；`runPage` 的状态控件由 `widget.Label` 换成 `canvas.Text`（自带 `TextSize`）；`tray.go` 增加一条禁用的状态菜单项并在状态变化时 `Refresh`；`app.go` 提供 `showRun`/`quit` 两个闭包，把退出流程集中在一处。

**Tech Stack:** Go 1.27、Fyne v2.8.1（`canvas.Text`、`fyne.MenuItem.Disabled`、`fyne.Menu.Refresh`）、`github.com/summonhim/gzgspd/engine`。

## Global Constraints

- **构建环境**：`CGO_ENABLED=1`，gcc 来自 `C:\ProgramData\msys2\ucrt64\bin`；必须用 `C:\Program Files\Go\bin\go.exe`，只把 ucrt64 加到 PATH 末尾供 gcc 用。
- **线程约束**：所有 UI 更新必须经 `fyne.Do()`；托盘与订阅回调都跑在非主线程。
- **退出停顿**：仅当 `ctrl.Running()` 为真时停顿，时长固定 `600 * time.Millisecond`。
- **状态文案**（必须逐字一致）：
  - `StateStarting` → `正在启动`
  - `StateNotLoggedIn` → `未登录`
  - `StateLoggingIn` → `正在登录`
  - `StateLoggedIn` → `登录成功`
  - `StatePaused` → `已暂停（多次失败）`
  - `StateLoggingOut` → `正在登出`
  - `StateStopped` → `已停止`
  - 其它 → `未知状态`
- **运行页字号**：`TextSize: 28`，粗体，居中。
- **测试策略（用户指定）**：测试与实现一起写，全部任务完成后统一运行一次 `go test ./...`。
- 不实现：退出倒计时、托盘图标变色、退出确认对话框、托盘的启动/停止菜单项。

---

## 文件结构

| 文件 | 动作 | 职责 |
|------|------|------|
| `internal/ui/state.go` | 新增 | `stateLabel(engine.State) string` 纯函数 |
| `internal/ui/state_test.go` | 新增 | 状态文案单元测试 |
| `internal/ui/runpage.go` | 修改 | 状态控件换成 `canvas.Text`，字号 28 |
| `internal/ui/tray.go` | 修改 | 状态首行 + 退出回调；移除对 `controller` 的直接依赖 |
| `internal/ui/app.go` | 修改 | 提供 `showRun`/`quit`，装配托盘 |

---

## Task 1: 状态中文化函数

**Files:**
- Create: `internal/ui/state.go`
- Test: `internal/ui/state_test.go`

**Interfaces:**
- Produces: `stateLabel(s engine.State) string`

- [ ] **Step 1: 写测试**

`internal/ui/state_test.go`:

```go
package ui

import (
	"testing"

	"github.com/summonhim/gzgspd/engine"
)

func TestStateLabel(t *testing.T) {
	cases := []struct {
		in   engine.State
		want string
	}{
		{engine.StateStarting, "正在启动"},
		{engine.StateNotLoggedIn, "未登录"},
		{engine.StateLoggingIn, "正在登录"},
		{engine.StateLoggedIn, "登录成功"},
		{engine.StatePaused, "已暂停（多次失败）"},
		{engine.StateLoggingOut, "正在登出"},
		{engine.StateStopped, "已停止"},
	}

	for _, tc := range cases {
		if got := stateLabel(tc.in); got != tc.want {
			t.Errorf("stateLabel(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestStateLabelUnknown(t *testing.T) {
	// 未来 engine 新增状态时必须回退到非空文案，不能显示空串
	got := stateLabel(engine.State(99))
	if got != "未知状态" {
		t.Fatalf("expected 未知状态, got %q", got)
	}
	if got == "" {
		t.Fatal("label must never be empty")
	}
}
```

- [ ] **Step 2: 运行测试，确认失败**

```powershell
$go = "C:\Program Files\Go\bin\go.exe"
$env:PATH = $env:PATH + ";C:\ProgramData\msys2\ucrt64\bin"
$env:CGO_ENABLED = "1"
& $go test ./internal/ui/ -run TestStateLabel -v
```

Expected: 编译失败，`undefined: stateLabel`。

- [ ] **Step 3: 实现**

`internal/ui/state.go`:

```go
package ui

import "github.com/summonhim/gzgspd/engine"

// stateLabel 把 engine 状态翻译为中文展示文案。
// 运行页与托盘共用，保证两处永远一致。
func stateLabel(s engine.State) string {
	switch s {
	case engine.StateStarting:
		return "正在启动"
	case engine.StateNotLoggedIn:
		return "未登录"
	case engine.StateLoggingIn:
		return "正在登录"
	case engine.StateLoggedIn:
		return "登录成功"
	case engine.StatePaused:
		return "已暂停（多次失败）"
	case engine.StateLoggingOut:
		return "正在登出"
	case engine.StateStopped:
		return "已停止"
	default:
		return "未知状态"
	}
}
```

- [ ] **Step 4: 运行测试，确认通过**

```powershell
& $go test ./internal/ui/ -run TestStateLabel -v
```

Expected: 两个测试均 PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/ui/state.go internal/ui/state_test.go
git commit -m "feat: 添加 engine 状态的中文文案"
```

---

## Task 2: 运行页改用 canvas.Text

**Files:**
- Modify: `internal/ui/runpage.go`

**Interfaces:**
- Consumes: `stateLabel(engine.State) string`（Task 1）
- Produces:
  - `newRunPage(onLogout func()) *runPage`
  - `(*runPage) root fyne.CanvasObject`（不变）
  - `(*runPage) setState(s engine.State)`（不变）

- [ ] **Step 1: 改写 runpage.go**

`internal/ui/runpage.go` 全文替换为：

```go
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
```

- [ ] **Step 2: 构建验证**

```powershell
$go = "C:\Program Files\Go\bin\go.exe"
$env:PATH = $env:PATH + ";C:\ProgramData\msys2\ucrt64\bin"
$env:CGO_ENABLED = "1"
& $go build ./...
```

Expected: exit 0。

- [ ] **Step 3: 提交**

```bash
git add internal/ui/runpage.go
git commit -m "refact: 运行页状态改中文并用 canvas.Text 放大字号"
```

---

## Task 3: 托盘状态首行与退出回调

**Files:**
- Modify: `internal/ui/tray.go`

**Interfaces:**
- Consumes: `stateLabel(engine.State) string`（Task 1）
- Produces: `setupTray(a fyne.App, win fyne.Window, onShowWindow func(), onQuit func())`
  - `onShowWindow` 由托盘「显示主窗口」调用
  - `onQuit` 由托盘「退出」调用
  - 返回的 `trayController` 有方法 `setState(s engine.State)`，由 app.go 在事件回调中调用

- [ ] **Step 1: 改写 tray.go**

`internal/ui/tray.go` 全文替换为：

```go
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
```

- [ ] **Step 2: 构建（预期失败，app.go 尚未适配）**

```powershell
& $go build ./...
```

Expected: 编译报错，`setupTray` 调用处参数数量不匹配。这是预期的——下一个 Task 修复。

- [ ] **Step 3: 暂不提交，进入 Task 4**

---

## Task 4: app.go 装配退出流程

**Files:**
- Modify: `internal/ui/app.go`

**Interfaces:**
- Consumes: `setupTray`（Task 3）、`newRunPage`（Task 2）、`stateLabel`（Task 1）
- Produces: `ui.Run() error`（不变）

- [ ] **Step 1: 改写 app.go 的视图与装配部分**

`internal/ui/app.go` 全文替换为：

```go
// Package ui 是 gzgspg 的 Fyne 界面层，只负责展示与交互。
package ui

import (
	"log/slog"
	"os"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"

	"github.com/summonhim/gzgspd/engine"
	"github.com/summonhim/gzgspg/internal/configdir"
	"github.com/summonhim/gzgspg/internal/controller"
)

// quitPause 是退出时用于让人看见「正在登出」的停顿。
const quitPause = 600 * time.Millisecond

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
	win.SetIcon(appIcon())
	win.Resize(fyne.NewSize(420, 640))

	// 三视图共用一个 Stack，切换时只替换内容
	stack := container.NewStack()

	// 闭包声明在前、赋值在后，方便视图之间互相引用
	var (
		home       *homePage
		run        *runPage
		tray       *trayController
		showHome   func()
		showRun    func()
		onLogin    func()
		onSettings func()
		onLogout   func()
		quit       func()
	)

	showHome = func() {
		home = newHomePage(ctrl, onLogin, onSettings)
		stack.Objects = []fyne.CanvasObject{home.root}
		stack.Refresh()
	}

	// showRun 切换到运行页。进入前先按当前状态设一次文案，
	// 之后由事件回调持续更新。
	showRun = func() {
		run = newRunPage(onLogout)
		run.setState(ctrl.State())
		stack.Objects = []fyne.CanvasObject{run.root}
		stack.Refresh()
	}

	onBackFromSettings := func(s *settingsPage) func() {
		return func() {
			// 离开设置页时收集，返回首页即生效
			s.collect()
			showHome()
		}
	}

	showSettings := func() {
		var s *settingsPage
		s = newSettingsPage(ctrl, func() { onBackFromSettings(s)() })
		stack.Objects = []fyne.CanvasObject{s.root}
		stack.Refresh()
	}

	onSettings = func() {
		// 先收集首页输入，避免切换时丢失
		if home != nil {
			home.collect()
		}
		showSettings()
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
			// Stop 只发起取消；等引擎真正停稳（含登出）再切回首页，
			// 否则切页后残留的引擎事件无处可去。
			ctrl.Stop()
			ctrl.WaitStopped()
			fyne.Do(showHome)
		}()
	}

	// quit 处理托盘「退出」：运行中先显示运行页、完成登出再退出。
	quit = func() {
		if !ctrl.Running() {
			// 没有会话可登出，直接退出
			fyne.Do(func() { a.Quit() })
			return
		}
		go func() {
			fyne.Do(func() {
				win.Show()
				showRun()
			})
			ctrl.Stop()
			ctrl.WaitStopped()
			time.Sleep(quitPause)
			fyne.Do(func() { a.Quit() })
		}()
	}

	ctrl.Subscribe(func(ev engine.Event) {
		fyne.Do(func() {
			if run != nil {
				run.setState(ev.State)
			}
			tray.setState(ev.State)
		})
	})

	showHome()

	win.SetContent(stack)
	win.SetCloseIntercept(func() { win.Hide() })

	tray = setupTray(a, win,
		func() { fyne.Do(func() { win.Show() }) },
		quit,
	)

	win.ShowAndRun()
	return nil
}
```

**注意**：`tray.setState` 写为带 nil 接收者保护的方法（Task 3 已实现），所以托盘不可用时 `tray` 为 nil 也不会 panic。

- [ ] **Step 2: 构建验证**

```powershell
$go = "C:\Program Files\Go\bin\go.exe"
$env:PATH = $env:PATH + ";C:\ProgramData\msys2\ucrt64\bin"
$env:CGO_ENABLED = "1"
& $go build ./...
& $go vet ./...
```

Expected: 均 exit 0。若报未使用 import，逐一清理。

- [ ] **Step 3: 提交**

```bash
git add internal/ui/app.go internal/ui/tray.go
git commit -m "feat: 托盘显示运行状态，退出前先完成登出"
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

Expected: `autostart`、`configdir`、`controller`、`ui` 均 `ok`，0 FAIL。

- [ ] **Step 3: 手动验收（无法自动化）**

```powershell
# 用隔离目录跑，避免污染真实配置
$env:GZGSPG_CONFIG_DIR = "$env:TEMP\gzgspg-accept2"
& $go run .
```

逐项确认：

1. 托盘菜单首行显示「状态：未登录」（或当前状态）。
2. 填入账号密码 → 登陆 → 运行页显示「正在登录 / 登录成功」，字号明显大于按钮文字。
3. 托盘首行随之显示「状态：登录成功」。
4. **运行中**点托盘「退出」→ 主窗口自动弹出并显示运行页 → 短暂看到「正在登出」→ 约 0.6s 后进程退出；`%TEMP%\gzgspg-accept2\gzgspg.log` 中能看到登出相关日志。
5. 重新启动 → **未登录状态**点托盘「退出」→ 立即退出，无窗口闪烁。
6. 点托盘「显示主窗口」→ 窗口出现。
7. 关闭窗口 → 隐藏而非退出。

- [ ] **Step 4: 提交**

```bash
git add -A
git commit -m "chore: 格式化与收尾"
```

---

## 覆盖对照（spec → task）

| Spec 要求 | 对应 Task |
|-----------|-----------|
| 退出：运行中先显示、等登出、再退出 | Task 4 |
| 退出：未运行直接退 | Task 4 |
| 600ms 可见停顿、仅运行中 | Task 4（`quitPause`） |
| 状态中文化 | Task 1 + Task 2 |
| 运行页字号加大（canvas.Text, 28） | Task 2 |
| 托盘首行显示状态 | Task 3 |
| 状态变化时刷新托盘 | Task 4（订阅回调） |
| 未知状态非空回退 | Task 1（`state_test.go`） |

## 已知缺口

- **托盘行为无自动化测试**：Fyne 托盘无法在测试中驱动，靠 Task 5 Step 3 手动验收。
- **`Running()` 与 `Stop()` 的竞态**：仅导致多等一次 600ms，不处理。
- **托盘字体不可控**：托盘菜单项字号由操作系统渲染，Fyne 无接口，故「字体大」只体现在运行页。
