//go:build !windows

package gui

import (
	"os"
	"syscall"

	"fyne.io/fyne/v2"
)

// restartApp 重启当前程序（Unix-like：syscall.Exec）。
func restartApp() {
	exe, err := os.Executable()
	if err != nil {
		fyne.LogError("restartApp: get executable path", err)
		return
	}
	if err := syscall.Exec(exe, os.Args, os.Environ()); err != nil {
		fyne.LogError("restartApp: exec", err)
		return
	}
	os.Exit(0)
}
