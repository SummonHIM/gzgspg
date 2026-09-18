# gzgspg 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现 gzgspg——单账号、托盘常驻的校园网登录器 GUI，进程内复用 gzgspd 的 engine。

**Architecture:** `ui → controller → engine(gzgspd)`。controller 是 UI 与 engine 之间的唯一边界，也是唯一持有业务状态的地方；Fyne UI 只做展示。engine 通过 go.work 指向平级目录的 gzgspd。

**Tech Stack:** Go 1.25、Fyne v2.8.1、`fyne.io/fyne/v2/driver/desktop`（托盘）、`log/slog`。构建需 CGO + gcc。

## Global Constraints

- **UI 只管一个账号**，始终操作 `config.Config` 的 `instance[0]`。
- **config.json 保持数组结构**，与 gzgspd 双向兼容；不新建独立配置文件格式。
- **Fyne 版本锁定 v2.8.1**（`fyne.Do` 与托盘 API 以此版本实测为准）。
- **托盘 API**：断言 `desktop.App` 后使用；**必须调用 `SetSystemTrayWindow(win)`**，否则部分平台关窗即退出。
- **线程约束**：所有 UI 更新必须经 `fyne.Do()`；controller 的事件回调不得直接触碰 Fyne 组件。
- **所有阻塞调用不得在 UI 主线程执行**：`Start()`/`Stop()` 需在 goroutine 中调用。
- **构建环境**：Windows 上须 `CGO_ENABLED=1` 且 PATH 含 `C:\ProgramData\msys2\ucrt64\bin`。
- **测试策略（用户指定）**：测试与实现一起写，全部任务完成后统一运行一次 `go test ./...`。
- 不实现：密码加密、开机自启、定时任务、自动更新、多语言、主题、多账号。

---

## 文件结构

| 文件 | 职责 |
|------|------|
| `go.mod` | module `github.com/summonhim/gzgspg` |
| `main.go` | 入口，调用 `ui.Run()` |
| `FyneApp.toml` | Fyne 应用元数据 |
| `internal/logwriter/writer.go` | `io.Writer` → 有界内存缓冲，供日志面板读取 |
| `internal/logwriter/writer_test.go` | 测试 |
| `internal/controller/controller.go` | 配置 + engine 生命周期 + 事件分发 |
| `internal/controller/controller_test.go` | 测试 |
| `internal/ui/app.go` | 应用装配、主窗口、生命周期 |
| `internal/ui/editor.go` | 单账号表单 |
| `internal/ui/logpanel.go` | 日志面板 |
| `internal/ui/tray.go` | 托盘图标与菜单 |

---

## Task 1: 项目骨架与依赖

**Files:**
- Create: `gzgspg/go.mod`
- Create: `gzgspg/main.go`
- Create: `gzgspg/.gitignore`
- Create: `Golang/go.work`

**Interfaces:**
- Produces: 可编译的空骨架

- [ ] **Step 1: 创建 go.mod 并拉取依赖**

在 `C:\Users\SummonHIM\Standard\Projects\Golang\gzgspg` 执行：

```powershell
$env:GOPROXY = "https://goproxy.cn,direct"
go mod init github.com/summonhim/gzgspg
go get fyne.io/fyne/v2@v2.8.1
go get github.com/summonhim/gzgspd@v1.3.0
```

- [ ] **Step 2: 创建 go.work 工作区**

在 `C:\Users\SummonHIM\Standard\Projects\Golang` 执行：

```powershell
go work init ./gzgspd ./gzgspg
```

验证 `go.work` 内容包含两个目录：

```powershell
Get-Content go.work
```

期望输出类似：

```
go 1.25.0

use (
	./gzgspd
	./gzgspg
)
```

- [ ] **Step 3: 创建 .gitignore**

`gzgspg/.gitignore`:

```
# Binaries
*.exe
*.exe~
*.dll
*.so
*.dylib

# Test artifacts
*.test
*.out
coverage.*

# Build output
build/
dist/

# Config (contains credentials)
/config.json

# Editor
.vscode
```

