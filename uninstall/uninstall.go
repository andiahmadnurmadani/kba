package uninstall

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func Run() error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println()
	fmt.Println("  ╔══════════════════════════════════════════╗")
	fmt.Println("  ║   Kroombox Backup Agent — Uninstall      ║")
	fmt.Println("  ╚══════════════════════════════════════════╝")
	fmt.Println()

	// Show what will be removed
	fmt.Println("  The following will be REMOVED:")
	fmt.Println("    • Binary:   /usr/local/bin/kba")
	if runtime.GOOS == "linux" {
		fmt.Println("    • Service:  /etc/systemd/system/kroombox-backup.*")
	}
	fmt.Println("    • Config:   /etc/kroombox/backup-agent/")
	fmt.Println("    • DB:       ~/.kroombox/backup-agent.db")
	fmt.Println("    • Logs:     ./logs/")
	fmt.Println()
	fmt.Println("  The following will be KEPT:")
	fmt.Println("    • Backup data: /var/backups/kroombox/")
	fmt.Println("    • .my.cnf, .pgpass (MySQL/PostgreSQL credentials)")
	fmt.Println()

	// Purge option
	fmt.Print("  Remove ALL dependencies (Go, Git, etc.) too? [y/N]: ")
	purgeDeps, _ := reader.ReadString('\n')
	purgeDeps = strings.TrimSpace(purgeDeps)

	// Purge backups option
	fmt.Print("  Remove ALL backup data too? (--purge) [y/N]: ")
	purgeBackups, _ := reader.ReadString('\n')
	purgeBackups = strings.TrimSpace(purgeBackups)

	fmt.Println()
	fmt.Print("  Continue with uninstall? [y/N]: ")
	confirm, _ := reader.ReadString('\n')
	confirm = strings.TrimSpace(confirm)

	if confirm != "y" && confirm != "Y" {
		fmt.Println("  Cancelled.")
		return nil
	}

	fmt.Println()
	fmt.Println("  Uninstalling...")

	// 1. Stop & remove service
	fmt.Println("  • Removing service...")
	stopService()

	// 2. Remove binary
	fmt.Println("  • Removing binary...")
	os.Remove("/usr/local/bin/kba")

	// 3. Remove config
	fmt.Println("  • Removing config...")
	removePath("/etc/kroombox/backup-agent")

	// 4. Remove DB
	fmt.Println("  • Removing database...")
	home, _ := os.UserHomeDir()
	removePath(filepath.Join(home, ".kroombox"))

	// 5. Remove logs
	fmt.Println("  • Removing logs...")
	os.RemoveAll("logs")
	os.RemoveAll("backups") // project-level test dirs

	// 6. Remove source (if in home)
	srcDir := filepath.Join(home, "kroombox-backup-agent")
	if fi, err := os.Stat(srcDir); err == nil && fi.IsDir() {
		fmt.Println("  • Removing source directory...")
		os.RemoveAll(srcDir)
	}

	// 7. Remove MySQL credentials (optional)
	fmt.Println()
	fmt.Print("  Remove MySQL/PostgreSQL credentials [y/N]: ")
	rmCred, _ := reader.ReadString('\n')
	rmCred = strings.TrimSpace(rmCred)
	if rmCred == "y" || rmCred == "Y" {
		os.Remove(filepath.Join(home, ".my.cnf"))
		os.Remove(filepath.Join(home, ".pgpass"))
		fmt.Println("  • Credentials removed")
	}

	// 8. Remove dependencies
	if purgeDeps == "y" || purgeDeps == "Y" {
		fmt.Println("  • Removing Go...")
		removePath("/usr/local/go")
		fmt.Println("  • Go removed")
	}

	// 9. Remove backup data (only if --purge)
	if purgeBackups == "y" || purgeBackups == "Y" {
		fmt.Println("  • Removing backup data...")
		for _, p := range []string{"/var/backups/kroombox", filepath.Join(home, "backups/kroombox")} {
			removePath(p)
		}
		fmt.Println("  • Backup data removed")
	} else {
		fmt.Println()
		fmt.Println("  Backup data preserved:")
		fmt.Println("    /var/backups/kroombox/")
	}

	fmt.Println()
	fmt.Println("  ✔ KBA uninstalled.")
	fmt.Println("  Run 'sudo rm -rf /var/backups/kroombox' to also remove backup data.")
	return nil
}

func stopService() {
	switch runtime.GOOS {
	case "linux":
		if _, err := exec.LookPath("systemctl"); err == nil {
			exec.Command("sudo", "systemctl", "stop", "kroombox-backup.timer").Run()
			exec.Command("sudo", "systemctl", "disable", "kroombox-backup.timer").Run()
			exec.Command("sudo", "systemctl", "stop", "kroombox-backup.service").Run()
			exec.Command("sudo", "rm", "-f", "/etc/systemd/system/kroombox-backup.service").Run()
			exec.Command("sudo", "rm", "-f", "/etc/systemd/system/kroombox-backup.timer").Run()
			exec.Command("sudo", "systemctl", "daemon-reload").Run()
		}
		// Remove cron entry
		out, _ := exec.Command("crontab", "-l").Output()
		if len(out) > 0 && strings.Contains(string(out), "kba") {
			var newCron []string
			for _, line := range strings.Split(string(out), "\n") {
				if !strings.Contains(line, "kba") {
					newCron = append(newCron, line)
				}
			}
			tmpFile := "/tmp/kba-cron-uninstall"
			os.WriteFile(tmpFile, []byte(strings.Join(newCron, "\n")), 0644)
			exec.Command("crontab", tmpFile).Run()
			os.Remove(tmpFile)
		}
	case "darwin":
		home, _ := os.UserHomeDir()
		plistPath := filepath.Join(home, "Library/LaunchAgents/com.kroombox.backup-agent.plist")
		exec.Command("launchctl", "unload", plistPath).Run()
		os.Remove(plistPath)
	}
}

func removePath(path string) {
	if _, err := os.Stat(path); err == nil {
		if strings.HasPrefix(path, "/etc/") || strings.HasPrefix(path, "/usr/") || strings.HasPrefix(path, "/var/") {
			exec.Command("sudo", "rm", "-rf", path).Run()
		} else {
			os.RemoveAll(path)
		}
	}
}
