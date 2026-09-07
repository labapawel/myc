//go:build !windows

package gui

import "syscall"

func getSysProcAttr() *syscall.SysProcAttr {
	return nil
}
