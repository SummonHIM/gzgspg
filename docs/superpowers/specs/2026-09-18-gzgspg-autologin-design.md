# gzgspg 自动登陆与自启校验设计

日期：2026-09-18
状态：在现有 UI（`2026-09-18-gzgspg-ui-redesign.md`、`2026-09-18-gzgspg-tray-quit-and-status-design.md`）基础上的增量

## 目标

1. 设置页新增「自动登陆」开关：开启后 App 启动即自动登录，无需手动点「登陆」。
2. 「开机自启」的真实状态镜像到 Fyne preferences，并以其为期望值；启动时校验 OS 实际状态，不正确则修正（含可执行文件路径失效的情况）。

明确不做（YAGNI）：

- 不改 `gzgspd`。`auto_login` 不进入 `config.json`，配置结构不动。
- 不写从旧状态迁移的逻辑（尚未发布，无历史用户）。
- 从运行页登出返回首页后不自动重登。
- 自动登录失败不弹窗、不额外重试（沿用引擎自身的重试策略）。
- 不新增独立的配置文件字段或 GUI 之外的开关。

## 一、存储：Fyne preferences

两个布尔键，均通过 `fyne.App.Preferences()` 读写：

| 键 | 含义 | 默认 |
|----|------|------|
| `auto_login` | 是否启动自动登陆 | `false` |
| `autostart` | 开机自启的期望值 | `false` |

- 读取用 `BoolWithFallback(key, false)`（键缺失才用 fallback），写入用 `SetBool(key, v)`（Fyne v2.8.1 `fyne.Preferences`，已核对接口）。
- preferences 是 UI 层唯一显示来源；`autostart` 不再直接以 `autostart.Enabled()` 作为勾选框的初始值。
- 获取方式用全局 `fyne.CurrentApp().Preferences()`，避免改动 `newSettingsPage` 的签名：
  - 设置页在 `newSettingsPage` 内部取一次，用于 `autoLogin` / `autostart` 的初始化与 `OnChanged` 写入。
  - `app.go` 在启动流程中取一次，用于读取 `auto_login` 与调用 `Reconcile`。

## 二、自动登陆

### 设置页

`internal/ui/settingspage.go`：

- 结构体新增 `autoLogin *widget.Check`，`widget.NewCheck("开启", nil)`，与现有自启开关文案一致。
- `form` 中「开机自启」上方新增 `widget.NewFormItem("自动登陆", s.autoLogin)`。
- `load()` 时按 preference 初始化勾选状态；`OnChanged` 立即写 preference，不依赖「保存并返回」。
- 该开关不写入 `config.json`，`collect()` 不处理它。

### 触发

`internal/ui/app.go`：把现 `onLogin`（`app.go:110-125`）的实现提取为共用闭包 `login()`：

```
login():
  home.collect()
  inst := ctrl.Instance()
  if inst.Username == "" || inst.Password == "" { return }   // 静默跳过
  if err := ctrl.Save(); err != nil { home.setStatus("保存失败: " + err.Error()); return }
  if err := ctrl.Start(); err != nil { home.setStatus("启动失败: " + err.Error()); return }
  showRun()
```

- `onLogin = func() { home.setStatus(""); login() }`，按钮语义不变。
- 启动自动登录：在 `win.ShowAndRun()` **之前**执行；若 `auto_login` 为真且账号、密码均非空则调 `login()`。
  - 必须在 `ShowAndRun` 之前：该调用会阻塞至退出（`app.go:175`），放在之后这行代码永远到不了。`login()` 内部的切页依赖已构造好的 `stack`/`win`，在此时已就绪。
  - `login()` 在 Fyne 主线程上同步执行；其中 `ctrl.Start()` 本身非阻塞（`app.go:120` 已在按钮回调里这样用），故不会卡住事件循环启动。
  - 空凭据静默跳过，停在首页，由用户自行填写；不提示、不保存、不启动。
  - 登录失败（网络等原因）由引擎按 `retry_max`/`retry_time` 自行处理，界面显示运行页与对应状态。

## 三、开机自启校验

### 现状问题

各平台的 `Enabled()` 只判断「注册表值 / desktop 文件 / plist 是否存在」，不检查其指向的可执行文件路径是否仍正确。程序被移动后，自启项存在但指向旧路径，`Enabled()` 仍返回 true。

### 新增 `autostart.Reconcile(enabled bool) error`

以 preferences 的 `autostart` 为期望值，让 OS 实际状态与之相符：

