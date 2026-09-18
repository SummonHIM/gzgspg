package autostart

import "testing"

func TestSupportedPlatform(t *testing.T) {
	// 本仓库面向 windows/linux/darwin；CI 与开发机应为三者之一。
	if !Supported() {
		t.Skip("autostart not supported on this platform")
	}
}

func TestEnableDisableRoundTrip(t *testing.T) {
	if !Supported() {
		t.Skip("autostart not supported on this platform")
	}

	// 记录原状态，测试结束后恢复，避免污染开发机
	wasEnabled := Enabled()
	t.Cleanup(func() {
		if wasEnabled {
			_ = Enable()
		} else {
			_ = Disable()
		}
	})

	if err := Enable(); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if !Enabled() {
		t.Fatal("expected enabled after Enable()")
	}
	if err := Disable(); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if Enabled() {
		t.Fatal("expected disabled after Disable()")
	}
}
