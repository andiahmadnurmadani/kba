package modules

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type BackupResult struct {
	Name     string
	Success  bool
	Size     int64
	Error    string
	Skipped  bool
	Reason   string // why it failed/skipped
}

type BackupModule interface {
	Name() string
	Backup(backupDir string) (*BackupResult, error)
	Detect() bool
	DetectVerbose() []string // returns info about detection + creds
}

// checkMySQLLogin checks if mysql client can connect
func mysqlCnfPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".my.cnf")
}

func checkMySQLLogin() bool {
	cmd := exec.Command("mysql", "-e", "SELECT 1", "--batch", "--skip-column-names")
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) == "1"
}

// MySQL
type MySQLModule struct{}

func (m *MySQLModule) Name() string { return "mysql" }
func (m *MySQLModule) Detect() bool {
	_, err := exec.LookPath("mysqldump")
	return err == nil
}
func (m *MySQLModule) DetectVerbose() []string {
	info := []string{}
	if _, err := exec.LookPath("mysql"); err == nil {
		info = append(info, "binary: mysql ✓")
	} else {
		info = append(info, "binary: mysql ✗")
		return info
	}
	if _, err := exec.LookPath("mysqldump"); err == nil {
		info = append(info, "binary: mysqldump ✓")
	} else {
		info = append(info, "binary: mysqldump ✗")
	}
	if checkMySQLLogin() {
		info = append(info, "login: ✓ (passwordless via .my.cnf or socket)")
	} else {
		info = append(info, "login: ✗ NEEDS CREDENTIALS — buat ~/.my.cnf: [client] user=root password=YOUR_PASSWORD")
	}
	return info
}
func (m *MySQLModule) Backup(dir string) (*BackupResult, error) {
	result := &BackupResult{Name: "mysql", Skipped: true}
	if !m.Detect() { result.Reason = "mysqldump not installed"; return result, nil }

	outDir := filepath.Join(dir, "mysql")
	os.MkdirAll(outDir, 0755)

	if !checkMySQLLogin() {
		result.Skipped = true
		result.Reason = "LOGIN FAILED - butuh kredensial MySQL. Buat ~/.my.cnf dengan [client] user=root password=..."
		return result, nil
	}

	// List user databases (skip system DBs)
	listCmd := exec.Command("mysql", "--defaults-file="+mysqlCnfPath(), "-e", "SHOW DATABASES", "--batch", "--skip-column-names")
	out, err := listCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list databases: %w", err)
	}

	skipDBs := map[string]bool{
		"information_schema": true,
		"performance_schema": true,
		"mysql":             true,
		"sys":               true,
	}
	var dbs []string
	for _, db := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		db = strings.TrimSpace(db)
		if db != "" && !skipDBs[db] {
			dbs = append(dbs, db)
		}
	}

	if len(dbs) == 0 {
		result.Skipped = true
		result.Reason = "no user databases found"
		return result, nil
	}

	// Dump only user databases
	dumpFile := filepath.Join(outDir, "all.sql")
	f, err := os.Create(dumpFile)
	if err != nil { return nil, fmt.Errorf("create dump file: %w", err) }
	defer f.Close()

	args := []string{"--defaults-file="+mysqlCnfPath(), "--databases", "--single-transaction", "--routines", "--triggers"}
	args = append(args, dbs...)
	dumpCmd := exec.Command("mysqldump", args...)
	dumpCmd.Stdout = f
	if err := dumpCmd.Run(); err != nil {
		os.Remove(dumpFile)
		return nil, fmt.Errorf("mysqldump: %w", err)
	}

	fi, _ := os.Stat(dumpFile)
	if fi != nil { result.Size = fi.Size() }
	result.Success = true
	result.Skipped = false
	return result, nil
}

// PostgreSQL
type PostgresModule struct{}

