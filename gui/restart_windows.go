//go:build windows

package gui

import (
	"os"
	"os/exec"
	"syscall"

	"fyne.io/fyne/v2"
)

// restartApp 重启当前程序（Windows）。
// 直接启动新进程并隐藏控制台窗口，不再走 cmd /c start（会额外弹黑框）。
func restartApp() {
	exe, err := os.Executable()
	if err != nil {
		fyne.LogError("restartApp: get executable path", err)
		return
	}

	cmd := exec.Command(exe)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		fyne.LogError("restartApp: start new process", err)
		return
	}

	// 退出当前程序
	os.Exit(0)
}
