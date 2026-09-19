//go:build !linux && !darwin && !windows

package singleinstance

import "os"

// tryLock 在不支持锁的平台恒返回 true（视为始终可成为主实例）。
func tryLock(f *os.File) bool { return true }

// unlock 在不支持锁的平台为空操作。
func unlock(f *os.File) {}
