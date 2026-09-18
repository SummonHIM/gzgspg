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

func wantDefaults(t *testing.T, c *Controller) {
	t.Helper()
	inst := c.Instance()
	if inst.Interface != "" {
		t.Fatalf("expected empty interface, got %q", inst.Interface)
	}
	if inst.UserAgent != DefaultUserAgent {
		t.Fatalf("expected default user agent, got %q", inst.UserAgent)
	}
	if inst.KAliveLink != DefaultKAliveLink {
		t.Fatalf("expected default keep_alive_link, got %q", inst.KAliveLink)
	}
	if inst.KeepAlive != DefaultKeepAlive {
		t.Fatalf("expected default keep_alive %d, got %d", DefaultKeepAlive, inst.KeepAlive)
	}
	if inst.RetryMax != DefaultRetryMax {
		t.Fatalf("expected default retry_max %d, got %d", DefaultRetryMax, inst.RetryMax)
	}
	if inst.RetryTime != DefaultRetryTime {
		t.Fatalf("expected default retry_time %d, got %d", DefaultRetryTime, inst.RetryTime)
	}
}

func TestNewWithMissingFileWritesDefaultConfig(t *testing.T) {
	c, path := newTestController(t)

	if len(c.Config().Instance) != 1 {
		t.Fatalf("expected 1 default instance, got %d", len(c.Config().Instance))
	}
	wantDefaults(t, c)

	// 文件应已被创建，且能被重新加载
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected config file to be created: %v", err)
	}
	c2, err := New(Options{ConfigPath: path})
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	wantDefaults(t, c2)
}

func TestNewWithExistingEmptyFieldsKeepsThem(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	// 手动写一份字段刻意留空但合法的配置
	raw := `{"log_level":0,"log_path":"","instance":[{"username":"u","password":"p","interface":"","user_agent":"","keep_alive":5,"keep_alive_link":"","retry_max":0,"retry_time":5}]}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	c, err := New(Options{ConfigPath: path})
	if err != nil {
		t.Fatalf("new: %v", err)
	}

	inst := c.Instance()
	if inst.UserAgent != "" || inst.KAliveLink != "" {
		t.Fatalf("existing empty fields must not be filled: %+v", inst)
	}
	if inst.RetryMax != 0 {
		t.Fatalf("expected retry_max 0 preserved, got %d", inst.RetryMax)
	}

	// 磁盘文件不得被改写
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != raw {
		t.Fatalf("file must be untouched\nwant: %s\ngot:  %s", raw, got)
	}
}

func TestNewWithInvalidFileDoesNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	// 非法 JSON：用户改坏了配置文件
	raw := `{"instance": [`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	c, err := New(Options{ConfigPath: path})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	wantDefaults(t, c)

	// 关键：非法文件必须原样保留，不能被默认值顶掉
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != raw {
		t.Fatalf("invalid file must be preserved\nwant: %s\ngot:  %s", raw, got)
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

func TestSaveWithoutEmptyingInstanceWritesFile(t *testing.T) {
	c, path := newTestController(t)
	// 首次运行已写出默认值文件，Save 必须可重复调用且不报错
	if err := c.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file to exist: %v", err)
	}
	if len(c.Config().Instance) != 1 {
		t.Fatalf("expected default instance preserved, got %d", len(c.Config().Instance))
	}
}

func TestStartRejectsMissingCredentials(t *testing.T) {
	c, _ := newTestController(t)
	// 默认值配置没有账号密码 → engine.New 的 Validate 失败
	if err := c.Start(); err == nil {
		t.Fatal("expected error starting without credentials")
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

func TestWaitStoppedReturnsAfterStop(t *testing.T) {
	c, _ := newTestController(t)
	c.UpdateInstance(validInstance())

	if err := c.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	stopped := make(chan struct{})
	go func() {
		c.Stop()
		c.WaitStopped()
		close(stopped)
	}()

	select {
	case <-stopped:
	case <-timeoutChan():
		t.Fatal("WaitStopped did not return within timeout")
	}
	if c.Running() {
		t.Fatal("expected not running once WaitStopped returned")
	}
}

func TestWaitStoppedWhenIdleReturnsImmediately(t *testing.T) {
	c, _ := newTestController(t)

	done := make(chan struct{})
	go func() {
		c.WaitStopped()
		close(done)
	}()

	select {
	case <-done:
	case <-timeoutChan():
		t.Fatal("WaitStopped should return immediately when idle")
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

func TestConfigIsCopy(t *testing.T) {
	c, _ := newTestController(t)
	got := c.Config()
	got.Instance[0].Username = "mutated"
	got.LogLevel = 99
	if c.Instance().Username != "" {
		t.Fatalf("expected username unchanged, got %q", c.Instance().Username)
	}
	if c.Config().LogLevel != 0 {
		t.Fatalf("expected log level unchanged, got %d", c.Config().LogLevel)
	}
}
