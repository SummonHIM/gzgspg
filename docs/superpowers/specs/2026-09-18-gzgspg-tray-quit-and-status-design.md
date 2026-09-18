# gzgspg 托盘退出流程与状态中文化设计

日期：2026-09-18
状态：在 `2026-09-18-gzgspg-ui-redesign.md` 基础上的增量

## 目标

1. 托盘「退出」在引擎运行中时，先让用户看到「正在登出」，等登出真正完成再退出。
2. `engine.State` 的展示改中文，运行页字号加大。
3. 托盘菜单首行显示当前运行状态。

## 一、托盘退出流程

### 现状

`internal/ui/tray.go` 的「退出」直接 `ctrl.Stop()` 后 `a.Quit()`。`Stop()` 只发起 context 取消（`controller.go`），登出由 engine 在后台完成，所以进程可能在登出请求还没发出去时就没了。

### 新流程

```
点「退出」
  ↓
ctrl.Running()?
  ├─ false → a.Quit()                    // 无会话可登出，直接退
  └─ true  → win.Show()                  // 先让用户看见
            window.SetContent(运行页)     // 显示「正在登出」
            ctrl.Stop()
            ctrl.WaitStopped()           // 阻塞至 session 跑完 doLogout
            time.Sleep(可见停顿)
            a.Quit()
```

关键点：

- `WaitStopped()` 的语义已经是「等 engine.Run 的 wg 归零」，即 session 的 `doLogout()` 执行完毕（`engine.go:106`），所以睡眠纯为可见性。
- **可见停顿取 600ms**，且只在运行中出现。未运行时直接退出，不制造无谓延迟。
- 整条退出流程跑在 goroutine 里，不阻塞 Fyne 主线程；UI 变更（Show/切页）经 `fyne.Do`。
- `ctrl.Running()` 的检查与 `Stop()` 之间存在的竞态（用户恰好在此刻登出）后果仅是多等一次 600ms，可接受。

### 装配改动

`setupTray` 需要拿到「显示运行页」的能力，签名从 `(a, win, ctrl)` 改为接收一个 `onQuit` 或者一个能切换到运行页的回调。倾向：

```go
setupTray(a fyne.App, win fyne.Window, ctrl *controller.Controller,
    showRun func(engine.State), quit func())
```

由 `app.go` 提供 `showRun`（切到运行页并设定初始文案）与 `quit`（实际执行退出流程）两个闭包，托盘只负责接线。

## 二、状态中文化

`engine.State.String()` 是 gzgspd 的英文文案，不改它（那是 gzgspd 的领域）。在 `internal/ui` 加映射：

| engine.State | 中文 |
|--------------|------|
| StateStarting | 正在启动 |
| StateNotLoggedIn | 未登录 |
| StateLoggingIn | 正在登录 |
| StateLoggedIn | 登录成功 |
| StatePaused | 已暂停（多次失败） |
| StateLoggingOut | 正在登出 |
| StateStopped | 已停止 |
| 其它 | 未知状态 |

未知值（未来 engine 新增状态）必须走 `default`，不能显示空串。

## 三、运行页字号加大

现在运行页用 `widget.NewLabelWithStyle`（`runpage.go`）。改为 `canvas.Text`：它自带 `TextSize` 字段（Fyne v2.8.1 `canvas/text.go:22`），而 `widget.Label` 要改字号得自定义 Theme。

- `TextSize: 28`，`TextStyle: Bold`，`Alignment: Center`
- 初始化文案为「未登录」，与本页 `setState` 的输入来源一致
- 更新方式：设 `Text.Text` 后 `canvas.Refresh(text)`

状态文案同时供运行页与托盘使用，放在同一个 `stateLabel(state)` 函数里。

## 四、托盘状态首行

Fyne 托盘无 tooltip，只有 `SetSystemTrayMenu`。用一条禁用的菜单项承载状态：

```go
fyne.NewMenu("gzgspg",
    statusItem,                       // Disabled，文本「状态：未登录」
    fyne.NewMenuItemSeparator(),
    fyne.NewMenuItem("显示主窗口", ...),
    fyne.NewMenuItemSeparator(),
    fyne.NewMenuItem("退出", quit),
)
```

- `statusItem.Label = "状态：" + stateLabel(state)`
- 状态变化时改 Label 并调 `menu.Refresh()`（Fyne v2.8.1 `menu.go` 的 `Refresh` 会重下发给托盘驱动）
- 订阅方式与运行页一致：`ctrl.Subscribe` + `fyne.Do`

## 五、测试与验证

- `stateLabel` 是纯函数，直接单元测试：七个状态各返回预期中文，未知值返回「未知状态」。
- UI 与托盘行为无法自动化（Fyne 成本高），靠手动验收：
  1. 运行中点托盘「退出」→ 窗口弹出并显示「正在登出」→ 约 0.6s 后进程退出；日志中有登出记录。
  2. 未运行时点「退出」→ 立即退出，无窗口闪烁。
  3. 登录后运行页显示「登录成功」，字号明显大于其它文字。
  4. 托盘首行随状态变化，未登录时为「状态：未登录」。

## 明确不做（YAGNI）

- 退出倒计时
- 托盘图标随状态变色（另需多套图标）
- 退出确认对话框
- 托盘「启动/停止」菜单项（旧 spec 有、新计划已裁掉，本次不恢复）
