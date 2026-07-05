package setup

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"kroombox-backup-agent/db"
	"kroombox-backup-agent/detect"
	"kroombox-backup-agent/tui"
)

func IsConfigured() bool {
	dest, _ := db.GetConfig("destination")
	return dest != ""
}

func RunWizard() error {
	reader := bufio.NewReader(os.Stdin)
	d := detect.Detect()

	fmt.Println()
	tui.StatusBox("KBA First-Time Setup", []string{
		"This wizard will configure your backup agent.",
		"Follow the steps below to get started.",
	})

	// ── Step 1: Detected Services ──
	fmt.Println()
	fmt.Printf("  %sStep 1/4: Detected Services%s\n", tui.Bold, tui.Reset)
	fmt.Println()
	rows := [][]string{}
	for _, s := range d.Services {
		icon := "✗"
		color := tui.Red
		if s.Present {
			icon = "✓"
			color = tui.Green
		}
		ver := s.Version
		if len(ver) > 25 {
			ver = ver[:24] + "…"
		}
		rows = append(rows, []string{
			fmt.Sprintf("%s%s %s%s", color, icon, s.Service, tui.Reset),
			ver,
		})
	}
	tui.Table([]string{"Service", "Version"}, rows)
	fmt.Println()
	fmt.Print("  Press Enter to continue...")
	reader.ReadString('\n')

	// ── Step 2: Credentials ──
	fmt.Println()
	fmt.Printf("  %sStep 2/4: Credentials Setup%s\n", tui.Bold, tui.Reset)
	fmt.Println()
	if err := setupCredentials(reader, d); err != nil {
		return err
	}
	fmt.Println()

	// ── Step 3: Backup Destination ──
	fmt.Println()
	fmt.Printf("  %sStep 3/4: Backup Destination%s\n", tui.Bold, tui.Reset)
	fmt.Println()

	dest, _ := db.GetConfig("destination")
	if dest == "" {
		dest = "/var/backups/kroombox"
	}
	fmt.Printf("  Backup directory [%s]: ", dest)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input != "" {
		dest = input
	}

	// Create directory
	if strings.HasPrefix(dest, "/var/") || strings.HasPrefix(dest, "/etc/") {
		exec.Command("sudo", "mkdir", "-p", dest).Run()
		exec.Command("sudo", "chown", os.Getenv("USER"), dest).Run()
	} else {
		os.MkdirAll(dest, 0755)
	}
	db.SetConfig("destination", dest)
	fmt.Printf("  %s Backup directory: %s%s\n", tui.Green, dest, tui.Reset)
	fmt.Println()

	// ── Step 4: Service Selection ──
	fmt.Println()
	fmt.Printf("  %sStep 4/4: Services to Backup%s\n", tui.Bold, tui.Reset)
	fmt.Println()
	fmt.Println("  Select which services to include in backup.")
	fmt.Println("  Press Enter for all, or enter comma-separated list.")
	fmt.Println()
	svcList := []string{}
	for _, s := range d.Services {
		if s.Present {
			svcList = append(svcList, string(s.Service))
		}
	}
	fmt.Printf("  Available: %s\n", strings.Join(svcList, ", "))
	fmt.Println("  Example: mysql,nginx,mongodb")
	fmt.Print("  Services [all]: ")
	svcInput, _ := reader.ReadString('\n')
	svcInput = strings.TrimSpace(svcInput)
	if svcInput != "" {
		db.SetConfig("services", svcInput)
	} else {
		db.SetConfig("services", strings.Join(svcList, ","))
	}
	fmt.Printf("  %s Services configured%s\n", tui.Green, tui.Reset)
	fmt.Println()

	// ── Summary ──
	configuredServices, _ := db.GetConfig("services")
	tui.StatusBox("Setup Complete", []string{
		fmt.Sprintf("Destination: %s", dest),
		fmt.Sprintf("Services:   %s", configuredServices),
		"",
		"Run 'kba backup' to start your first backup!",
	})
	return nil
}

