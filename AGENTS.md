# AGENTS.md

广工商校园网登录器 GUI。Fyne 桌面应用，业务核心在 `internal/controller`（管理配置与
`github.com/summonhim/gzgspd` 登录引擎的生命周期，并把事件分发给 UI），UI 层在
`internal/ui`，平台自启实现在 `internal/autostart`。

## 本地构建与测试

- 本项目依赖 glfw/GL（CGO）。在 Windows 上直接跑 `go vet ./...` / `go test ./...`
  会因 build constraints 失败（`go-gl/gl` 相关文件被排除）。
- vet 与测试在 CI 的 `ubuntu-latest` + `xvfb-run` 下运行（见
  `.github/workflows/ci.yml`）。本地改动应在 CI 验证，而非仅依赖本机编译。
- 本地只做编译检查：`go build ./...`。

## 配置与数据目录

- 配置与日志目录为 `os.UserConfigDir()/gzgspg`，可用环境变量 `GZGSPG_CONFIG_DIR`
  覆盖（见 `internal/configdir`）。
- `config.json` 含明文密码，已被 `.gitignore` 忽略，勿提交真实凭据。

## 常用命令

- 编译：`go build ./...`
- 测试（Linux/CI）：`xvfb-run -a go test ./... -count=1`
- 构建发布包：见 `.github/workflows/ci.yml` 的 build job。