- [ ] **Step 4: 创建最小 main.go**

`gzgspg/main.go`:

```go
package main

import (
	"fmt"

	"github.com/summonhim/gzgspg/internal/ui"
)

func main() {
	if err := ui.Run(); err != nil {
		fmt.Println(err)
	}
}
```

由于 `internal/ui` 尚不存在，此步先创建占位实现以便编译：

`gzgspg/internal/ui/app.go`:

```go
package ui

// Run 启动 GUI。Task 4 会替换为完整实现。
func Run() error {
	return nil
}
```

- [ ] **Step 5: 构建验证**

```powershell
$env:PATH = "C:\ProgramData\msys2\ucrt64\bin;" + $env:PATH
$env:CGO_ENABLED = "1"
go build ./...
```

Expected: exit 0。

- [ ] **Step 6: 提交**

```bash
git add go.mod go.sum .gitignore main.go internal/ui/app.go
git commit -m "chore: 初始化 gzgspg 项目骨架"
```

（`go.work` 在上一级目录，不进 gzgspg 版本库。）

---

## Task 2: logwriter 包

**Files:**
- Create: `gzgspg/internal/logwriter/writer.go`
- Test: `gzgspg/internal/logwriter/writer_test.go`

**Interfaces:**
- Produces:
  - `logwriter.New(maxLines int) *Writer`
  - `(*Writer) Write(p []byte) (int, error)` — 实现 `io.Writer`
  - `(*Writer) Lines() []string` — 返回当前缓冲的副本
  - `(*Writer) Subscribe(fn func(line string))`
  - 满足 `slog` 所需的 `io.Writer`

- [ ] **Step 1: 写测试**

`internal/logwriter/writer_test.go`:

```go
package logwriter

import (
	"strings"
	"testing"
)

func TestWriterKeepsLastLines(t *testing.T) {
	w := New(3)
	for _, s := range []string{"a\n", "b\n", "c\n", "d\n"} {
		if _, err := w.Write([]byte(s)); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	lines := w.Lines()
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "b" || lines[1] != "c" || lines[2] != "d" {
		t.Fatalf("unexpected lines: %v", lines)
	}
}

func TestWriterSplitsWithoutTrailingNewline(t *testing.T) {
	w := New(10)
	_, _ = w.Write([]byte("partial"))
	_, _ = w.Write([]byte(" line\n"))
	lines := w.Lines()
	if len(lines) != 1 || lines[0] != "partial line" {
		t.Fatalf("expected merged line, got %v", lines)
	}
}

func TestWriterSubscribe(t *testing.T) {
	w := New(10)
	var got []string
	w.Subscribe(func(line string) { got = append(got, line) })
	_, _ = w.Write([]byte("hello\nworld\n"))
	if len(got) != 2 || got[0] != "hello" || got[1] != "world" {
		t.Fatalf("unexpected notifications: %v", got)
	}
}

func TestWriterLinesIsCopy(t *testing.T) {
	w := New(10)
	_, _ = w.Write([]byte("x\n"))
	lines := w.Lines()
	lines[0] = "mutated"
	if w.Lines()[0] != "x" {
		t.Fatal("Lines() must return a copy")
	}
}

func TestWriterTrimsANSIEscape(t *testing.T) {
	w := New(10)
	_, _ = w.Write([]byte("plain line\n"))
	if strings.Contains(w.Lines()[0], "\x1b") {
		t.Fatal("unexpected escape sequence")
	}
}
```

- [ ] **Step 2: 实现**

`internal/logwriter/writer.go`:

