# gzgspg

广工商校园网登录器 GUI。基于 [Fyne](https://fyne.io/) 的桌面客户端，业务核心由
[`github.com/summonhim/gzgspd`](https://github.com/summonhim/gzgspd) 登录引擎驱动。

## 功能

- 账号密码登录 / 登出，运行状态实时展示
- 系统托盘常驻：显示状态、唤出主窗口、退出
- 高级设置：网卡选择、User-Agent、在线监测间隔与地址、重试策略
- 自动登陆、开机自启
- 跨平台：Windows（.exe / .zip / .msi）、macOS（.app / .dmg）、Linux（tar.gz / .deb / .rpm / AppImage）

## 安装

从 [Releases](https://github.com/SummonHIM/gzgspg/releases) 下载对应平台的安装包：

| 平台 | 格式 |
| --- | --- |
| Windows | `.msi`（system / user 两种）、`.exe`、`.zip` |
| macOS | `.dmg`、`.zip` |
| Linux | `.AppImage`、`.deb`、`.rpm`、`.tar.gz` |

## 配置与数据目录

配置与日志存放在 `os.UserConfigDir()/gzgspg`，可用环境变量 `GZGSPG_CONFIG_DIR` 覆盖：

- `config.json` — 账号、密码及高级设置（含明文密码，勿提交真实凭据）
- `gzgspg.log` — 运行日志

首次运行会自动生成一份带默认值的 `config.json`。

## 本地构建

```bash
go build ./...
```

> 本项目依赖 glfw/GL（CGO）。在 Windows 上直接跑 `go vet` / `go test` 会因
> build constraints 失败（`go-gl/gl` 相关文件被排除），vet 与测试在 CI 的
> `ubuntu-latest` + `xvfb-run` 下运行。详见 `AGENTS.md`。

构建发布包见 `.github/workflows/ci.yml`。

## License

[MIT](LICENSE) © SummonHIM
