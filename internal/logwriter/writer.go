// Package logwriter 提供一个有界的 io.Writer，用于把 slog 输出送给 UI。
package logwriter

import (
	"strings"
	"sync"
)

// Writer 收集以换行分隔的日志行，只保留最近 maxLines 行。
type Writer struct {
	mu        sync.Mutex
	maxLines  int
	lines     []string
	partial   string
	observers []func(string)
}

// New 创建 Writer，maxLines 为保留的最大行数。
func New(maxLines int) *Writer {
	if maxLines <= 0 {
		maxLines = 500
	}
	return &Writer{maxLines: maxLines}
}

// Write 实现 io.Writer。
func (w *Writer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	text := w.partial + string(p)
	parts := strings.Split(text, "\n")
	// 最后一段没有换行结尾，留作 partial
	w.partial = parts[len(parts)-1]

	for _, line := range parts[:len(parts)-1] {
		w.appendLocked(line)
	}
	return len(p), nil
}

func (w *Writer) appendLocked(line string) {
	w.lines = append(w.lines, line)
	if len(w.lines) > w.maxLines {
		w.lines = w.lines[len(w.lines)-w.maxLines:]
	}
	for _, fn := range w.observers {
		fn(line)
	}
}

// Lines 返回当前缓冲的副本。
func (w *Writer) Lines() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]string, len(w.lines))
	copy(out, w.lines)
	return out
}

// Subscribe 注册每行日志的回调。回调在 Write 的调用栈中同步执行。
func (w *Writer) Subscribe(fn func(line string)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.observers = append(w.observers, fn)
}
