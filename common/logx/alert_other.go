//go:build !windows

package logx

import (
	"fmt"
	"os"
)

// MessageBox 非 Windows 平台的降级实现：输出到 stderr。
func MessageBox(title, text string) {
	fmt.Fprintf(os.Stderr, "[%s] %s\n", title, text)
}
