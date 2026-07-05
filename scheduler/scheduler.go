package scheduler

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
	"kroombox-backup-agent/db"
	"kroombox-backup-agent/modules"
	"kroombox-backup-agent/tui"
	"strings"
)

type ScheduleConfig struct {
	Frequency string // daily, weekly, monthly, custom
	Hour      int
	Minute    int
	Weekday   int  // 0=Sunday, 1=Monday... (for weekly)
	MonthDay  int  // 1-31 (for monthly)
	CronExpr  string // custom cron expression
	Services  []string // which services to backup (empty = all)
	DestPath  string // backup destination
	Timezone  string // timezone (e.g. Asia/Jakarta, UTC)
}

func ParseFlags(hour, minute int, freq, dest, services string) *ScheduleConfig {
	cfg := DefaultSchedule()
	if freq != "" { cfg.Frequency = freq }
	if hour >= 0 && hour <= 23 { cfg.Hour = hour }
	if minute >= 0 && minute <= 59 { cfg.Minute = minute }
	if dest != "" { cfg.DestPath = dest }
	if services != "" { cfg.Services = strings.Split(services, ",") }
	cfg.CronExpr = buildCron(cfg)
	return cfg
}

func ParseFlagsFull(hour, minute int, freq, dest, services, tz string) *ScheduleConfig {
	cfg := ParseFlags(hour, minute, freq, dest, services)
	if tz != "" { cfg.Timezone = tz }
	return cfg
}

func DefaultSchedule() *ScheduleConfig {
	return &ScheduleConfig{
		Frequency: "daily",
		Hour:      3,
		Minute:    0,
		Weekday:   0,
		MonthDay:  1,
		CronExpr:  "0 3 * * *",
		DestPath:  "/var/backups/kroombox",
		Timezone:  "Asia/Jakarta",
	}
}

// getTimezone returns the local timezone name
func getTimezone() string {
	out, err := exec.Command("date", "+%Z").Output()
	if err != nil {
		return "Asia/Jakarta"
	}
	tz := strings.TrimSpace(string(out))
	if tz == "" || tz == "UTC" {
		return "Asia/Jakarta"
	}
	return tz
}

