//go:build linux

package autostart

import (
	"fmt"
	"os"
	"path/filepath"
)

func desktopFilePath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "autostart", "gzgspg.desktop"), nil
}

// Supported 报告当前平台是否支持自启。
func Supported() bool { return true }

// Enabled 报告自启是否已启用。
func Enabled() bool {
	p, err := desktopFilePath()
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}

// Enable 启用自启，写入 autostart desktop 文件。
func Enable() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	p, err := desktopFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	content := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=%s
Exec=%s
X-GNOME-Autostart-enabled=true
`, AppName, exe)
	return os.WriteFile(p, []byte(content), 0o644)
}

// Disable 关闭自启。
func Disable() error {
	p, err := desktopFilePath()
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
