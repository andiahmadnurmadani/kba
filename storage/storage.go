package storage

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type Provider interface {
	Save(sourceDir, backupName string) error
	Name() string
}

type LocalProvider struct {
	Destination string
}

func NewLocalProvider(dest string) *LocalProvider {
	return &LocalProvider{Destination: dest}
}

func (p *LocalProvider) Name() string { return "local" }

func (p *LocalProvider) Save(sourceDir, backupName string) error {
	destDir := filepath.Join(p.Destination, backupName)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("create destination: %w", err)
	}

	// rsync or cp
	cmd := exec.Command("cp", "-r", "--no-preserve=mode,ownership", sourceDir+"/.", destDir+"/")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("copy backup: %s: %w", string(out), err)
	}
	return nil
}

func NewProvider(storageType, destination string) (Provider, error) {
	switch storageType {
	case "local":
		return NewLocalProvider(destination), nil
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", storageType)
	}
}

