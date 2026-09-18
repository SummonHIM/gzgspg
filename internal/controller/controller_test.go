package controller

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/summonhim/gzgspd/config"
	"github.com/summonhim/gzgspd/engine"
)

func sleepShort() { time.Sleep(10 * time.Millisecond) }

func timeoutChan() <-chan time.Time { return time.After(5 * time.Second) }

func newTestController(t *testing.T) (*Controller, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	c, err := New(Options{ConfigPath: path})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	return c, path
}

func validInstance() config.ConfigInstance {
	return config.ConfigInstance{
		Username: "u", Password: "p", Interface: "lo",
		KeepAlive: 5, RetryTime: 5,
	}
}

func TestNewWithMissingFileUsesEmptyConfig(t *testing.T) {
	c, path := newTestController(t)
	if c.Config() == nil {
		t.Fatal("expected non-nil config")
	}
	if len(c.Config().Instance) != 0 {
		t.Fatalf("expected empty instance list, got %d", len(c.Config().Instance))
	}
	// 未保存前文件不应存在
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("config file should not exist yet, err=%v", err)
	}
}

func TestUpdateInstanceCreatesFirstSlot(t *testing.T) {
	c, _ := newTestController(t)
	c.UpdateInstance(validInstance())
	if got := c.Instance().Username; got != "u" {
		t.Fatalf("expected username u, got %q", got)
	}
	if len(c.Config().Instance) != 1 {
		t.Fatalf("expected 1 instance, got %d", len(c.Config().Instance))
	}
}

func TestUpdateInstanceReplacesFirstSlot(t *testing.T) {
	c, _ := newTestController(t)
	c.UpdateInstance(validInstance())
	inst := validInstance()
	inst.Username = "second"
	c.UpdateInstance(inst)
	if c.Instance().Username != "second" {
		t.Fatalf("expected replacement, got %q", c.Instance().Username)
	}
	if len(c.Config().Instance) != 1 {
		t.Fatalf("expected still 1 instance, got %d", len(c.Config().Instance))
	}
}

func TestInstanceIsCopy(t *testing.T) {
	c, _ := newTestController(t)
	c.UpdateInstance(validInstance())
	got := c.Instance()
	got.Username = "mutated"
	if c.Instance().Username != "u" {
		t.Fatal("Instance() must return a copy")
	}
}

func TestSaveRoundTrip(t *testing.T) {
	c, path := newTestController(t)
	c.UpdateInstance(validInstance())
	if err := c.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	insts, ok := raw["instance"].([]any)
	if !ok || len(insts) != 1 {
		t.Fatalf("expected array with 1 element, got %v", raw["instance"])
	}

	// 重新加载应保持一致
	c2, err := New(Options{ConfigPath: path})
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if c2.Instance().Username != "u" || c2.Instance().KeepAlive != 5 {
		t.Fatalf("unexpected reloaded instance: %+v", c2.Instance())
	}
}

func TestSaveWithoutInstanceStillWritesFile(t *testing.T) {
	c, path := newTestController(t)
	// 空配置时 Save 不应因 Validate 失败而报错——Save 不做校验
	if err := c.Save(); err != nil {
		t.Fatalf("save empty: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file to exist: %v", err)
	}
}

func TestStartRejectsInvalidConfig(t *testing.T) {
	c, _ := newTestController(t)
	// 空配置 → engine.New 的 Validate 失败
	if err := c.Start(); err == nil {
		t.Fatal("expected error starting with empty config")
	}
	if c.Running() {
		t.Fatal("should not be running after failed start")
	}
}

func TestStateWhenNotRunning(t *testing.T) {
	c, _ := newTestController(t)
	if c.State() != engine.StateStopped {
		t.Fatalf("expected StateStopped, got %v", c.State())
	}
}

func TestStartStopLifecycle(t *testing.T) {
	c, _ := newTestController(t)
	c.UpdateInstance(validInstance())

	// Start 非阻塞，后台会尝试登录（lo 网卡 → 3.3.3.3 不可达，会失败重试）。
	if err := c.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	if !c.Running() {
		t.Fatal("expected controller to be running")
	}

	c.Stop()
	// Stop 触发登出后异步复位
	deadline := 0
	for c.Running() && deadline < 200 {
		sleepShort()
		deadline++
	}
	if c.Running() {
		t.Fatal("expected controller to stop")
	}
	if c.State() != engine.StateStopped {
		t.Fatalf("expected StateStopped after stop, got %v", c.State())
	}
}

func TestSubscribeReceivesEvents(t *testing.T) {
	c, _ := newTestController(t)
	c.UpdateInstance(validInstance())

	got := make(chan engine.Event, 16)
	c.Subscribe(func(e engine.Event) {
		select {
		case got <- e:
		default:
		}
	})

	if err := c.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer c.Stop()

	select {
	case e := <-got:
		if e.Key == "" {
			t.Fatalf("expected non-empty key, got %+v", e)
		}
	case <-timeoutChan():
		t.Fatal("no event received within timeout")
	}
}
