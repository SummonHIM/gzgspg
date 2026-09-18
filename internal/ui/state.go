package ui

import "github.com/summonhim/gzgspd/engine"

// stateLabel 把 engine 状态翻译为中文展示文案。
// 运行页与托盘共用，保证两处永远一致。
func stateLabel(s engine.State) string {
	switch s {
	case engine.StateStarting:
		return "正在启动"
	case engine.StateNotLoggedIn:
		return "未登录"
	case engine.StateLoggingIn:
		return "正在登录"
	case engine.StateLoggedIn:
		return "登录成功"
	case engine.StatePaused:
		return "已暂停（多次失败）"
	case engine.StateLoggingOut:
		return "正在登出"
	case engine.StateStopped:
		return "已停止"
	default:
		return "未知状态"
	}
}
