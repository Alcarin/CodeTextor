//go:build windows

package utils

import (
	"os/exec"
	"syscall"
)

// SetHideWindow ensures that a command executed on Windows does not open a visible console window.
// This is critical for GUI applications (like Wails production builds) to avoid popup prompt windows.
func SetHideWindow(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
}
