#!/usr/bin/env bash
# 在 ubuntu:22.04 容器内构建 AppImage。
# 前置：工作目录为仓库根（挂载为 /work），dist/ 已存在，gzgspg 二进制已就位，VERSION 环境变量已注入。
set -xeuo pipefail

export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y curl file fuse libfuse2 desktop-file-utils \
  libgtk-3-0 libgtk-3-bin libayatana-appindicator3-1 \
  libgl1 libglx-mesa0 libegl1 libgles2 \
  dpkg-dev pkg-config libglib2.0-bin libgdk-pixbuf2.0-bin imagemagick

mkdir -p dist

# linuxdeploy 要求正方形且尺寸合规的图标；源图 319x320，先补成正方形再缩到 256。
mkdir -p /tmp/icon
convert assets/icon.png -background none -gravity center -extent 320x320 -resize 256x256 /tmp/icon/gzgspg.png

# 锁定 linuxdeploy 版本化 release（不用 continuous，避免上游漂移）。
curl -fsSL -o linuxdeploy https://github.com/linuxdeploy/linuxdeploy/releases/download/1-alpha-20251107-1/linuxdeploy-x86_64.AppImage
# plugin-gtk 没有 release 资源（404），且 linuxdeploy 只按 linuxdeploy-plugin-gtk 这个名字在 PATH 里找插件，故锁定 commit SHA。
curl -fsSL -o linuxdeploy-plugin-gtk https://raw.githubusercontent.com/linuxdeploy/linuxdeploy-plugin-gtk/7a3fbc31a9e5075073ff8790f26effbac5f84453/linuxdeploy-plugin-gtk.sh
chmod +x linuxdeploy linuxdeploy-plugin-gtk

export PATH="$PWD:$PATH"
export ARCH=x86_64
export DEPLOY_GTK_VERSION=3
export LINUXDEPLOY_OUTPUT_VERSION="${VERSION}"
# 容器内没有 fuse，让 AppImage 自解压运行
export APPIMAGE_EXTRACT_AND_RUN=1

./linuxdeploy --appdir AppDir \
  --desktop-file files/release/gzgspg.desktop \
  --icon-file /tmp/icon/gzgspg.png \
  --executable gzgspg \
  --plugin gtk \
  --output appimage

mv gzgspg*.AppImage "dist/gzgspg-linux-amd64-${VERSION}.AppImage"