```go
// Package logwriter 提供一个有界的 io.Writer，用于把 slog 输出送给 UI。
package logwriter

import (
	"strings"
	"sync"
)

// Writer 收集以换行分隔的日志行，只保留最近 maxLines 行。
type Writer struct {
	mu        sync.Mutex
	maxLines  int
	lines     []string
	partial   string
	observers []func(string)
}

// New 创建 Writer，maxLines 为保留的最大行数。
func New(maxLines int) *Writer {
	if maxLines <= 0 {
		maxLines = 500
	}
	return &Writer{maxLines: maxLines}
}

// Write 实现 io.Writer。
func (w *Writer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	text := w.partial + string(p)
	parts := strings.Split(text, "\n")
	// 最后一段没有换行结尾，留作 partial
	w.partial = parts[len(parts)-1]

	for _, line := range parts[:len(parts)-1] {
		w.appendLocked(line)
	}
	return len(p), nil
}

func (w *Writer) appendLocked(line string) {
	w.lines = append(w.lines, line)
	if len(w.lines) > w.maxLines {
		w.lines = w.lines[len(w.lines)-w.maxLines:]
	}
	for _, fn := range w.observers {
		fn(line)
	}
}

// Lines 返回当前缓冲的副本。
func (w *Writer) Lines() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]string, len(w.lines))
	copy(out, w.lines)
	return out
}

// Subscribe 注册每行日志的回调。回调在 Write 的调用栈中同步执行。
func (w *Writer) Subscribe(fn func(line string)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.observers = append(w.observers, fn)
}
```

- [ ] **Step 3: 构建验证**

```powershell
$env:PATH = "C:\ProgramData\msys2\ucrt64\bin;" + $env:PATH
$env:CGO_ENABLED = "1"
go build ./...
```

Expected: exit 0。

- [ ] **Step 4: 提交**

```bash
git add internal/logwriter/
git commit -m "feat: 添加日志缓冲 Writer"
```

---

## Task 3: controller 包

**Files:**
- Create: `gzgspg/internal/controller/controller.go`
- Test: `gzgspg/internal/controller/controller_test.go`

**Interfaces:**
- Consumes: `config.LoadConfig/Save/SetFilePath/Validate`、`engine.New/Run/Stop/Sessions/Events`、`engine.State/Event`
- Produces:
  - `controller.Options{ConfigPath string; Logger *slog.Logger}`
  - `controller.New(opts Options) (*Controller, error)`
  - `(*Controller) Config() *config.Config`
  - `(*Controller) Instance() config.ConfigInstance`
  - `(*Controller) UpdateInstance(inst config.ConfigInstance)`
  - `(*Controller) Save() error`
  - `(*Controller) Start() error`
  - `(*Controller) Stop()`
  - `(*Controller) Running() bool`
  - `(*Controller) State() engine.State`
  - `(*Controller) Subscribe(fn func(engine.Event))`

**关键约束（来自 gzgspd 实测）**：
- `config.Save()` 在未设置 filePath 时报错。配置不存在时 `LoadConfig` 失败，必须自行 `SetFilePath`。
- `engine.New` 内部调用 `cfg.Validate()`，空配置或非法配置会返回错误。`Start()` 必须把这个错误返回给调用方。

- [ ] **Step 1: 写测试**

`internal/controller/controller_test.go`:

