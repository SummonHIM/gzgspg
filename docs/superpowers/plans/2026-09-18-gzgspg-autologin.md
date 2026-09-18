# gzgspg 自动登陆与自启校验实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 设置页新增「自动登陆」开关（开启后 App 启动即自动登录）；「开机自启」以 Fyne preferences 为期望值，启动时校验并修正 OS 实际状态（含可执行文件路径失效）。

**Architecture:** 用 Fyne `Preferences` 存两个布尔键 `auto_login` / `autostart`，不碰 `gzgspd`。`internal/autostart` 新增平台相关的 `Reconcile(enabled bool) error`，比对本平台自启项指向的 exe 与 `os.Executable()`，不一致则重写。`app.go` 把「登录」动作提取为共用闭包 `login()`，登录按钮与启动自动登录共用；启动时先 `Reconcile` 自启，再按 `auto_login` 决定是否 `login()`。

**Tech Stack:** Go 1.27、Fyne v2.8.1（`fyne.Preferences`、`fyne.CurrentApp()`、`widget.Check`）、`golang.org/x/sys/windows/registry`。

## Global Constraints

- **构建环境**：`CGO_ENABLED=1`，gcc 来自 `C:\ProgramData\msys2\ucrt64\bin`；必须用 `C:\Program Files\Go\bin\go.exe`，只把 ucrt64 加到 PATH 末尾供 gcc 用。
- **线程约束**：所有 UI 更新必须经 `fyne.Do()`；订阅回调与托盘跑在非主线程。
- **preferences 键名**：逐字为 `auto_login` 与 `autostart`，默认值均为 `false`。读取用 `BoolWithFallback(key, false)`，写入用 `SetBool`。
- **自动登录跳过条件**：账号或密码为空时静默跳过，不提示、不保存、不启动。
- **不改动**：`gzgspd` 模块、`config.json` 结构、`internal/controller` 均不动；`auto_login` 不写入 `config.json`。
- **`autostart.Enabled()` 语义不变**（只判断存在性）；路径修正只在 `Reconcile` 内。
- **测试策略**：测试与实现一起写；autostart 测试沿用现有「记录原状态、`t.Cleanup` 恢复」模式，避免污染开发机。

---

## 文件结构

| 文件 | 动作 | 职责 |
|------|------|------|
| `internal/autostart/autostart.go` | 修改 | 声明 `Reconcile` 的平台契约注释 |
| `internal/autostart/autostart_windows.go` | 修改 | `Reconcile` + 注册表目标路径读取 |
| `internal/autostart/autostart_linux.go` | 修改 | `Reconcile` + desktop `Exec=` 读取 |
| `internal/autostart/autostart_darwin.go` | 修改 | `Reconcile` + plist 可执行路径读取 |
| `internal/autostart/autostart_other.go` | 修改 | `Reconcile` 返回 `ErrUnsupported` |
| `internal/autostart/autostart_test.go` | 修改 | `Reconcile` 行为测试 |
| `internal/ui/settingspage.go` | 修改 | 「自动登陆」开关；自启开关改以 preferences 为源 |
| `internal/ui/app.go` | 修改 | 提取 `login()`；启动自动登录；启动 `Reconcile` |
| `docs/superpowers/specs/2026-09-18-gzgspg-autologin-design.md` | 已有 | 设计依据 |

---

## Task 1: `autostart.Reconcile`（windows）

**Files:**
- Modify: `internal/autostart/autostart_windows.go`
- Modify: `internal/autostart/autostart.go`
- Test: `internal/autostart/autostart_test.go`

**Interfaces:**
- Produces: `Reconcile(enabled bool) error`（全平台同签名，各文件一个实现）
- Produces: `targetPath() (string, bool)`（包内私有，读取当前自启项指向的 exe）
- Produces: `writeTarget(path string) error`（包内私有，把自启项指向给定路径；`Enable()` 复用）

> 设计说明：`writeTarget` 是 `Enable()` 的底层实现（`Enable()` = `writeTarget(os.Executable())`），
> 属正常生产代码，而非测试钩子；测试用它在不启动真实 exe 的情况下构造「路径被改坏」的场景。

