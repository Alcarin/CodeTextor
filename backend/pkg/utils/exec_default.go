//go:build !windows

package utils

import (
	"os/exec"
)

// SetHideWindow is a no-op on non-Windows systems.
func SetHideWindow(cmd *exec.Cmd) {
	// No-op
}