```go
package controller

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/summonhim/gzgspd/config"
	"github.com/summonhim/gzgspd/engine"
)

func newTestController(t *testing.T) (*Controller, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	c, err := New(Options{ConfigPath: path})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	return c, path
}

func validInstance() config.ConfigInstance {
	return config.ConfigInstance{
		Username: "u", Password: "p", Interface: "lo",
		KeepAlive: 5, RetryTime: 5,
	}
}

func TestNewWithMissingFileUsesEmptyConfig(t *testing.T) {
	c, path := newTestController(t)
	if c.Config() == nil {
		t.Fatal("expected non-nil config")
	}
	if len(c.Config().Instance) != 0 {
		t.Fatalf("expected empty instance list, got %d", len(c.Config().Instance))
	}
	// 未保存前文件不应存在
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("config file should not exist yet, err=%v", err)
	}
}

func TestUpdateInstanceCreatesFirstSlot(t *testing.T) {
	c, _ := newTestController(t)
	c.UpdateInstance(validInstance())
	if got := c.Instance().Username; got != "u" {
		t.Fatalf("expected username u, got %q", got)
	}
	if len(c.Config().Instance) != 1 {
		t.Fatalf("expected 1 instance, got %d", len(c.Config().Instance))
	}
}

func TestUpdateInstanceReplacesFirstSlot(t *testing.T) {
	c, _ := newTestController(t)
	c.UpdateInstance(validInstance())
	inst := validInstance()
	inst.Username = "second"
	c.UpdateInstance(inst)
	if c.Instance().Username != "second" {
		t.Fatalf("expected replacement, got %q", c.Instance().Username)
	}
	if len(c.Config().Instance) != 1 {
		t.Fatalf("expected still 1 instance, got %d", len(c.Config().Instance))
	}
}

func TestInstanceIsCopy(t *testing.T) {
	c, _ := newTestController(t)
	c.UpdateInstance(validInstance())
	got := c.Instance()
	got.Username = "mutated"
	if c.Instance().Username != "u" {
		t.Fatal("Instance() must return a copy")
	}
}

func TestSaveRoundTrip(t *testing.T) {
	c, path := newTestController(t)
	c.UpdateInstance(validInstance())
	if err := c.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	insts, ok := raw["instance"].([]any)
	if !ok || len(insts) != 1 {
		t.Fatalf("expected array with 1 element, got %v", raw["instance"])
	}

	// 重新加载应保持一致
	c2, err := New(Options{ConfigPath: path})
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if c2.Instance().Username != "u" || c2.Instance().KeepAlive != 5 {
		t.Fatalf("unexpected reloaded instance: %+v", c2.Instance())
	}
}

func TestSaveWithoutInstanceStillWritesFile(t *testing.T) {
	c, path := newTestController(t)
	// 空配置时 Save 不应因 Validate 失败而报错——Save 不做校验
	if err := c.Save(); err != nil {
		t.Fatalf("save empty: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file to exist: %v", err)
	}
}

func TestStartRejectsInvalidConfig(t *testing.T) {
	c, _ := newTestController(t)
	// 空配置 → engine.New 的 Validate 失败
	if err := c.Start(); err == nil {
		t.Fatal("expected error starting with empty config")
	}
	if c.Running() {
		t.Fatal("should not be running after failed start")
	}
}

func TestStateWhenNotRunning(t *testing.T) {
	c, _ := newTestController(t)
	if c.State() != engine.StateStopped {
		t.Fatalf("expected StateStopped, got %v", c.State())
	}
}

func TestStartStopLifecycle(t *testing.T) {
	c, _ := newTestController(t)
	c.UpdateInstance(validInstance())

	// Start 非阻塞，后台会尝试登录（lo 网卡 → 3.3.3.3 不可达，会失败重试）。
	if err := c.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	if !c.Running() {
		t.Fatal("expected controller to be running")
	}

	c.Stop()
	// Stop 触发登出后异步复位
	deadline := 0
	for c.Running() && deadline < 200 {
		sleepShort()
		deadline++
	}
	if c.Running() {
		t.Fatal("expected controller to stop")
	}
	if c.State() != engine.StateStopped {
		t.Fatalf("expected StateStopped after stop, got %v", c.State())
	}
}

func TestSubscribeReceivesEvents(t *testing.T) {
	c, _ := newTestController(t)
	c.UpdateInstance(validInstance())

	got := make(chan engine.Event, 16)
	c.Subscribe(func(e engine.Event) {
		select {
		case got <- e:
		default:
		}
	})

	if err := c.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer c.Stop()

	select {
	case e := <-got:
		if e.Key == "" {
			t.Fatalf("expected non-empty key, got %+v", e)
		}
	case <-timeoutChan():
		t.Fatal("no event received within timeout")
	}
}
```

在同文件顶部补充测试辅助（放到 import 之后）：

```go
func sleepShort() { time.Sleep(10 * time.Millisecond) }

func timeoutChan() <-chan time.Time { return time.After(5 * time.Second) }
```

