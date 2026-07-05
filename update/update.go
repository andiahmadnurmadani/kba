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

	fmt.Printf("  Current: v1.0.0")
	if version != "" {
		fmt.Printf(" -> Latest: v%s", version)
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
	installCmd := exec.Command("cp", binaryPath, "/usr/local/bin/kba")
	if out, err := installCmd.CombinedOutput(); err != nil {
		// Try with sudo
		sudoCmd := exec.Command("sudo", "cp", binaryPath, "/usr/local/bin/kba")
		if out2, err2 := sudoCmd.CombinedOutput(); err2 != nil {
			return fmt.Errorf("install failed: %s / %s", string(out), string(out2))
		}
	}
	os.Chmod("/usr/local/bin/kba", 0755)

	os.RemoveAll(tmpDir)

	if version != "" {
		fmt.Printf("  Updated to v%s!\n", version)
	} else {
		fmt.Println("  Update complete!")
	}
	fmt.Println("  Run 'kba version' to confirm.")
	return nil
}
