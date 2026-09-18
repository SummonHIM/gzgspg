# gzgspg 加固与优化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复停止后状态不复位、自启路径含空格失效、错误退出码等正确性问题，并给设置页加即时校验、改进 dev 版本号、补充本地开发说明。

**Architecture:** 纯 Go + Fyne 桌面应用。业务核心在 `internal/controller`（管理 `gzgspd` 引擎生命周期与事件分发），UI 在 `internal/ui`。本次改动集中在 controller 的事件分发与配置访问、autostart 的平台实现、UI 设置页校验、version 默认值，外加根目录 AGENTS.md。

**Tech Stack:** Go 1.27、Fyne v2.8.1、github.com/summonhim/gzgspd v1.3.0（引擎）、golang.org/x/sys（Windows 注册表）。

## Global Constraints

- 引擎校验规则（与 `gzgspd` `config.Validate()` 一致）：`keep_alive > 0`、`retry_max >= 0`、`retry_time > 0`。
- 状态枚举来自 `github.com/summonhim/gzgspd/engine`，`StateStopped` 为停止态；`engine.Event` 结构体字段为 `Key/Time/State/Message/Err`。
- 所有修改遵循 TDD：先写失败测试，再实现，跑测试，提交。
- 本地（Windows）无法运行 `go vet`/`go test`（glfw/GL CGO 约束），测试在 CI 的 `ubuntu-latest` + `xvfb-run` 下跑；任务中的「跑测试」步骤在 CI 或具备 CGO 的 Linux 环境执行。
- 提交信息遵循仓库既有风格（`feat:` / `fix:` / `chore:` 前缀，中文说明）。

---

### Task 1: controller 状态复位广播 + Config 深拷贝

**Files:**
- Modify: `internal/controller/controller.go`
- Test: `internal/controller/controller_test.go`

**Interfaces:**
- Consumes: `engine.Event`（字段 `State engine.State`、`Time time.Time`）、`engine.StateStopped`、`config.Config` / `config.ConfigInstance`。
- Produces: `Controller.notify(e engine.Event)`（内部方法）；`Controller.Config()` 返回深拷贝；`Controller.consume` 在复位后广播合成 `StateStopped` 事件。

- [ ] **Step 1: 写失败测试**

在 `internal/controller/controller_test.go` 末尾追加两个测试：

```go
func TestConfigIsCopy(t *testing.T) {
	c, _ := newTestController(t)
	got := c.Config()
	got.Instance[0].Username = "mutated"
	got.LogLevel = 99
	if c.Instance().Username != "" {
		t.Fatalf("expected username unchanged, got %q", c.Instance().Username)
	}
	if c.Config().LogLevel != 0 {
		t.Fatalf("expected log level unchanged, got %d", c.Config().LogLevel)
	}
}

func TestStopBroadcastsFinalStoppedEvent(t *testing.T) {
	c, _ := newTestController(t)
	c.UpdateInstance(validInstance())

	events := make(chan engine.Event, 64)
	c.Subscribe(func(e engine.Event) {
		select {
		case events <- e:
		default:
		}
	})

	if err := c.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	c.Stop()
	c.WaitStopped()

	var last engine.Event
	for drained := false; !drained; {
		select {
		case e := <-events:
			last = e
		default:
			drained = true
		}
	}
	if last.State != engine.StateStopped {
		t.Fatalf("expected final StateStopped, got %v", last.State)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `xvfb-run -a go test ./internal/controller/ -run 'TestConfigIsCopy|TestStopBroadcastsFinalStoppedEvent' -count=1`
Expected: `TestConfigIsCopy` FAIL（`Config()` 返回内部指针，`Instance[0]` 被改）；`TestStopBroadcastsFinalStoppedEvent` FAIL（最后事件不是 `StateStopped`）。

- [ ] **Step 3: 实现**

修改 `internal/controller/controller.go`：

(1) import 增加 `"time"`：

```go
import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/summonhim/gzgspd/config"
	"github.com/summonhim/gzgspd/engine"
)
```

(2) 把 `Config()` 改为返回深拷贝：

```go
// Config 返回当前配置的副本。调用方修改返回值不会影响内部状态。
func (c *Controller) Config() *config.Config {
	c.mu.Lock()
	defer c.mu.Unlock()
	cp := *c.cfg
	cp.Instance = append([]config.ConfigInstance(nil), c.cfg.Instance...)
	return &cp
}
```

(3) 新增 `notify` 方法（放在 `Subscribe` 之后）：

```go
// notify 在锁内更新状态并拷贝订阅者列表，锁外回调，避免回调期间持锁。
func (c *Controller) notify(e engine.Event) {
	c.mu.Lock()
	c.state = e.State
	c.lastErr = e.Err
	observers := append([]func(engine.Event){}, c.observer...)
	c.mu.Unlock()

	for _, fn := range observers {
		fn(e)
	}
}
```

(4) 改写 `consume`：事件循环改用 `notify`，并在复位后广播合成 `StateStopped` 事件：

```go
// consume 运行引擎并把事件转发给订阅者，结束后复位状态。
func (c *Controller) consume(ctx context.Context, eng *engine.Engine) {
	defer c.wg.Done()

	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := eng.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			c.logger.Error("engine stopped with error", "error", err)
		}
	}()

	for e := range eng.Events() {
		c.notify(e)
	}
	<-done

	c.mu.Lock()
	c.running = false
	c.eng = nil
	c.cancel = nil
	c.state = engine.StateStopped
	c.mu.Unlock()

	// 复位后广播一次停止事件，让托盘/运行页从「正在登出」回到「已停止」。
	c.notify(engine.Event{State: engine.StateStopped, Time: time.Now()})
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `xvfb-run -a go test ./internal/controller/ -run 'TestConfigIsCopy|TestStopBroadcastsFinalStoppedEvent' -count=1`
Expected: PASS（两个都通过）。