并把 `"time"` 加入 import 块。

- [ ] **Step 2: 实现 controller.go**

`internal/controller/controller.go`:

```go
// Package controller 是 gzgspg 的业务核心：管理配置与登录引擎的生命周期，
// 并把 engine 的事件分发给 UI。它是唯一持有可变业务状态的地方。
package controller

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/summonhim/gzgspd/config"
	"github.com/summonhim/gzgspd/engine"
)

// Options 配置 Controller。
type Options struct {
	// ConfigPath 是 config.json 的路径。
	ConfigPath string
	// Logger 为空时使用 slog.Default()。
	Logger *slog.Logger
}

// Controller 管理单账号的配置与登录引擎。
type Controller struct {
	path   string
	logger *slog.Logger

	mu       sync.Mutex
	cfg      *config.Config
	eng      *engine.Engine
	cancel   context.CancelFunc
	running  bool
	state    engine.State
	lastErr  error
	observer []func(engine.Event)
}

// New 加载配置。配置不存在或无效时，返回一个持有空配置的 Controller（不报错），
// 以便首次运行时用户可以在界面里填写。
func New(opts Options) (*Controller, error) {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	c := &Controller{
		path:   opts.ConfigPath,
		logger: logger,
		state:  engine.StateStopped,
	}

	cfg, err := config.LoadConfig(opts.ConfigPath)
	if err != nil {
		// 文件不存在或内容非法：使用空配置，路径先记下，Save 时写出
		c.logger.Info("starting with empty config", "path", opts.ConfigPath, "reason", err)
		cfg = &config.Config{}
		cfg.SetFilePath(opts.ConfigPath)
	}
	c.cfg = cfg

	return c, nil
}

// Config 返回当前配置。调用方不应修改返回值。
func (c *Controller) Config() *config.Config {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cfg
}

// Instance 返回 instance[0] 的副本；不存在时返回零值。
func (c *Controller) Instance() config.ConfigInstance {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.cfg.Instance) == 0 {
		return config.ConfigInstance{}
	}
	return c.cfg.Instance[0]
}

// UpdateInstance 写回 instance[0]；不存在时创建。
func (c *Controller) UpdateInstance(inst config.ConfigInstance) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.cfg.Instance) == 0 {
		c.cfg.Instance = []config.ConfigInstance{inst}
		return
	}
	c.cfg.Instance[0] = inst
}

// Save 把配置落盘。不做 Validate——允许保存未填写完整的配置。
func (c *Controller) Save() error {
	c.mu.Lock()
	cfg := c.cfg
	c.mu.Unlock()
	return cfg.Save()
}

// Start 启动登录引擎。非阻塞：实际运行在后台 goroutine 中。
// 已在运行或配置非法时返回错误。
func (c *Controller) Start() error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return errors.New("already running")
	}

	inst := config.ConfigInstance{}
	if len(c.cfg.Instance) > 0 {
		inst = c.cfg.Instance[0]
	}
	// 单账号：只跑一个 Session
	single := &config.Config{
		LogLevel: c.cfg.LogLevel,
		LogPath:  c.cfg.LogPath,
		Instance: []config.ConfigInstance{inst},
	}

	eng, err := engine.New(single, engine.Options{Logger: c.logger})
	if err != nil {
		c.mu.Unlock()
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	c.eng = eng
	c.cancel = cancel
	c.running = true
	c.mu.Unlock()

	// 事件转发与运行都在后台，Start 立即返回
	go c.consume(ctx, eng)
	return nil
}

// consume 运行引擎并把事件转发给订阅者，结束后复位状态。
func (c *Controller) consume(ctx context.Context, eng *engine.Engine) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := eng.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			c.logger.Error("engine stopped with error", "error", err)
		}
	}()

	for e := range eng.Events() {
		c.mu.Lock()
		c.state = e.State
		c.lastErr = e.Err
		observers := append([]func(engine.Event){}, c.observer...)
		c.mu.Unlock()

		for _, fn := range observers {
			fn(e)
		}
	}
	<-done

	c.mu.Lock()
	c.running = false
	c.eng = nil
	c.cancel = nil
	c.state = engine.StateStopped
	c.mu.Unlock()
}

// Stop 取消引擎并等待其完成登出。
func (c *Controller) Stop() {
	c.mu.Lock()
	cancel := c.cancel
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// Running 返回引擎是否在运行。
func (c *Controller) Running() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}

// State 返回当前状态；未运行时为 engine.StateStopped。
func (c *Controller) State() engine.State {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state
}

// Subscribe 注册事件回调。回调在 controller 的事件 goroutine 中被调用，
// UI 侧必须自行切回主线程（fyne.Do）。
func (c *Controller) Subscribe(fn func(engine.Event)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.observer = append(c.observer, fn)
}
```

