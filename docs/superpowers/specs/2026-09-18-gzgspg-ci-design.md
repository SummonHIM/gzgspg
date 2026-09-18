# gzgspg CI 构建与发布设计

日期：2026-09-18
状态：待实现
参考：`gzgspd/.github/workflows/ci.yml`

## 背景

gzgspg 是 Fyne GUI 应用（`fyne.io/fyne/v2 v2.8.1`），依赖 `github.com/summonhim/gzgspd v1.3.0` 的 `config` 与 `engine` 包。需要一个 GitHub Actions CI，产出 Windows / macOS / Linux 三个平台 × 32 位 / 64 位 / arm64 的安装包与裸二进制，并支持 tag 发布。

### 与 gzgspd 的关键差异

gzgspd CI 用 `CGO_ENABLED=0` 在 ubuntu 上纯交叉编译所有平台。**gzgspg 不能这样**：Fyne 的 glfw driver 依赖 `github.com/go-gl/gl/v2.1/gl`，该包有构建约束要求 CGO，`CGO_ENABLED=0` 时 `go build` 直接报 `build constraints exclude all Go files`（已在本机验证）。因此 gzgspg 的构建必须：

- 每个 OS 跑在**对应的原生 runner** 上；
- 全程 `CGO_ENABLED=1`。

## 目标产物

| 平台 | 架构 | 产物 |
|------|------|------|
| Windows | `386` / `amd64` / `arm64` | 纯 exe（zip）、system MSI、user MSI |
| Linux | `386` / `amd64` / `arm64` | 裸二进制（tar.gz）、deb、rpm、AppImage |
| macOS | `amd64` / `arm64` | `.app`（zip）、`.dmg` |

macOS 不存在 32 位：Go 早已移除 `darwin/386`，故 mac 只有两个 64 位架构。这是硬约束，不是取舍。

## Workflow 结构

三段：`check` → `build` → `release`，与 gzgspd 同名同职责。

### 触发

```yaml
on:
  push:
    paths-ignore: ["README.md", ".gitignore", "docs/**", "**/*.md"]
    branches: ["main"]
    tags: ["v*.*.*"]
  pull_request:
    branches: ["main"]
```

PR 只跑 check 与构建验证，不发布。

### 版本号

发布包版本**取自 tag**，只有 MSI 需要数字化映射：

| 用途 | tag `v1.3.0` 时 | 说明 |
|------|-----------------|------|
| 文件名 / 展示 | `1.3.0` | deb、rpm、AppImage、mac、zip、tar.gz 全用这个 |
| WiX MSI | `1.3.0.0` | 补第三段为 0，凑满 WiX 要求的四段数字 |
| `internal/version.Value` | `1.3.0` | 构建期 `-ldflags -X` 注入 |

- deb/rpm/AppImage 直接接受 `1.3.0`，**不使用 run number**；run number 只用于非 tag 构建的版本标记 `0.0.0-<GITHUB_RUN_NUMBER>`，该形态只做构建验证、不发布。
- 预发布 tag（如 `v1.3.0-rc1`）的包版本记 `1.3.0-rc1`；实现时先确认 nfpm 对该格式的处理，若被拒则记为 `1.3.0~rc1`（deb 惯例）。
- 版本字符串注入 `internal/version.Value`，构建时间注入 `BuildTime`，均由 `-ldflags -X` 完成。
- `-trimpath -w -s`，与 gzgspd 的构建参数保持一致。
- `GOTOOLCHAIN=local`，避免 runner 临时下载工具链。

### check job

`runs-on: ubuntu-latest`，`CGO_ENABLED=1`，需要 GUI 依赖：

```bash
sudo apt-get update
sudo apt-get install -y gcc libgl1-mesa-dev xorg-dev libgtk-3-dev libayatana-appindicator3-dev
```

步骤：`go vet ./...`、`go test ./... -count=1`。若出现需要显示服务的测试，用 `xvfb-run -a` 包一层。

