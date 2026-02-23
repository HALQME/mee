// Package platform provides OS-specific utilities.
package platform

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Open opens a file or URL with the system default application.
func Open(target string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	case "linux":
		cmd = exec.Command("xdg-open", target)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", target)
	default:
		return nil
	}
	return cmd.Start()
}

// ConfigDir returns the config directory.
func ConfigDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return xdg + "/mee"
	}
	home, _ := os.UserHomeDir()
	return home + "/.config/mee"
}

// ExecutableDir returns the directory of the running executable.
func ExecutableDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}
