package main

import (
	"flag"
	"fmt"
	"os"
	"time"
	"os/exec"
	"path/filepath"

	"kroombox-backup-agent/backup"
	"kroombox-backup-agent/config"
	"kroombox-backup-agent/db"
	"kroombox-backup-agent/detect"
	"kroombox-backup-agent/logs"
	"kroombox-backup-agent/modules"
	"strings"

	"kroombox-backup-agent/restore"
	"kroombox-backup-agent/setup"
	"kroombox-backup-agent/update"
	"encoding/json"
	"kroombox-backup-agent/scheduler"
	"kroombox-backup-agent/tui"
	"kroombox-backup-agent/uninstall"
)

const version = "1.0.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]

	switch cmd {
	case "backup":
		cmdBackup()
	case "restore":
		cmdRestore()
	case "status":
		cmdStatus()
	case "detect":
		cmdDetect()
	case "schedule":
		cmdSchedule()
	case "uninstall":
		cmdUninstall()
	case "setup":
		cmdSetup()
	case "update":
		cmdUpdate()
	case "logs":
		cmdLogs()
	case "version":
		fmt.Printf("Kroombox Backup Agent v%s\n", version)
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Kroombox Backup Agent (KBA) v" + version)
	fmt.Println("Usage:")
	fmt.Println("  kba backup             Run full backup")
	fmt.Println("  kba backup --config FILE  Backup with custom config")
	fmt.Println("  kba restore <source>   Restore from backup directory")
	fmt.Println("  kba status             Show agent status")
	fmt.Println("  kba detect             Detect available services")
	fmt.Println("  kba schedule           Interactive backup schedule wizard")
	fmt.Println("  kba schedule --list    Show current schedule")
	fmt.Println("  kba schedule --remove  Remove current schedule")
	fmt.Println("  kba schedule --cleanup [days]  Delete backups older than N days (default 7)")
	fmt.Println("  kba schedule --daily [HH:MM]  Quick daily schedule (default WIB)")
	fmt.Println("  kba schedule --cron 'EXPR'    Quick custom cron")
	fmt.Println("  kba uninstall           Remove KBA (keeps backups)")
	fmt.Println("  kba setup              Interactive credential setup")
	fmt.Println("  kba update             Update to latest version")
	fmt.Println("  kba logs               Show recent backup logs")
	fmt.Println("  kba version            Show version")
}
func findConfig() string {
	paths := []string{
		"./config.yaml",
		"./config.yml",
		"/etc/kroombox/backup-agent/config.yaml",
		"/etc/kroombox/backup-agent/config.yml",
		filepath.Join(os.Getenv("HOME"), ".kroombox", "backup-agent.yaml"),
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func cmdBackup() {
	db.Init()
	defer db.Close()
	// Check if setup has been completed
	if !setup.IsConfigured() {
		setup.CheckAndWarn()
		fmt.Println("  Run 'kba setup' to configure first, then run 'kba backup' again.")
		os.Exit(1)
	}
	// Parse --config flag from remaining args
	cfgPath := ""
	if len(os.Args) > 2 {
		fs := flag.NewFlagSet("backup", flag.ExitOnError)
		fs.StringVar(&cfgPath, "config", "", "path to config file")
		fs.Parse(os.Args[2:])
	}

	if cfgPath == "" {
		cfgPath = findConfig()
	}

	if cfgPath == "" {
		cfg := config.DefaultConfig()
		// Check DB for destination
		dbDest, _ := db.GetConfig("destination")
		if dbDest != "" {
			cfg.Storage.Destination = dbDest
		}
		logs.Init("logs")
		report := backup.Run(cfg)
		logs.Close()
		printReport(report)
		return
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Check DB for destination override (from kba setup)
	dbDest, _ := db.GetConfig("destination")
	if dbDest != "" {
		cfg.Storage.Destination = dbDest
	} else if cfg.Storage.Destination == "" {
		cfg.Storage.Destination = "/var/backups/kroombox"
	}

	logs.Init("logs")
	report := backup.Run(cfg)
	logs.Close()
	printReport(report)
}

func cmdSchedule() {
	db.Init()
	defer db.Close()	// Cleanup mode
	if len(os.Args) > 2 && os.Args[2] == "--cleanup" {
		days := 7
		if len(os.Args) > 3 {
			fmt.Sscanf(os.Args[3], "%d", &days)
		}
		if days < 1 { days = 1 }
		fmt.Printf("\nCleaning backups older than %d days...\n", days)
		if err := scheduler.CleanupOldBackups(days, ""); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Remove mode
	if len(os.Args) > 2 && (os.Args[2] == "--remove" || os.Args[2] == "--delete") {
		fmt.Print("Remove current schedule? [y/N]: ")
		var confirm string
		fmt.Scanf("%s\n", &confirm)
		if confirm == "y" || confirm == "Y" {
			if err := scheduler.RemoveSchedule(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		} else {
			fmt.Println("Cancelled.")
		}
		return
	}

	if len(os.Args) > 2 && (os.Args[2] == "--list" || os.Args[2] == "--status") {
		scheduler.Describe()
		return
	}
	// Check for quick flags
	if len(os.Args) > 2 && os.Args[2] == "--cron" {
		// Quick cron mode: kba schedule --cron "0 3 * * *"
		var cronExpr string
		if len(os.Args) > 3 {
			cronExpr = os.Args[3]
		} else {
			cronExpr = "0 3 * * *"
		}
		scfg := scheduler.ParseFlags(3, 0, "custom", "", "")
		scfg.CronExpr = cronExpr
		kbaPath, _ := exec.LookPath("kba")
		if kbaPath == "" { kbaPath = "/usr/local/bin/kba" }
		if err := scheduler.Install(scfg, kbaPath); err != nil {
			fmt.Fprintf(os.Stderr, "Install failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("\n✓ Schedule installed!")
		return
	}

	if len(os.Args) > 2 && os.Args[2] == "--daily" {
		hour, minute := 3, 0
		tz := "Asia/Jakarta"
		if len(os.Args) > 3 {
			if strings.Contains(os.Args[3], ":") {
				fmt.Sscanf(os.Args[3], "%d:%d", &hour, &minute)
				if len(os.Args) > 4 && os.Args[4] == "--tz" && len(os.Args) > 5 {
					tz = os.Args[5]
				}
			} else if os.Args[3] == "--tz" && len(os.Args) > 4 {
				tz = os.Args[4]
			}
		}
		scfg := scheduler.ParseFlagsFull(hour, minute, "daily", "", "", tz)
		kbaPath, _ := exec.LookPath("kba")
		if kbaPath == "" { kbaPath = "/usr/local/bin/kba" }
		if err := scheduler.Install(scfg, kbaPath); err != nil {
			fmt.Fprintf(os.Stderr, "Install failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("\n✓ Daily schedule installed! (%02d:%02d %s)\n", hour, minute, tz)
		return
	}

	// Interactive mode
	scfg, err := scheduler.RunWizard()
	if err != nil {
		if err.Error() == "cancelled" {
			fmt.Println("\nSchedule cancelled.")
			return
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	kbaPath, _ := exec.LookPath("kba")
	if kbaPath == "" { kbaPath = "/usr/local/bin/kba" }
	if err := scheduler.Install(scfg, kbaPath); err != nil {
		fmt.Fprintf(os.Stderr, "Install failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("\n✓ Schedule installed! Backup will run automatically.")
	scheduler.Describe()
}

func cmdSetup() {
	db.Init()
	defer db.Close()
	if err := setup.RunWizard(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func cmdUninstall() {
	if err := uninstall.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func cmdUpdate() {
	if err := update.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func cmdLogs() {
	db.Init()
	defer db.Close()
	logDir := "logs"
	backupDir := "/home/minibox/backups/kroombox"
	
	// Try to find log directory
	for _, p := range []string{"logs", "/var/log/kroombox", filepath.Join(os.Getenv("HOME"), "kroombox-backup-agent", "logs")} {
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			logDir = p
			break
		}
	}
	
	fmt.Println("=== Recent Backup Logs ===")
	
	// List log files
	files, err := os.ReadDir(logDir)
	if err != nil {
		fmt.Printf("No log directory found at %s\n", logDir)
	} else {
		// Show last 3 log files
		count := 0
		for i := len(files) - 1; i >= 0 && count < 3; i-- {
			if !files[i].IsDir() && strings.HasPrefix(files[i].Name(), "backup_") {
				fi, _ := os.Stat(filepath.Join(logDir, files[i].Name()))
				if fi != nil {
					fmt.Printf("\n%s %s [%s]:\n", "\u2502", files[i].Name(), timeToWIB(fi.ModTime()).Format("Jan 02 15:04"))
					data, err := os.ReadFile(filepath.Join(logDir, files[i].Name()))
					if err == nil {
						lines := strings.Split(string(data), "\n")
						for _, line := range lines {
							if len(line) > 0 {
								fmt.Printf("  %s\n", line)
							}
						}
					}
					count++
				}
			}
		}
		if count == 0 {
			fmt.Println("  No backup logs found.")
		}
	}
	
	// Show recent backup directories
	fmt.Println("\n=== Recent Backups ===")
	if fi, err := os.Stat(backupDir); err == nil && fi.IsDir() {
		entries, _ := os.ReadDir(backupDir)
		for i := len(entries) - 1; i >= 0; i-- {
			manifestPath := filepath.Join(backupDir, entries[i].Name(), "manifest.json")
			if data, err := os.ReadFile(manifestPath); err == nil {
				fmt.Printf("  %s/\n", entries[i].Name())
				for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
					fmt.Printf("    %s\n", line)
				}
				fmt.Println()
			}
		}
	} else {
		fmt.Println("  No backup directories found.")
	}
}

func timeToWIB(t time.Time) time.Time {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return t
	}
	return t.In(loc)
}

func printReport(r *backup.BackupReport) {
	fmt.Println()
	fmt.Println("=== Backup Report ===")
	fmt.Printf("Success: %v\n", r.Success)
	fmt.Printf("Date: %s\n", r.Date)
	fmt.Printf("Path: %s\n", r.Path)
	if r.Error != "" {
		fmt.Printf("Error: %s\n", r.Error)
	}
	fmt.Println("Services:")
	for _, res := range r.Results {
		status := "SKIPPED"
		if res.Success {
			status = "OK"
		} else if !res.Skipped {
			status = "FAILED"
		}
		size := modules.FormatSize(res.Size)
		fmt.Printf("  - %s: %s (%s)\n", res.Name, status, size)
	}
	fmt.Printf("Total size: %s\n", modules.FormatSize(r.TotalSize))
}

func cmdRestore() {
	if err := restore.RunInteractive(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
func cmdStatus() {
	d := detect.Detect()

	fmt.Println()
	tui.StatusBox("System Info", []string{
		fmt.Sprintf("Hostname: %s%s%s", tui.Bold, d.Hostname, tui.Reset),
		fmt.Sprintf("OS:       %s  %s", d.OS, d.Kernel),
	})

	// Services detected
	fmt.Printf("  %sDetected Services:%s\n", tui.Bold, tui.Reset)
	rows := [][]string{}
	mods := modules.GetModules()
	for _, mod := range mods {
		detected := mod.Detect()
		icon := "\u2717"
		color := tui.Red
		if detected {
			icon = "\u2713"
			color = tui.Green
		}
		// Find version from detect result
		ver := ""
		for _, s := range d.Services {
			if string(s.Service) == mod.Name() && s.Present {
				ver = s.Version
				if len(ver) > 25 { ver = ver[:24] + "\u2026" }
			}
		}
		rows = append(rows, []string{
			fmt.Sprintf("%s%s %s%s", color, icon, mod.Name(), tui.Reset),
			ver,
		})
	}
	tui.Table([]string{"Service", "Version"}, rows)

	// Last backup info
	backupDirs := []string{"/var/backups/kroombox", "/home/minibox/backups/kroombox", "./backups"}
	lastBackup := ""
	for _, bd := range backupDirs {
		if fi, err := os.Stat(bd); err == nil && fi.IsDir() {
			entries, _ := os.ReadDir(bd)
			if len(entries) > 0 { lastBackup = entries[len(entries)-1].Name() }
		}
	}

	if lastBackup != "" {
		// Find backup directory with content (latest)
		manifestPath := ""
		for _, bd := range backupDirs {
			fullDir := filepath.Join(bd, lastBackup)
			if fi, err := os.Stat(fullDir); err == nil && fi.IsDir() {
				// Check if this dir has entries
				entries, _ := os.ReadDir(fullDir)
				if len(entries) > 0 {
					p := filepath.Join(fullDir, "manifest.json")
					if _, err := os.Stat(p); err == nil { manifestPath = p; break }
				}
			}
		}
		if manifestPath != "" {
			data, _ := os.ReadFile(manifestPath)
			type Manifest struct {
				Hostname   string          `json:"hostname"`
				OS         string          `json:"os"`
				BackupDate string          `json:"backup_date"`
				Services   map[string]bool `json:"services"`
				Size       string          `json:"size"`
			}
			var m Manifest
			json.Unmarshal(data, &m)
			
			lines := []string{
				fmt.Sprintf("Date:     %s", m.BackupDate),
				fmt.Sprintf("Size:     %s", m.Size),
			}
			// Which services were backed up
			svcList := []string{}
			for svc, ok := range m.Services {
				if ok { svcList = append(svcList, svc) }
			}
			if len(svcList) > 0 {
				lines = append(lines, fmt.Sprintf("Backed up: %s", strings.Join(svcList, ", ")))
			}
			tui.StatusBox("Last Backup ("+lastBackup+")", lines)
		}
	} else {
		fmt.Println()
		fmt.Printf("  %sLast backup:%s %snever%s\n", tui.Bold, tui.Reset, tui.Dim, tui.Reset)
	}

	// Schedule info
	scheduler.Describe()
}

func cmdDetect() {
	d := detect.Detect()
	fmt.Printf("Hostname: %s\n", d.Hostname)
	fmt.Printf("OS: %s\n", d.OS)
	fmt.Printf("Kernel: %s\n", d.Kernel)
	fmt.Printf("Arch: %s\n", d.Arch)
	fmt.Println()
	fmt.Println("Services:")
	for _, s := range d.Services {
		mark := "\u2713"
		if !s.Present {
			mark = "\u2717"
		}
		ver := s.Version
		if ver != "" {
			ver = " (" + ver + ")"
		}
		fmt.Printf("  %s %s%s\n", mark, s.Service, ver)
	}
	fmt.Println()
	fmt.Println("Verbose check:")
	for _, mod := range modules.GetModules() {
		if dv, ok := mod.(modules.BackupModule); ok {
			info := dv.DetectVerbose()
			fmt.Printf("  %s:\n", mod.Name())
			for _, line := range info {
				if strings.Contains(line, "NEEDS") || strings.Contains(line, "failed") || strings.Contains(line, "✗") {
					fmt.Printf("    \033[33m%s\033[0m\n", line)
				} else if strings.Contains(line, "✓") {
					fmt.Printf("    \033[32m%s\033[0m\n", line)
				} else {
					fmt.Printf("    %s\n", line)
				}
			}
		}
	}
}
