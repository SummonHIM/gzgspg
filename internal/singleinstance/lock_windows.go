//go:build windows

package singleinstance

import (
	"os"

	"golang.org/x/sys/windows"
)

// tryLock 尝试对文件加排他非阻塞锁。成功返回 true；已被持有返回 false。
func tryLock(f *os.File) bool {
	var ol windows.Overlapped
	return windows.LockFileEx(
		windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0, 1, 0, &ol,
	) == nil
}

// unlock 释放锁。
func unlock(f *os.File) {
	var ol windows.Overlapped
	_ = windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, &ol)
}
