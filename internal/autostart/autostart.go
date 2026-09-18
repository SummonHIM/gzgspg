// Package autostart 管理开机自启。各平台实现见 autostart_<goos>.go。
package autostart

import "errors"

// ErrUnsupported 表示当前平台不支持自启。
var ErrUnsupported = errors.New("autostart is not supported on this platform")

const (
	// AppName 用于系统自启项的名称。
	AppName = "gzgspg"
	// AppID 用于 macOS 的 LaunchAgent label 与标识。
	AppID = "top.summonhim.gzgspg"
)
