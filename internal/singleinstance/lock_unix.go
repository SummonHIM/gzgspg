//go:build linux || darwin

package singleinstance

import (
	"os"
	"syscall"
)

// tryLock 尝试对文件加排他非阻塞锁。成功返回 true；已被持有返回 false。
func tryLock(f *os.File) bool {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) == nil
}

// unlock 释放锁。
func unlock(f *os.File) {
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
