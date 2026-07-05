package update

import (
	"fmt"
	"os"
	"os/exec"
	"time"
	"path/filepath"
	"strings"
)

const repoURL = "https://github.com/andiahmadnurmadani/kba.git"

func Run() error {
	fmt.Println()
	fmt.Println("  Checking for updates...")

	goPath := ""
	// Try common Go paths
	for _, p := range []string{"go", "/usr/local/go/bin/go", "/usr/lib/go/bin/go", "/opt/homebrew/bin/go"} {
		if path, err := exec.LookPath(p); err == nil {
			goPath = path
			break
		}
	}
	if goPath == "" {
		return fmt.Errorf("Go not found - install Go first (or run installer)")
	}

	tmpDir := filepath.Join(os.TempDir(), "kba-update")
	os.RemoveAll(tmpDir)

	fmt.Println("  Downloading latest version...")
	cmd := exec.Command("git", "clone", "--depth", "1", repoURL, tmpDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("clone failed: %s", string(out))
	}

	// Get version from main.go
	versionFile := filepath.Join(tmpDir, "main.go")
	var version string
	if data, err := os.ReadFile(versionFile); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.Contains(line, "version = ") {
				// Find text between quotes
				start := strings.Index(line, "\"")
				end := strings.LastIndex(line, "\"")
				if start >= 0 && end > start {
					version = line[start+1 : end]
				}
			}
		}
	}

	if version != "" {
		if version == "1.0.0" {
			fmt.Printf("  %s KBA v%s - up to date%s\n", "\033[32m\u2713\033[0m", version, "\033[0m")
		} else {
			fmt.Printf("  %s KBA v%s -> v%s - updating...%s\n", "\033[33m\u2191\033[0m", "1.0.0", version, "\033[0m")
		}
	} else {
		fmt.Printf("  %s Checking version...%s\n", "\033[36m\u2192\033[0m", "\033[0m")
	}
	fmt.Println()

	// Download & build
	fmt.Println("  Building...")
	buildEnv := append(os.Environ(),
		"GONOSUMCHECK=*",
		"GONOSUMDB=*",
		"GOFLAGS=-mod=mod",
		"GOPROXY=https://proxy.golang.org,direct",
	)

	cmd = exec.Command(goPath, "build", "-ldflags=-s -w", "-o", "kba", ".")
	cmd.Dir = tmpDir
	cmd.Env = buildEnv
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("build failed: %s", string(out))
	}

	// Install
	fmt.Println("  Installing...")
	binaryPath := filepath.Join(tmpDir, "kba")
	destPath := "/usr/local/bin/kba"

	// Kill any running kba backup/schedule, but NOT ourselves
	exec.Command("sudo", "pkill", "-f", "kba backup").Run()
	exec.Command("sudo", "pkill", "-f", "kba schedule").Run()
	// Wait a moment for processes to die
	time.Sleep(500 * time.Millisecond)

	// Try cp, fallback to sudo
	err := os.WriteFile(destPath, nil, 0755)
	canWrite := err == nil
	if canWrite {
		copyErr := exec.Command("cp", binaryPath, destPath).Run()
		if copyErr != nil {
			// Try with sudo
			canWrite = false
		}
	}

	if !canWrite {
		askpass := os.Getenv("SUDO_ASKPASS")
		sudoPrefix := "sudo"
		if askpass != "" { sudoPrefix = "sudo -A" }

		// Remove old binary first (avoids "Text file busy")
		exec.Command("sh", "-c", sudoPrefix+" rm -f "+destPath).Run()
		exec.Command("sh", "-c", sudoPrefix+" cp "+binaryPath+" "+destPath).Run()
		exec.Command("sh", "-c", sudoPrefix+" chmod +x "+destPath).Run()

		// Verify
		if _, err := os.Stat(destPath); err != nil {
			return fmt.Errorf("install failed — try: sudo kba update")
		}
	} else {
		os.Remove(destPath)
		os.Rename(binaryPath, destPath)
		os.Chmod(destPath, 0755)
	}

	os.RemoveAll(tmpDir)

	fmt.Println()
	fmt.Println("  ┌──────────────────────────────────────┐")
	fmt.Println("  │  ✓ Update successful!                 │")
	if version != "" && version != "1.0.0" {
		fmt.Printf("  │  v1.0.0 → v%-18s  │\n", version)
	}
	fmt.Println("  │  Run 'kba version' to confirm.        │")
	fmt.Println("  └──────────────────────────────────────┘")
	fmt.Println()
	return nil
}
