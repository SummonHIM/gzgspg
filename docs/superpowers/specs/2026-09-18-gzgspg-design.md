# gzgspg 设计（广工商校园网登录器）

日期：2026-09-18

## 定位

`gzgspg` 是广州工商学院校园网登录器的桌面 GUI，独立运行，进程内直接复用 `gzgspd` 的 `engine` 包执行登录，不需要额外的 `gzgspd` 进程或系统服务。它与 `gzgspd` 读写同一份 `config.json`。

一句话：gzgspd 是 core + CLI，gzgspg 是它的图形前台。

## 已确认的决策

| 决策 | 选择 |
|------|------|
| 形态 | 托盘常驻 + 主窗口（关窗隐藏，托盘退出才退出） |
| GUI 框架 | Fyne v2.8.1（纯 Go，跨平台） |
| 目标平台 | Windows、Linux、macOS |
| 运行模型 | GUI 进程内跑 engine，不依赖外部 gzgspd 进程/服务 |
| engine 依赖方式 | `go.work` 工作区，指向平级目录的 gzgspd |
| 项目位置 | `C:\Users\SummonHIM\Standard\Projects\Golang\gzgspg` |
| config.json | 保持数组结构，GUI 只读写 `instance[0]`，与 gzgspd 兼容 |
| 账号数量 | **UI 只管一个账号** |

## 目录结构

```
gzgspg\
├── go.mod                     module github.com/summonhim/gzgspg
├── main.go                    入口
├── FyneApp.toml               应用元数据（名称/图标/版本）
├── assets\icon.png            图标（复用 gzgspd 的 logo）
├── internal\
│   ├── controller\            UI 与 engine 之间的桥，唯一的业务状态层
│   │   ├── controller.go
│   │   └── controller_test.go
│   ├── ui\                    Fyne 界面，纯展示与交互
│   │   ├── app.go             fyne.App 装配、主窗口
│   │   ├── editor.go          单账号编辑表单
│   │   ├── status.go          状态展示
│   │   ├── logpanel.go        日志面板
│   │   └── tray.go            托盘图标与菜单
│   └── logwriter\             slog → io.Writer 适配，供日志面板消费
│       └── writer.go
└── docs\superpowers\...       本 spec 与实现计划
```

平级目录另有：

```
Golang\
├── gzgspd\                    core（已存在）
├── gzgspg\                    本项目
└── go.work                    把两个模块编入工作区（不进版本库）
```

## 架构与职责

依赖方向：

```
ui → controller → engine (gzgspd) → portal / nnet / config
```

- **engine（gzgspd）**：已完成。GUI 只用其公共 API：`engine.New`、`Engine.Run/Stop/Sessions/Events`、`Session.Start/Pause/Resume/State/Key`、`config.LoadConfig/Save`、`engine.State`、`engine.Event`。
- **controller（gzgspg 核心）**：UI 与 engine 之间的唯一边界，也是唯一持有可变业务状态的地方。
- **ui（Fyne）**：只负责展示与交互，不含业务逻辑。
- **logwriter**：把 `slog` 输出转发给 Fyne 的日志文本组件，避免 UI 轮询。

## controller 接口

```go
type Controller struct { /* 私有 */ }

type Options struct {
    ConfigPath string
    Logger     *slog.Logger
}

func New(opts Options) (*Controller, error)   // 加载配置（不存在则用空配置）

func (c *Controller) Config() *config.Config        // 当前配置快照
func (c *Controller) Instance() config.ConfigInstance // instance[0] 的副本，无则零值
func (c *Controller) UpdateInstance(inst config.ConfigInstance) error // 写回 instance[0]
func (c *Controller) Save() error                   // 落盘 config.json

func (c *Controller) Start() error                  // 启动 engine
func (c *Controller) Stop()                         // 取消 engine 并等待登出
func (c *Controller) Running() bool
func (c *Controller) State() engine.State           // 单账号状态；未运行时为 StateStopped

func (c *Controller) Subscribe(fn func(engine.Event))  // 注册事件回调
```

要点：

- 单账号：`Instance()` / `UpdateInstance()` 始终操作 `instance[0]`；配置里没有实例时，`UpdateInstance` 负责追加一个。
- `Start()` 用 `instance[0]` 构造 `engine.New(&config.Config{Instance: []ConfigInstance{inst}}, ...)`，只跑一个 Session。
- controller 内部起一个 goroutine 消费 `engine.Events()`，对每个事件调用所有已注册回调。
- **不阻塞调用方**：`Stop()` 会等待登出完成（engine 的设计如此），UI 侧需在后台调用。

