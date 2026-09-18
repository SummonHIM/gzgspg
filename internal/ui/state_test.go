package ui

import (
	"testing"

	"github.com/summonhim/gzgspd/engine"
)

func TestStateLabel(t *testing.T) {
	cases := []struct {
		in   engine.State
		want string
	}{
		{engine.StateStarting, "正在启动"},
		{engine.StateNotLoggedIn, "未登录"},
		{engine.StateLoggingIn, "正在登录"},
		{engine.StateLoggedIn, "登录成功"},
		{engine.StatePaused, "已暂停（多次失败）"},
		{engine.StateLoggingOut, "正在登出"},
		{engine.StateStopped, "已停止"},
	}

	for _, tc := range cases {
		if got := stateLabel(tc.in); got != tc.want {
			t.Errorf("stateLabel(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestStateLabelUnknown(t *testing.T) {
	// 未来 engine 新增状态时必须回退到非空文案，不能显示空串
	got := stateLabel(engine.State(99))
	if got != "未知状态" {
		t.Fatalf("expected 未知状态, got %q", got)
	}
	if got == "" {
		t.Fatal("label must never be empty")
	}
}
