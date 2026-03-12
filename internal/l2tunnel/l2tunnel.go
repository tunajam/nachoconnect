package l2tunnel

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

// Interface represents a network interface returned by l2tunnel list
type Interface struct {
	Index       int      `json:"index"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Flags       []string `json:"flags,omitempty"`
}

// Discovery represents a discovered MAC address from l2tunnel discover
type Discovery struct {
	SrcMAC string `json:"srcMAC"`
	DstMAC string `json:"dstMAC"`
}

// TunnelConfig holds the configuration for a tunnel session
type TunnelConfig struct {
	Interface  string `json:"interface"`
	FilterMode string `json:"filterMode"` // "-s" (source) or "-d" (destination)
	MAC        string `json:"mac"`
	LocalAddr  string `json:"localAddr"`
	LocalPort  string `json:"localPort"`
	RemoteAddr string `json:"remoteAddr"`
	RemotePort string `json:"remotePort"`
}

// Tunnel manages a running l2tunnel subprocess
type Tunnel struct {
	mu     sync.RWMutex
	cmd    *exec.Cmd
	cancel context.CancelFunc
	active bool
	output []string
	err    error
}

// BinaryPath is the path to the l2tunnel binary. Set during init.
var BinaryPath = "lib/l2tunnel/l2tunnel"

// List returns available network interfaces by running `l2tunnel list`
func List() ([]Interface, error) {
	out, err := exec.Command(BinaryPath, "list").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("l2tunnel list failed: %w: %s", err, string(out))
	}
	return parseListOutput(string(out)), nil
}

func parseListOutput(output string) []Interface {
	var interfaces []Interface
	var current *Interface

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "device ") {
			// "device 0: en0"
			if current != nil {
				interfaces = append(interfaces, *current)
			}
			current = &Interface{}
			parts := strings.SplitN(line, ": ", 2)
			if len(parts) == 2 {
				current.Name = parts[1]
			}
			fmt.Sscanf(parts[0], "device %d", &current.Index)
		} else if current != nil {
			if strings.HasPrefix(line, "- description: ") {
				current.Description = strings.TrimPrefix(line, "- description: ")
			} else if strings.HasPrefix(line, "- flags:") {
				flagStr := strings.TrimPrefix(line, "- flags:")
				for _, f := range strings.Fields(flagStr) {
					current.Flags = append(current.Flags, f)
				}
			}
		}
	}
	if current != nil {
		interfaces = append(interfaces, *current)
	}
	return interfaces
}

// Discover runs `l2tunnel discover <iface>` and streams discovered MAC addresses.
// It runs until ctx is cancelled. Results are sent on the returned channel.
func Discover(ctx context.Context, iface string) (<-chan Discovery, error) {
	ctx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(ctx, BinaryPath, "discover", iface)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to get stdout: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("l2tunnel discover failed to start: %w", err)
	}

	ch := make(chan Discovery, 32)
	go func() {
		defer close(ch)
		defer cancel()
		defer cmd.Wait()

		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			// Format: "00:0d:3a:38:ac:2e to ff:ff:ff:ff:ff:ff"
			parts := strings.Split(line, " to ")
			if len(parts) == 2 {
				d := Discovery{
					SrcMAC: strings.TrimSpace(parts[0]),
					DstMAC: strings.TrimSpace(parts[1]),
				}
				select {
				case ch <- d:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return ch, nil
}

// StartTunnel starts an l2tunnel tunnel subprocess with the given config
func StartTunnel(cfg TunnelConfig) (*Tunnel, error) {
	if cfg.FilterMode == "" {
		cfg.FilterMode = "-s"
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, BinaryPath, "tunnel",
		cfg.Interface,
		cfg.FilterMode,
		cfg.MAC,
		cfg.LocalAddr,
		cfg.LocalPort,
		cfg.RemoteAddr,
		cfg.RemotePort,
	)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to get stderr: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to get stdout: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("l2tunnel tunnel failed to start: %w", err)
	}

	t := &Tunnel{
		cmd:    cmd,
		cancel: cancel,
		active: true,
	}

	// Read output in background
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			t.mu.Lock()
			t.output = append(t.output, scanner.Text())
			t.mu.Unlock()
		}
	}()

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			t.mu.Lock()
			t.output = append(t.output, scanner.Text())
			t.mu.Unlock()
		}
	}()

	// Wait for process in background
	go func() {
		err := cmd.Wait()
		t.mu.Lock()
		t.active = false
		t.err = err
		t.mu.Unlock()
	}()

	return t, nil
}

// IsActive returns whether the tunnel subprocess is still running
func (t *Tunnel) IsActive() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.active
}

// Stop kills the tunnel subprocess
func (t *Tunnel) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.cancel != nil {
		t.cancel()
	}
	t.active = false
}

// Output returns collected output lines
func (t *Tunnel) Output() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]string, len(t.output))
	copy(out, t.output)
	return out
}

// Error returns any error from the subprocess
func (t *Tunnel) Error() error {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.err
}
