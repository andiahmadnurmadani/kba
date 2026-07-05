package restore

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"kroombox-backup-agent/logs"
	"kroombox-backup-agent/manifest"
)

type RestoreOptions struct {
	Source    string
	Services  []string
	DryRun    bool
	TargetDir string
}

func Run(opts RestoreOptions) error {
	manifestPath := filepath.Join(opts.Source, "manifest.json")
	if _, err := os.Stat(manifestPath); err != nil {
		return fmt.Errorf("manifest not found at %s: %w", manifestPath, err)
	}

	m, err := manifest.Load(manifestPath)
	if err != nil {
		return fmt.Errorf("load manifest: %w", err)
	}

	logs.Info("Restoring backup from: %s", opts.Source)
	logs.Info("Server: %s | Date: %s", m.Hostname, m.BackupDate)
	logs.Info("Services in backup: %d", len(m.Services))

	services := opts.Services
	if len(services) == 0 {
		for svc := range m.Services {
			if m.Services[svc] {
				services = append(services, svc)
			}
		}
	}

	targetDir := opts.TargetDir
	if targetDir == "" {
		targetDir = opts.Source
	}

	for _, svc := range services {
		logs.Info("Restoring: %s", svc)
		svcDir := filepath.Join(opts.Source, svc)
		if _, err := os.Stat(svcDir); err != nil {
			logs.Error("  %s backup data not found, skipping", svc)
			continue
		}

		if opts.DryRun {
			logs.Info("  DRY-RUN: would restore %s", svcDir)
			continue
		}

		if err := restoreService(svc, svcDir, targetDir); err != nil {
			logs.Error("  Failed to restore %s: %v", svc, err)
		} else {
			logs.Info("  %s restored successfully", svc)
		}
	}

	return nil
}

func restoreService(name, sourceDir, targetDir string) error {
	switch name {
	case "mysql":
		return restoreMySQL(sourceDir)
	case "postgres":
		return restorePostgres(sourceDir)
	case "mongodb":
		return restoreMongoDB(sourceDir)
	case "nginx":
		return restoreConfig(sourceDir, "/etc/nginx")
	case "pm2":
		return restorePM2(sourceDir)
	case "cron":
		return restoreCron(sourceDir)
	case "ssl":
		return restoreConfig(sourceDir, "/etc/letsencrypt")
	case "docker", "git":
		logs.Info("  %s restore: files available at %s (manual restore recommended)", name, sourceDir)
		return nil
	default:
		return fmt.Errorf("unknown service: %s", name)
	}
	return nil
}

func restoreMySQL(src string) error {
	files, err := os.ReadDir(src)
	if err != nil { return err }

	for _, f := range files {
		if filepath.Ext(f.Name()) != ".sql" { continue }
		dbName := f.Name()[:len(f.Name())-4]
		if dbName == "all" { continue }

		logs.Info("  Restoring database: %s", dbName)

		exec.Command("mysql", "-e", fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", dbName)).Run()
		cmd := exec.Command("mysql", dbName, "-e", fmt.Sprintf("source %s", filepath.Join(src, f.Name())))
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("restore %s: %s: %w", dbName, string(out), err)
		}
	}
	return nil
}

func restorePostgres(src string) error {
	dumpFile := filepath.Join(src, "all.sql")
	if _, err := os.Stat(dumpFile); err != nil {
		return fmt.Errorf("postgres dump not found: %w", err)
	}
	cmd := exec.Command("psql", "-f", dumpFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("restore postgres: %s: %w", string(out), err)
	}
	return nil
}

func restoreMongoDB(src string) error {
	cmd := exec.Command("mongorestore", "--drop", src)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("restore mongodb: %s: %w", string(out), err)
	}
	return nil
}

func restorePM2(src string) error {
	dst := filepath.Join(os.Getenv("HOME"), ".pm2")
	os.MkdirAll(dst, 0755)
	cpCmd := exec.Command("cp", "-a", src+"/.", dst+"/")
	if out, err := cpCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("restore pm2: %s: %w", string(out), err)
	}
	exec.Command("pm2", "resurrect").Run()
	return nil
}

func restoreCron(src string) error {
	cronFile := filepath.Join(src, "crontab.txt")
	if _, err := os.Stat(cronFile); err == nil {
		cmd := exec.Command("crontab", cronFile)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("restore crontab: %s: %w", string(out), err)
		}
	}
	return nil
}

func restoreConfig(src, dst string) error {
	os.MkdirAll(dst, 0755)
	cpCmd := exec.Command("cp", "-a", src+"/.", dst+"/")
	if out, err := cpCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("restore config to %s: %s: %w", dst, string(out), err)
	}
	return nil
}

