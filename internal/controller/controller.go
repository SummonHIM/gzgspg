// Package controller 是 gzgspg 的业务核心：管理配置与登录引擎的生命周期，
// 并把 engine 的事件分发给 UI。它是唯一持有可变业务状态的地方。
package controller

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"

	"github.com/summonhim/gzgspd/config"
	"github.com/summonhim/gzgspd/engine"
)

// 高级字段的默认值。必须与 gzgspd engine 的内部默认保持一致。
// 界面层应引用这些常量，避免两处默认值漂移。
const (
	DefaultUserAgent  = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36"
	DefaultKAliveLink = "http://3.3.3.3"
	DefaultKeepAlive  = 5
	DefaultRetryMax   = 3
	DefaultRetryTime  = 5
)

// defaultInstance 返回首次运行使用的实例：账号密码留空，高级字段取默认值。
// interface 保持空，语义为自动探测网卡。
func defaultInstance() config.ConfigInstance {
	return config.ConfigInstance{
		UserAgent:  DefaultUserAgent,
		KAliveLink: DefaultKAliveLink,
		KeepAlive:  DefaultKeepAlive,
		RetryMax:   DefaultRetryMax,
		RetryTime:  DefaultRetryTime,
	}
}

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

// New 加载配置。配置文件缺失时，写出一份带默认值的文件供用户填写；
// 文件存在但内容非法时，只在内存里退回默认值，绝不覆盖用户的文件。
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
		cfg = &config.Config{}
		cfg.SetFilePath(opts.ConfigPath)
		cfg.Instance = []config.ConfigInstance{defaultInstance()}

		if errors.Is(err, os.ErrNotExist) {
			// 首次运行：把默认值写成一份文件，让界面与磁盘同源
			if saveErr := cfg.Save(); saveErr != nil {
				c.logger.Error("write default config failed", "path", opts.ConfigPath, "error", saveErr)
			} else {
				c.logger.Info("created default config", "path", opts.ConfigPath)
			}
		} else {
			// 文件存在但非法：保留原文件，仅在内存里使用默认值
			c.logger.Info("config invalid, keeping file as-is", "path", opts.ConfigPath, "reason", err)
		}
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
