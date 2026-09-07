package gui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// LaunchAppWindow launches the native browser in standalone app mode (--app=...).
// This produces a standalone window without browser tabs or URL bars, behaving
// exactly like a native Total Commander desktop window.
func LaunchAppWindow(url string) (*exec.Cmd, error) {
	tempProfile := filepath.Join(os.TempDir(), "myc_desktop_profile")
	_ = os.MkdirAll(tempProfile, 0o700)

	var candidates []string

	switch runtime.GOOS {
	case "windows":
		candidates = []string{
			filepath.Join(os.Getenv("ProgramFiles(x86)"), "Microsoft", "Edge", "Application", "msedge.exe"),
			filepath.Join(os.Getenv("ProgramFiles"), "Microsoft", "Edge", "Application", "msedge.exe"),
			filepath.Join(os.Getenv("LOCALAPPDATA"), "Microsoft", "Edge", "Application", "msedge.exe"),
			filepath.Join(os.Getenv("ProgramFiles"), "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(os.Getenv("ProgramFiles(x86)"), "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(os.Getenv("LOCALAPPDATA"), "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(os.Getenv("ProgramFiles"), "BraveSoftware", "Brave-Browser", "Application", "brave.exe"),
			filepath.Join(os.Getenv("LOCALAPPDATA"), "BraveSoftware", "Brave-Browser", "Application", "brave.exe"),
			"msedge.exe",
			"chrome.exe",
			"brave.exe",
		}
	case "darwin":
		candidates = []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
		}
	default: // Linux / BSD
		candidates = []string{
			"google-chrome-stable",
			"google-chrome",
			"chromium",
			"chromium-browser",
			"brave-browser",
			"brave",
			"microsoft-edge-stable",
			"microsoft-edge",
		}
	}

	for _, cand := range candidates {
		var path string
		if filepath.IsAbs(cand) {
			if _, err := os.Stat(cand); err == nil {
				path = cand
			}
		} else {
			if p, err := exec.LookPath(cand); err == nil {
				path = p
			}
		}

		if path != "" {
			cmd := exec.Command(path,
				fmt.Sprintf("--app=%s", url),
				fmt.Sprintf("--user-data-dir=%s", tempProfile),
				"--window-size=1240,840",
				"--no-first-run",
				"--no-default-browser-check",
				"--disable-sync",
				"--disable-extensions",
			)
			cmd.SysProcAttr = getSysProcAttr()
			if err := cmd.Start(); err == nil {
				return cmd, nil
			}
		}
	}

	// Fallback to default system browser if no Chromium-based app mode is found
	var fallbackCmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		fallbackCmd = exec.Command("cmd.exe", "/c", "start", url)
	case "darwin":
		fallbackCmd = exec.Command("open", url)
	default:
		fallbackCmd = exec.Command("xdg-open", url)
	}

	fallbackCmd.SysProcAttr = getSysProcAttr()
	if err := fallbackCmd.Start(); err != nil {
		return nil, fmt.Errorf("nie udało się otworzyć okna przeglądarki dla GUI: %w", err)
	}
	return fallbackCmd, nil
}