| 平台 | 期望开启 | 期望关闭 |
|------|----------|----------|
| windows | 比对注册表 `AppName` 值是否等于 `os.Executable()`，不等则重写 | 删除该值 |
| linux | 比对 `.desktop` 文件的 `Exec=` 行是否等于当前 exe，不等则重写文件 | 删除文件 |
| darwin | 比对 plist 的 `ProgramArguments` 首个可执行路径，不等则重写 | 删除文件 |
| other | 返回 `ErrUnsupported` | 返回 `ErrUnsupported` |

- 实现上各平台可复用现有的 `Enable()`/`Disable()`（重写即调用 `Enable()`），并新增一个「读取当前配置的目标路径」的内部读取函数用于比对。
- 幂等：状态正确时不做任何写入。
- 校验标准为字符串相等；当前写入方式（`os.Executable()`）决定写入值，故比对同一来源即可。

各平台现有写入均用 `fmt.Sprintf` 直接拼接，未做转义；比对按「与写入完全一致的做法」取值即可，不额外引入转义/反转义，避免误判为不一致而反复重写：

- **linux**：desktop 文件中 `Exec=` 后的整行内容。
- **darwin**：plist 中 `ProgramArguments` 下的第一个 `<string>` 文本。
- **windows**：注册表 `REG_SZ` 值，直接等值比较。

若某平台无法可靠解析出目标路径（文件损坏、格式异常），`Reconcile` 视为「不一致」并调用 `Enable()` 重写，而不是报错。

### 调用时机

`internal/ui/app.go` 启动流程中（`controller.New` 之后、`win.ShowAndRun()` 之前）调一次：

```go
prefs := fyne.CurrentApp().Preferences()
if autostart.Supported() {
    if err := autostart.Reconcile(prefs.BoolWithFallback("autostart", false)); err != nil {
        logger.Warn("autostart reconcile failed", "error", err)
    }
}
```

- 失败只记日志，不回退 preferences（保留用户期望值，下次启动再试）。
- 设置页勾选自启时仍即时生效：先写 preference，成功调用 `autostart.Enable()`/`Disable()`；失败则将 preference 回退为 `autostart.Enabled()` 的实际值并显示「设置失败」，沿用现有行为（`settingspage.go:59-73`）。
- 关闭自启时若 OS 中无残留，`Reconcile(false)` 为幂等空操作。
- `autostart.Enabled()` 保持不变（仍只判断存在性）；修正路径的职责全部收敛在 `Reconcile`，UI 不再依赖 `Enabled()` 作为显示源。

## 四、测试与验证

### 自动化

- `internal/autostart/autostart_test.go` 新增 `Reconcile` 测试，沿用现有「记录原状态、`t.Cleanup` 恢复」模式，避免污染开发机：
  1. `Reconcile(true)` 后 `Enabled()` 为真，且指向当前 exe。
  2. 人为改成错误路径后再 `Reconcile(true)`，目标路径被修正为当前 exe。
  3. `Reconcile(false)` 后 `Enabled()` 为假。
- `internal/autostart/autostart_other.go` 的平台返回 `ErrUnsupported` 可加平台无关的断言（在 `!windows && !linux && !darwin` 上跳过）。
- 设置页与启动流程属 Fyne 交互层，不做单元测试（与现有测试范围一致）。

### 手动验收

1. 设置页开启「自动登陆」，重启 App → 直接进入运行页并开始登录。
2. 关闭「自动登陆」，重启 → 停在首页，需手动点「登陆」。
3. 开启「自动登陆」但清空账号或密码（并保存），重启 → 停在首页，无报错、无自动登录。
4. 开启「开机自启」，手动改变注册表 / desktop / plist 中指向的 exe 路径，重启 App → 路径被修正回当前 exe。
5. 关闭「开机自启」，若 OS 中仍有残留项，重启 App → 残留项被清除。
6. 设置页自启开关显示的值来自 preferences 而非直接读 OS。

## 五、受影响文件

| 文件 | 改动 |
|------|------|
| `internal/ui/settingspage.go` | 新增「自动登陆」开关；自启开关改以 preferences 为源；失败回退写 preference |
| `internal/ui/app.go` | 提取 `login()`；启动时按 `auto_login` 自动登录；启动时 `Reconcile` 自启 |
| `internal/autostart/autostart.go` | 声明 `Reconcile` 的平台契约（文档注释） |
| `internal/autostart/autostart_windows.go` | 实现 `Reconcile` + 目标路径读取 |
| `internal/autostart/autostart_linux.go` | 实现 `Reconcile` + 目标路径读取 |
| `internal/autostart/autostart_darwin.go` | 实现 `Reconcile` + 目标路径读取 |
| `internal/autostart/autostart_other.go` | `Reconcile` 返回 `ErrUnsupported` |
| `internal/autostart/autostart_test.go` | 新增 `Reconcile` 测试 |

`config.json` 结构、`gzgspd` 模块、`controller` 均不改动。
