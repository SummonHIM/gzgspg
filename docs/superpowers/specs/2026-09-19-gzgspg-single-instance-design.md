# gzgspg 单实例设计

## 背景

当前应用可以被反复启动出多个进程实例，各自持有独立的登录引擎与托盘，会造成重复登录、状态混乱。需要限制同一时间只允许一个实例运行，且当用户再次启动时，把已运行的窗口带到前台（激活），而不是静默失败或再开一个新窗口。

## 目标与非目标

**目标**
- 同一时间只允许一个 gzgspg 实例运行
- 再次启动时，激活已运行实例的主窗口（`Show` + `RequestFocus`），随后次实例静默退出（退出码 0）
- 覆盖 Windows / macOS / Linux 三平台

**非目标**
- 不做跨进程传输业务数据，仅传输一个「激活」信号
- 不做多账号、多实例共存
- 不引入第三方依赖（纯标准库 + 已有的 `golang.org/x/sys`）

## 方案选型

跨进程通信（IPC）机制采用 **锁文件 + 本地 TCP 随机端口**：

- 主实例用 OS 排他文件锁仲裁「谁是唯一实例」：Linux/macOS 用 `flock`，Windows 用 `LockFileEx`
- 主实例锁成功后监听 `127.0.0.1:0`（内核分配随机端口），把实际端口写入端口文件供次实例发现
- 次实例锁失败 → 读端口文件 → 连接 → 写 1 字节激活信号 → 退出

不选 Unix socket + 命名管道（Windows 命名管道需第三方库或分平台两套代码），不选固定 TCP 端口（可能被其他程序占用导致误判）。

## 架构

新增包 `internal/singleinstance`，职责单一：管理单实例锁与「激活窗口」信号通道。

### 公共接口

```go
var ErrAlreadyRunning = errors.New("another instance is already running")

// Acquire 尝试成为主实例。成功返回 *Primary；已有实例在运行时，
// 向它发送激活信号并返回 ErrAlreadyRunning。
func Acquire(dir string) (*Primary, error)

type Primary struct { /* 内部持有 lockFile、TCP listener、activations channel 等，实现细节不对外暴露 */ }

// Activations 返回通道：每有后续实例请求激活时收到一个值。
func (p *Primary) Activations() <-chan struct{}

// Release 释放锁、关闭监听、清理端口文件。
func (p *Primary) Release()
```

### 文件布局

锁是唯一平台特定部分，IPC 逻辑跨平台统一：

- `singleinstance.go` — 公共接口 + TCP IPC（listen/accept、端口文件、activate 协议）
- `lock_unix.go` — `//go:build linux || darwin`，`syscall.Flock`
- `lock_windows.go` — `//go:build windows`，`golang.org/x/sys/windows` 的 `LockFileEx`
- `lock_other.go` — 其他平台退化为「不锁」（`tryLock` 恒 true）

### 磁盘文件

放在配置目录（`configdir.Dir()`，即 `os.UserConfigDir()/gzgspg`，可用 `GZGSPG_CONFIG_DIR` 覆盖）：

- `gzgspg.lock` — 空文件，仅作 flock/LockFileEx 的排他锁目标，**不删除**（删除会破坏 inode 语义，导致不同进程锁到不同文件）
- `gzgspg.port` — 主实例写入实际监听端口（文本），供次实例发现 IPC 地址；主实例覆盖写

## 数据流

### 主实例启动（`ui.Run` 开头，创建窗口前尽早检测）

1. `Acquire(dir)`：打开/创建 `gzgspg.lock`，`tryLock` 成功 → 成为主实例
2. 监听 `127.0.0.1:0`，把端口号写入 `gzgspg.port`
3. 后台 accept 循环：接受连接，读到任意字节即向 `Activations()` 通道投递一个值

### 次实例启动

1. `tryLock` 失败（锁被持有）→ 读 `gzgspg.port` → 拨号 → 写 1 字节激活信号 → 返回 `ErrAlreadyRunning`
2. `ui.Run` 捕获 `ErrAlreadyRunning` 后 `return nil`（静默退出）

### 激活动作

主实例的激活监听 goroutine 收到信号后：

```go
fyne.Do(func() { win.Show(); win.RequestFocus() })
```

`fyne.Window.RequestFocus` 文档语义为「raise and focus this window」，配合 `Show` 覆盖窗口被隐藏（托盘常驻）的情况。

## 竞态与边界

- **端口文件尚未就绪**：次实例锁失败但读到空端口或连接失败时重试（50ms 间隔 × 10 次 ≈ 500ms）。锁已确认被持有，主实例必然很快写好端口，重试窗口足够覆盖
- **主实例崩溃**：OS 在 fd 关闭时自动释放 flock/LockFileEx，次实例下次 `tryLock` 会成功并接管成主实例，不会卡死
- **锁文件不删除**：避免 stale inode 问题；端口文件由主实例每次覆盖写，无需清理逻辑依赖

## 错误处理

- `Acquire` 仅三类结果：锁成功（返回 `*Primary`）、已有实例（返回 `ErrAlreadyRunning`）、真实错误（目录不可用、监听失败等，向上返回）
- `ui.Run` 吞掉 `ErrAlreadyRunning`（视为正常退出），其余错误继续向上
- `main.go` 无需改动

## 集成

`internal/ui/app.go` 的 `Run()`：

```go
dir, err := configdir.Dir()
if err != nil { return err }

primary, err := singleinstance.Acquire(dir)
if err != nil {
    if errors.Is(err, singleinstance.ErrAlreadyRunning) {
        return nil
    }
    return err
}
defer primary.Release()

// 窗口创建后、ShowAndRun 前，启动激活监听
go func() {
    for range primary.Activations() {
        fyne.Do(func() { win.Show(); win.RequestFocus() })
    }
}()
```

## 测试

`internal/singleinstance/singleinstance_test.go`（在 CI linux 上运行，覆盖 flock + TCP 路径）：

- 同一目录连续 `Acquire`：第一次成功、第二次返回 `ErrAlreadyRunning`，且第一个实例的 `Activations()` 收到信号
- `Release()` 后再 `Acquire` 能重新成功（锁可复用）
- 端口文件重试逻辑：模拟端口文件为空时，次实例最终成功 connect