- [ ] **Step 5: 跑全量 controller 测试**

Run: `xvfb-run -a go test ./internal/controller/ -count=1`
Expected: PASS（现有测试不回归）。

- [ ] **Step 6: 提交**

```bash
git add internal/controller/controller.go internal/controller/controller_test.go
git commit -m "fix: 停止后广播 StateStopped 事件，Config 返回深拷贝"
```

---

### Task 2: 自启路径加引号（Windows / Linux）

**Files:**
- Modify: `internal/autostart/autostart_windows.go`
- Modify: `internal/autostart/autostart_linux.go`

**Interfaces:**
- Consumes: `writeTarget(path string) error`、`targetPath() (string, bool)`（两平台各自已有）。
- Produces: 读写两侧对引号做对称处理，保证 `Reconcile` 的路径相等比较不变。

- [ ] **Step 1: 确认现有测试**

Run: `xvfb-run -a go test ./internal/autostart/ -count=1`
Expected: 当前 PASS（作为基线；本任务不改测试，靠现有往返用例保证读写一致）。

- [ ] **Step 2: 修改 Windows 实现**

`internal/autostart/autostart_windows.go`：

(1) import 增加 `"strings"`：

```go
import (
	"errors"
	"os"
	"strings"
	"syscall"

	"golang.org/x/sys/windows/registry"
)
```

(2) `writeTarget` 存值时加引号；`targetPath` 读回时去掉引号：

```go
// writeTarget 把自启项指向指定路径（不存在则创建）。
// 路径整体加引号，避免含空格路径（如 Program Files）在 Run 键里被截断。
func writeTarget(path string) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetStringValue(AppName, `"`+path+`"`)
}

// targetPath 返回当前自启项指向的可执行路径；不存在时 ok 为 false。
func targetPath() (string, bool) {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return "", false
	}
	defer k.Close()
	v, _, err := k.GetStringValue(AppName)
	if err != nil {
		return "", false
	}
	return strings.Trim(v, `"`), true
}
```

- [ ] **Step 3: 修改 Linux 实现**

`internal/autostart/autostart_linux.go` 的 `writeTarget` 与 `targetPath`：

```go
// writeTarget 把自启项指向指定路径，写入 autostart desktop 文件。
// 路径含空格时用引号包裹，避免 Exec= 行被按空格拆解。
func writeTarget(path string) error {
	p, err := desktopFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	exec := path
	if strings.Contains(path, " ") {
		exec = `"` + path + `"`
	}
	content := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=%s
Exec=%s
X-GNOME-Autostart-enabled=true
`, AppName, exec)
	return os.WriteFile(p, []byte(content), 0o644)
}

// targetPath 返回 desktop 文件中 Exec= 后的内容；读不到时 ok 为 false。
func targetPath() (string, bool) {
	p, err := desktopFilePath()
	if err != nil {
		return "", false
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "Exec=") {
			// 容错 CRLF 与引号：去掉行尾回车与包裹引号，保证 Reconcile 相等比较稳定。
			v := strings.TrimPrefix(line, "Exec=")
			v = strings.TrimRight(v, "\r")
			return strings.Trim(v, `"`), true
		}
	}
	return "", false
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `xvfb-run -a go test ./internal/autostart/ -count=1`
Expected: PASS（`TestReconcileEnablesWithCorrectPath` 与 `TestReconcileRepairsStalePath` 仍通过，因 `targetPath` 读回时已去掉引号）。

