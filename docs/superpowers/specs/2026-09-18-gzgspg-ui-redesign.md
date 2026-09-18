# gzgspg UI 重构设计（双页面 + 配置/日志落盘 + 开机自启）

日期：2026-09-18
状态：替换 `2026-09-18-gzgspg-design.md` 中的界面与存储部分

## 本次变更概要

| 方面 | 旧设计 | 新设计 |
|------|--------|--------|
| 配置位置 | 工作目录 `config.json` | `os.UserConfigDir()/gzgspg/config.json` |
| 日志 | UI 内日志面板 | 落盘到同一目录，**GUI 内不显示** |
| 界面 | 单页（列表+表单+日志+托盘） | 双页面：首页（登录）/ 运行页 |
| 高级设置 | 折叠面板常驻 | 右上角设置图标 → 独立页面（含开机自启开关） |
| 首页 | — | Logo 居中 + 账号/密码/登陆 |
| 开机自启 | 不做 | 高级设置页开关，三平台实现 |
| 托盘 | 保留 | 保留（不变） |

## 配置与日志路径

统一用 `os.UserConfigDir()`：

| 平台 | 实际路径 |
|------|----------|
| Windows | `%AppData%\gzgspg\` |
| Linux | `~/.config/gzgspg/` |
| macOS | `~/Library/Application Support/gzgspg/` |

目录内容：

```
<configDir>/gzgspg/
├── config.json     # 与 gzgspd 同格式（instance 数组，GUI 只用 [0]）
└── gzgspg.log      # slog 文本日志，追加写
```

- 启动时确保目录存在（`os.MkdirAll`，权限 `0755`）。
- 配置文件仍是 `instance` 数组，保持与 gzgspd 双向兼容。
- 日志不再进入 UI。slog 输出到文件。GUI 内不显示任何日志。

## 界面结构

### 首页（登录页）

从上到下：

1. **Logo**（居中显示，使用 `assets/icon.png`）
2. **账号输入框**（`Entry`）
3. **密码输入框**（`PasswordEntry`）
4. **登陆按钮**（大按钮，主操作）
5. **右上角设置图标按钮**（齿轮）→ 打开高级设置页

### 高级设置页

从首页设置图标进入，含账号的全部扩展字段：

- **开机自启开关**（`Check`）
- 界面 interface
- User-Agent
- keep_alive（秒）
- keep_alive_link
- retry_max
- retry_time

**行为**：每个输入框在加载时，若配置中的值为空，则**自动填入该项的默认值**（不是占位符，是真实文本）。用户可修改。返回首页时生效。

默认值（与 gzgspd engine 内部一致）：

| 字段 | 默认值 |
|------|--------|
| interface | `""`（空 = 自动探测网卡） |
| User-Agent | `Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36` |
| keep_alive | `5` |
| keep_alive_link | `http://3.3.3.3` |
| retry_max | `3` |
| retry_time | `5` |

### 运行页

从首页点「登陆」后自动跳转。内容：

- **当前状态**（大字，来自 `engine.State`）
- **登出按钮**

**登出行为**：调用 `controller.Stop()`（触发校园网登出 + 停本地引擎），完成后**返回首页**。

### 托盘（保持不变）

图标 + 菜单：显示主窗口 / 启动 / 停止 / 退出。关窗隐藏。

## 数据流

```
首页「登陆」
   ↓
editor.collect() 写回 controller（含高级设置页的字段）
   ↓
controller.Save()  →  <configDir>/gzgspg/config.json
   ↓
controller.Start()（非阻塞）
   ↓
切换到运行页  ←── controller.Subscribe 事件 → fyne.Do → 状态标签
   ↓
运行页「登出」→ controller.Stop() → 切回首页
```

## 新增/修改的组件

### 新增：`internal/configdir`

```go
// Dir 返回配置目录 <UserConfigDir>/gzgspg，并确保其存在。
func Dir() (string, error)

// ConfigPath 返回 config.json 的完整路径。
func ConfigPath() (string, error)

// LogPath 返回 gzgspg.log 的完整路径。
func LogPath() (string, error)
```

### 新增：`internal/autostart`

三平台接口：

```go
// Supported 返回当前平台是否支持自启。
func Supported() bool

// Enabled 返回自启是否已启用。
func Enabled() bool

// Enable 启用自启，name 为应用显示名，exePath 为可执行文件路径。
func Enable(name, exePath string) error

// Disable 关闭自启。
func Disable() error
```

实现分平台文件：

| 文件 | 平台 | 机制 |
|------|------|------|
| `autostart_windows.go` | windows | 注册表 `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`，值为 exe 路径 |
| `autostart_linux.go` | linux | `~/.config/autostart/gzgspg.desktop` |
| `autostart_darwin.go` | darwin | `~/Library/LaunchAgents/top.summonhim.gzgspg.plist` |
| `autostart_other.go` | 其他 | `Supported()` 返回 false，其余返回错误 |

Windows 用 `golang.org/x/sys/windows/registry`（已有依赖链）。

### 修改：`internal/ui`

- `app.go`：双页面切换（`container.NewStack` + `Show/Hide`，或 `setContent`）
- 新增 `homepage.go`：首页（账号/密码/自启/登陆/设置图标）
- 新增 `settingspage.go`：高级设置（含默认值回填）
- 新增 `runpage.go`：运行页（状态 + 登出）
- 删除 `logpanel.go`（日志不再进 UI）
- `tray.go`：保留

### 修改：`internal/controller`

- `Options.ConfigPath` 由调用方传入（现在来自 `configdir`）。
- 其余不变。

## 测试策略

- `configdir`：测 `Dir()` 返回路径以 `gzgspg` 结尾、目录确实被创建。不写 config.json 到真实用户目录——测试通过环境变量重定向（见下）。
- `autostart`：Windows 走注册表，测试**只做读写往返并在结束后删除**，避免污染系统；若担心污染则标记为需手动验收。
- 为可测性，`configdir` 支持环境变量覆盖：`GZGSPG_CONFIG_DIR` 非空时直接使用该路径（不再拼 `gzgspg` 子目录）。测试用它指向 `t.TempDir()`。
- 界面层：不做自动化测试（Fyne 成本高），靠手动验收清单。

## 错误处理

- 配置目录创建失败 → 启动时报错，窗口内提示。
- 配置文件读写失败 → 在首页状态文本显示错误，不崩溃。
- 自启操作失败 → 开关回退到实际状态，并在状态文本提示。
- 自启在 `Supported() == false` 的平台 → 开关禁用。

## 明确不做（YAGNI）

密码加密、多账号、定时任务、自动更新、多语言、主题、日志查看器、日志轮转。

## 风险

- 自启路径：开发时 `go run` 的 exePath 是临时路径，开启自启会指向临时二进制。默认值自动填入，界面无法验证路径有效性（不额外做校验）。
- **自启无自动化测试**：会真实修改系统状态（注册表/用户目录），测试只能验证「读写往返」且需在临时位置，或跳过。计划中默认只做手动验收。
- **双页面状态**：登陆失败（如账号为空）时不应跳转到运行页，需留在首页并显示错误。

## 相对旧 spec 的取代关系

本文档取代旧 spec 中以下部分：目录结构中的 `logpanel.go`、界面一节、配置读写一节的路径、以及「明确不做」中关于开机自启的条目。其余（形态、框架、运行模型、engine 依赖方式）不变。