### build job

`runs-on: ${{ matrix.jobs.runs-on || 'ubuntu-latest' }}`，`needs: check`。matrix 每行形如：

```yaml
- { goos: windows, goarch: 386,   output: 386,   win: true, msi: i386 }
```

行内标志：`runs-on`、`gui`（windows 打图标）、`msi`（MSI 平台串）、`deb`/`rpm`（nfpm 架构串）。

构建主线（各平台一致）：

```bash
go build -v -trimpath -ldflags "-X 'github.com/summonhim/gzgspg/internal/version.Value=${VERSION}' \
  -X 'github.com/summonhim/gzgspg/internal/version.BuildTime=${BUILDTIME}' -w -s" -o gzgspg
```

Windows 构建前用 `rsrc` 生成 `rsrc.syso`（照 gzgspd）：

```bash
go install github.com/akavel/rsrc@latest
$(go env GOPATH)/bin/rsrc -arch ${{ matrix.jobs.goarch }} -ico assets/icon.ico -o rsrc.syso
```

（需要先把 `assets/icon.png` 转出 `assets/icon.ico`，转换脚本或提交静态文件均可，倾向提交静态文件避免 CI 再装转换工具。）

### release job

与 gzgspd 相同结构：`if: startsWith(github.ref, 'refs/tags/')`，`needs: [check, build]`，`permissions: contents: write`，`actions/download-artifact@v8` + `merge-multiple: true` 汇总到 `bin/`，`softprops/action-gh-release@v2` 创建 **draft** release，`files: bin/*`，`generate_release_notes: true`。

## Windows 打包

### MSI（WiX v4）

WiX 官方对 per-user / per-machine 的推荐做法就是**两个安装包**：单包内动态改安装目录与 Component GUID 会在升级和卸载时出问题。故出两个 MSI，都带标准 Wizard：

| 包名 | 安装位置 | 属性 |
|------|----------|------|
| `gzgspg-<ver>-windows-<arch>-system.msi` | `ProgramFiles64Folder\gzgspg` | `ALLUSERS=1`，需要管理员 |
| `gzgspg-<ver>-windows-<arch>-user.msi` | `LocalAppDataFolder\gzgspg` | `ALLUSERS=2` + `MSIINSTALLPERUSER=1`，免管理员 |

其中 MSI 内部 `Version` 用四段数字 `1.3.0.0`，文件名仍用 `1.3.0`。

实现方式：

- 模板 `files/release/Product.wxs`，占位符 `@@VERSION@@` / `@@ARCH@@` / `@@SCOPE@@` / `@@INSTALLDIR@@`。
- 生成脚本 `files/release/build-msi.ps1 -Scope system|user`：替换占位符 → `wix build` 出 MSI。两个 scope 共用同一 `UpgradeCode`（固定 GUID 常量），`Product Id="*"`，`MajorUpgrade` 保证升级。
- 快捷方式：开始菜单「广工商校园网登录器」（取自 `FyneApp.toml` 的 `[Release] Name`）。
- MSI 元信息：`Manufacturer="SummonHIM"`，`ProductName="广工商校园网登录器"`。
- WiX 版本号 `1.3.0.0`（第三段补 0）。

### 纯 exe

同一 matrix 行额外产出 `gzgspg-windows-<arch>.exe` 并打进 `gzgspg-windows-<arch>-<ver>.zip`。

## Linux 打包

### deb / rpm

用 **nfpm**（单文件 Go 二进制，替代 gzgspd 的 `gem install fpm`：免 Ruby、更快）。配置 `files/release/nfpm.yaml`：

- 二进制 → `/usr/bin/gzgspg`
- 桌面项 `files/release/gzgspg.desktop` → `/usr/share/applications/gzgspg.desktop`
- 图标 → `/usr/share/icons/hicolor/256x256/apps/gzgspg.png`

