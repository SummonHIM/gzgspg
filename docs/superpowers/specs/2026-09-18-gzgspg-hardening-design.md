# gzgspg 加固与优化设计

日期：2026-09-18

## 背景

对项目整体审查后，发现若干正确性 / 安全问题与若干低风险优化点。本设计排除密码加密（用户确认明文密码可接受）与 mutex 拆分（单账号低并发场景无收益），聚焦其余改动。

## 目标

1. 停止后 UI 状态正确复位，不残留「正在登出」。
2. 自启项在含空格的安装路径下仍能正常工作。
3. 程序出错时以非零退出码退出，便于脚本与自启场景感知失败。
4. `Config()` 不再暴露可变内部指针。
5. 设置页数值字段在输入时即时校验，不再等启动才报错。
6. dev 构建的版本号可读性更好。
7. 为后续会话补充本地构建 / 测试说明。

## 改动

### 1. 停止后状态复位广播（`internal/controller/controller.go`）

**问题**：`consume` 在事件通道关闭后直接修改 `state` 为 `StateStopped`，但不通知观察者。若引擎最后发出的事件是 `StateLoggingOut`，托盘与运行页会永久停留在「正在登出」。

**方案**：

- 抽出 `notify(e engine.Event)` 方法：在锁内更新 `state` / `lastErr` 并拷贝 `observer` 列表，锁外调用各回调。`consume` 的事件循环改用它。
- `consume` 完成 `running/eng/cancel` 复位后，合成并广播 `engine.Event{State: StateStopped, Time: time.Now()}`，让所有订阅者（含托盘 `tray.setState`）收到最终状态。
- 需要引入 `time` 包。

**测试**：新增 `TestStopBroadcastsFinalStoppedEvent`，订阅者捕获事件序列，断言最后一个是 `StateStopped`。

### 2. 自启路径加引号（`autostart_windows.go` / `autostart_linux.go`）

**问题**：Windows `Run` 键与 Linux `Exec=` 直接写 `os.Executable()` 结果，路径含空格（如 `C:\Program Files\...`）时自启失败。

**方案**：

- Windows `writeTarget`：存入 `"` + path + `"`；`targetPath` 读回后 `strings.Trim(path, "\"")`，保证 `Reconcile` 的相等比较不变。
- Linux `writeTarget`：仅当路径含空格时用引号包裹；`targetPath` 读回时 `strings.Trim(..., "\"")`。
- darwin 的 LaunchAgent plist 用独立 `<string>` 元素，本就无需引号，不改。

**测试**：现有 `autostart_test.go` 的往返 / 路径修复用例已覆盖写入与读回的一致性，保持通过即可。

### 3. `main.go` 非零退出码

**方案**：`Run()` 出错时 `fmt.Fprintln(os.Stderr, err)` 并 `os.Exit(1)`。需要引入 `os` 包。

### 4. `Config()` 返回深拷贝（`internal/controller/controller.go`）

**问题**：`Config()` 返回内部 `*config.Config`，调用方修改会绕过 mutex，与 `Instance()` 的副本语义不一致。

**方案**：`Config()` 返回浅拷结构体并 `append` 拷贝 `Instance` 切片：

```go
func (c *Controller) Config() *config.Config {
    c.mu.Lock()
    defer c.mu.Unlock()
    cp := *c.cfg
    cp.Instance = append([]config.ConfigInstance(nil), c.cfg.Instance...)
    return &cp
}
```

`filePath` 字段（未导出）随结构体拷贝保留，调用方仍无法访问。

**测试**：新增 `TestConfigIsCopy`，修改返回值的 `Instance[0]` 后断言内部配置不变。

### 5. 数值字段内联校验（`internal/ui/settingspage.go`）

**问题**：`atoiOr` 只在空串 / 非数字时回退默认值，用户输入 0 或负数会被保留并导致启动失败；且非法值要到启动时才报错。

**方案**：

- 新增两个校验函数，规则与 `gzgspd` 的 `config.Validate()` 对齐：
  - `validatePositiveInt(s string) error`：可解析为正整数（>0），否则报「请输入正整数」。
  - `validateNonNegativeInt(s string) error`：可解析为非负整数（≥0），否则报「请输入非负整数」。
- 三个 Entry（keepAlive / retryMax / retryTime）分别设 `Validator` 与 `AlwaysShowValidationError = true`，输入即时在输入框下方显示错误。
- `collect()` 改为返回 `bool`：逐一调用 `Validate()`，任一失败即返回 `false` 且不写回 controller。
- `app.go` 的 `onBackFromSettings` 依据 `collect()` 返回值决定是否离开设置页。
- 删除 `atoiOr` 与三个 `default*` 常量（不再有回退语义）。

**测试**：新增 `settingspage` 单测覆盖两个校验函数（合法 / 非法 / 空 / 负数 / 0）。

### 6. `BuildTime` 默认值（`internal/version/version.go`）

**方案**：`BuildTime = "unknown"`（原 `"0"`）。dev 构建显示 `dev unknown`。

**测试**：更新 `TestDefaults` 断言。

### 7. 新增 `AGENTS.md`

**方案**：在仓库根目录新增 `AGENTS.md`，说明：

- 项目定位：广工商校园网登录器 GUI，基于 Fyne，业务核心在 `internal/controller`，登录引擎来自外部 `github.com/summonhim/gzgspd`。
- 本地构建 / 测试：Windows 上 `go vet` / `go test` 因 glfw/GL 的 CGO 约束失败；测试与 vet 在 CI 的 `ubuntu-latest` + `xvfb-run` 下运行。
- 配置目录：`os.UserConfigDir()/gzgspg`，可用 `GZGSPG_CONFIG_DIR` 覆盖。
- 变更后应在 CI 验证，而非仅依赖本地。

## 不做

- 密码加密 / keyring：用户确认明文密码可接受。
- mutex 拆分：单账号低并发，无收益。
- 本地测试工具链搭建（Makefile 封装等）：仅记录说明，不新增工具。

## 一致性

- `validatePositiveInt` / `validateNonNegativeInt` 的边界与 `gzgspd` `config.Validate()` 一致（keep_alive>0、retry_max≥0、retry_time>0）。
- 状态复位广播复用现有 `engine.Event` 类型，不引入新协议。
- `Config()` 深拷贝不影响现有调用方（测试仅读取字段）。
