package update

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const repoURL = "https://github.com/andiahmadnurmadani/kba.git"
const currentVersion = "1.0.0"

func Run() error {
	tmpDir := filepath.Join(os.TempDir(), "kba-update")

	fmt.Println()
	fmt.Println("  Checking for updates...")

	// Clean up any stale clone
	if err := os.RemoveAll(tmpDir); err != nil {
		// Try with sudo if permission denied
		exec.Command("sudo", "rm", "-rf", tmpDir).Run()
	}

	// Clone
	fmt.Println("  Downloading latest version...")
	os.MkdirAll(tmpDir, 0755)

	cmd := exec.Command("git", "clone", "--depth", "1", repoURL, tmpDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		// Try again with clean dir
		os.RemoveAll(tmpDir)
		cmd = exec.Command("git", "clone", "--depth", "1", repoURL, tmpDir)
		if out2, err2 := cmd.CombinedOutput(); err2 != nil {
			return fmt.Errorf("clone failed: %s", strings.TrimSpace(string(out2)))
		}
		_ = out
	}

	// Get latest version from cloned repo's main.go
	latestVersion := ""
	versionFile := filepath.Join(tmpDir, "main.go")
	if data, err := os.ReadFile(versionFile); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.Contains(line, "version = ") {
				start := strings.Index(line, "\"")
				end := strings.LastIndex(line, "\"")
				if start >= 0 && end > start {
					latestVersion = line[start+1 : end]
				}
			}
		}
	}

	// Check if update needed
	if latestVersion == currentVersion || latestVersion == "" {
		fmt.Printf("  %s KBA v%s - already up to date%s\n", "\033[32m\u2713\033[0m", currentVersion, "\033[0m")
		fmt.Println()
		fmt.Println("  No update available.")
		os.RemoveAll(tmpDir)
		return nil
	}

	fmt.Printf("  %s v%s -> v%s - updating...%s\n", "\033[33m\u2191\033[0m", currentVersion, latestVersion, "\033[0m")
	fmt.Println()

	// Build
	fmt.Println("  Building...")
	buildEnv := append(os.Environ(),
		"GONOSUMCHECK=*",
		"GONOSUMDB=*",
		"GOFLAGS=-mod=mod",
		"GOPROXY=https://proxy.golang.org,direct",
	)

	cmd = exec.Command("go", "build", "-ldflags=-s -w", "-o", "kba", ".")
	cmd.Dir = tmpDir
	cmd.Env = buildEnv
	if out, err := cmd.CombinedOutput(); err != nil {
		// Try with full go path
		for _, gp := range []string{"/usr/local/go/bin/go", "/usr/lib/go/bin/go"} {
			if _, e := os.Stat(gp); e == nil {
				cmd = exec.Command(gp, "build", "-ldflags=-s -w", "-o", "kba", ".")
				cmd.Dir = tmpDir
				cmd.Env = buildEnv
				if out2, err2 := cmd.CombinedOutput(); err2 == nil {
					out = out2
					err = nil
					break
				}
			}
		}
		if err != nil {
			os.RemoveAll(tmpDir)
			return fmt.Errorf("build failed: %s", strings.TrimSpace(string(out)))
		}
	}

	// Install
	fmt.Println("  Installing...")
	binaryPath := filepath.Join(tmpDir, "kba")
	destPath := "/usr/local/bin/kba"

	// Check if we can write to dest
	canWrite := false
	if f, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE, 0755); err == nil {
		f.Close()
		canWrite = true
	} else {
		os.Remove(destPath)
	}

	if !canWrite {
		askpass := os.Getenv("SUDO_ASKPASS")
		sudoPrefix := "sudo"
		if askpass != "" {
			sudoPrefix = "sudo -A"
		}

		// Remove old binary first, then copy
		exec.Command("sh", "-c", sudoPrefix+" rm -f "+destPath).Run()
		exec.Command("sh", "-c", sudoPrefix+" cp "+binaryPath+" "+destPath).Run()
		exec.Command("sh", "-c", sudoPrefix+" chmod +x "+destPath).Run()

		// Verify
		if _, err := os.Stat(destPath); err != nil {
			os.RemoveAll(tmpDir)
			return fmt.Errorf("install failed - try: sudo kba update")
		}
	} else {
		os.Remove(destPath)
		os.Rename(binaryPath, destPath)
		os.Chmod(destPath, 0755)
	}

	os.RemoveAll(tmpDir)

	// Success
	fmt.Println()
	fmt.Println("  \u250c\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u250c")
	fmt.Println("  \u2502  \u2713 KBA updated: v" + currentVersion + " \u2192 v" + latestVersion + "    \u2502")
	fmt.Println("  \u2502  Run 'kba version' to confirm.          \u2502")
	fmt.Println("  \u2514\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2518")
	fmt.Println()
	return nil
}
