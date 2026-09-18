// Package controller 是 gzgspg 的业务核心：管理配置与登录引擎的生命周期，
// 并把 engine 的事件分发给 UI。它是唯一持有可变业务状态的地方。
package controller

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/summonhim/gzgspd/config"
	"github.com/summonhim/gzgspd/engine"
)

// Options 配置 Controller。
type Options struct {
	// ConfigPath 是 config.json 的路径。
	ConfigPath string
	// Logger 为空时使用 slog.Default()。
	Logger *slog.Logger
}

// Controller 管理单账号的配置与登录引擎。
type Controller struct {
	path   string
	logger *slog.Logger

	mu       sync.Mutex
	cfg      *config.Config
	eng      *engine.Engine
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	running  bool
	state    engine.State
	lastErr  error
	observer []func(engine.Event)
}

// New 加载配置。配置不存在或无效时，返回一个持有空配置的 Controller（不报错），
// 以便首次运行时用户可以在界面里填写。
func New(opts Options) (*Controller, error) {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	c := &Controller{
		path:   opts.ConfigPath,
		logger: logger,
		state:  engine.StateStopped,
	}

	cfg, err := config.LoadConfig(opts.ConfigPath)
	if err != nil {
		// 文件不存在或内容非法：使用空配置，路径先记下，Save 时写出
		c.logger.Info("starting with empty config", "path", opts.ConfigPath, "reason", err)
		cfg = &config.Config{}
		cfg.SetFilePath(opts.ConfigPath)
	}
	c.cfg = cfg

	return c, nil
}

// Config 返回当前配置。调用方不应修改返回值。
func (c *Controller) Config() *config.Config {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cfg
}

// Instance 返回 instance[0] 的副本；不存在时返回零值。
func (c *Controller) Instance() config.ConfigInstance {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.cfg.Instance) == 0 {
		return config.ConfigInstance{}
	}
	return c.cfg.Instance[0]
}

// UpdateInstance 写回 instance[0]；不存在时创建。
func (c *Controller) UpdateInstance(inst config.ConfigInstance) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.cfg.Instance) == 0 {
		c.cfg.Instance = []config.ConfigInstance{inst}
		return
	}
	c.cfg.Instance[0] = inst
}

// Save 把配置落盘。不做 Validate——允许保存未填写完整的配置。
func (c *Controller) Save() error {
	c.mu.Lock()
	cfg := c.cfg
	c.mu.Unlock()
	return cfg.Save()
}

// Start 启动登录引擎。非阻塞：实际运行在后台 goroutine 中。
// 已在运行或配置非法时返回错误。
func (c *Controller) Start() error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return errors.New("already running")
	}

	inst := config.ConfigInstance{}
	if len(c.cfg.Instance) > 0 {
		inst = c.cfg.Instance[0]
	}
	// 单账号：只跑一个 Session
	single := &config.Config{
		LogLevel: c.cfg.LogLevel,
		LogPath:  c.cfg.LogPath,
		Instance: []config.ConfigInstance{inst},
	}

	eng, err := engine.New(single, engine.Options{Logger: c.logger})
	if err != nil {
		c.mu.Unlock()
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	c.eng = eng
	c.cancel = cancel
	c.running = true
	c.wg.Add(1)
	c.mu.Unlock()

	// 事件转发与运行都在后台，Start 立即返回
	go c.consume(ctx, eng)
	return nil
}

// consume 运行引擎并把事件转发给订阅者，结束后复位状态。
func (c *Controller) consume(ctx context.Context, eng *engine.Engine) {
	defer c.wg.Done()

	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := eng.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			c.logger.Error("engine stopped with error", "error", err)
		}
	}()

	for e := range eng.Events() {
		c.mu.Lock()
		c.state = e.State
		c.lastErr = e.Err
		observers := append([]func(engine.Event){}, c.observer...)
		c.mu.Unlock()

		for _, fn := range observers {
			fn(e)
		}
	}
	<-done

	c.mu.Lock()
	c.running = false
	c.eng = nil
	c.cancel = nil
	c.state = engine.StateStopped
	c.mu.Unlock()
}

// Stop 请求停止引擎。非阻塞：只取消 context，登出与复位在后台完成。
// 需要确认已完全停止（例如切换界面）时，配合 WaitStopped 使用。
func (c *Controller) Stop() {
	c.mu.Lock()
	cancel := c.cancel
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// WaitStopped 阻塞至引擎完全停止（事件循环结束、状态复位）。
// 若当前未运行则立即返回。可在任意 goroutine 调用。
func (c *Controller) WaitStopped() {
	c.wg.Wait()
}

// Running 返回引擎是否在运行。
func (c *Controller) Running() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}

// State 返回当前状态；未运行时为 engine.StateStopped。
func (c *Controller) State() engine.State {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state
}

// Subscribe 注册事件回调。回调在 controller 的事件 goroutine 中被调用，
// UI 侧必须自行切回主线程（fyne.Do）。
func (c *Controller) Subscribe(fn func(engine.Event)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.observer = append(c.observer, fn)
}
