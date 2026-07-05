package update

import (

	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const repoURL = "https://github.com/andiahmadnurmadani/kba.git"

func Run() error {
	tmpDir := filepath.Join(os.TempDir(), "kba-update")

	fmt.Println()
	fmt.Println("  Checking for updates...")

	// Clean up any stale clone
	os.RemoveAll(tmpDir)
	exec.Command("sudo", "rm", "-rf", tmpDir).Run()

	// Clone
	fmt.Println("  Downloading latest version...")
	os.MkdirAll(tmpDir, 0755)

	cmd := exec.Command("git", "clone", "--depth", "1", repoURL, tmpDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(tmpDir)
		return fmt.Errorf("clone failed: %s", strings.TrimSpace(string(out)))
	}

	// Get HEAD commit hash from cloned repo
	cmd = exec.Command("git", "-C", tmpDir, "rev-parse", "--short", "HEAD")
	remoteHash, _ := cmd.Output()
	remoteHashStr := strings.TrimSpace(string(remoteHash))

	// Get local commit hash (if available)
	localHashStr := ""
	if localHash, err := exec.Command("git", "-C", filepath.Dir(os.Args[0]), "rev-parse", "--short", "HEAD").Output(); err == nil {
		localHashStr = strings.TrimSpace(string(localHash))
	}
	if localHashStr == "" {
		// Try the source directory
		if localHash, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output(); err == nil {
			localHashStr = strings.TrimSpace(string(localHash))
		}
	}

	// Also get commit message
	cmd = exec.Command("git", "-C", tmpDir, "log", "--oneline", "-1")
	msg, _ := cmd.Output()
	msgStr := strings.TrimSpace(string(msg))

	// Get version string from cloned main.go
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

	// Check if update needed (compare commit hashes)
	if localHashStr != "" && localHashStr == remoteHashStr {
		fmt.Printf("  %s KBA v%s - already up to date (commit %s)%s\n", "\033[32m\u2713\033[0m", latestVersion, localHashStr, "\033[0m")
		fmt.Println()
		fmt.Println("  Your version matches the latest commit.")
		os.RemoveAll(tmpDir)
		return nil
	}

	// Show what's new
	fmt.Println()
	fmt.Printf("  %s Update available!%s\n", "\033[33m\u2191\033[0m", "\033[0m")
	if localHashStr != "" && len(localHashStr) >= 6 {
		fmt.Printf("  Commit: %s \u2192 %s\n", localHashStr[:6], remoteHashStr[:6])
	} else {
		// Installed via script - no local git history
		shortRemote := remoteHashStr
		if len(shortRemote) > 6 { shortRemote = shortRemote[:6] }
		fmt.Printf("  Commit: (installed via script) \u2192 %s\n", shortRemote)
	}
	if msgStr != "" {
		fmt.Printf("  Latest: %s\n", msgStr)
	}
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
		for _, gp := range []string{"/usr/local/go/bin/go", "/usr/lib/go/bin/go", "/opt/homebrew/bin/go", "/opt/homebrew/opt/go/libexec/bin/go"} {
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

		exec.Command("sh", "-c", sudoPrefix+" rm -f "+destPath).Run()
		exec.Command("sh", "-c", sudoPrefix+" cp "+binaryPath+" "+destPath).Run()
		exec.Command("sh", "-c", sudoPrefix+" chmod +x "+destPath).Run()

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
	boxWidth := 44
	title := fmt.Sprintf("\u2713 KBA updated!")
	if localHashStr != "" && len(localHashStr) >= 6 {
		title = fmt.Sprintf("\u2713 KBA updated: %s \u2192 %s", localHashStr[:6], remoteHashStr[:6])
	} else if len(remoteHashStr) >= 6 {
		title = fmt.Sprintf("\u2713 KBA updated (commit %s)", remoteHashStr[:6])
	}
	if latestVersion != "" {
		title = fmt.Sprintf("\u2713 KBA v%s updated", latestVersion)
	}
	padding := boxWidth - len(title) - 2
	if padding < 0 { padding = 0 }
	fmt.Printf("  \u250c%s\u2510\n", strings.Repeat("\u2500", boxWidth))
	fmt.Printf("  \u2502  %s%s \u2502\n", title, strings.Repeat(" ", padding))
	fmt.Printf("  \u2502  Run 'kba version' to confirm.     \u2502\n")
	fmt.Printf("  \u2514%s\u2518\n", strings.Repeat("\u2500", boxWidth))
	fmt.Println()
	return nil
}
