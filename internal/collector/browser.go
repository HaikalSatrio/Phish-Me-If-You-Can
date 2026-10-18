package collector

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func FindBrowser() string {
	for _, p := range []string{
		filepath.Join(os.Getenv("PROGRAMFILES"), "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(os.Getenv("PROGRAMFILES(X86)"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Google", "Chrome", "Application", "chrome.exe"),
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
	} {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
	}
	for _, name := range []string{"google-chrome", "chromium", "chromium-browser", "msedge"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

type cappedOutput struct{ bytes.Buffer }

func (b *cappedOutput) Write(p []byte) (int, error) {
	if b.Len()+len(p) > 2<<20 {
		return 0, fmt.Errorf("rendered HTML exceeds 2 MiB")
	}
	return b.Buffer.Write(p)
}

func renderGoogle(ctx context.Context, browser, raw string, timeout time.Duration) ([]byte, string, string) {
	u, err := cleanURL(raw)
	if err != nil || u.Scheme != "https" || u.Host != "www.google.com" || u.Path != "/search" {
		return nil, "BLOCKED", "Browser navigation is restricted to Google search"
	}
	if browser == "" {
		return nil, "BLOCKED", "Chrome or Edge was not found. Install a supported browser or use an API provider."
	}
	profile, err := os.MkdirTemp("", "pmiyc-browser-")
	if err != nil {
		return nil, "FAILED", "Could not create isolated browser profile"
	}
	defer os.RemoveAll(profile)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, browser, "--headless", "--disable-gpu", "--no-first-run", "--no-default-browser-check", "--disable-extensions", "--disable-sync", "--disable-background-networking", "--user-data-dir="+profile, "--dump-dom", "--timeout=10000", raw)
	configureBrowser(cmd)
	var output cappedOutput
	cmd.Stdout = &output
	if err := cmd.Run(); err != nil {
		return nil, "FAILED", "Browser rendering failed or timed out"
	}
	if output.Len() == 0 {
		return nil, "FAILED", "Browser returned no HTML"
	}
	return output.Bytes(), "SUCCESS", "Rendered in an isolated local browser profile"
}
