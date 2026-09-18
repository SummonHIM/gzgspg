//go:build windows

package autostart

import (
	"errors"
	"os"
	"strings"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

const runKey = `Software\Microsoft\Windows\CurrentVersion\Run`

// Supported 报告当前平台是否支持自启。
func Supported() bool { return true }

// Enabled 报告自启是否已启用。
func Enabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	_, _, err = k.GetStringValue(AppName)
	return err == nil
}

// Enable 启用自启，指向当前可执行文件。
func Enable() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return writeTarget(exe)
}

// writeTarget 把自启项指向指定路径（不存在则创建）。
// 路径整体加引号，避免含空格路径（如 Program Files）在 Run 键里被截断。
func writeTarget(path string) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetStringValue(AppName, `"`+path+`"`)
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
	return strings.Trim(v, `"`), true
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

// Disable 关闭自启。
func Disable() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return nil // 键不存在即已经是关闭状态
	}
	defer k.Close()
	err = k.DeleteValue(AppName)
	if err != nil && !errors.Is(err, syscall.ERROR_FILE_NOT_FOUND) {
		return err
	}
	return nil
}
