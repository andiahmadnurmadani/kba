package restore

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"kroombox-backup-agent/tui"
)

type BackupInfo struct {
	Path     string
	Date     string
	Size     string
	Services map[string]bool
	Hostname string
}

func RunInteractive() error {
	reader := bufio.NewReader(os.Stdin)

	backups := findBackups()
	if len(backups) == 0 {
		fmt.Println()
		tui.StatusBox("No Backups Found", []string{
			"No backup directories found.",
			"Run 'kba backup' first.",
		})
		return nil
	}

	fmt.Println()
	tui.StatusBox("Restore Wizard", []string{
		"Select a backup to restore.",
		"You can restore all services or choose specific ones.",
	})

	// ── Step 1: Select backup ──
	fmt.Println()
	fmt.Printf("  %sStep 1/3: Select Backup%s\n", tui.Bold, tui.Reset)
	fmt.Println()

	for i, b := range backups {
		svcList := []string{}
		for svc, ok := range b.Services {
			if ok {
				svcList = append(svcList, svc)
			}
		}
		fmt.Printf("  %2d) %s  %10s  %s\n", i+1, b.Date, b.Size, strings.Join(svcList, ", "))
	}
	fmt.Println()
	fmt.Print("  Select backup [1]: ")
	var n int
	fmt.Scanf("%d\n", &n)
	if n < 1 || n > len(backups) {
		n = 1
	}
	selected := backups[n-1]
	fmt.Printf("  Selected: %s (%s)\n", selected.Date, selected.Size)

	// Show detail
	fmt.Println()
	fmt.Printf("  %sBackup Details%s\n", tui.Bold, tui.Reset)
	fmt.Printf("    Date:     %s\n", selected.Date)
	fmt.Printf("    Size:     %s\n", selected.Size)
	fmt.Printf("    Hostname: %s\n", selected.Hostname)
	fmt.Println("    Services:")
	for svc, ok := range selected.Services {
		if ok {
			fmt.Printf("      ✓ %s\n", svc)
		}
	}
	fmt.Println()

	// ── Step 2: Select services ──
	fmt.Println()
	fmt.Printf("  %sStep 2/3: Services to Restore%s\n", tui.Bold, tui.Reset)
	fmt.Println()

	available := []string{}
	for svc, ok := range selected.Services {
		if ok {
			available = append(available, svc)
		}
	}
	sort.Strings(available)

	fmt.Printf("  Available: %s\n", strings.Join(available, ", "))
	fmt.Println("  Enter comma-separated, or 'all' for everything.")
	fmt.Print("  Services [all]: ")
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	var selectedServices []string
	if input == "" || input == "all" {
		selectedServices = available
	} else {
		for _, s := range strings.Split(input, ",") {
			s = strings.TrimSpace(s)
			for _, a := range available {
				if strings.EqualFold(s, a) {
					selectedServices = append(selectedServices, a)
				}
			}
		}
	}
	if len(selectedServices) == 0 {
		fmt.Printf("  %s Invalid, restoring all.%s\n", tui.Yellow, tui.Reset)
		selectedServices = available
	}

	// ── Step 3: Destination ──
	fmt.Println()
	fmt.Printf("  %sStep 3/4: Restore Destination%s\n", tui.Bold, tui.Reset)
	fmt.Println()
	fmt.Println("  1) Restore to original location (e.g. MySQL→mysql, Nginx→/etc/nginx/)")
	fmt.Println("  2) Restore to a custom folder (dry-run / preview)")
	fmt.Println()
	fmt.Print("  Choice [1]: ")
	var destChoice int
	fmt.Scanf("%d\n", &destChoice)

	customDest := ""
	if destChoice == 2 {
		fmt.Println()
		fmt.Print("  Destination folder: ")
		customDest, _ = reader.ReadString('\n')
		customDest = strings.TrimSpace(customDest)
		if customDest == "" {
			customDest = "./restored"
		}
		fmt.Printf("  %s Files will be saved to: %s%s\n", tui.Yellow, customDest, tui.Reset)
		fmt.Println("  (No system changes will be made)")
	} else {
		fmt.Println("  %s Restoring to original locations %s(may need sudo)%s\n", tui.Green, tui.Yellow, tui.Reset)
	}
	fmt.Println()

	// ── Step 4: Confirm ──
	fmt.Println()
	fmt.Printf("  %sStep 4/4: Confirm%s\n", tui.Bold, tui.Reset)
	fmt.Println()
	fmt.Printf("  Backup:    %s\n", selected.Date)
	fmt.Printf("  Source:    %s\n", selected.Path)
	fmt.Printf("  Restore:   %s\n", strings.Join(selectedServices, ", "))
	if destChoice == 2 {
		fmt.Printf("  Dest:      %s (custom folder)\n", customDest)
	} else {
		fmt.Printf("  Dest:      original locations\n")
	}
	fmt.Println()
	fmt.Print("  Start restore? [Y/n]: ")
	confirm, _ := reader.ReadString('\n')
	confirm = strings.TrimSpace(confirm)
	if confirm == "n" || confirm == "N" {
		fmt.Println("  Cancelled.")
		return nil
	}

	// ── Execute ──
	fmt.Println()
	fmt.Println("  Restoring...")
	for _, svc := range selectedServices {
		svcDir := filepath.Join(selected.Path, svc)
		if fi, err := os.Stat(svcDir); err != nil || !fi.IsDir() {
			fmt.Printf("  %s %s: no data%s\n", tui.Yellow, svc, tui.Reset)
			continue
		}
		fmt.Printf("  %s %s: restoring...%s\n", tui.Cyan, svc, tui.Reset)

		if destChoice == 2 {
			// Custom destination: copy files as-is
			svcDest := filepath.Join(customDest, svc)
			copyDir(svcDir, svcDest)
			fmt.Printf("    %s Copied to %s%s\n", tui.Green, svcDest, tui.Reset)
		} else {
			// Original location restore
			switch svc {
			case "mysql":
				restoreMySQL(svcDir)
			case "mongodb":
				restoreMongo(svcDir)
			case "pm2":
				restorePM2(svcDir)
			case "nginx":
				restoreFile("Nginx", svcDir, "/etc/nginx/")
			default:
				restoreFile(svc, svcDir, "./restored_"+svc)
			}
		}
	}

	fmt.Println()
	tui.StatusBox("Restore Complete", []string{
		fmt.Sprintf("Backup: %s", selected.Date),
		fmt.Sprintf("Restored: %s", strings.Join(selectedServices, ", ")),
	})
	return nil
}

