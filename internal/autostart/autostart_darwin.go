//go:build darwin

package autostart

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func plistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", AppID+".plist"), nil
}

// Supported 报告当前平台是否支持自启。
func Supported() bool { return true }

// Enabled 报告自启是否已启用。
func Enabled() bool {
	p, err := plistPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
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

// writeTarget 把自启项指向指定路径，写入 LaunchAgent plist。
func writeTarget(path string) error {
	p, err := plistPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	content := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
</dict>
</plist>
`, AppID, path)
	return os.WriteFile(p, []byte(content), 0o644)
}

// targetPath 返回 plist 中 ProgramArguments 下的第一个可执行路径。
func targetPath() (string, bool) {
	p, err := plistPath()
	if err != nil {
		return "", false
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return "", false
	}
	rest := string(data)
	idx := strings.Index(rest, "<key>ProgramArguments</key>")
	if idx < 0 {
		return "", false
	}
	rest = rest[idx:]
	open := strings.Index(rest, "<string>")
	if open < 0 {
		return "", false
	}
	rest = rest[open+len("<string>"):]
	closeIdx := strings.Index(rest, "</string>")
	if closeIdx < 0 {
		return "", false
	}
	return rest[:closeIdx], true
}

// Reconcile 让 OS 自启状态与期望一致。
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
	p, err := plistPath()
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
