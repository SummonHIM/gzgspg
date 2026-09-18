// Package autostart 管理开机自启。各平台实现见 autostart_<goos>.go。
package autostart

import (
	"errors"
	"strings"
)

// ErrUnsupported 表示当前平台不支持自启。
var ErrUnsupported = errors.New("autostart is not supported on this platform")

const (
	// AppName 用于系统自启项的名称。
	AppName = "gzgspg"
	// AppID 用于 macOS 的 LaunchAgent label 与标识。
	AppID = "top.summonhim.gzgspg"
)

// needsRepair 判断已存储的自启值是否需要重写为规范形式。
// raw 是存储的原始值（可能带引号），exe 是当前可执行文件路径。
// 路径不同视为过期；路径相同但含空格且未加引号视为需修复。
func needsRepair(raw, exe string) bool {
	if strings.Trim(raw, `"`) != exe {
		return true
	}
	return strings.Contains(exe, " ") && !strings.HasPrefix(raw, `"`)
}

// Reconcile 让当前平台的自启状态与期望值一致：期望开启时确保自启项
// 指向当前可执行文件（路径失效则重写），期望关闭时移除自启项。
// 状态已正确时不做写入。各平台实现见 autostart_<goos>.go。
//
// 注意：与 Enabled() 不同，Reconcile 会检查目标路径是否仍然有效。