- [ ] **Step 5: 提交**

```bash
git add internal/autostart/autostart_windows.go internal/autostart/autostart_linux.go
git commit -m "fix: 自启路径加引号，兼容含空格安装路径"
```

---

### Task 3: main.go 非零退出码

**Files:**
- Modify: `main.go`

**Interfaces:**
- Consumes: `ui.Run() error`（保持不变）。
- Produces: 无新接口。

- [ ] **Step 1: 修改 main.go**

```go
package main

import (
	"fmt"
	"os"

	"github.com/summonhim/gzgspg/internal/ui"
)

func main() {
	if err := ui.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

- [ ] **Step 2: 编译验证**

Run: `go build ./...`
Expected: 成功（无法在本地跑 vet/test，但可编译）。

- [ ] **Step 3: 提交**

```bash
git add main.go
git commit -m "fix: 启动失败时以非零退出码退出"
```

---

### Task 4: 设置页数值内联校验

**Files:**
- Modify: `internal/ui/settingspage.go`
- Modify: `internal/ui/app.go`
- Test: `internal/ui/settingspage_test.go`

**Interfaces:**
- Consumes: `settingsPage.collect()`（改为返回 `bool`）、`strconv.Atoi`、Fyne `widget.Entry.Validator` / `AlwaysShowValidationError`。
- Produces: `validatePositiveInt(s string) error`、`validateNonNegativeInt(s string) error`；`settingsPage.collect() bool`（供 `app.go` 的 `onBackFromSettings` 判断是否离开设置页）。

- [ ] **Step 1: 写失败测试**

在 `internal/ui/settingspage_test.go` 末尾追加：

```go
func TestValidatePositiveInt(t *testing.T) {
	for _, in := range []string{"1", "5", "999"} {
		if err := validatePositiveInt(in); err != nil {
			t.Errorf("validatePositiveInt(%q) = %v, want nil", in, err)
		}
	}
	for _, in := range []string{"", "0", "-1", "abc", "1.5"} {
		if err := validatePositiveInt(in); err == nil {
			t.Errorf("validatePositiveInt(%q) = nil, want error", in)
		}
	}
}