- [ ] **Step 1: 写失败测试**

在 `internal/autostart/autostart_test.go` 末尾追加。测试通过 `Enabled()` + `targetPath()` 验证；两者都是包内函数，测试与实现同包可直接调用。

```go
func TestReconcileEnablesWithCorrectPath(t *testing.T) {
	if !Supported() {
		t.Skip("autostart not supported on this platform")
	}

	wasEnabled := Enabled()
	t.Cleanup(func() {
		if wasEnabled {
			_ = Enable()
		} else {
			_ = Disable()
		}
	})

	if err := Reconcile(true); err != nil {
		t.Fatalf("reconcile(true): %v", err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("executable: %v", err)
	}
	got, ok := targetPath()
	if !ok {
		t.Fatal("expected a registered target path")
	}
	if got != exe {
		t.Fatalf("target path = %q, want %q", got, exe)
	}
}

func TestReconcileRepairsStalePath(t *testing.T) {
	if !Supported() {
		t.Skip("autostart not supported on this platform")
	}

	wasEnabled := Enabled()
	t.Cleanup(func() {
		if wasEnabled {
			_ = Enable()
		} else {
			_ = Disable()
		}
	})

	// 人为把目标改成一个不存在的路径，模拟程序被移动
	const stale = "/nonexistent/gzgspg.exe"
	if err := writeTarget(stale); err != nil {
		t.Fatalf("writeTarget: %v", err)
	}
	if got, _ := targetPath(); got != stale {
		t.Fatalf("precondition failed, target = %q", got)
	}

	if err := Reconcile(true); err != nil {
		t.Fatalf("reconcile(true): %v", err)
	}
	exe, _ := os.Executable()
	if got, _ := targetPath(); got != exe {
		t.Fatalf("stale path not repaired: got %q, want %q", got, exe)
	}
}

func TestReconcileDisable(t *testing.T) {
	if !Supported() {
		t.Skip("autostart not supported on this platform")
	}

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
	if err := Reconcile(false); err != nil {
		t.Fatalf("reconcile(false): %v", err)
	}
	if Enabled() {
		t.Fatal("expected disabled after Reconcile(false)")
	}
}
```

需要在测试文件 import 中加入 `"os"`。

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/autostart/ -run TestReconcile -v`
Expected: 编译失败（`undefined: Reconcile`、`undefined: targetPath`、`undefined: writeTarget`）

- [ ] **Step 3: 实现 windows 版**

`internal/autostart/autostart_windows.go`：把现有 `Enable()` 改为委托 `writeTarget`，并追加 `targetPath` 与 `Reconcile`（`registry`、`os`、`syscall` 已 import）：

```go
// Enable 启用自启，指向当前可执行文件。
func Enable() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return writeTarget(exe)
}

// writeTarget 把自启项指向指定路径（不存在则创建）。
func writeTarget(path string) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetStringValue(AppName, path)
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
	return v, true
}