func (m *PostgresModule) Name() string { return "postgres" }
func (m *PostgresModule) Detect() bool {
	_, err := exec.LookPath("pg_dumpall")
	return err == nil
}
func (m *PostgresModule) DetectVerbose() []string {
	info := []string{}
	if _, err := exec.LookPath("pg_dumpall"); err == nil {
		info = append(info, "binary: pg_dumpall ✓")
		cmd := exec.Command("pg_isready", "-q")
		if cmd.Run() == nil {
			info = append(info, "service: running ✓")
		} else {
			info = append(info, "service: not running or NEEDS CREDENTIALS — set pg_hba.conf trust or use .pgpass")
		}
	} else {
		info = append(info, "binary: pg_dumpall ✗ (not installed)")
	}
	return info
}
func (m *PostgresModule) Backup(dir string) (*BackupResult, error) {
	result := &BackupResult{Name: "postgres", Skipped: true}
	if !m.Detect() { result.Reason = "pg_dumpall not installed"; return result, nil }
	outDir := filepath.Join(dir, "postgres")
	os.MkdirAll(outDir, 0755)
	dumpFile := filepath.Join(outDir, "all.sql")
	f, err := os.Create(dumpFile)
	if err != nil { return nil, fmt.Errorf("create dump file: %w", err) }
	defer f.Close()
	pgDump := exec.Command("pg_dumpall", "--clean")
	pgDump.Stdout = f
	if err := pgDump.Run(); err != nil {
		result.Reason = fmt.Sprintf("pg_dumpall failed: %v (check credentials)", err)
		os.Remove(dumpFile)
		return result, nil
	}
	result.Success = true
	result.Skipped = false
	fi, _ := os.Stat(dumpFile)
	if fi != nil { result.Size = fi.Size() }
	return result, nil
}

// MongoDB
type MongoDBModule struct{}

func (m *MongoDBModule) Name() string { return "mongodb" }
func (m *MongoDBModule) Detect() bool {
	_, err := exec.LookPath("mongodump")
	return err == nil && checkMongoConnect()
}
func (m *MongoDBModule) DetectVerbose() []string {
	info := []string{}
	if _, err := exec.LookPath("mongodump"); err == nil {
		info = append(info, "binary: mongodump ✓")
	} else {
		info = append(info, "binary: mongodump ✗ (not installed)")
		return info
	}
	if checkMongoConnect() {
		info = append(info, "connection: ✓ (local, no auth)")
	} else {
		info = append(info, "connection: ✗ mongod not running or NEEDS AUTH — systemctl start mongod / brew services start mongod")
	}
	return info
}
func checkMongoConnect() bool {
	cmd := exec.Command("mongosh", "--quiet", "--eval", "db.adminCommand('ping')")
	return cmd.Run() == nil
}
func (m *MongoDBModule) Backup(dir string) (*BackupResult, error) {
	result := &BackupResult{Name: "mongodb", Skipped: true}
	if !m.Detect() { result.Reason = "mongodump not installed"; return result, nil }
	outDir := filepath.Join(dir, "mongodb")
	os.MkdirAll(outDir, 0755)
	dumpCmd := exec.Command("mongodump", "--out", outDir)
	if out, err := dumpCmd.CombinedOutput(); err != nil {
		result.Reason = fmt.Sprintf("mongodump failed: %s", string(out))
		return result, nil
	}
	filepath.Walk(outDir, func(p string, fi os.FileInfo, err error) error {
		if err == nil && !fi.IsDir() { result.Size += fi.Size() }
		return nil
	})
	result.Success = true
	result.Skipped = false
	return result, nil
}

// PM2
type PM2Module struct{}

