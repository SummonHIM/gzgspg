//go:build linux

package autostart

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// Enable 启用自启，指向当前可执行文件。
func Enable() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return writeTarget(exe)
}

// writeTarget 把自启项指向指定路径，写入 autostart desktop 文件。
// 路径含空格时用引号包裹，避免 Exec= 行被按空格拆解。
func writeTarget(path string) error {
	p, err := desktopFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	exec := path
	if strings.Contains(path, " ") {
		exec = `"` + path + `"`
	}
	content := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=%s
Exec=%s
X-GNOME-Autostart-enabled=true
`, AppName, exec)
	return os.WriteFile(p, []byte(content), 0o644)
}

// rawTarget 返回 desktop 文件中 Exec= 后的原始内容（不去引号）；读不到时 ok 为 false。
func rawTarget() (string, bool) {
	p, err := desktopFilePath()
	if err != nil {
		return "", false
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "Exec=") {
			// 容错 CRLF：去掉行尾回车。
			return strings.TrimRight(strings.TrimPrefix(line, "Exec="), "\r"), true
		}
	}
	return "", false
}

// targetPath 返回 desktop 文件中 Exec= 后的内容（去掉引号）；读不到时 ok 为 false。
func targetPath() (string, bool) {
	v, ok := rawTarget()
	if !ok {
		return "", false
	}
	return strings.Trim(v, `"`), true
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
	if raw, ok := rawTarget(); ok && !needsRepair(raw, exe) {
		return nil
	}
	return writeTarget(exe)
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
