#!/usr/bin/env bash
# 在 macOS runner 上打包 .app / .zip / .dmg。
# 前置：工作目录为仓库根，dist/ 已存在，gzgspg 二进制已编译（BIN 环境变量）。
# 用法：bash files/release/build-macos.sh <VERSION> <output-arch>
set -euo pipefail

VERSION="${1:?missing version}"
OUTPUT="${2:?missing output arch}"
BIN="${BIN:-gzgspg}"

APP="dist/gzgspg.app"
rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"

cp "${BIN}" "$APP/Contents/MacOS/gzgspg"
chmod +x "$APP/Contents/MacOS/gzgspg"

sed "s/@@VERSION@@/${VERSION}/" files/release/Info.plist > "$APP/Contents/Info.plist"

# icon.png -> icon.icns
ICONSET="$(mktemp -d)/gzgspg.iconset"
mkdir -p "$ICONSET"
for s in 16 32 64 128 256 512; do
  sips -z $s $s assets/icon.png --out "$ICONSET/icon_${s}x${s}.png" >/dev/null
  d=$((s * 2))
  sips -z $d $d assets/icon.png --out "$ICONSET/icon_${s}x${s}@2x.png" >/dev/null
done
iconutil -c icns "$ICONSET" -o "$APP/Contents/Resources/gzgspg.icns"

(cd dist && zip -r "gzgspg-macos-${OUTPUT}-${VERSION}.zip" "gzgspg.app")

# dmg：app + Applications 软链
STAGE="$(mktemp -d)/dmg"
mkdir -p "$STAGE"
cp -R "$APP" "$STAGE/"
ln -s /Applications "$STAGE/Applications"
hdiutil create -volname "gzgspg" -srcfolder "$STAGE" -ov -format UDZO \
  "dist/gzgspg-macos-${OUTPUT}-${VERSION}.dmg"
