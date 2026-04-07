//go:build windows

package embedding

import (
	"context"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"CodeTextor/backend/pkg/utils"
)

// DetectGPUVRAM returns the dedicated GPU VRAM in MB on Windows.
// It uses dxdiag or nvidia-smi to query GPU information.
func DetectGPUVRAM() int {
	// Try nvidia-smi first (most accurate for NVIDIA GPUs)
	if vram := detectVRAMNvidiaSMI(); vram > 0 {
		return vram
	}

	// Fallback to PowerShell WMI query
	if vram := detectVRAMWMI(); vram > 0 {
		return vram
	}

	return 0
}

// DetectAvailableGPUVRAM returns the available GPU VRAM in MB on Windows.
func DetectAvailableGPUVRAM() int {
	total := DetectGPUVRAM()
	if total <= 0 {
		return 0
	}

	// Try to get actual usage via nvidia-smi
	used := GetTotalVRAMUsage()
	if used <= 0 {
		// Fallback: estimate 70% of total is available if we can't query current usage (e.g. non-NVIDIA)
		return int(float64(total) * 0.7)
	}

	available := total - used
	if available < 0 {
		available = 0
	}
	return available
}


// detectVRAMNvidiaSMI queries NVIDIA GPU VRAM using nvidia-smi.
func detectVRAMNvidiaSMI() int {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "nvidia-smi", "--query-gpu=memory.total", "--format=csv,noheader,nounits")
	utils.SetHideWindow(cmd)
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			log.Printf("Warning: nvidia-smi (memory.total) timed out after 3s")
		}
		return 0
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 {
		return 0
	}

	// Take the first GPU's VRAM (in MiB from nvidia-smi)
	vramStr := strings.TrimSpace(lines[0])
	vram, err := strconv.Atoi(vramStr)
	if err != nil {
		return 0
	}

	log.Printf("Detected GPU VRAM via nvidia-smi: %d MB", vram)
	return vram
}

// detectVRAMWMI queries GPU VRAM using Windows WMI via PowerShell.
func detectVRAMWMI() int {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command",
		"(Get-CimInstance Win32_VideoController | Where-Object { $_.AdapterRAM -gt 0 } | Sort-Object AdapterRAM -Descending | Select-Object -First 1).AdapterRAM")
	utils.SetHideWindow(cmd)
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			log.Printf("Warning: powershell (WMI VRAM) timed out after 3s")
		}
		return 0
	}

	vramStr := strings.TrimSpace(string(output))
	vramBytes, err := strconv.ParseInt(vramStr, 10, 64)
	if err != nil {
		return 0
	}

	vramMB := int(vramBytes / (1024 * 1024))
	log.Printf("Detected GPU VRAM via WMI: %d MB", vramMB)
	return vramMB
}

// GetProcessVRAMUsage returns the VRAM used by the current process, or the total GPU usage as fallback.
func GetProcessVRAMUsage() int {
	pid := os.Getpid()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "nvidia-smi", "--query-compute-apps=pid,used_memory", "--format=csv,noheader,nounits")
	utils.SetHideWindow(cmd)
	output, err := cmd.Output()
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, line := range lines {
			parts := strings.Split(line, ",")
			if len(parts) >= 2 {
				foundPid, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
				if foundPid == pid {
					vram, err := strconv.Atoi(strings.TrimSpace(parts[1]))
					if err == nil && vram > 0 {
						return vram
					}
				}
			}
		}
	} else if ctx.Err() == context.DeadlineExceeded {
		log.Printf("Warning: nvidia-smi (process usage) timed out after 3s")
	}

	// Fallback: Total GPU memory used
	return GetTotalVRAMUsage()
}

// GetTotalVRAMUsage returns the total occupied VRAM on the first GPU.
func GetTotalVRAMUsage() int {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "nvidia-smi", "--query-gpu=memory.used", "--format=csv,noheader,nounits")
	utils.SetHideWindow(cmd)
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			log.Printf("Warning: nvidia-smi (memory.used) timed out after 3s")
		}
		return 0
	}
	vram, _ := strconv.Atoi(strings.TrimSpace(string(output)))
	return vram
}