// Reconcile 让 OS 自启状态与期望一致：期望开启时确保指向当前 exe，
// 期望关闭时移除自启项。指向与当前 exe 一致时不写入。
func Reconcile(enabled bool) error {
	if !enabled {
		return Disable()
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if cur, ok := targetPath(); ok && cur == exe {
		return nil
	}
	return writeTarget(exe)
}
```

`internal/autostart/autostart.go` 追加契约注释：

```go
// Reconcile 让当前平台的自启状态与期望值一致：期望开启时确保自启项
// 指向当前可执行文件（路径失效则重写），期望关闭时移除自启项。
// 状态已正确时不做写入。各平台实现见 autostart_<goos>.go。
//
// 注意：与 Enabled() 不同，Reconcile 会检查目标路径是否仍然有效。
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/autostart/ -run TestReconcile -v`
Expected: PASS（3 个用例）

- [ ] **Step 5: 提交**

```bash
git add internal/autostart/autostart.go internal/autostart/autostart_windows.go internal/autostart/autostart_test.go
git commit -m "feat: autostart 新增 Reconcile 校验并修正自启路径"
```

---

## Task 2: `autostart.Reconcile`（linux / darwin / other）

**Files:**
- Modify: `internal/autostart/autostart_linux.go`
- Modify: `internal/autostart/autostart_darwin.go`
- Modify: `internal/autostart/autostart_other.go`

**Interfaces:**
- Consumes: `Reconcile` 契约（Task 1）
- Produces: 三平台各自的 `Reconcile` / `targetPath` / `writeTarget`

> 这些文件带 build tag，Windows 开发机编译不到。写完用 `GOOS` 交叉编译确认语法：
> `$env:GOOS="linux"; go build ./internal/autostart/`，`darwin` 同理。交叉编译不需要 CGO。
> 测试只在当前 OS（windows）上运行，故 linux/darwin 的 `writeTarget` 不被测试直接调用，
> 但必须与 windows 版同签名，保证 `autostart_test.go` 在任一平台都能编译。

- [ ] **Step 1: 实现 linux 版**

`internal/autostart/autostart_linux.go`：把现有 `Enable()` 改为委托 `writeTarget`，并追加 `targetPath` 与 `Reconcile`：

```go
// Enable 启用自启，指向当前可执行文件。
func Enable() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return writeTarget(exe)
}

// writeTarget 把自启项指向指定路径，写入 autostart desktop 文件。
func writeTarget(path string) error {
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
`, AppName, path)
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
			return strings.TrimPrefix(line, "Exec="), true
		}
	}
	return "", false
}

// Reconcile 让 OS 自启状态与期望一致。
func Reconcile(enabled bool) error {
	if !enabled {
		return Disable()
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if cur, ok := targetPath(); ok && cur == exe {
		return nil
	}
	return writeTarget(exe)
}
```

需要在 import 中加入 `"strings"`。

- [ ] **Step 2: 实现 darwin 版**

`internal/autostart/autostart_darwin.go`：同样把 `Enable()` 改为委托 `writeTarget`，并追加：

```go
// Enable 启用自启，指向当前可执行文件。
func Enable() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return writeTarget(exe)
}

// writeTarget 把自启项指向指定路径，写入 LaunchAgent plist。
func writeTarget(path string) error {
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
`, AppID, path)
	return os.WriteFile(p, []byte(content), 0o644)
}

// targetPath 返回 plist 中 ProgramArguments 下的第一个可执行路径。
func targetPath() (string, bool) {
	p, err := plistPath()
	if err != nil {
		return "", false
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return "", false
	}
	rest := string(data)
	idx := strings.Index(rest, "<key>ProgramArguments</key>")
	if idx < 0 {
		return "", false
	}
	rest = rest[idx:]
	open := strings.Index(rest, "<string>")
	if open < 0 {
		return "", false
	}
	rest = rest[open+len("<string>"):]
	closeIdx := strings.Index(rest, "</string>")
	if closeIdx < 0 {
		return "", false
	}
	return rest[:closeIdx], true
}

