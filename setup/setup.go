package setup

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"kroombox-backup-agent/detect"
	"kroombox-backup-agent/tui"
)

func RunCredentialWizard() error {
	d := detect.Detect()
	reader := bufio.NewReader(os.Stdin)

	fmt.Println()
	tui.StatusBox("Credential Setup", []string{
		"Set up database and service credentials",
		"for passwordless backups.",
		"",
	})

	for _, s := range d.Services {
		if !s.Present {
			continue
		}

		switch string(s.Service) {
		case "mysql":
			if err := setupMySQL(reader); err != nil {
				return err
			}
		case "postgres":
			if err := setupPostgres(reader); err != nil {
				return err
			}
		case "mongodb":
			setupMongo(reader)
		}
	}

	fmt.Println()
	tui.StatusBox("Credential Setup Complete", []string{
		"All credentials configured.",
		"Run 'kba detect' to verify.",
	})
	return nil
}

func setupMySQL(reader *bufio.Reader) error {
	// Check if .my.cnf already works
	if checkMySQLLogin() {
		fmt.Printf("  %s MySQL: already configured (login OK)%s\n", tui.Green, tui.Reset)
		return nil
	}

	fmt.Printf("\n  %sMySQL%s requires credentials for backup.\n", tui.Bold, tui.Reset)
	fmt.Println()

	fmt.Print("  MySQL user [root]: ")
	user, _ := reader.ReadString('\n')
	user = strings.TrimSpace(user)
	if user == "" {
		user = "root"
	}

	fmt.Print("  MySQL password: ")
	pass, _ := reader.ReadString('\n')
	pass = strings.TrimSpace(pass)
	if pass == "" {
		// Try socket auth
		home, _ := os.UserHomeDir()
		cnfPath := filepath.Join(home, ".my.cnf")
		content := "[client]\nuser=" + user + "\nsocket=/var/run/mysqld/mysqld.sock"
		if err := os.WriteFile(cnfPath, []byte(content), 0600); err != nil {
			return fmt.Errorf("write .my.cnf: %w", err)
		}
		fmt.Printf("  %s Using socket auth (no password)%s\n", tui.Yellow, tui.Reset)
	} else {
		home, _ := os.UserHomeDir()
		cnfPath := filepath.Join(home, ".my.cnf")
		content := "[client]\nuser=" + user + "\npassword=" + pass
		if err := os.WriteFile(cnfPath, []byte(content), 0600); err != nil {
			return fmt.Errorf("write .my.cnf: %w", err)
		}
	}

	// Verify
	if checkMySQLLogin() {
		fmt.Printf("  %s MySQL login OK!%s\n", tui.Green, tui.Reset)
	} else {
		fmt.Printf("  %s MySQL login failed — check credentials%s\n", tui.Yellow, tui.Reset)
		fmt.Println("  You can re-run: kba setup")
	}
	return nil
}

func setupPostgres(reader *bufio.Reader) error {
	if checkPgLogin() {
		fmt.Printf("  %s PostgreSQL: already configured (login OK)%s\n", tui.Green, tui.Reset)
		return nil
	}

	fmt.Printf("\n  %sPostgreSQL%s requires credentials for backup.\n", tui.Bold, tui.Reset)
	fmt.Println()

	fmt.Print("  PostgreSQL host [localhost]: ")
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

	fmt.Print("  Database [*]: ")
	dbname, _ := reader.ReadString('\n')
	dbname = strings.TrimSpace(dbname)
	if dbname == "" {
		dbname = "*"
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
	content := fmt.Sprintf("%s:%s:%s:%s:%s\n", host, port, dbname, user, pass)
	if err := os.WriteFile(pgpassPath, []byte(content), 0600); err != nil {
		return fmt.Errorf("write .pgpass: %w", err)
	}

	os.Setenv("PGPASSWORD", pass)
	os.Setenv("PGUSER", user)
	if checkPgLogin() {
		fmt.Printf("  %s PostgreSQL login OK!%s\n", tui.Green, tui.Reset)
	} else {
		fmt.Printf("  %s PostgreSQL login failed — check credentials%s\n", tui.Yellow, tui.Reset)
	}
	return nil
}

func setupMongo(reader *bufio.Reader) {
	if checkMongoLogin() {
		fmt.Printf("  %s MongoDB: running (no auth needed)%s\n", tui.Green, tui.Reset)
		return
	}
	fmt.Printf("  %s MongoDB: not running. Start with:%s\n", tui.Yellow, tui.Reset)
	fmt.Println("    Linux:  sudo systemctl start mongod")
	fmt.Println("    macOS:  brew services start mongod-community")
}

func checkMySQLLogin() bool {
	cmd := exec.Command("mysql", "-e", "SELECT 1", "--batch", "--skip-column-names")
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) == "1"
}

func checkPgLogin() bool {
	cmd := exec.Command("pg_isready", "-q")
	return cmd.Run() == nil
}

func checkMongoLogin() bool {
	cmd := exec.Command("mongosh", "--quiet", "--eval", "db.adminCommand('ping')")
	return cmd.Run() == nil
}
