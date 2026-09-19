// Package singleinstance 保证同一时间只运行一个 gzgspg 实例，
// 并提供跨进程「激活主窗口」的信号通道。
package singleinstance

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ErrAlreadyRunning 表示已有实例在运行，且已向其发送激活信号。
var ErrAlreadyRunning = errors.New("another instance is already running")

const (
	lockFileName = "gzgspg.lock"
	portFileName = "gzgspg.port"
	activateByte = "a"

	// connectRetries / connectDelay 是次实例发现端口文件的等待窗口：
	// 主实例先拿到锁、后写端口文件，中间有极小窗口端口文件尚不存在。
	connectRetries = 10
	connectDelay   = 50 * time.Millisecond
)

// Primary 代表持有单实例锁的主实例。
type Primary struct {
	lockFile    *os.File
	listener    net.Listener
	activations chan struct{}
}

// Activations 返回通道：每有后续实例请求激活时收到一个值。
func (p *Primary) Activations() <-chan struct{} {
	return p.activations
}

// Release 释放锁、关闭监听。可安全重复调用。
func (p *Primary) Release() {
	if p.listener != nil {
		_ = p.listener.Close()
	}
	if p.lockFile != nil {
		unlock(p.lockFile)
		_ = p.lockFile.Close()
		p.lockFile = nil
	}
}

// Acquire 尝试成为主实例。成功返回 *Primary；已有实例在运行时，
// 向其发送激活信号（尽力而为）并返回 ErrAlreadyRunning。
// 锁文件只做排他仲裁，永不删除；端口文件由主实例覆盖写。
func Acquire(dir string) (*Primary, error) {
	lf, err := os.OpenFile(filepath.Join(dir, lockFileName), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if !tryLock(lf) {
		_ = lf.Close()
		// 激活失败也不影响主目标：绝不再启动第二个实例。
		_ = signalExisting(dir)
		return nil, ErrAlreadyRunning
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		unlock(lf)
		_ = lf.Close()
		return nil, err
	}

	p := &Primary{
		lockFile:    lf,
		listener:    ln,
		activations: make(chan struct{}, 1),
	}

	if err := writePortFile(dir, ln.Addr().(*net.TCPAddr).Port); err != nil {
		p.Release()
		return nil, err
	}

	go p.serve()
	return p, nil
}

// serve 接受来自次实例的连接，读到激活字节后投递一个信号。
func (p *Primary) serve() {
	for {
		conn, err := p.listener.Accept()
		if err != nil {
			return // listener 被 Release 关闭
		}
		buf := make([]byte, 1)
		_ = conn.SetReadDeadline(time.Now().Add(time.Second))
		if _, err := conn.Read(buf); err == nil {
			select {
			case p.activations <- struct{}{}:
			default:
				// 缓冲区已满时丢弃：激活是幂等的，一次信号足够唤起窗口
			}
		}
		_ = conn.Close()
	}
}

// writePortFile 把监听端口写入端口文件。
func writePortFile(dir string, port int) error {
	return os.WriteFile(portPath(dir), []byte(strconv.Itoa(port)), 0o644)
}

// readPort 读取端口文件内容。
func readPort(dir string) (int, error) {
	data, err := os.ReadFile(portPath(dir))
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

func portPath(dir string) string {
	return filepath.Join(dir, portFileName)
}

// signalExisting 通知已运行实例激活窗口。端口文件可能尚未就绪，
// 故在窗口内重试读取与连接。
func signalExisting(dir string) error {
	var lastErr error
	for i := 0; i < connectRetries; i++ {
		port, err := readPort(dir)
		if err != nil {
			lastErr = err
		} else if err := dialAndActivate(port); err != nil {
			lastErr = err
		} else {
			return nil
		}
		time.Sleep(connectDelay)
	}
	return lastErr
}

// dialAndActivate 连接主实例并写入一个激活字节。
func dialAndActivate(port int) error {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 500*time.Millisecond)
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = conn.Write([]byte(activateByte))
	return err
}