func setupCredentials(reader *bufio.Reader, d *detect.Result) error {
	for _, s := range d.Services {
		if !s.Present {
			continue
		}
		switch string(s.Service) {
		case "mysql":
			if checkMySQLLogin() {
				fmt.Printf("  %s MySQL: already configured (login OK)%s\n", tui.Green, tui.Reset)
				continue
			}
			fmt.Printf("  %sMySQL%s requires login credentials.\n", tui.Bold, tui.Reset)
			fmt.Println()
			
			for attempt := 0; attempt < 5; attempt++ {
				fmt.Print("  MySQL user [root]: ")
				user, _ := reader.ReadString('\n')
				user = strings.TrimSpace(user)
				if user == "" {
					user = "root"
				}
				fmt.Print("  MySQL password (enter for socket auth): ")
				pass, _ := reader.ReadString('\n')
				pass = strings.TrimSpace(pass)
				cnfPath := mysqlCnfPath()
				if pass == "" {
					c := "[client]\nuser=" + user + "\nsocket=/var/run/mysqld/mysqld.sock"
					os.WriteFile(cnfPath, []byte(c), 0600)
					fmt.Printf("  Written: %s (socket auth)\n", cnfPath)
				} else {
					c := "[client]\nuser=" + user + "\npassword=" + pass
					os.WriteFile(cnfPath, []byte(c), 0600)
					fmt.Printf("  Written: %s\n", cnfPath)
				}
				// Show content (hide password)
				if data, err := os.ReadFile(cnfPath); err == nil {
					for _, l := range strings.Split(string(data), "\n") {
						if strings.Contains(l, "password") {
							fmt.Printf("           %s=***\n", strings.Split(l, "=")[0])
						} else if l != "" {
							fmt.Printf("           %s\n", l)
						}
					}
				}
				// Also try MYSQL_PWD env for direct testing
				os.Setenv("MYSQL_PWD", pass)
				if checkMySQLLogin() {
					fmt.Printf("  %s MySQL login OK!%s\n", tui.Green, tui.Reset)
					break
				}
				if attempt < 4 {
					// Show the actual error
					debugCmd := exec.Command("mysql", "-u", user, "-p"+pass, "-e", "SELECT 1", "--batch", "--skip-column-names")
					debugOut, _ := debugCmd.CombinedOutput()
					errMsg := strings.TrimSpace(string(debugOut))
					if errMsg != "" {
						fmt.Printf("  %s Error: %s%s\n", tui.Yellow, errMsg, tui.Reset)
					}
					fmt.Printf("  %s Login failed (%d/5). Try again.%s\n", tui.Yellow, attempt+1, tui.Reset)
				} else {
					fmt.Printf("  %s Login failed after 5 attempts. Skipping MySQL.%s\n", tui.Yellow, tui.Reset)
				}
			}

		case "postgres":
			if checkPgLogin() {
				fmt.Printf("  %s PostgreSQL: already configured%s\n", tui.Green, tui.Reset)
				continue
			}
			fmt.Printf("  %sPostgreSQL%s requires login credentials.\n", tui.Bold, tui.Reset)
			fmt.Println()
			fmt.Print("  Host [localhost]: ")
			host, _ := reader.ReadString('\n')
			host = strings.TrimSpace(host)
			if host == "" {
				host = "localhost"
			}
			fmt.Print("  Port [5432]: ")
			port, _ := reader.ReadString('\n')
			port = strings.TrimSpace(port)
			if port == "" {
				port = "5432"
			}
			fmt.Print("  User [postgres]: ")
			user, _ := reader.ReadString('\n')
			user = strings.TrimSpace(user)
			if user == "" {
				user = "postgres"
			}
			fmt.Print("  Password: ")
			pass, _ := reader.ReadString('\n')
			pass = strings.TrimSpace(pass)
			home, _ := os.UserHomeDir()
			pgpassPath := filepath.Join(home, ".pgpass")
			content := fmt.Sprintf("%s:%s:*:%s:%s\n", host, port, user, pass)
			os.WriteFile(pgpassPath, []byte(content), 0600)
			if checkPgLogin() {
				fmt.Printf("  %s PostgreSQL login OK!%s\n", tui.Green, tui.Reset)
			} else {
				fmt.Printf("  %s PostgreSQL login failed%s\n", tui.Yellow, tui.Reset)
			}
		}
	}
	return nil
}

func checkMySQLLogin() bool {
	// Try with --defaults-file to be explicit
	cmd := exec.Command("mysql", "--defaults-file="+mysqlCnfPath(), "-e", "SELECT 1", "--batch", "--skip-column-names")
	out, err := cmd.CombinedOutput()
	if err == nil && strings.TrimSpace(string(out)) == "1" {
		return true
	}
	errMsg := strings.TrimSpace(string(out))
	_ = errMsg

	// Try without defaults-file (standard client path)
	cmd2 := exec.Command("mysql", "-e", "SELECT 1", "--batch", "--skip-column-names")
	out2, err2 := cmd2.CombinedOutput()
	if err2 == nil && strings.TrimSpace(string(out2)) == "1" {
		return true
	}

	return false
}

func mysqlCnfPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".my.cnf")
}

func checkPgLogin() bool {
	cmd := exec.Command("pg_isready", "-q")
	return cmd.Run() == nil
}

func CheckAndWarn() {
	fmt.Println()
	tui.StatusBox("Setup Required", []string{
		"You haven't configured KBA yet!",
		"",
		"Run 'kba setup' to configure:",
		"  • Detected services",
		"  • Database credentials",
		"  • Backup destination",
		"  • Services to include",
	})
	fmt.Println()
}
