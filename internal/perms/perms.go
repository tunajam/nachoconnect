package perms

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
)

// CheckResult describes the permission state
type CheckResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

const (
	launchDaemonPath  = "/Library/LaunchDaemons/com.nachoconnect.chmod-bpf.plist"
	chmodBPFScript    = "/Library/Application Support/NachoConnect/chmod-bpf.sh"
	bpfGroupName      = "access_bpf"
	installedFlagFile = ".bpf-setup-done"
)

//go:embed chmod-bpf.sh
var chmodBPFScriptContent string

//go:embed com.nachoconnect.chmod-bpf.plist
var launchDaemonContent string

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
	// Check if /dev/bpf0 is readable by current user
	f, err := os.Open("/dev/bpf0")
	if err == nil {
		f.Close()
		return CheckResult{OK: true, Message: "Packet capture permissions OK"}
	}

	return CheckResult{
		OK:      false,
		Message: "NachoConnect needs permission to capture network traffic. This is a one-time setup.",
	}
}

func checkWindows() CheckResult {
	out, err := exec.Command("sc", "query", "npcap").CombinedOutput()
	if err != nil || !strings.Contains(string(out), "RUNNING") {
		return CheckResult{
			OK:      false,
			Message: "Npcap is required for packet capture on Windows. Please install it from npcap.com",
		}
	}
	return CheckResult{OK: true, Message: "Npcap detected"}
}

// IsSetupDone checks if BPF permissions have already been installed
func IsSetupDone() bool {
	if runtime.GOOS != "darwin" {
		return true
	}
	flagPath := flagFilePath()
	_, err := os.Stat(flagPath)
	return err == nil
}

// RequestElevatedPermissions installs the ChmodBPF LaunchDaemon and group.
// This is the Wireshark/ChmodBPF approach:
// 1. Create access_bpf group
// 2. Add current user to access_bpf group
// 3. Install LaunchDaemon that runs chmod on /dev/bpf* at boot
// 4. Run chmod immediately
// 5. Mark setup as done
func RequestElevatedPermissions(l2tunnelPath string) error {
	if runtime.GOOS != "darwin" {
		return nil
	}

	currentUser, err := user.Current()
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}
	username := currentUser.Username

	// Build the privileged script that does everything in one shot
	script := buildInstallScript(username)

	// Run via osascript with admin privileges
	cmd := exec.Command("osascript", "-e",
		fmt.Sprintf(`do shell script "%s" with administrator privileges`, escapeForAppleScript(script)))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install BPF permissions: %w\nOutput: %s", err, string(out))
	}

	// Mark setup as done
	markSetupDone()

	return nil
}

// buildInstallScript creates a shell script that sets up everything
func buildInstallScript(username string) string {
	// Escape the embedded file contents for shell
	scriptContent := strings.ReplaceAll(chmodBPFScriptContent, "'", "'\\''")
	plistContent := strings.ReplaceAll(launchDaemonContent, "'", "'\\''")

	parts := []string{
		// 1. Create the access_bpf group (ignore error if it already exists)
		fmt.Sprintf("/usr/sbin/dseditgroup -o create %s 2>/dev/null || true", bpfGroupName),

		// 2. Add current user to the group
		fmt.Sprintf("/usr/sbin/dseditgroup -o edit -a %s -t user %s", username, bpfGroupName),

		// 3. Create the script directory
		"mkdir -p '/Library/Application Support/NachoConnect'",

		// 4. Write the chmod-bpf script
		fmt.Sprintf("printf '%%s' '%s' > '%s'", scriptContent, chmodBPFScript),
		fmt.Sprintf("chmod 755 '%s'", chmodBPFScript),

		// 5. Write the LaunchDaemon plist
		fmt.Sprintf("printf '%%s' '%s' > '%s'", plistContent, launchDaemonPath),
		fmt.Sprintf("chown root:wheel '%s'", launchDaemonPath),
		fmt.Sprintf("chmod 644 '%s'", launchDaemonPath),

		// 6. Load the LaunchDaemon
		fmt.Sprintf("launchctl bootout system '%s' 2>/dev/null || true", launchDaemonPath),
		fmt.Sprintf("launchctl bootstrap system '%s'", launchDaemonPath),

		// 7. Run chmod immediately so it works without reboot
		fmt.Sprintf("'%s'", chmodBPFScript),
	}

	return strings.Join(parts, " && ")
}

func escapeForAppleScript(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return s
}

func flagFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/" + installedFlagFile
	}
	dir := filepath.Join(home, ".config", "nachoconnect")
	return filepath.Join(dir, installedFlagFile)
}

func markSetupDone() {
	path := flagFilePath()
	dir := filepath.Dir(path)
	os.MkdirAll(dir, 0755)
	os.WriteFile(path, []byte("installed\n"), 0644)
}

// IsNpcapInstalled checks if Npcap is installed on Windows
func IsNpcapInstalled() bool {
	if runtime.GOOS != "windows" {
		return true
	}
	out, err := exec.Command("sc", "query", "npcap").CombinedOutput()
	return err == nil && strings.Contains(string(out), "RUNNING")
}