// RunWizard runs the interactive TUI
func RunWizard() (*ScheduleConfig, error) {
	cfg := DefaultSchedule()

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════╗")
	fmt.Println("║   Kroombox Backup Schedule Wizard        ║")
	fmt.Println("╚══════════════════════════════════════════╝")
	fmt.Println()

	// 1. Frequency - interactive
	freqOpts := []tui.SelectOption{
		{"Daily      (every day at specified time)", "daily"},
		{"Weekly     (every specific day of week)", "weekly"},
		{"Monthly    (every specific day of month)", "monthly"},
		{"Custom     (custom cron expression)", "custom"},
	}
	freqVal, err := tui.Select("Backup Frequency", freqOpts)
	if err == nil {
		cfg.Frequency = freqVal
	}

	// 2. Time
	fmt.Print("Hour (0-23, default=3): ")
	var h int
	if _, err := fmt.Scanf("%d\n", &h); err == nil && h >= 0 && h <= 23 {
		cfg.Hour = h
	}
	fmt.Print("Minute (0-59, default=0): ")
	var m int
	if _, err := fmt.Scanf("%d\n", &m); err == nil && m >= 0 && m <= 59 {
		cfg.Minute = m
	}

	// 3. Day selection
	switch cfg.Frequency {
	case "weekly":
		dayOpts := []tui.SelectOption{
			{"Sunday", "0"}, {"Monday", "1"}, {"Tuesday", "2"}, {"Wednesday", "3"},
			{"Thursday", "4"}, {"Friday", "5"}, {"Saturday", "6"},
		}
		dayVal, _ := tui.Select("Select day of week", dayOpts)
		cfg.Weekday, _ = fmt.Sscanf(dayVal, "%d", &cfg.Weekday)
		if cfg.Weekday < 0 || cfg.Weekday > 6 { cfg.Weekday = 0 }
	case "monthly":
		fmt.Print("Day of month (1-31, default=1): ")
		var md int
		if _, err := fmt.Scanf("%d\n", &md); err == nil && md >= 1 && md <= 31 {
			cfg.MonthDay = md
		}
	case "custom":
		fmt.Println()
		fmt.Println("Enter cron expression (min hour dom month dow)")
		fmt.Println("  Examples:")
		fmt.Println("   0 3 * * *     = daily at 3:00 AM")
		fmt.Println("   0 3 * * 0     = every Sunday at 3:00 AM")
		fmt.Println("   0 3 1 * *     = 1st of month at 3:00 AM")
		fmt.Println("   */30 * * * *  = every 30 minutes")
		fmt.Print("Expression [default=0 3 * * *]: ")
		cfg.CronExpr = readLine()
		if cfg.CronExpr == "" {
			cfg.CronExpr = fmt.Sprintf("%d %d * * *", cfg.Minute, cfg.Hour)
		}
	}

	// Build cron expression
	if cfg.Frequency != "custom" {
		cfg.CronExpr = buildCron(cfg)
	}

	// 4. Destination
	fmt.Println()
	fmt.Printf("Backup destination [default: %s]: ", cfg.DestPath)
	dest := readLine()
	if dest != "" {
		cfg.DestPath = dest
	}

	// 5. Timezone
	fmt.Println()
	currentTz := getTimezone()
	fmt.Printf("Timezone [%s]: ", currentTz)
	fmt.Println("  Common: Asia/Jakarta (WIB), Asia/Makassar (WITA), Asia/Jayapura (WIT)")
	fmt.Println("  Others: UTC, Asia/Singapore, etc.  [enter=keep, type new to change]")
	fmt.Printf("Timezone [%s]: ", currentTz)
	tzInput := readLine()
	if tzInput == "" {
		cfg.Timezone = currentTz
	} else {
		cfg.Timezone = tzInput
	}

	// 6. Services
	fmt.Println()
	fmt.Println("Select services to backup (comma separated, empty=all):")
	fmt.Println("  Available: mysql, postgres, mongodb, pm2, nginx, ssl, git, cron, docker")
	fmt.Println("  Example: mysql,mongodb,nginx")
	fmt.Print("Services [default=all]: ")
	svc := readLine()
	if svc != "" {
		cfg.Services = strings.Split(strings.ReplaceAll(svc, " ", ""), ",")
	}

	// 6. Summary
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════╗")
	fmt.Println("║   Schedule Summary                        ║")
	fmt.Println("╚══════════════════════════════════════════╝")
	fmt.Printf("  Frequency:  %s\n", cfg.Frequency)
	fmt.Printf("  Time:       %02d:%02d\n", cfg.Hour, cfg.Minute)
	if cfg.Frequency == "weekly" {
		days := []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
		fmt.Printf("  Day:        %s\n", days[cfg.Weekday])
	} else if cfg.Frequency == "monthly" {
		fmt.Printf("  Day:        %d\n", cfg.MonthDay)
	}
	fmt.Printf("  Cron:       %s\n", cfg.CronExpr)
	fmt.Printf("  Timezone:   %s\n", cfg.Timezone)
	fmt.Printf("  Destination: %s\n", cfg.DestPath)
	if len(cfg.Services) > 0 {
		fmt.Printf("  Services:   %s\n", strings.Join(cfg.Services, ", "))
	} else {
		fmt.Printf("  Services:   all\n")
	}
	fmt.Println()

	fmt.Print("Apply this schedule? [Y/n]: ")
	confirm := readLine()
	if confirm == "n" || confirm == "N" {
		return nil, fmt.Errorf("cancelled")
	}

	return cfg, nil
}

var reader = bufio.NewReader(os.Stdin)

func readLine() string {
	s, _ := reader.ReadString('\n')
	return strings.TrimRight(s, "\r\n")
}

func buildCron(cfg *ScheduleConfig) string {
	switch cfg.Frequency {
	case "daily":
		return fmt.Sprintf("%d %d * * *", cfg.Minute, cfg.Hour)
	case "weekly":
		return fmt.Sprintf("%d %d * * %d", cfg.Minute, cfg.Hour, cfg.Weekday)
	case "monthly":
		return fmt.Sprintf("%d %d %d * *", cfg.Minute, cfg.Hour, cfg.MonthDay)
	}
	return "0 3 * * *"
}

// Install installs the schedule
func Install(cfg *ScheduleConfig, kbaPath string) error {
	osType := runtime.GOOS

	fmt.Println()
	fmt.Println("Installing schedule...")

	switch osType {
	case "linux":
		return installLinux(cfg, kbaPath)
	case "darwin":
		return installMacOS(cfg, kbaPath)
	default:
		return installCron(cfg, kbaPath)
	}
}

