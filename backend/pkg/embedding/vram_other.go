//go:build !windows

package embedding

// DetectGPUVRAM returns the dedicated GPU VRAM in MB.
// On non-Windows platforms, returns 0 (detection not implemented).
func DetectGPUVRAM() int {
	return 0
}

// DetectAvailableGPUVRAM returns 0 for non-windows platforms.
func DetectAvailableGPUVRAM() int {
	return 0
}

// GetProcessVRAMUsage returns 0 for now on non-windows.
func GetProcessVRAMUsage() int {
	return 0
}

// GetTotalVRAMUsage returns 0 for now on non-windows.
func GetTotalVRAMUsage() int {
	return 0
}
