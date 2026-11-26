package config

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os/exec"
	"runtime"
	"time"
)

const browserOpenTimeout = 5 * time.Second

// BrowserURL converts a bound TCP listen address into a localhost HTTP URL.
func BrowserURL(listenAddr string) string {
	host, port, err := net.SplitHostPort(listenAddr)
	if err != nil {
		return "http://" + listenAddr
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port)
}

// OpenBrowser opens the given http/https URL in the default OS browser.
func OpenBrowser(ctx context.Context, rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parse browser url %q: %w", rawURL, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("unsupported browser url scheme %q", parsed.Scheme)
	}

	var bin string
	switch runtime.GOOS {
	case "darwin":
		bin = "open"
	case "linux":
		bin = "xdg-open"
	default:
		return fmt.Errorf("browser auto-open is unsupported on %s", runtime.GOOS)
	}

	openCtx, cancel := context.WithTimeout(ctx, browserOpenTimeout)
	defer cancel()

	cmd := exec.CommandContext(openCtx, bin, parsed.String()) //nolint:gosec // scheme is validated as http/https above
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run %s: %w", bin, err)
	}
	return nil
}
