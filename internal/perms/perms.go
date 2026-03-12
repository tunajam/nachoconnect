package perms

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// CheckResult describes the permission state
type CheckResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

// CheckPcapPermissions checks if the current process can use libpcap
func CheckPcapPermissions() CheckResult {
	switch runtime.GOOS {
	case "darwin":
		return checkMacOS()
	case "windows":
		return checkWindows()
	default:
		return CheckResult{OK: true, Message: "Permissions check not required on " + runtime.GOOS}
	}
}

func checkMacOS() CheckResult {
	// Try to access BPF devices (libpcap needs this on macOS)
	// Check if /dev/bpf0 is readable
	out, err := exec.Command("test", "-r", "/dev/bpf0").CombinedOutput()
	if err != nil {
		_ = out
		return CheckResult{
			OK:      false,
			Message: "NachoConnect needs permission to capture network traffic. Please grant access when prompted.",
		}
	}
	return CheckResult{OK: true, Message: "Packet capture permissions OK"}
}

func checkWindows() CheckResult {
	// Check if Npcap is installed
	out, err := exec.Command("sc", "query", "npcap").CombinedOutput()
	if err != nil || !strings.Contains(string(out), "RUNNING") {
		return CheckResult{
			OK:      false,
			Message: "Npcap is required for packet capture on Windows. Please install it from npcap.com",
		}
	}
	return CheckResult{OK: true, Message: "Npcap detected"}
}

// RequestElevatedPermissions prompts for admin access on macOS using osascript
func RequestElevatedPermissions(l2tunnelPath string) error {
	if runtime.GOOS != "darwin" {
		return nil
	}

	// Use osascript to set BPF device permissions
	script := fmt.Sprintf(`do shell script "chmod o+r /dev/bpf*" with administrator privileges`)
	cmd := exec.Command("osascript", "-e", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set permissions: %w: %s", err, string(out))
	}
	return nil
}

// IsNpcapInstalled checks if Npcap is installed on Windows
func IsNpcapInstalled() bool {
	if runtime.GOOS != "windows" {
		return true
	}
	out, err := exec.Command("sc", "query", "npcap").CombinedOutput()
	return err == nil && strings.Contains(string(out), "RUNNING")
}
