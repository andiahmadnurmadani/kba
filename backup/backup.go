package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"kroombox-backup-agent/config"
	"kroombox-backup-agent/detect"
	"kroombox-backup-agent/logs"
	"kroombox-backup-agent/manifest"
	"kroombox-backup-agent/modules"
	"kroombox-backup-agent/db"
	"kroombox-backup-agent/progress"
	"kroombox-backup-agent/storage"
)

type BackupReport struct {
	Success   bool
	Date      string
	Path      string
	Manifest  *manifest.Manifest
	Results   []*modules.BackupResult
	TotalSize int64
	Error     string
}

func Run(cfg *config.Config) *BackupReport {
	report := &BackupReport{
		Date:    time.Now().Format("2006-01-02"),
		Results: []*modules.BackupResult{},
	}

	logs.Init("logs")
	logs.Info("=== Kroombox Backup Agent ===")

	disp := progress.New()
	disp.Header()

	detection := detect.Detect()

	backupName := report.Date
	workDir := filepath.Join("/tmp", "kba-"+backupName)
	os.RemoveAll(workDir)
	os.MkdirAll(workDir, 0755)

	m := manifest.New(detection.Hostname, detection.OS, detection.Kernel, detection.Arch)

	allModules := modules.GetModules()
	layerOrder := []string{}
	layerMap := make(map[string]*progress.Layer)

	for _, mod := range allModules {
		if isDisabled(mod.Name(), cfg) {
			continue
		}
		l := disp.AddLayer(mod.Name())
		layerMap[mod.Name()] = l
		layerOrder = append(layerOrder, mod.Name())
	}

	disp.Render()

	for _, name := range layerOrder {
		l := layerMap[name]
		mod := findModule(allModules, name)

		l.Start()
		disp.Render()

		if !mod.Detect() {
			l.Skip()
			disp.Render()
			report.Results = append(report.Results, &modules.BackupResult{
				Name: name, Success: false, Skipped: true,
			})
			m.AddService(name, false)
			continue
		}

		// Animate progress while backing up
		done := make(chan bool)
		go func(layer *progress.Layer) {
			for i := 1; i <= 30; i++ {
				select {
				case <-done:
					return
				default:
					layer.SetProgress(float64(i) / 35.0)
					disp.Render()
					time.Sleep(120 * time.Millisecond)
				}
			}
		}(l)

		result, err := mod.Backup(workDir)
		close(done)

		if err != nil {
			l.Fail()
			disp.Render()
			logs.Error("[%s] error: %v", name, err)
			report.Results = append(report.Results, &modules.BackupResult{
				Name: name, Success: false, Error: err.Error(),
			})
			m.AddService(name, false)
			continue
		}

		l.SetProgress(1.0)
		if result.Skipped {
			l.Skip()
		} else if result.Success {
			l.Done()
		} else {
			l.Fail()
		}
		disp.Render()
		report.Results = append(report.Results, result)
		report.TotalSize += result.Size
		m.AddService(name, result.Success)
		logs.Info("[%s] done (%s)", name, modules.FormatSize(result.Size))
	}

	m.SetSize(modules.FormatSize(report.TotalSize))
	m.Save(workDir)
	report.Manifest = m

	// Save layer
	saveLayer := disp.AddLayer("save")
	saveLayer.Start()
	disp.Render()

	done := make(chan bool)
	go func() {
		for i := 1; i <= 20; i++ {
			select {
			case <-done:
				return
			default:
				saveLayer.SetProgress(float64(i) / 25.0)
				disp.Render()
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()

	provider, err := storage.NewProvider(cfg.Storage.Type, cfg.Storage.Destination)
	if err != nil {
		report.Error = fmt.Sprintf("storage: %v", err)
		report.Success = false
		close(done)
		saveLayer.Fail()
		disp.Render()
		logs.Error(report.Error)
		return report
	}

	if err := provider.Save(workDir, backupName); err != nil {
		report.Error = fmt.Sprintf("save: %v", err)
		report.Success = false
		close(done)
		saveLayer.Fail()
		disp.Render()
		logs.Error(report.Error)
		return report
	}

	close(done)
	saveLayer.SetProgress(1.0)
	saveLayer.Done()
	disp.Render()
	os.RemoveAll(workDir)

	report.Path = filepath.Join(cfg.Storage.Destination, backupName)
	report.Success = true

	// Record in DB
	svcNames := []string{}
	for _, res := range report.Results {
		if res.Success { svcNames = append(svcNames, res.Name) }
	}
	svcStr := strings.Join(svcNames, ",")
	db.SaveBackup(report.Path, report.Date, report.TotalSize, svcStr, detection.Hostname, "ok")

	disp.Done()
	fmt.Printf(" status: %s backed up (%s)\n\n",
		"\033[32m\033[1m\u2714\033[0m",
		"\033[1m"+modules.FormatSize(report.TotalSize)+"\033[0m")

	logs.Info("Backup completed! Location: %s", report.Path)
	logs.Info("Total size: %s", modules.FormatSize(report.TotalSize))

	return report
}

func isDisabled(name string, cfg *config.Config) bool {
	switch name {
	case "mysql": return cfg.Backup.MySQL == "disabled"
	case "postgres": return cfg.Backup.Postgres == "disabled"
	case "mongodb": return cfg.Backup.MongoDB == "disabled"
	case "nginx": return cfg.Backup.Nginx == "disabled"
	case "pm2": return cfg.Backup.PM2 == "disabled"
	case "docker": return cfg.Backup.Docker == "disabled"
	case "ssl": return cfg.Backup.SSL == "disabled"
	case "git": return cfg.Backup.Git == "disabled"
	case "cron": return cfg.Backup.Cron == "disabled"
	}
	return false
}

func findModule(mods []modules.BackupModule, name string) modules.BackupModule {
	for _, m := range mods {
		if m.Name() == name {
			return m
		}
	}
	return nil
}