## 数据流

```
config.json ──LoadConfig──→ controller ──engine.New──→ Engine.Run(ctx)
                                ↑                            │
                                │                     engine.Event
                           UpdateInstance                    │
                                │                            ↓
                         编辑表单 ←── fyne.Do ──── controller 事件循环
                                                             │
                                                             ↓
                                                    状态展示 / 日志面板
```

## 界面

单窗口：

- **账号区**：用户名、密码输入框
- **高级区**（可折叠）：网卡 interface、User-Agent、keep_alive、keep_alive_link、retry_max、retry_time
- **状态区**：当前状态徽标（颜色对应 `engine.State`）+ 最近一条消息
- **日志区**：滚动文本，接 `slog` 输出
- **底部按钮**：保存、启动、停止

**托盘**：图标 + 右键菜单「显示主窗口 / 启动 / 停止 / 退出」。关闭主窗口只隐藏，托盘「退出」才真正结束进程。

托盘 API 实测（Fyne v2.8.1）：

```go
import "fyne.io/fyne/v2/driver/desktop"

d, ok := a.(desktop.App)   // a 为 fyne.App
if ok {
    d.SetSystemTrayWindow(win)      // 必须先设，否则某些平台关窗即退出
    d.SetSystemTrayIcon(iconRes)
    d.SetSystemTrayMenu(fyne.NewMenu("gzgspg",
        fyne.NewMenuItem("显示主窗口", func() { win.Show() }),
        fyne.NewMenuItem("启动", func() { /* ... */ }),
        fyne.NewMenuItem("停止", func() { /* ... */ }),
        fyne.NewMenuItemSeparator(),
        fyne.NewMenuItem("退出", func() { a.Quit() }),
    ))
}
```

- `SetSystemTrayMenu` / `SetSystemTrayIcon` / `SetSystemTrayWindow` 不在 `fyne.App` 接口上，需断言 `desktop.App`。
- `SetSystemTrayWindow` 必须调用，它同时承担「托盘驱动生命周期」的职责。
- `Window.SetCloseIntercept` 用于把关闭动作改为隐藏。

**线程约束**：Fyne 要求所有 UI 更新在主线程，controller 的事件回调一律经 `fyne.Do()` 投递（v2.6+ 提供，v2.8.1 已确认存在）。

## 配置读写

- 默认路径 `config.json`（工作目录），可经 `Options.ConfigPath` 指定。
- 与 gzgspd 同一格式：`instance` 为数组，GUI 只用第一条。
- 保存沿用 `config.Save()`（本次 core 重构新增）。
- 配置不存在时：使用空配置（内存中），首次保存时写出。

## 错误处理

- 配置非法 → 表单内联提示，不启动 engine。
- engine 启动失败（如网卡不存在）→ 通过 `engine.Event.Err` 显示在日志面板，窗口不崩。
- 引擎运行中再次点击启动 → 忽略（幂等）。

## 测试

- `controller`：为关键逻辑写单测——加载/更新/保存往返、启动与停止、事件回调被调用、未运行时状态为 Stopped。
- engine 侧的 network seam（`sessionDialer`）目前在 gzgspd 内未导出，无法在 gzgspg 测试中注入 fake。**controller 测试不注入 fake**，只测配置读写与生命周期（不触发真实网络的路径）。
- UI 层不做自动化测试（Fyne 测试成本高），手动验收。

## 明确不做（YAGNI）

密码加密存储、开机自启、定时任务、自动更新、多语言、主题定制、多账号支持。

## 风险

- **CGO/gcc**：Fyne 需要 CGO。Windows 上须把 `C:\ProgramData\msys2\ucrt64\bin` 加入 PATH；Linux 需 gtk3 + libayatana-appindicator3；macOS 需 Xcode 命令行工具。
- **go.work**：不进版本库；克隆后需手动 `go work init` 或依赖 `go.mod` 中的版本号。
- **Linux 托盘**：依赖 libayatana-appindicator。若目标环境缺失，托盘不可用，需要回退为纯窗口模式。
- **单一登录权**：GUI 内置引擎与已安装的 gzgspd 服务若同时运行，会互相抢登录。本设计不做检测（用户需自行避免）。后续可加「检测到 gzgspd 服务则告警」。
