//go:build windows

package logx

import (
	"syscall"
	"unsafe"
)

// MessageBox 弹出原生 Windows 错误对话框，用于 GUI 尚未创建时的致命错误提示。
func MessageBox(title, text string) {
	user32 := syscall.NewLazyDLL("user32.dll")
	proc := user32.NewProc("MessageBoxW")
	t, _ := syscall.UTF16PtrFromString(title)
	p, _ := syscall.UTF16PtrFromString(text)
	// MB_OK | MB_ICONERROR
	proc.Call(0, uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(t)), 0x10)
}