func (m *PM2Module) Name() string { return "pm2" }
func (m *PM2Module) Detect() bool {
	_, err := exec.LookPath("pm2")
	return err == nil
}
func (m *PM2Module) DetectVerbose() []string {
	info := []string{}
	if _, err := exec.LookPath("pm2"); err == nil {
		info = append(info, "binary: pm2 ✓")
	} else {
		info = append(info, "binary: pm2 ✗ (not installed)")
	}
	return info
}
func (m *PM2Module) Backup(dir string) (*BackupResult, error) {
	result := &BackupResult{Name: "pm2", Skipped: true}
	if !m.Detect() { result.Reason = "pm2 not installed"; return result, nil }
	outDir := filepath.Join(dir, "pm2")
	os.MkdirAll(outDir, 0755)
	exec.Command("pm2", "save").Run()
	home, _ := os.UserHomeDir()
	pm2Dir := filepath.Join(home, ".pm2")
	if fi, err := os.Stat(pm2Dir); err == nil && fi.IsDir() {
		exec.Command("cp", "-r", pm2Dir+"/.", outDir+"/").Run()
	}
	listCmd := exec.Command("pm2", "jlist")
	if out, err := listCmd.Output(); err == nil {
		os.WriteFile(filepath.Join(outDir, "pm2_list.json"), out, 0644)
	}
	filepath.Walk(outDir, func(p string, fi os.FileInfo, err error) error {
		if err == nil && !fi.IsDir() { result.Size += fi.Size() }
		return nil
	})
	result.Success = true
	result.Skipped = false
	return result, nil
}

// Nginx
type NginxModule struct{}

func (m *NginxModule) Name() string { return "nginx" }
func (m *NginxModule) Detect() bool {
	_, err := exec.LookPath("nginx")
	return err == nil
}
func (m *NginxModule) DetectVerbose() []string {
	info := []string{}
	if _, err := exec.LookPath("nginx"); err == nil {
		info = append(info, "binary: nginx ✓")
	} else {
		info = append(info, "binary: nginx ✗ (not installed)")
	}
	return info
}
func (m *NginxModule) Backup(dir string) (*BackupResult, error) {
	result := &BackupResult{Name: "nginx", Skipped: true}
	if !m.Detect() { result.Reason = "nginx not installed"; return result, nil }
	outDir := filepath.Join(dir, "nginx")
	os.MkdirAll(outDir, 0755)
	paths := []string{"/etc/nginx", "/usr/local/etc/nginx", "/opt/homebrew/etc/nginx"}
	var nginxDir string
	for _, p := range paths {
		if fi, err := os.Stat(p); err == nil && fi.IsDir() { nginxDir = p; break }
	}
	if nginxDir == "" {
		cmd := exec.Command("nginx", "-t")
		out, _ := cmd.CombinedOutput()
		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(line, "/nginx.conf") {
				parts := strings.Split(line, " ")
				for _, p := range parts {
					if strings.HasPrefix(p, "/") && strings.Contains(p, "nginx.conf") {
						nginxDir = filepath.Dir(filepath.Dir(p))
					}
				}
			}
		}
	}
	if nginxDir != "" {
		exec.Command("cp", "-r", nginxDir+"/.", outDir+"/").Run()
		verCmd := exec.Command("nginx", "-V")
		out, _ := verCmd.CombinedOutput()
		os.WriteFile(filepath.Join(outDir, "nginx_version.txt"), out, 0644)
	}
	filepath.Walk(outDir, func(p string, fi os.FileInfo, err error) error {
		if err == nil && !fi.IsDir() { result.Size += fi.Size() }
		return nil
	})
	result.Success = true
	result.Skipped = false
	return result, nil
}

// SSL
type SSLModule struct{}

func (m *SSLModule) Name() string { return "ssl" }
func (m *SSLModule) Detect() bool {
		// paths := []string{"/etc/letsencrypt"}
	fi, err := os.Stat("/etc/letsencrypt")
	return err == nil && fi.IsDir()
}
func (m *SSLModule) DetectVerbose() []string {
	info := []string{}
	if fi, err := os.Stat("/etc/letsencrypt"); err == nil && fi.IsDir() {
		info = append(info, "letsencrypt: ✓ (/etc/letsencrypt exists)")
	} else {
		info = append(info, "letsencrypt: ✗ not found (no SSL certs to backup)")
	}
	if fi, err := os.Stat("/opt/homebrew/etc/openssl*"); err == nil && fi != nil {
		info = append(info, "openssl(macOS): ✓")
	}
	return info
}
func (m *SSLModule) Backup(dir string) (*BackupResult, error) {
	result := &BackupResult{Name: "ssl", Skipped: true}
	outDir := filepath.Join(dir, "ssl")
	os.MkdirAll(outDir, 0755)
	paths := []string{"/etc/letsencrypt"}
	var backedUp bool
	for _, p := range paths {
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			os.MkdirAll(filepath.Join(outDir, filepath.Base(p)), 0755)
			exec.Command("cp", "-r", p+"/.", filepath.Join(outDir, filepath.Base(p))+"/").Run()
			backedUp = true
		}
	}
	if backedUp {
		filepath.Walk(outDir, func(p string, fi os.FileInfo, err error) error {
			if err == nil && !fi.IsDir() { result.Size += fi.Size() }
			return nil
		})
		result.Success = true
		result.Skipped = false
	} else { result.Reason = "no SSL directories found"; result.Skipped = true }
	return result, nil
}

