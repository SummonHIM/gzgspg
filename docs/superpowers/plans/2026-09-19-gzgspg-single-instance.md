# 单实例功能实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 限制同一时间只运行一个 gzgspg 实例，再次启动时激活已有实例的主窗口并静默退出。

**Architecture:** 新增 `internal/singleinstance` 包：用 OS 排他文件锁（Linux/macOS `flock`，Windows `LockFileEx`）仲裁唯一实例，主实例监听 `127.0.0.1:0` 随机端口并把端口写入端口文件，次实例锁失败后连接该端口发一个字节的「激活」信号。UI 层 `app.go` 在启动时 `Acquire`，收到激活信号时 `win.Show()+win.RequestFocus()`。

**Tech Stack:** Go 标准库（`net`、`os`、`syscall`）+ 已有依赖 `golang.org/x/sys`（Windows `LockFileEx`）。无新增第三方依赖。

## Global Constraints

- 平台覆盖：Windows / macOS / Linux（见 spec「目标」）
- 不新增第三方依赖；仅复用 `golang.org/x/sys`（已在 `go.mod` 中，版本 `v0.39.0`）
- 磁盘文件放在配置目录：`gzgspg.lock`（锁，不删除）、`gzgspg.port`（端口）
- 锁文件不删除；端口文件由主实例每次覆盖写
- 代码风格：与现有包一致，中文注释，`//go:build` 构建标签与 `autostart` 包保持一致
- 本地编译检查：`go build ./...`；测试在 CI `ubuntu-latest` + `xvfb-run` 下运行（见 `AGENTS.md`）

---

### Task 1: 新增 `internal/singleinstance` 包

**Files:**
- Create: `internal/singleinstance/singleinstance.go`
- Create: `internal/singleinstance/lock_unix.go`
- Create: `internal/singleinstance/lock_windows.go`
- Create: `internal/singleinstance/lock_other.go`
- Test: `internal/singleinstance/singleinstance_test.go`

**Interfaces:**
- Consumes: 无（独立新包）
- Produces:
  - `var ErrAlreadyRunning = errors.New("another instance is already running")`
  - `func Acquire(dir string) (*Primary, error)`
  - `type Primary` 含方法 `func (p *Primary) Activations() <-chan struct{}`、`func (p *Primary) Release()`

- [ ] **Step 1: 写失败测试**

创建 `internal/singleinstance/singleinstance_test.go`：

```go
package singleinstance

import (
	"errors"
	"net"
	"os"
	"testing"
	"time"
)

func TestAcquireSecondInstanceReturnsAlreadyRunning(t *testing.T) {
	dir := t.TempDir()

	p1, err := Acquire(dir)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	defer p1.Release()

	_, err = Acquire(dir)
	if !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("second acquire = %v, want ErrAlreadyRunning", err)
	}
}

func TestActivationSignalDelivered(t *testing.T) {
	dir := t.TempDir()

	p1, err := Acquire(dir)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	defer p1.Release()

	_, err = Acquire(dir)
	if !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("second acquire = %v, want ErrAlreadyRunning", err)
	}

	select {
	case <-p1.Activations():
		// 激活信号已送达
	case <-time.After(2 * time.Second):
		t.Fatal("expected activation signal")
	}
}

func TestReleaseAllowsReacquire(t *testing.T) {
	dir := t.TempDir()

	p1, err := Acquire(dir)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	p1.Release()

	p2, err := Acquire(dir)
	if err != nil {
		t.Fatalf("reacquire after release: %v", err)
	}
	defer p2.Release()
}

func TestSignalExistingRetriesUntilPortAvailable(t *testing.T) {
	dir := t.TempDir()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			_, _ = c.Read(make([]byte, 1))
			_ = c.Close()
		}
	}()

	port := ln.Addr().(*net.TCPAddr).Port

	// 延迟写入端口文件，模拟「锁已持有但端口文件尚未就绪」
	go func() {
		time.Sleep(200 * time.Millisecond)
		if err := writePortFile(dir, port); err != nil {
			t.Errorf("writePortFile: %v", err)
		}
	}()

	if err := signalExisting(dir); err != nil {
		t.Fatalf("signalExisting: %v", err)
	}
}

func TestPortFileRoundTrip(t *testing.T) {
	dir := t.TempDir()

	if err := writePortFile(dir, 12345); err != nil {
		t.Fatalf("writePortFile: %v", err)
	}
	got, err := readPort(dir)
	if err != nil {
		t.Fatalf("readPort: %v", err)
	}
	if got != 12345 {
		t.Fatalf("readPort = %d, want 12345", got)
	}
}
```