// Reconcile 让 OS 自启状态与期望一致。
func Reconcile(enabled bool) error {
	if !enabled {
		return Disable()
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if cur, ok := targetPath(); ok && cur == exe {
		return nil
	}
	return writeTarget(exe)
}
```

需要在 import 中加入 `"strings"`。

- [ ] **Step 3: 实现 other 版**

`internal/autostart/autostart_other.go` 追加：

```go
// Reconcile 在当前平台不可用。
func Reconcile(enabled bool) error { return ErrUnsupported }
```

- [ ] **Step 4: 交叉编译验证语法**

```powershell
$env:GOOS="linux"; go build ./internal/autostart/; $env:GOOS="darwin"; go build ./internal/autostart/; $env:GOOS=""
```
Expected: 均无输出（成功）。若报错，先修正再继续。

- [ ] **Step 5: 本机测试仍通过**

Run: `go test ./internal/autostart/ -v`
Expected: PASS（含 Task 1 的 `Reconcile` 用例）

- [ ] **Step 6: 提交**

```bash
git add internal/autostart/autostart_linux.go internal/autostart/autostart_darwin.go internal/autostart/autostart_other.go
git commit -m "feat: Reconcile 支持 linux/darwin/other"
```

---

## Task 3: 设置页「自动登陆」与自启改以 preferences 为源

**Files:**
- Modify: `internal/ui/settingspage.go`

**Interfaces:**
- Consumes: `autostart.Supported/Enabled/Enable/Disable`（既有）
- Produces: preference 键 `auto_login`（读写在设置页）、键 `autostart`（本任务负责写入；`app.go` 在 Task 4 负责读）

- [ ] **Step 1: 加结构体字段与控件**

在 `settingsPage` 结构体（`settingspage.go:23-36`）中，`autostart` 一行后加：

```go
	autoLogin  *widget.Check
```

在 `newSettingsPage` 的构造字面量（`settingspage.go:39-49`）中，`autostart:` 一行后加：

```go
		autoLogin:  widget.NewCheck("开启", nil),
```

- [ ] **Step 2: 读取 preferences 并接线**

在 `newSettingsPage` 里、现有「自启开关」块（`settingspage.go:52-74`）前插入：

```go
	prefs := fyne.CurrentApp().Preferences()

	// 自动登陆：只存 preferences，OnChanged 即时写入
	s.autoLogin.SetChecked(prefs.BoolWithFallback("auto_login", false))
	s.autoLogin.OnChanged = func(on bool) {
		prefs.SetBool("auto_login", on)
	}
```

把现有「自启开关」块中读取状态的一行：

```go
		s.autostart.SetChecked(autostart.Enabled())
```

替换为以 preferences 为准（先写 preference，再执行动作；失败回退 preference 与实际状态）：

```go
		s.autostart.SetChecked(prefs.BoolWithFallback("autostart", false))
		s.autostart.OnChanged = func(on bool) {
			prefs.SetBool("autostart", on)
			var err error
			if on {
				err = autostart.Enable()
			} else {
				err = autostart.Disable()
			}
			if err != nil {
				s.autoStatus.SetText("设置失败: " + err.Error())
				// 回退到实际状态，并让 preferences 与实际一致
				actual := autostart.Enabled()
				prefs.SetBool("autostart", actual)
				s.autostart.SetChecked(actual)
				return
			}
			s.autoStatus.SetText("")
		}
```

即：删掉原来的 `s.autostart.SetChecked(autostart.Enabled())` 与整个 `s.autostart.OnChanged = func(on bool) { ... }` 块，用上面替换。注意 `SetChecked` 会触发 `OnChanged`（Fyne v2.8.1 `check.go:75-89`），所以**必须先 `SetChecked` 再赋 `OnChanged`**，否则初始化会被当成用户操作写入。

- [ ] **Step 3: 加表单项**

在 `form` 的 `widget.NewFormItem("开机自启", s.autostart)`（`settingspage.go:83`）上方加：

```go
		widget.NewFormItem("自动登陆", s.autoLogin),
```

- [ ] **Step 4: 编译**

Run: `go build ./...`
Expected: 成功，无输出。

- [ ] **Step 5: 提交**

```bash
git add internal/ui/settingspage.go
git commit -m "feat: 设置页新增自动登陆开关，自启改以 preferences 为源"
```

---

## Task 4: 启动自动登录与启动自启校验

**Files:**
- Modify: `internal/ui/app.go`

**Interfaces:**
- Consumes: preference 键 `auto_login` / `autostart`（Task 3）、`autostart.Reconcile`（Task 1-2）
- Produces: 内部闭包 `login()`、`onLogin`

- [ ] **Step 1: 加入 `autostart` import**

`internal/ui/app.go` 现有 import（`app.go:4-16`）在 `"github.com/summonhim/gzgspd/engine"` 与 `"github.com/summonhim/gzgspg/internal/configdir"` 之间加入：

```go
	"github.com/summonhim/gzgspg/internal/autostart"
```

- [ ] **Step 2: 提取共用 `login()` 闭包**

在 `app.go` 的闭包声明块（`app.go:60-70`）中，`onLogin func()` 一行后加：

```go
		login      func()
```

把现有 `onLogin` 的实现（`app.go:110-125`）整段替换为：

```go
	// login 是登录按钮与启动自动登录共用的动作。
	login = func() {
		if home == nil {
			return
		}
		home.collect()
		inst := ctrl.Instance()
		if inst.Username == "" || inst.Password == "" {
			// 空凭据：静默跳过，停在首页由用户填写
			return
		}
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

	onLogin = func() {
		if home == nil {
			return
		}
		home.setStatus("")
		login()
	}
```

`login` 的赋值必须在使用它的 `onLogin` 之前（Go 闭包按执行顺序求值，此处顺序即声明顺序，满足）。

- [ ] **Step 3: 启动时校验自启**

在 `showHome()` 调用（`app.go:165`）之后、`win.SetContent(stack)` 之前插入：

```go
	// 启动时让 OS 自启状态与用户期望一致（含 exe 路径失效的修正）
	prefs := fyne.CurrentApp().Preferences()
	if autostart.Supported() {
		if err := autostart.Reconcile(prefs.BoolWithFallback("autostart", false)); err != nil {
			logger.Warn("autostart reconcile failed", "error", err)
		}
	}
```

- [ ] **Step 4: 启动时按 `auto_login` 自动登录**

在 `win.ShowAndRun()`（`app.go:175`）**之前**插入：

```go
	// 自动登陆：开启且有账号密码时，启动即登录（ShowAndRun 会阻塞，故在此之前）
	if prefs.BoolWithFallback("auto_login", false) {
		inst := ctrl.Instance()
		if inst.Username != "" && inst.Password != "" {
			login()
		}
	}
```

- [ ] **Step 5: 编译并跑全量测试**

Run: `go build ./... ; go test ./...`
Expected: 编译成功；测试全部 PASS。

- [ ] **Step 6: 手动验证**

1. 设置页开启「自动登陆」，保存账号密码，重启 App → 直接进入运行页并开始登录。
2. 关闭「自动登陆」，重启 → 停在首页。
3. 开启「自动登陆」但清空账号或密码（并保存），重启 → 停在首页，无报错、无自动登录；日志无异常。
4. 开启「开机自启」，手动把注册表 `HKCU\Software\Microsoft\Windows\CurrentVersion\Run` 中的 `gzgspg` 值改成不存在的路径，重启 App → 该值被改回当前 exe。
5. 关闭「开机自启」，确认注册表中该项消失。
6. 设置页自启勾选框显示的值来自 preferences（改注册表后重开设置页，勾选框不变，直到重启才修正）。

- [ ] **Step 7: 提交**

```bash
git add internal/ui/app.go
git commit -m "feat: 启动自动登录与自启校验"
```

---

## Self-Review

**Spec 覆盖：**

| Spec 要求 | 对应任务 |
|-----------|----------|
| preferences 键 `auto_login`/`autostart` | Task 3、4 |
| 设置页「自动登陆」开关 | Task 3 |
| 启动自动登录（空凭据跳过） | Task 4 |
| `autostart.Reconcile` 各平台 | Task 1、2 |
| 启动时调用 `Reconcile` + 失败仅记日志 | Task 4 |
| 自启失败回退 preference | Task 3 |
| `Reconcile` 测试 | Task 1 |
| 不改 `gzgspd`/`config.json`/`controller` | 所有任务均未触碰 |

**占位符扫描：** 无 TBD/TODO；每个代码步骤均含完整代码。

**类型一致性：** 全平台 `Reconcile(enabled bool) error`、`targetPath() (string, bool)`、`writeTarget(string) error` 签名一致；preference 键名 `auto_login`/`autostart` 在 Task 3、4 中逐字一致。

**设计说明：** `Enable()` 重构为 `writeTarget(os.Executable())`，`writeTarget` 是生产路径（非测试钩子），测试借它构造「路径被改坏」的场景，避免在测试里复制各平台的写入模板。