func TestValidateNonNegativeInt(t *testing.T) {
	for _, in := range []string{"0", "3", "999"} {
		if err := validateNonNegativeInt(in); err != nil {
			t.Errorf("validateNonNegativeInt(%q) = %v, want nil", in, err)
		}
	}
	for _, in := range []string{"", "-1", "abc", "1.5"} {
		if err := validateNonNegativeInt(in); err == nil {
			t.Errorf("validateNonNegativeInt(%q) = nil, want error", in)
		}
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `xvfb-run -a go test ./internal/ui/ -run 'TestValidatePositiveInt|TestValidateNonNegativeInt' -count=1`
Expected: FAIL（函数未定义）。

- [ ] **Step 3: 修改 settingspage.go**

(1) import 增加 `"errors"`：

```go
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
```

(2) 删除三个 `default*` 常量块（第 16-21 行）：

```go
// 数值字段的兜底默认值，仅在输入框为空或内容非法时使用。
const (
	defaultKeepAlive = controller.DefaultKeepAlive
	defaultRetryMax  = controller.DefaultRetryMax
	defaultRetryTime = controller.DefaultRetryTime
)
```

（该块整段删除；`controller` import 仍被 settingsPage 的 `ctrl *controller.Controller` 使用，保留。）

(3) 在 `newSettingsPage` 中 `s.load()` 之后、`prefs := ...` 之前，给三个数值 Entry 装校验器：

```go
	s.load()

	// 数值字段内联校验：规则与 engine 的 Validate 一致，输入即时提示。
	s.keepAlive.Validator = validatePositiveInt
	s.keepAlive.AlwaysShowValidationError = true
	s.retryMax.Validator = validateNonNegativeInt
	s.retryMax.AlwaysShowValidationError = true
	s.retryTime.Validator = validatePositiveInt
	s.retryTime.AlwaysShowValidationError = true

	prefs := fyne.CurrentApp().Preferences()
```

(4) 把 `collect()` 改为校验 + 返回 `bool`：

```go
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
```

(5) 删除 `atoiOr` 函数（第 152-159 行整段），替换为两个校验函数：

```go
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
```

- [ ] **Step 4: 修改 app.go**

`internal/ui/app.go` 的 `onBackFromSettings`（第 89-95 行）改为依据 `collect()` 返回值决定是否返回：

```go
	onBackFromSettings := func(s *settingsPage) func() {
		return func() {
			// 离开设置页时校验并收集；数值非法则留在设置页让用户修正。
			if !s.collect() {
				return
			}
			showHome()
		}
	}
```

- [ ] **Step 5: 运行测试确认通过**

Run: `xvfb-run -a go test ./internal/ui/ -run 'TestValidatePositiveInt|TestValidateNonNegativeInt|TestVersionLabelPrefixesVersion' -count=1`
Expected: PASS。

- [ ] **Step 6: 编译验证**

Run: `go build ./...`
Expected: 成功。

- [ ] **Step 7: 提交**

```bash
git add internal/ui/settingspage.go internal/ui/app.go internal/ui/settingspage_test.go
git commit -m "feat: 设置页数值字段内联校验，非法值阻止保存返回"
```

---

### Task 5: version.BuildTime 默认值

**Files:**
- Modify: `internal/version/version.go`
- Test: `internal/version/version_test.go`

**Interfaces:**
- Consumes: 无。
- Produces: `version.BuildTime` 默认值改为 `"unknown"`。

- [ ] **Step 1: 改失败测试**

`internal/version/version_test.go` 的 `TestDefaults` 改为：

```go
func TestDefaults(t *testing.T) {
	if Value != "dev" {
		t.Fatalf("Value default = %q, want %q", Value, "dev")
	}
	if BuildTime != "unknown" {
		t.Fatalf("BuildTime default = %q, want %q", BuildTime, "unknown")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `xvfb-run -a go test ./internal/version/ -run TestDefaults -count=1`
Expected: FAIL（当前 `BuildTime` 为 `"0"`）。

- [ ] **Step 3: 改实现**

`internal/version/version.go`：

```go
	// BuildTime 由构建期 -ldflags -X 注入。
	BuildTime = "unknown"
```

- [ ] **Step 4: 运行测试确认通过**

Run: `xvfb-run -a go test ./internal/version/ -count=1`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/version/version.go internal/version/version_test.go
git commit -m "chore: dev 构建 BuildTime 默认值改为 unknown"
```

---

### Task 6: 新增 AGENTS.md

**Files:**
- Create: `AGENTS.md`

**Interfaces:**
- Consumes: 无。
- Produces: 仓库级开发说明文档。

- [ ] **Step 1: 创建 AGENTS.md**

```markdown
# AGENTS.md

广工商校园网登录器 GUI。Fyne 桌面应用，业务核心在 `internal/controller`（管理配置与
`github.com/summonhim/gzgspd` 登录引擎的生命周期，并把事件分发给 UI），UI 层在
`internal/ui`，平台自启实现在 `internal/autostart`。

## 本地构建与测试

- 本项目依赖 glfw/GL（CGO）。在 Windows 上直接跑 `go vet ./...` / `go test ./...`
  会因 build constraints 失败（`go-gl/gl` 相关文件被排除）。
- vet 与测试在 CI 的 `ubuntu-latest` + `xvfb-run` 下运行（见
  `.github/workflows/ci.yml`）。本地改动应在 CI 验证，而非仅依赖本机编译。
- 本地只做编译检查：`go build ./...`。

## 配置与数据目录

- 配置与日志目录为 `os.UserConfigDir()/gzgspg`，可用环境变量 `GZGSPG_CONFIG_DIR`
  覆盖（见 `internal/configdir`）。
- `config.json` 含明文密码，已被 `.gitignore` 忽略，勿提交真实凭据。

## 常用命令

- 编译：`go build ./...`
- 测试（Linux/CI）：`xvfb-run -a go test ./... -count=1`
- 构建发布包：见 `.github/workflows/ci.yml` 的 build job。
```

- [ ] **Step 2: 提交**

```bash
git add AGENTS.md
git commit -m "docs: 新增 AGENTS.md，说明本地构建与测试约束"
```

---

## 自审记录

- **Spec 覆盖**：7 项改动各对应一个任务（Task 1 含改动 1 与 4；Task 2 对应改动 2；Task 3 对应改动 3；Task 4 对应改动 5；Task 5 对应改动 6；Task 6 对应改动 7）。
- **占位符**：无 TBD/TODO；每个代码步骤含完整代码。
- **类型一致性**：`notify` / `Config` / `collect() bool` / `validatePositiveInt` / `validateNonNegativeInt` 在定义与调用处签名一致；`engine.Event` 字段与 gzgspd 源码一致。
