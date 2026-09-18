# gzgspg 空配置默认值设计

日期：2026-09-18
状态：补充 `2026-09-18-gzgspg-ui-redesign.md`，修正首次落盘的字段来源

## 问题

高级设置页的默认值目前只活在界面的 `load()` 里（`internal/ui/settingspage.go`）：

```go
s.userAgent.SetText(orDefault(inst.UserAgent, defaultUserAgent))
s.keepAlive.SetText(orDefaultInt(inst.KeepAlive, defaultKeepAlive))
```

而首页的 `collect()`（`internal/ui/homepage.go`）只写账号密码，其余字段原样带回：

```go
inst := h.ctrl.Instance()   // 新配置 → ConfigInstance{} 零值
inst.Username = h.username.Text
inst.Password = h.password.Text
h.ctrl.UpdateInstance(inst)
```

后果：新用户**没进过设置页**就直接点「登陆」时，落盘的 config.json 里 `user_agent`、`keep_alive_link` 是空字符串，`keep_alive`/`retry_max`/`retry_time` 是 0。其中 `keep_alive <= 0` 与 `retry_time <= 0` 会让 gzgspd 的 `Config.Validate()` 失败，`Start()` 直接报错。

也就是说，文件里是空值还是默认值，取决于用户有没有点过齿轮图标。

## 决策

采用「以文件是否存在为准」的规则，并在**文件缺失时把默认值直接写成一份 config.json**：

| 情形 | 结果 |
|------|------|
| config.json 不存在 | 写出一份默认值配置（无账号密码；高级字段为默认值；`interface` 留空） |
| config.json 存在 | 完全不动磁盘文件。字段按文件为准，用户清空即视为清空 |

即：默认值只在**首次创建配置文件**时落盘；文件一旦存在，用户对每个字段的显式修改（包括清空）都被尊重，程序不再向磁盘写入任何默认值。

不追踪「字段级历史」，不在 `ConfigInstance` 上加指针字段或影子副本，不修改 gzgspd 的 `config` 包。

## 实现

### 落点：`internal/controller/controller.go` 的 `New()`

「文件是否存在」这一事实只有加载配置处知道。`LoadConfig` 失败时先区分两种原因，再决定是否落盘：

```go
cfg, err := config.LoadConfig(opts.ConfigPath)
if err != nil {
    cfg = &config.Config{}
    cfg.SetFilePath(opts.ConfigPath)

    if errors.Is(err, os.ErrNotExist) {
        // 首次运行：把默认值写成一份文件，让界面与磁盘同源
        cfg.Instance = []config.ConfigInstance{defaultInstance()}
        if saveErr := cfg.Save(); saveErr != nil {
            c.logger.Error("write default config", "error", saveErr)
        }
    } else {
        // 文件存在但非法：只在内存里退回空配置，绝不覆盖用户文件
        cfg.Instance = []config.ConfigInstance{defaultInstance()}
        c.logger.Info("config invalid, keeping file as-is", "path", opts.ConfigPath, "reason", err)
    }
}
```

关键约束：**新增默认值只在文件确实缺失时写盘（`errors.Is(err, os.ErrNotExist)`）**。文件存在但解析/校验失败时不得覆盖，否则用户改错一个字符就会丢失账号密码。

内存中的 `Instance[0]` 在两种情况下都预置默认值，保证 `Start()` 不会因 `keep_alive`/`retry_time` 为 0 失败。

### 默认值常量移到 controller 包（导出）

`ui` 不能被子包反向依赖，且默认值只应有一份来源。常量从 `internal/ui/settingspage.go` 移到 `internal/controller`，改为导出：

```go
const (
    DefaultUserAgent  = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36"
    DefaultKAliveLink = "http://3.3.3.3"
    DefaultKeepAlive  = 5
    DefaultRetryMax   = 3
    DefaultRetryTime  = 5
)
```

`settingspage.go` 改为引用 `controller.DefaultXxx`；其 `load()` 的回填逻辑保留作为兜底（文件存在但字段为空时界面仍显示默认值文本）。

## 行为变化

- 首次启动即产生 `config.json`（含默认值高级字段，账号密码为空），界面与磁盘内容一致。
- 新配置直接登陆：`config.json` 不再出现空 `user_agent` 与 0 值数值；`Start()` 不会因 `keep_alive`/`retry_time` 为 0 失败。
- 已有配置：行为完全不变，磁盘文件不被触碰。
- 文件存在但非法：内存退回默认值，磁盘文件保留原样，便于用户修复。
- 用户在设置页清空某字段并保存：该字段保持为空（文件已存在），这是 B1 规则的有意结果。
- `interface` 语义不变：默认即空，表示自动探测网卡。

## 测试

- `New` 指向不存在的路径后：`Instance()` 的高级字段等于默认值、`Interface` 为空，且 `config.json` 已被创建并可被 `LoadConfig` 读回。
- `New` 指向已存在且字段为空的配置后：`Instance()` 保持空值，不被默认值覆盖，文件内容不被改写。
- `New` 指向已存在但非法的配置后：`Instance()` 为默认值，但文件内容**保持非法原文不变**。
- 现有测试 `TestNewWithMissingFileUsesEmptyConfig` 断言 `len(Instance) == 0` 且文件不存在，需改写为「1 个默认值实例 + 文件已创建」。
- `TestSaveWithoutInstanceStillWritesFile` 需确认与默认值预置不冲突。

## 明确不做（YAGNI）

- 字段级「填过/清空」追踪（指针字段、影子副本）
- 修改 gzgspd `config` 包的结构或 `Validate` 规则
- 为 `retry_max` 这类「0 合法」字段做特殊处理
