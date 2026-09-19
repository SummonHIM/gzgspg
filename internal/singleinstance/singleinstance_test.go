package singleinstance

import (
	"errors"
	"net"
	"testing"
	"time"
)

func TestAcquireSecondInstanceReturnsAlreadyRunning(t *testing.T) {
	dir := t.TempDir()

	p1, err := Acquire(dir)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	defer p1.Release()

	_, err = Acquire(dir)
	if !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("second acquire = %v, want ErrAlreadyRunning", err)
	}
}

func TestActivationSignalDelivered(t *testing.T) {
	dir := t.TempDir()

	p1, err := Acquire(dir)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	defer p1.Release()

	_, err = Acquire(dir)
	if !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("second acquire = %v, want ErrAlreadyRunning", err)
	}

	select {
	case <-p1.Activations():
		// 激活信号已送达
	case <-time.After(2 * time.Second):
		t.Fatal("expected activation signal")
	}
}

func TestReleaseAllowsReacquire(t *testing.T) {
	dir := t.TempDir()

	p1, err := Acquire(dir)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	p1.Release()

	p2, err := Acquire(dir)
	if err != nil {
		t.Fatalf("reacquire after release: %v", err)
	}
	defer p2.Release()
}

func TestSignalExistingRetriesUntilPortAvailable(t *testing.T) {
	dir := t.TempDir()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			_, _ = c.Read(make([]byte, 1))
			_ = c.Close()
		}
	}()

	port := ln.Addr().(*net.TCPAddr).Port

	// 延迟写入端口文件，模拟「锁已持有但端口文件尚未就绪」
	go func() {
		time.Sleep(200 * time.Millisecond)
		if err := writePortFile(dir, port); err != nil {
			t.Errorf("writePortFile: %v", err)
		}
	}()

	if err := signalExisting(dir); err != nil {
		t.Fatalf("signalExisting: %v", err)
	}
}

func TestReleaseIsSafeToCallTwice(t *testing.T) {
	dir := t.TempDir()

	p, err := Acquire(dir)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	p.Release()
	p.Release() // 第二次释放不应 panic
}

func TestPortFileRoundTrip(t *testing.T) {
	dir := t.TempDir()

	if err := writePortFile(dir, 12345); err != nil {
		t.Fatalf("writePortFile: %v", err)
	}
	got, err := readPort(dir)
	if err != nil {
		t.Fatalf("readPort: %v", err)
	}
	if got != 12345 {
		t.Fatalf("readPort = %d, want 12345", got)
	}
}