- [ ] **Step 3: 构建验证**

```powershell
$env:PATH = "C:\ProgramData\msys2\ucrt64\bin;" + $env:PATH
$env:CGO_ENABLED = "1"
go build ./...
```

Expected: exit 0。

- [ ] **Step 4: 提交**

```bash
git add internal/controller/
git commit -m "feat: 添加 controller 管理配置与引擎生命周期"
```

---

## Task 4: ui 层（应用装配、表单、日志面板）

**Files:**
- Create: `gzgspg/internal/ui/app.go`（替换 Task 1 占位）
- Create: `gzgspg/internal/ui/editor.go`
- Create: `gzgspg/internal/ui/logpanel.go`
- Create: `gzgspg/FyneApp.toml`

**Interfaces:**
- Consumes: `controller.Controller`、`logwriter.Writer`、`engine.State/Event`
- Produces: `ui.Run() error`

- [ ] **Step 1: 创建 FyneApp.toml**

`FyneApp.toml`:

```toml
[Details]
Icon = "assets/icon.png"
Name = "gzgspg"
ID = "top.summonhim.gzgspg"
Version = "0.1.0"
Build = 1

[Release]
Name = "广工商校园网登录器"
```

并从 gzgspd 复制图标：

```powershell
New-Item -ItemType Directory -Path assets -Force | Out-Null
Copy-Item "..\gzgspd\files\release\gzgspg-gui-logo.png" "assets\icon.png"
```

- [ ] **Step 2: 实现日志面板**

`internal/ui/logpanel.go`:

```go
package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/summonhim/gzgspg/internal/logwriter"
)

// logPanel 是一个只读的滚动日志视图。
type logPanel struct {
	writer *logwriter.Writer
	entry  *widget.Entry
	box    *container.Scroll
}

func newLogPanel(w *logwriter.Writer) *logPanel {
	entry := widget.NewMultiLineEntry()
	entry.Wrapping = fyne.TextWrapWord
	entry.SetText(joinLines(w.Lines()))
	entry.Disable()

	p := &logPanel{
		writer: w,
		entry:  entry,
		box:    container.NewVScroll(entry),
	}

	// 每来一行就追加，并切回主线程
	w.Subscribe(func(line string) {
		fyne.Do(func() {
			entry.SetText(joinLines(w.Lines()))
			p.box.ScrollToBottom()
		})
	})

	return p
}

func joinLines(lines []string) string {
	out := ""
	for i, l := range lines {
		if i > 0 {
			out += "\n"
		}
		out += l
	}
	return out
}
```

- [ ] **Step 3: 实现表单**

`internal/ui/editor.go`:

```go
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

	username *widget.Entry
	password *widget.Entry
	iface    *widget.Entry
	userAgent *widget.Entry
	keepAlive *widget.Entry
	kAliveLink *widget.Entry
	retryMax *widget.Entry
	retryTime *widget.Entry
	advanced *widget.Accordion

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
```

- [ ] **Step 4: 实现应用装配**

`internal/ui/app.go`（整体替换 Task 1 的占位）：

