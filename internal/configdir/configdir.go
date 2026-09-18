// Package configdir 解析 gzgspg 的配置与日志目录。
package configdir

import (
	"os"
	"path/filepath"
)

// EnvOverride 是覆盖配置目录的环境变量名。非空时直接作为目录使用。
const EnvOverride = "GZGSPG_CONFIG_DIR"

// Dir 返回配置目录并确保其存在。
// 若设置了 GZGSPG_CONFIG_DIR，直接使用其值；否则为 <UserConfigDir>/gzgspg。
func Dir() (string, error) {
	dir := os.Getenv(EnvOverride)
	if dir == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(base, "gzgspg")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// ConfigPath 返回 config.json 的完整路径。
func ConfigPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// LogPath 返回日志文件的完整路径。
func LogPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "gzgspg.log"), nil
}