架构映射：`386`→deb `i386` / rpm `i386`；`amd64`→`amd64` / `x86_64`；`arm64`→`arm64` / `aarch64`。

### AppImage

用 `linuxdeploy` + `linuxdeploy-plugin-gtk`，在**容器内**构建（`ubuntu:22.04`，保证 glibc 向后兼容），AppDir 布局：

```
gzgspg.AppDir/
  AppRun            → 启动脚本
  gzgspg.desktop
  usr/bin/gzgspg
  usr/share/icons/hicolor/256x256/apps/gzgspg.png
```

容器内产物拷回 workspace 再上传。这是整条流水线最容易失败的一环（linuxdeploy 拉库、glib 版本、FUSE 缺失），实现时留出调试余量。

### 裸二进制

`gzgspg-linux-<arch>` → `gzgspg-linux-<arch>-<ver>.tar.gz`。

## macOS 打包

### .app 组装

不依赖 `fyne` CLI 的 Xcode 链路，手动组装，避免额外工具依赖：

```
gzgspg.app/Contents/
  Info.plist                  ← files/release/Info.plist
  MacOS/gzgspg                ← 构建出的二进制（chmod +x）
  Resources/gzgspg.icns       ← sips + iconutil 从 assets/icon.png 生成
```

`Info.plist` 关键字段：`CFBundleIdentifier=top.summonhim.gzgspg`（取自 `FyneApp.toml`）、`CFBundleName=gzgspg`、`CFBundleDisplayName=广工商校园网登录器`、`CFBundleExecutable=gzgspg`、`CFBundlePackageType=APPL`、`CFBundleShortVersionString=<ver>`、`LSMinimumSystemVersion`、`NSHighResolutionCapable=true`。

### 签名与公证

**不做**。需要 Apple Developer 证书与 CI secrets，当前无。后果：Gatekeeper 会拦未签名应用，用户需右键 →「打开」或在「系统设置 → 隐私与安全性」放行。README 需说明。日后如需，加 secrets 与 `codesign`/`notarytool` 步骤即可。

### 分发

- `.app` → `gzgspg-macos-<arch>-<ver>.zip`
- `.dmg`：`hdiutil create`，内含 `gzgspg.app` 与指向 `/Applications` 的软链。

## 需要新增的文件

| 文件 | 用途 |
|------|------|
| `.github/workflows/ci.yml` | 主 workflow |
| `internal/version/version.go` | 构建期注入的版本信息（照 gzgspd） |
| `files/release/Product.wxs` | WiX 模板 |
| `files/release/build-msi.ps1` | 生成 system / user 两个 MSI |
| `files/release/nfpm.yaml` | deb + rpm 配置 |
| `files/release/gzgspg.desktop` | Linux 桌面项 |
| `files/release/Info.plist` | macOS `.app` 模板 |
| `assets/icon.ico` | Windows 图标（由 icon.png 转换，静态提交） |

## 风险

1. **AppImage**：GTK 依赖打包在 CI 中最易失败，容器内 linuxdeploy 的库拉取与 glibc 兼容性需实测调通。
2. **macOS 未签名**：Gatekeeper 拦截，属已知行为，靠文档说明而非技术修复。
3. **windows/386 + Fyne**：Fyne 对 32 位 Windows 的支持需实测；若上游不支持，该行从矩阵移除并在 spec 中标注。
4. **`go.mod` 声明 `go 1.27.0`**：runner 上的 `setup-go` 必须能提供 1.27；若尚不可用，需改用可用的最新版本并同步 `go.mod`。

## 明确不做（YAGNI）

- macOS 代码签名与公证
- 通用（universal）macOS 二进制
- FreeBSD / Android / mips / s390x 等 gzgspd 覆盖但本需求未要求的平台
- Windows 单包内选 scope
- 自动更新（更新器/自更新通道）
- AUR、Pacman、Snap、Flatpak 等其它 Linux 包格式