```go
// Package ui 是 gzgspg 的 Fyne 界面层，只负责展示与交互。
package ui

import (
	"fmt"
	"log/slog"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/summonhim/gzgspd/engine"
	"github.com/summonhim/gzgspg/internal/controller"
	"github.com/summonhim/gzgspg/internal/logwriter"
)

// Run 启动 GUI，阻塞至退出。
func Run() error {
	a := app.NewWithID("top.summonhim.gzgspg")
	a.SetIcon(theme.InfoIcon())

	logBuf := logwriter.New(1000)
	logger := slog.New(slog.NewTextHandler(logBuf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	cfgPath, err := filepath.Abs("config.json")
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
	win.Resize(fyne.NewSize(560, 640))

	ed := newEditor(ctrl)
	status := widget.NewLabel("未运行")
	logs := newLogPanel(logBuf)

	var startBtn, stopBtn *widget.Button
	startBtn = widget.NewButton("启动", func() {
		ed.collect()
		if err := ctrl.Save(); err != nil {
			logger.Error("save config failed", "error", err)
		}
		startBtn.Disable()
		stopBtn.Enable()
		status.SetText("启动中…")
		// Start 非阻塞，可直接调用；失败时恢复按钮状态
		if err := ctrl.Start(); err != nil {
			status.SetText("启动失败: " + err.Error())
			startBtn.Enable()
			stopBtn.Disable()
		}
	})
	stopBtn = widget.NewButton("停止", func() {
		stopBtn.Disable()
		status.SetText("停止中…")
		go func() {
			ctrl.Stop()
			fyne.Do(func() {
				startBtn.Enable()
				stopBtn.Disable()
				status.SetText("已停止")
			})
		}()
	})
	stopBtn.Disable()

	saveBtn := widget.NewButton("保存", func() {
		ed.collect()
		if err := ctrl.Save(); err != nil {
			status.SetText("保存失败: " + err.Error())
			return
		}
		status.SetText("已保存")
	})

	ctrl.Subscribe(func(ev engine.Event) {
		fyne.Do(func() {
			status.SetText(statusText(ev))
		})
	})

	bottom := container.NewHBox(saveBtn, startBtn, stopBtn)
	content := container.NewBorder(
		nil,
		container.NewVBox(container.NewHBox(status), bottom),
		nil, nil,
		container.NewVSplit(ed.root, container.NewBorder(
			widget.NewLabel("日志"), nil, nil, nil, logs.box,
		)),
	)
	win.SetContent(content)

	// 关窗只隐藏（托盘继续运行）
	win.SetCloseIntercept(func() {
		win.Hide()
	})

	setupTray(a, win, ctrl, status, startBtn, stopBtn)

	win.ShowAndRun()
	return nil
}

func statusText(ev engine.Event) string {
	if ev.Err != nil {
		return fmt.Sprintf("%s: %v", ev.State.String(), ev.Err)
	}
	if ev.Message != "" {
		return fmt.Sprintf("%s - %s", ev.State.String(), ev.Message)
	}
	return ev.State.String()
}
```

**注意**：`quit` 必须由托盘触发（`a.Quit()`）。上面 `setupTray` 在 Task 6 提供。

- [ ] **Step 5: 构建验证**

在 Task 6 提供 `setupTray` 后统一构建（当前会因缺少 `setupTray` 而编译失败）。

- [ ] **Step 6: 提交**

```bash
git add internal/ui/ FyneApp.toml assets/icon.png
git commit -m "feat: 实现 Fyne 界面（表单、状态、日志面板）"
```

---

## Task 5: 托盘集成与收尾

**Files:**
- Create: `gzgspg/internal/ui/tray.go`
- Modify: `gzgspg/main.go`（无需改动，确认即可）

**Interfaces:**
- Consumes: `fyne.io/fyne/v2/driver/desktop`
- Produces: `setupTray(a fyne.App, win fyne.Window, ctrl *controller.Controller, status *widget.Label, startBtn, stopBtn *widget.Button)`

- [ ] **Step 1: 实现托盘**

