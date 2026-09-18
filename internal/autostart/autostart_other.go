//go:build !windows && !linux && !darwin

package autostart

// Supported 报告当前平台是否支持自启。
func Supported() bool { return false }

// Enabled 报告自启是否已启用。
func Enabled() bool { return false }

// Enable 在当前平台不可用。
func Enable() error { return ErrUnsupported }

// Disable 在当前平台不可用。
func Disable() error { return ErrUnsupported }