// Git
type GitModule struct{}

func (m *GitModule) Name() string { return "git" }
func (m *GitModule) Detect() bool { return true }
func (m *GitModule) DetectVerbose() []string {
	return []string{"always runs (scans /mnt/web/www, /var/www, /home for config files)"}
}
func (m *GitModule) Backup(dir string) (*BackupResult, error) {
	result := &BackupResult{Name: "git", Skipped: true}
	outDir := filepath.Join(dir, "git")
	os.MkdirAll(outDir, 0755)
	searchDirs := []string{"/mnt/web/www", "/var/www", "/home"}
	patterns := []string{".env", "composer.json", "package.json", "Dockerfile", "docker-compose.yml", "docker-compose.yaml"}
	var backedUp bool
	for _, sd := range searchDirs {
		if fi, err := os.Stat(sd); err != nil || !fi.IsDir() { continue }
		for _, pattern := range patterns {
			findCmd := exec.Command("find", sd, "-maxdepth", "3", "-name", pattern, "-type", "f")
			out, err := findCmd.Output()
			if err != nil { continue }
			for _, f := range strings.Split(strings.TrimSpace(string(out)), "\n") {
				if f == "" { continue }
				relPath := strings.TrimLeft(strings.TrimPrefix(f, sd), "/")
				destPath := filepath.Join(outDir, strings.ReplaceAll(relPath, "/", "_"))
				if err := exec.Command("cp", f, destPath).Run(); err == nil { backedUp = true }
			}
		}
	}
	if backedUp {
		filepath.Walk(outDir, func(p string, fi os.FileInfo, err error) error {
			if err == nil && !fi.IsDir() { result.Size += fi.Size() }
			return nil
		})
		result.Success = true
		result.Skipped = false
	} else { result.Reason = "no project config files found"; result.Skipped = true }
	return result, nil
}

// Cron
type CronModule struct{}

func (m *CronModule) Name() string { return "cron" }
func (m *CronModule) Detect() bool {
	_, err := exec.LookPath("crontab")
	return err == nil
}
func (m *CronModule) DetectVerbose() []string {
	info := []string{}
	if _, err := exec.LookPath("crontab"); err == nil {
		out, _ := exec.Command("crontab", "-l").Output()
		if len(out) > 0 {
			info = append(info, "crontab: ✓ has entries")
		} else {
			info = append(info, "crontab: binary found but no crontab entries")
		}
	} else {
		info = append(info, "crontab: ✗ not installed")
	}
	return info
}
func (m *CronModule) Backup(dir string) (*BackupResult, error) {
	result := &BackupResult{Name: "cron", Skipped: true}
	if !m.Detect() { result.Reason = "crontab not installed"; return result, nil }
	outDir := filepath.Join(dir, "cron")
	os.MkdirAll(outDir, 0755)
	cmd := exec.Command("crontab", "-l")
	out, err := cmd.Output()
	if err == nil && len(out) > 0 {
		os.WriteFile(filepath.Join(outDir, "crontab.txt"), out, 0644)
		result.Success = true
		result.Skipped = false
		spoolPath := "/var/spool/cron/crontabs/" + os.Getenv("USER")
		if _, err := os.Stat(spoolPath); err == nil {
			exec.Command("cp", spoolPath, filepath.Join(outDir, "crontab.spool")).Run()
		}
		if _, err := os.Stat("/etc/crontab"); err == nil {
			exec.Command("cp", "/etc/crontab", filepath.Join(outDir, "system_crontab")).Run()
		}
		if fi, err := os.Stat("/etc/cron.d"); err == nil && fi.IsDir() {
			exec.Command("cp", "-r", "/etc/cron.d", outDir+"/").Run()
		}
		result.Size = 0
		filepath.Walk(outDir, func(p string, fi os.FileInfo, err error) error {
			if err == nil && !fi.IsDir() { result.Size += fi.Size() }
			return nil
		})
	} else { result.Reason = "no crontab entries for this user"; result.Skipped = true }
	return result, nil
}