`internal/ui/tray.go`:

```go
package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/summonhim/gzgspg/internal/controller"
)

// setupTray 在支持托盘的平台上安装托盘图标与菜单。
func setupTray(a fyne.App, win fyne.Window, ctrl *controller.Controller,
	status *widget.Label, startBtn, stopBtn *widget.Button) {

	d, ok := a.(desktop.App)
	if !ok {
		// 平台不支持托盘（如缺少 libayatana-appindicator 的 Linux），
		// 退化为纯窗口模式：关窗即退出。
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
		fyne.NewMenuItem("启动", func() {
			startBtn.OnTapped()
		}),
		fyne.NewMenuItem("停止", func() {
			stopBtn.OnTapped()
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("退出", func() {
			ctrl.Stop()
			a.Quit()
		}),
	))
}
```

- [ ] **Step 2: 构建验证**

```powershell
$env:PATH = "C:\ProgramData\msys2\ucrt64\bin;" + $env:PATH
$env:CGO_ENABLED = "1"
go build ./...
```

Expected: exit 0。

- [ ] **Step 3: vet**

```powershell
go vet ./...
```

Expected: exit 0。

- [ ] **Step 4: 提交**

```bash
git add internal/ui/tray.go
git commit -m "feat: 添加托盘图标与菜单"
```

---

## Task 6: 统一测试与手动验收

**Files:** 无新增

- [ ] **Step 1: 格式化**

```powershell
gofmt -l .
```

Expected: 无输出（或仅有已忽略目录）。若有文件列出，`gofmt -w <file>`。

- [ ] **Step 2: 统一运行测试**

```powershell
$env:PATH = "C:\ProgramData\msys2\ucrt64\bin;" + $env:PATH
$env:CGO_ENABLED = "1"
go test ./... -count=1
```

Expected: `logwriter`、`controller` 两个包 `ok`，其余 `no test files`，0 FAIL。

- [ ] **Step 3: 手动验收（无法自动化）**

```powershell
go run . 
```

逐项确认：

1. 窗口打开，标题为「广工商校园网登录器」。
2. 填写用户名/密码，点「保存」→ 工作目录出现 `config.json`，格式为 `instance` 数组。
3. 点「启动」→ 状态文本随事件变化；日志面板出现 slog 输出。
4. 关闭窗口 → 窗口隐藏而非退出；托盘图标仍存在。
5. 托盘右键 → 「显示主窗口」能重新显示。
6. 托盘「退出」→ 进程结束。
7. 若本机有 gzgspd，确认同一账号不会同时被两边登录。

- [ ] **Step 4: 提交**

```bash
git add -A
git commit -m "chore: 格式化与收尾"
```

---

## 覆盖对照（spec → task）

| Spec 要求 | 对应 Task |
|-----------|-----------|
| 项目结构、go.work、依赖 | Task 1 |
| 日志面板数据源 | Task 2、Task 5 |
| controller（配置读写、生命周期、事件） | Task 3 |
| 单账号（instance[0]）语义 | Task 3 |
| Fyne 界面（表单/状态/日志） | Task 5 |
| 托盘常驻、关窗隐藏、托盘退出 | Task 6 |
| `SetSystemTrayWindow` 必要性 | Task 6 |
| `fyne.Do` 线程约束 | Task 5 |
| 不支持托盘平台的退化 | Task 6 |
| 测试（统一运行） | Task 2/3 写测试，Task 7 运行 |

## 已知缺口

- **UI 层无自动化测试**：Fyne 组件测试成本高，Task 7 Step 3 手动验收覆盖。
- **controller 的 Start 测试会触发真实网络请求**（lo 网卡 → 3.3.3.3）。测试只验证生命周期与事件，不验证登录结果，避免依赖外部网络。
- **engine 的 network seam 未导出**，gzgspg 侧无法注入 fake dialer。若后续需要更细的测试，需在 gzgspd 的 engine 包暴露一个测试钩子（不在本计划范围）。