注意：`TestSignalExistingRetriesUntilPortAvailable` 用到 `writePortFile`/`readPort`/`signalExisting` 这些未导出函数；测试是白盒测试（同包），可访问它们。

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/singleinstance/ -count=1`
Expected: 编译失败，报 `no required module provides package` 或 `undefined: Acquire` 等（包尚不存在）。

- [ ] **Step 3: 实现跨平台核心**

创建 `internal/singleinstance/singleinstance.go`：

```go
// Package singleinstance 保证同一时间只运行一个 gzgspg 实例，
// 并提供跨进程「激活主窗口」的信号通道。
package singleinstance

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// ErrAlreadyRunning 表示已有实例在运行，且已向其发送激活信号。
var ErrAlreadyRunning = errors.New("another instance is already running")

const (
	lockFileName = "gzgspg.lock"
	portFileName = "gzgspg.port"
	activateByte = "a"

	// connectRetries / connectDelay 是次实例发现端口文件的等待窗口：
	// 主实例先拿到锁、后写端口文件，中间有极小窗口端口文件尚不存在。
	connectRetries = 10
	connectDelay   = 50 * time.Millisecond
)

// Primary 代表持有单实例锁的主实例。
type Primary struct {
	lockFile    *os.File
	listener    net.Listener
	activations chan struct{}
}

// Activations 返回通道：每有后续实例请求激活时收到一个值。
func (p *Primary) Activations() <-chan struct{} {
	return p.activations
}

// Release 释放锁、关闭监听。可安全重复调用。
func (p *Primary) Release() {
	if p.listener != nil {
		_ = p.listener.Close()
		p.listener = nil
	}
	if p.lockFile != nil {
		unlock(p.lockFile)
		_ = p.lockFile.Close()
		p.lockFile = nil
	}
}

// Acquire 尝试成为主实例。成功返回 *Primary；已有实例在运行时，
// 向其发送激活信号（尽力而为）并返回 ErrAlreadyRunning。
// 锁文件只做排他仲裁，永不删除；端口文件由主实例覆盖写。
func Acquire(dir string) (*Primary, error) {
	lf, err := os.OpenFile(filepath.Join(dir, lockFileName), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if !tryLock(lf) {
		_ = lf.Close()
		// 激活失败也不影响主目标：绝不再启动第二个实例。
		_ = signalExisting(dir)
		return nil, ErrAlreadyRunning
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		unlock(lf)
		_ = lf.Close()
		return nil, err
	}

	p := &Primary{
		lockFile:    lf,
		listener:    ln,
		activations: make(chan struct{}, 1),
	}

	if err := writePortFile(dir, ln.Addr().(*net.TCPAddr).Port); err != nil {
		p.Release()
		return nil, err
	}

	go p.serve()
	return p, nil
}

// serve 接受来自次实例的连接，读到激活字节后投递一个信号。
func (p *Primary) serve() {
	for {
		conn, err := p.listener.Accept()
		if err != nil {
			return // listener 被 Release 关闭
		}
		buf := make([]byte, 1)
		if _, err := conn.Read(buf); err == nil {
			select {
			case p.activations <- struct{}{}:
			default:
				// 缓冲区已满时丢弃：激活是幂等的，一次信号足够唤起窗口
			}
		}
		_ = conn.Close()
	}
}

// writePortFile 把监听端口写入端口文件。
func writePortFile(dir string, port int) error {
	return os.WriteFile(portPath(dir), []byte(strconv.Itoa(port)), 0o644)
}

// readPort 读取端口文件内容。
func readPort(dir string) (int, error) {
	data, err := os.ReadFile(portPath(dir))
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(string(data))
}

func portPath(dir string) string {
	return filepath.Join(dir, portFileName)
}

// signalExisting 通知已运行实例激活窗口。端口文件可能尚未就绪，
// 故在窗口内重试读取与连接。
func signalExisting(dir string) error {
	var lastErr error
	for i := 0; i < connectRetries; i++ {
		port, err := readPort(dir)
		if err != nil {
			lastErr = err
		} else if err := dialAndActivate(port); err != nil {
			lastErr = err
		} else {
			return nil
		}
		time.Sleep(connectDelay)
	}
	return lastErr
}

// dialAndActivate 连接主实例并写入一个激活字节。
func dialAndActivate(port int) error {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 500*time.Millisecond)
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = conn.Write([]byte(activateByte))
	return err
}
```

- [ ] **Step 4: 实现平台锁**

创建 `internal/singleinstance/lock_unix.go`：

```go
//go:build linux || darwin

package singleinstance

import (
	"os"
	"syscall"
)

// tryLock 尝试对文件加排他非阻塞锁。成功返回 true；已被持有返回 false。
func tryLock(f *os.File) bool {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) == nil
}