func installLinux(cfg *ScheduleConfig, kbaPath string) error {
	// Try systemd timer first
	if _, err := exec.LookPath("systemctl"); err == nil {
		return installSystemdTimer(cfg, kbaPath)
	}
	// Fallback to cron
	return installCron(cfg, kbaPath)
}

func installMacOS(cfg *ScheduleConfig, kbaPath string) error {
	// Try launchctl
	if _, err := exec.LookPath("launchctl"); err == nil {
		return installLaunchd(cfg, kbaPath)
	}
	return installCron(cfg, kbaPath)
}

func installSystemdTimer(cfg *ScheduleConfig, kbaPath string) error {
	user := os.Getenv("USER")

	svcContent := fmt.Sprintf(`[Unit]
Description=Kroombox Backup Agent (scheduled)
After=network.target

[Service]
Type=oneshot
ExecStart=%s backup
User=%s
Nice=19
IOSchedulingClass=idle
`, kbaPath, user)

	svcPath := "/etc/systemd/system/kroombox-backup.service"
	if err := writeFileSudo(svcPath, svcContent); err != nil {
		return fmt.Errorf("write service: %w", err)
	}
	fmt.Printf("  \u2713 Created: %s\n", svcPath)

	// Build timer expression with configured timezone
	tz := cfg.Timezone
	if tz == "" { tz = getTimezone() }
	var timerExpr string
	switch cfg.Frequency {
	case "daily":
		timerExpr = fmt.Sprintf("OnCalendar=*-*-* %02d:%02d:00 %s\nPersistent=true", cfg.Hour, cfg.Minute, tz)
	case "weekly":
		days := []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
		timerExpr = fmt.Sprintf("OnCalendar=%s *-*-* %02d:%02d:00 %s\nPersistent=true", days[cfg.Weekday], cfg.Hour, cfg.Minute, tz)
	case "monthly":
		timerExpr = fmt.Sprintf("OnCalendar=*-*-%02d %02d:%02d:00 %s\nPersistent=true", cfg.MonthDay, cfg.Hour, cfg.Minute, tz)
	case "custom":
		parts := strings.Fields(cfg.CronExpr)
		if len(parts) >= 5 {
			min := parts[0]; if min == "*" { min = "0" }
			hr  := parts[1]; if hr == "*" { hr = "0" }
			day := parts[2]; if day == "*" { day = "*" }
			mon := parts[3]; if mon == "*" { mon = "*" }
			dow := parts[4]; if dow == "*" { dow = "*-*-*" } else { dow = mapDow(dow) }
			timerExpr = fmt.Sprintf("OnCalendar=%s %s-%s-%s %s:%s:00 %s\nPersistent=true", dow, mon, day, hr, min, tz)
		} else {
			timerExpr = fmt.Sprintf("OnCalendar=*-*-* %02d:%02d:00 %s\nPersistent=true", cfg.Hour, cfg.Minute, tz)
		}
	}

	timerContent := fmt.Sprintf(`[Unit]
Description=Kroombox Backup Timer (%s)

[Timer]
%s
RandomizedDelaySec=30m

[Install]
WantedBy=timers.target
`, cfg.Frequency, timerExpr)

	timerPath := "/etc/systemd/system/kroombox-backup.timer"
	if err := writeFileSudo(timerPath, timerContent); err != nil {
		return fmt.Errorf("write timer: %w", err)
	}
	fmt.Printf("  \u2713 Created: %s\n", timerPath)

	cmds := []string{
		"systemctl daemon-reload",
		"systemctl enable kroombox-backup.timer",
		"systemctl start kroombox-backup.timer",
	}
	for _, c := range cmds {
		exec.Command("sudo", "sh", "-c", c).Run()
	}

	out, _ := exec.Command("systemctl", "list-timers", "--no-pager").Output()
	fmt.Println()
	fmt.Println("Active timers:")
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "kroombox") {
			fmt.Printf("  %s\n", line)
		}
	}

	return nil
}