func findBackups() []BackupInfo {
	home, _ := os.UserHomeDir()
	searchPaths := []string{
		"/var/backups/kroombox",
		filepath.Join(home, "backup"),
		filepath.Join(home, "backups/kroombox"),
		"./backups",
	}
	var backups []BackupInfo
	seen := map[string]bool{}

	for _, base := range searchPaths {
		if fi, err := os.Stat(base); err != nil || !fi.IsDir() {
			continue
		}
		entries, _ := os.ReadDir(base)
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			date := entry.Name()
			if len(date) != 10 || date[4] != '-' || date[7] != '-' {
				continue
			}
			fullPath := filepath.Join(base, date)
			if seen[fullPath] {
				continue
			}
			seen[fullPath] = true

			info := BackupInfo{
				Path:     fullPath,
				Date:     date,
				Services: map[string]bool{},
			}

			// Read manifest
			mp := filepath.Join(fullPath, "manifest.json")
			if data, err := os.ReadFile(mp); err == nil {
				var m struct {
					Hostname string          `json:"hostname"`
					Size     string          `json:"size"`
					Services map[string]bool `json:"services"`
				}
				if json.Unmarshal(data, &m) == nil {
					info.Hostname = m.Hostname
					info.Size = m.Size
					info.Services = m.Services
				}
			}

			// Fallback: scan dirs
			if len(info.Services) == 0 {
				svcDirs, _ := os.ReadDir(fullPath)
				for _, se := range svcDirs {
					if se.IsDir() {
						info.Services[se.Name()] = true
					}
				}
				var totalSize int64
				filepath.Walk(fullPath, func(p string, fi os.FileInfo, err error) error {
					if err == nil && !fi.IsDir() {
						totalSize += fi.Size()
					}
					return nil
				})
				info.Size = fmt.Sprintf("%.1fMB", float64(totalSize)/1024/1024)
			}
			backups = append(backups, info)
		}
	}
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Date > backups[j].Date
	})
	return backups
}

func restoreMySQL(dir string) {
	filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() || (!strings.HasSuffix(p, ".sql") && !strings.HasSuffix(p, ".sql.gz")) {
			return nil
		}
		fmt.Printf("    Importing: %s\n", filepath.Base(p))
		// Read the SQL file and pipe to mysql
		data, err := os.ReadFile(p)
		if err != nil {
			fmt.Printf("    %s Read error: %s%s\n", tui.Yellow, err, tui.Reset)
			return nil
		}
		cmd := exec.Command("mysql", "--defaults-file="+mysqlCnfPath(), "-f")
		cmd.Stdin = strings.NewReader(string(data))
		if out, err := cmd.CombinedOutput(); err != nil {
			fmt.Printf("    %s Error: %s%s\n", tui.Yellow, strings.TrimSpace(string(out)), tui.Reset)
		} else {
			fmt.Printf("    %s Done%s\n", tui.Green, tui.Reset)
		}
		return nil
	})
}

func restoreMongo(dir string) {
	cmd := exec.Command("mongorestore", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("    %s Error: %s%s\n", tui.Yellow, strings.TrimSpace(string(out)), tui.Reset)
	} else {
		fmt.Printf("    %s Done%s\n", tui.Green, tui.Reset)
	}
}

func restoreFile(name, srcDir, destDir string) {
	entries, _ := os.ReadDir(srcDir)
	for _, entry := range entries {
		src := filepath.Join(srcDir, entry.Name())
		dst := filepath.Join(destDir, entry.Name())
		fmt.Printf("    %s\n", src)
		if strings.HasPrefix(destDir, "/etc/") {
			os.MkdirAll(filepath.Dir(dst), 0755)
			exec.Command("sudo", "cp", "-r", src, dst).Run()
		} else {
			os.MkdirAll(destDir, 0755)
			exec.Command("cp", "-r", src, dst).Run()
		}
	}
	fmt.Printf("    %s Done%s\n", tui.Green, tui.Reset)
}

func restorePM2(dir string) {
	pm2File := filepath.Join(dir, "dump.pm2")
	if _, err := os.Stat(pm2File); err == nil {
		exec.Command("pm2", "start", pm2File).Run()
		fmt.Printf("    %s PM2 restored%s\n", tui.Green, tui.Reset)
	}
}

func copyDir(src, dst string) {
	os.MkdirAll(dst, 0755)
	entries, _ := os.ReadDir(src)
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			copyDir(srcPath, dstPath)
		} else {
			data, err := os.ReadFile(srcPath)
			if err == nil {
				os.WriteFile(dstPath, data, 0644)
			}
		}
	}
}

func mysqlCnfPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".my.cnf")
}