// unlock 释放锁。
func unlock(f *os.File) {
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
```

创建 `internal/singleinstance/lock_windows.go`：

```go
//go:build windows

package singleinstance

import (
	"os"

	"golang.org/x/sys/windows"
)

// tryLock 尝试对文件加排他非阻塞锁。成功返回 true；已被持有返回 false。
func tryLock(f *os.File) bool {
	var ol windows.Overlapped
	return windows.LockFileEx(
		windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0, 1, 0, &ol,
	) == nil
}

// unlock 释放锁。
func unlock(f *os.File) {
	var ol windows.Overlapped
	_ = windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, &ol)
}
```

创建 `internal/singleinstance/lock_other.go`：

```go
//go:build !linux && !darwin && !windows

package singleinstance

import "os"

// tryLock 在不支持锁的平台恒返回 true（视为始终可成为主实例）。
func tryLock(f *os.File) bool { return true }

// unlock 在不支持锁的平台为空操作。
func unlock(f *os.File) {}
```

- [ ] **Step 5: 运行测试确认通过**

Run: `go test ./internal/singleinstance/ -count=1`
Expected: PASS（5 个测试全部通过）

Run: `go build ./...`
Expected: 编译通过

- [ ] **Step 6: 提交**

```bash
git add internal/singleinstance/
git commit -m "feat: 新增 singleinstance 包，限制单实例运行"
```

---

### Task 2: 集成到 `internal/ui/app.go`

**Files:**
- Modify: `internal/ui/app.go`

**Interfaces:**
- Consumes: `singleinstance.Acquire(dir string) (*Primary, error)`、`singleinstance.ErrAlreadyRunning`、`(*Primary).Activations()`、`(*Primary).Release()`（Task 1）
- Produces: `ui.Run()` 现在在启动时仲裁单实例，次实例静默退出

- [ ] **Step 1: 修改 import**

`internal/ui/app.go` 当前 import 块（第 4-17 行）中已含 `"github.com/summonhim/gzgspg/internal/configdir"`。新增 `errors` 与 `singleinstance`：

```go
import (
	"errors"
	"log/slog"
	"os"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"

	"github.com/summonhim/gzgspd/engine"
	"github.com/summonhim/gzgspg/internal/autostart"
	"github.com/summonhim/gzgspg/internal/configdir"
	"github.com/summonhim/gzgspg/internal/controller"
	"github.com/summonhim/gzgspg/internal/singleinstance"
)
```

- [ ] **Step 2: 在 `Run()` 开头仲裁单实例**

在 `func Run() error {` 之后、`logPath, err := configdir.LogPath()` 之前插入：

```go
	a := app.NewWithID("top.summonhim.gzgspg")

	dir, err := configdir.Dir()
	if err != nil {
		return err
	}
	primary, err := singleinstance.Acquire(dir)
	if err != nil {
		if errors.Is(err, singleinstance.ErrAlreadyRunning) {
			// 已有实例在运行：激活信号已发出，静默退出
			return nil
		}
		return err
	}
	defer primary.Release()
```

- [ ] **Step 3: 启动激活监听**

在 `win.ShowAndRun()` 之前插入激活监听 goroutine：

```go
	go func() {
		for range primary.Activations() {
			fyne.Do(func() { win.Show(); win.RequestFocus() })
		}
	}()

	win.ShowAndRun()
```

- [ ] **Step 4: 编译与测试确认**

Run: `go build ./...`
Expected: 编译通过

Run: `go test ./internal/singleinstance/ -count=1`
Expected: PASS

（完整 `go test ./...` 需在 CI linux 环境运行，见 `AGENTS.md`。）

- [ ] **Step 5: 提交**

```bash
git add internal/ui/app.go
git commit -m "feat: 启动时仲裁单实例，重复启动激活已有窗口"
```

---

## Self-Review

- **Spec coverage:** spec 的「公共接口」「文件布局」「数据流」「竞态与边界」「错误处理」「集成」「测试」均有对应任务覆盖：Task 1 覆盖包实现与全部 5 个测试（含重试、激活送达、锁复用），Task 2 覆盖 app.go 集成与 `ErrAlreadyRunning` 静默退出。
- **Placeholder scan:** 无 TBD/TODO；所有步骤含完整代码。
- **Type consistency:** `Acquire(dir string) (*Primary, error)`、`ErrAlreadyRunning`、`Primary.Activations() <-chan struct{}`、`Primary.Release()`、`tryLock(*os.File) bool`、`unlock(*os.File)`、`writePortFile(dir string, port int) error`、`readPort(dir string) (int, error)`、`signalExisting(dir string) error` 在 Task 1 定义、Task 2 与测试中引用一致。