// Docker
type DockerModule struct{}

func (m *DockerModule) Name() string { return "docker" }
func (m *DockerModule) Detect() bool {
	_, err := exec.LookPath("docker")
	return err == nil
}
func (m *DockerModule) DetectVerbose() []string {
	info := []string{}
	if _, err := exec.LookPath("docker"); err == nil {
		info = append(info, "binary: docker ✓")
		out, _ := exec.Command("docker", "info", "--format", "{{.ServerVersion}}").Output()
		if len(out) > 0 {
			info = append(info, fmt.Sprintf("service: running v%s ✓", strings.TrimSpace(string(out))))
		} else {
			info = append(info, "service: daemon not running (try: systemctl start docker)")
		}
	} else {
		info = append(info, "binary: docker ✗ (not installed)")
	}
	return info
}
func (m *DockerModule) Backup(dir string) (*BackupResult, error) {
	result := &BackupResult{Name: "docker", Skipped: true}
	if !m.Detect() { result.Reason = "docker not installed"; return result, nil }
	outDir := filepath.Join(dir, "docker")
	os.MkdirAll(outDir, 0755)
	var backedUp bool
	findCmd := exec.Command("find", "/", "-maxdepth", "4", "-name", "compose.yaml", "-o", "-name", "compose.yml", "-o", "-name", "docker-compose.yml", "2>/dev/null")
	findCmdOutput, _ := findCmd.Output()
	if len(findCmdOutput) > 0 {
		composeDir := filepath.Join(outDir, "compose")
		os.MkdirAll(composeDir, 0755)
		for _, f := range strings.Split(strings.TrimSpace(string(findCmdOutput)), "\n") {
			if f == "" { continue }
			exec.Command("cp", f, filepath.Join(composeDir, strings.ReplaceAll(strings.TrimLeft(f, "/"), "/", "_"))).Run()
			backedUp = true
		}
	}
	imgCmd := exec.Command("docker", "images", "--format", "{{.Repository}}:{{.Tag}}")
	if out, err := imgCmd.Output(); err == nil && len(out) > 0 {
		os.WriteFile(filepath.Join(outDir, "images.txt"), out, 0644)
		backedUp = true
	}
	volCmd := exec.Command("docker", "volume", "ls", "-q")
	if out, err := volCmd.Output(); err == nil && len(out) > 0 {
		os.WriteFile(filepath.Join(outDir, "volumes.txt"), out, 0644)
		backedUp = true
	}
	if backedUp {
		filepath.Walk(outDir, func(p string, fi os.FileInfo, err error) error {
			if err == nil && !fi.IsDir() { result.Size += fi.Size() }
			return nil
		})
		result.Success = true
		result.Skipped = false
	} else { result.Reason = "no docker resources found"; result.Skipped = true }
	return result, nil
}

func GetModules() []BackupModule {
	return []BackupModule{
		&MySQLModule{},
		&PostgresModule{},
		&MongoDBModule{},
		&PM2Module{},
		&NginxModule{},
		&SSLModule{},
		&GitModule{},
		&CronModule{},
		&DockerModule{},
	}
}

func FormatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit { return fmt.Sprintf("%dB", bytes) }
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit { div *= unit; exp++ }
	return fmt.Sprintf("%.1f%cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