func installLaunchd(cfg *ScheduleConfig, kbaPath string) error {
	home, _ := os.UserHomeDir()
	label := "com.kroombox.backup-agent"

	// Hourly/daily interval
	plistContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>%s</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>backup</string>
    </array>
    <key>StartCalendarInterval</key>
    <dict>
        <key>Hour</key>
        <integer>%d</integer>
        <key>Minute</key>
        <integer>%d</integer>
    </dict>
    <key>Nice</key>
    <integer>19</integer>
    <key>LowPriorityIO</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/var/log/kroombox-backup.log</string>
    <key>StandardErrorPath</key>
    <string>/var/log/kroombox-backup.error.log</string>
</dict>
</plist>`, label, kbaPath, cfg.Hour, cfg.Minute)

	plistPath := filepath.Join(home, "Library/LaunchAgents", label+".plist")
	if err := os.MkdirAll(filepath.Dir(plistPath), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(plistPath, []byte(plistContent), 0644); err != nil {
		return fmt.Errorf("write plist: %w", err)
	}
	fmt.Printf("  ✓ Created: %s\n", plistPath)

	// Load
	exec.Command("launchctl", "unload", plistPath).Run()
	exec.Command("launchctl", "load", plistPath).Run()
	fmt.Println("  ✓ LaunchAgent loaded")

	return nil
}

func installCron(cfg *ScheduleConfig, kbaPath string) error {
	cronLine := fmt.Sprintf("%s %s backup\n", cfg.CronExpr, kbaPath)

	// Get existing crontab
	cmd := exec.Command("crontab", "-l")
	existing, _ := cmd.Output()

	// Filter out old kba lines
	var newCron []string
	for _, line := range strings.Split(string(existing), "\n") {
		if !strings.Contains(line, kbaPath) {
			newCron = append(newCron, line)
		}
	}

	// Add comment and new line
	newCron = append(newCron,
		fmt.Sprintf("# Kroombox Backup Agent - %s backup at %02d:%02d", cfg.Frequency, cfg.Hour, cfg.Minute),
		cronLine)

	cronContent := strings.Join(newCron, "\n") + "\n"

	tmpFile := filepath.Join(os.TempDir(), "kba-crontab")
	if err := os.WriteFile(tmpFile, []byte(cronContent), 0644); err != nil {
		return fmt.Errorf("write temp crontab: %w", err)
	}

	cmd = exec.Command("crontab", tmpFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("install crontab: %s: %w", string(out), err)
	}
	os.Remove(tmpFile)

	fmt.Printf("  ✓ Crontab installed: %s\n", cronLine)
	out, _ := exec.Command("crontab", "-l").Output()
	fmt.Println("  Current crontab:")
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "kba") || strings.Contains(line, "Kroombox") {
			fmt.Printf("    %s\n", line)
		}
	}

	return nil
}

func mapDow(d string) string {
	switch d {
	case "0": return "Sun"
	case "1": return "Mon"
	case "2": return "Tue"
	case "3": return "Wed"
	case "4": return "Thu"
	case "5": return "Fri"
	case "6": return "Sat"
	case "7": return "Sun"
	}
	return "*-*-*"
}

func writeFileSudo(path, content string) error {
	if err := os.WriteFile(path, []byte(content), 0644); err == nil {
		return nil
	}

	tmpPath := filepath.Join(os.TempDir(), "kba-schedule-tmp")
	if err := os.WriteFile(tmpPath, []byte(content), 0644); err != nil {
		return err
	}

	// Try sudo -A (askpass) first, then plain sudo
	askpass := os.Getenv("SUDO_ASKPASS")
	if askpass != "" {
		cmd := exec.Command("sudo", "-A", "mv", tmpPath, path)
		if _, err := cmd.CombinedOutput(); err == nil {
			return nil
		}
	}

	cmd := exec.Command("sudo", "mv", tmpPath, path)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("sudo mv: %s: %w", string(out), err)
	}
	return nil
}

// Describe describes the installed schedule
func Describe() error {
	osType := runtime.GOOS
	fmt.Println()
	fmt.Println("Current backup schedule:")

	switch osType {
	case "linux":
		askpass := os.Getenv("SUDO_ASKPASS")
		var timerOut []byte
		if askpass != "" {
			timerOut, _ = exec.Command("sudo", "-A", "systemctl", "show", "kroombox-backup.timer", "--property=NextElapseUSecRealtime,LastTriggerUSec", "--no-pager").Output()
		}
		if len(timerOut) == 0 {
			timerOut, _ = exec.Command("systemctl", "show", "kroombox-backup.timer", "--property=NextElapseUSecRealtime,LastTriggerUSec", "--no-pager").Output()
		}
		// Read timezone from timer config
		tzName := "UTC"
		var svcOut2 []byte
		if askpass != "" {
			svcOut2, _ = exec.Command("sudo", "-A", "cat", "/etc/systemd/system/kroombox-backup.timer").Output()
		} else {
			svcOut2, _ = exec.Command("cat", "/etc/systemd/system/kroombox-backup.timer").Output()
		}
		if len(svcOut2) > 0 {
			for _, line := range strings.Split(string(svcOut2), "\n") {
				if strings.Contains(line, "OnCalendar") {
					parts := strings.Fields(line)
					if len(parts) >= 2 {
						// Last field might be timezone (e.g., Asia/Jakarta)
						last := strings.TrimSpace(parts[len(parts)-1])
						if strings.Contains(last, "/") || len(last) == 3 {
							tzName = last
						}
					}
				}
			}
		}
		if len(timerOut) > 0 {
			for _, prop := range strings.Split(string(timerOut), "\n") {
				if strings.Contains(prop, "NextElapse") {
					val := strings.TrimPrefix(prop, "NextElapseUSecRealtime=")
					if val != "" {
						// Try to convert to configured timezone
						converted := convertTZ(val, tzName)
						fmt.Printf("  Next run:  %s\n", converted)
					}
				} else if strings.Contains(prop, "LastTrigger") {
					val := strings.TrimPrefix(prop, "LastTriggerUSec=")
					if val != "" {
						converted := convertTZ(val, tzName)
						fmt.Printf("  Last run:  %s\n", converted)
					}
				}
			}
			fmt.Printf("  Status:    active (systemd timer) — %s\n", tzName)

			// Show timer config
			var svcOut []byte
			if askpass != "" {
				svcOut, _ = exec.Command("sudo", "-A", "cat", "/etc/systemd/system/kroombox-backup.timer").Output()
			} else {
				svcOut, _ = exec.Command("cat", "/etc/systemd/system/kroombox-backup.timer").Output()
			}
			if len(svcOut) > 0 {
				for _, line := range strings.Split(string(svcOut), "\n") {
					if strings.Contains(line, "OnCalendar") {
						cal := strings.TrimPrefix(strings.TrimSpace(line), "OnCalendar=")
						parts := strings.SplitN(cal, "\n", 2)
						fmt.Printf("  Schedule:  %s\n", parts[0])
					}
					if strings.Contains(line, "Description") {
						desc := strings.TrimPrefix(strings.TrimSpace(line), "Description=")
						fmt.Printf("  Type:      %s\n", desc)
					}
				}
			}
			fmt.Println()
			fmt.Println("  Use: kba schedule --remove  to disable")
			return nil
		}

		// Fallback to cron
		out2, _ := exec.Command("crontab", "-l").Output()
		if len(out2) > 0 {
			fmt.Println("  Type: cron")
			for _, line := range strings.Split(string(out2), "\n") {
				if strings.Contains(line, "kba") {
					fmt.Printf("  %s\n", line)
				}
			}
			return nil
		}

	case "darwin":
		home, _ := os.UserHomeDir()
		plistPath := filepath.Join(home, "Library/LaunchAgents/com.kroombox.backup-agent.plist")
		if _, err := os.Stat(plistPath); err == nil {
			fmt.Println("  Type: launchd")
			out, _ := exec.Command("launchctl", "list", "com.kroombox.backup-agent").Output()
			fmt.Printf("  %s\n", string(out))
			return nil
		}
	}

	fmt.Println("  No schedule configured.")
	fmt.Println("  Run 'kba schedule' to set one up.")
	return nil
}

func RemoveSchedule() error {
	osType := runtime.GOOS
	askpass := os.Getenv("SUDO_ASKPASS")

	switch osType {
	case "linux":
		cmds := []string{}
		if _, err := exec.Command("systemctl", "is-active", "kroombox-backup.timer").Output(); err == nil {
			cmds = append(cmds,
				"systemctl stop kroombox-backup.timer",
				"systemctl disable kroombox-backup.timer",
				"rm -f /etc/systemd/system/kroombox-backup.timer",
				"rm -f /etc/systemd/system/kroombox-backup.service",
				"systemctl daemon-reload",
			)
		}
		out, _ := exec.Command("crontab", "-l").Output()
		if len(out) > 0 && strings.Contains(string(out), "kba") {
			var newCron []string
			for _, line := range strings.Split(string(out), "\n") {
				if !strings.Contains(line, "kba") && !strings.Contains(line, "Kroombox") {
					newCron = append(newCron, line)
				}
			}
			tmpFile := filepath.Join(os.TempDir(), "kba-cron-remove")
			os.WriteFile(tmpFile, []byte(strings.Join(newCron, "\n")), 0644)
			cmds = append(cmds, "crontab "+tmpFile)
		}
		if len(cmds) == 0 {
			return fmt.Errorf("no schedule found")
		}
		sudo := "sudo"
		if askpass != "" { sudo = "sudo -A" }
		for _, c := range cmds {
			exec.Command("sh", "-c", sudo+" "+c).Run()
		}
		fmt.Println("\n  Schedule removed.")
		return nil

	case "darwin":
		home, _ := os.UserHomeDir()
		plistPath := filepath.Join(home, "Library/LaunchAgents/com.kroombox.backup-agent.plist")
		if _, err := os.Stat(plistPath); err == nil {
			exec.Command("launchctl", "unload", plistPath).Run()
			os.Remove(plistPath)
			fmt.Println("\n  LaunchAgent removed.")
			return nil
		}
		return fmt.Errorf("no schedule found")
	}

	return fmt.Errorf("unsupported OS")
}

func CleanupOldBackups(keepDays int, backupPath string) error {
	// Init DB and find old backups
	records, err := db.GetOldBackups(keepDays)
	if err != nil {
		// DB not available, use folder scan
		backupDirs := []string{}
		if backupPath != "" { backupDirs = append(backupDirs, backupPath) }
		backupDirs = append(backupDirs,
			filepath.Join(os.Getenv("HOME"), "backups/kroombox"),
			"/var/backups/kroombox",
		)
		cutoff := time.Now().AddDate(0, 0, -keepDays)
		for _, bd := range backupDirs {
			if fi, err := os.Stat(bd); err != nil || !fi.IsDir() { continue }
			entries, err := os.ReadDir(bd)
			if err != nil { continue }
			for _, entry := range entries {
				if !entry.IsDir() { continue }
				folderDate, err := time.Parse("2006-01-02", entry.Name())
				if err != nil { continue }
				if folderDate.Before(cutoff) {
					folderPath := filepath.Join(bd, entry.Name())
					var size int64
					filepath.Walk(folderPath, func(p string, fi os.FileInfo, err error) error {
						if err == nil && !fi.IsDir() { size += fi.Size() }
						return nil
					})
					os.RemoveAll(folderPath)
					fmt.Printf("  \u2713 Removed %s (%s)\n", entry.Name(), modules.FormatSize(size))
				}
			}
		}
		return nil
	}

	var deleted int
	var totalSize int64
	for _, r := range records {
		if fi, err := os.Stat(r.Path); err == nil && fi.IsDir() {
			os.RemoveAll(r.Path)
		}
		db.DeleteBackup(r.ID)
		deleted++
		totalSize += r.Size
		fmt.Printf("  \u2713 Removed %s (%s)\n", r.Date, modules.FormatSize(r.Size))
	}

	if deleted == 0 {
		fmt.Printf("\n  No backups older than %d days found.\n", keepDays)
	} else {
		fmt.Printf("\n  Cleaned up %d backups (%s freed)\n", deleted, modules.FormatSize(totalSize))
	}
	return nil
}

func convertTZ(timeStr, tzName string) string {
	// systemctl show returns format like "Sun 2026-07-05 20:29:29 UTC"
	// Parse it and convert to target timezone
	loc, err := time.LoadLocation(tzName)
	if err != nil {
		// Can't convert, show as-is with timezone name
		return timeStr + " (" + tzName + ")"
	}
	
	// Try parsing various formats
	formats := []string{
		"Mon 2006-01-02 15:04:05 MST",
		"Mon 2006-01-02 15:04:05",
		"2006-01-02 15:04:05",
	}
	
	for _, fmt := range formats {
		t, err := time.Parse(fmt, timeStr)
		if err == nil {
			// Convert to target timezone
			t = t.In(loc)
			return t.Format("Mon 02-Jan-2006 15:04:05")
		}
	}
	
	// Can't parse, append timezone
	return timeStr + " (" + tzName + ")"
}
