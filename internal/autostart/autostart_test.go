package autostart

import (
	"os"
	"testing"
)

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

func TestReconcileEnablesWithCorrectPath(t *testing.T) {
	if !Supported() {
		t.Skip("autostart not supported on this platform")
	}

	wasEnabled := Enabled()
	t.Cleanup(func() {
		if wasEnabled {
			_ = Enable()
		} else {
			_ = Disable()
		}
	})

	if err := Reconcile(true); err != nil {
		t.Fatalf("reconcile(true): %v", err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("executable: %v", err)
	}
	got, ok := targetPath()
	if !ok {
		t.Fatal("expected a registered target path")
	}
	if got != exe {
		t.Fatalf("target path = %q, want %q", got, exe)
	}
}

func TestReconcileRepairsStalePath(t *testing.T) {
	if !Supported() {
		t.Skip("autostart not supported on this platform")
	}

	wasEnabled := Enabled()
	t.Cleanup(func() {
		if wasEnabled {
			_ = Enable()
		} else {
			_ = Disable()
		}
	})

	// 人为把目标改成一个不存在的路径，模拟程序被移动
	const stale = "/nonexistent/gzgspg.exe"
	if err := writeTarget(stale); err != nil {
		t.Fatalf("writeTarget: %v", err)
	}
	if got, _ := targetPath(); got != stale {
		t.Fatalf("precondition failed, target = %q", got)
	}

	if err := Reconcile(true); err != nil {
		t.Fatalf("reconcile(true): %v", err)
	}
	exe, _ := os.Executable()
	if got, _ := targetPath(); got != exe {
		t.Fatalf("stale path not repaired: got %q, want %q", got, exe)
	}
}

func TestReconcileDisable(t *testing.T) {
	if !Supported() {
		t.Skip("autostart not supported on this platform")
	}

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
	if err := Reconcile(false); err != nil {
		t.Fatalf("reconcile(false): %v", err)
	}
	if Enabled() {
		t.Fatal("expected disabled after Reconcile(false)")
	}
}
