package l2tunnel

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DetectPublicIP returns the host's public IP address using ipify.
func DetectPublicIP() (string, error) {
	client := &http.Client{Timeout: 5 * time.Second}

	// Try ipify first
	resp, err := client.Get("https://api.ipify.org")
	if err == nil {
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err == nil && resp.StatusCode == http.StatusOK {
			ip := strings.TrimSpace(string(body))
			if ip != "" {
				return ip, nil
			}
		}
	}

	// Fallback: icanhazip
	resp, err = client.Get("https://icanhazip.com")
	if err != nil {
		return "", fmt.Errorf("failed to detect public IP: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}
	ip := strings.TrimSpace(string(body))
	if ip == "" {
		return "", fmt.Errorf("empty public IP response")
	}
	return ip, nil
}
