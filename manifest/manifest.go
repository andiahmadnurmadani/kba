package manifest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Manifest struct {
	Hostname   string          `json:"hostname"`
	OS         string          `json:"os"`
	Kernel     string          `json:"kernel"`
	Arch       string          `json:"arch"`
	BackupDate string          `json:"backup_date"`
	Services   map[string]bool `json:"services"`
	Size       string          `json:"size"`
}

func New(hostname, os, kernel, arch string) *Manifest {
	return &Manifest{
		Hostname:   hostname,
		OS:         os,
		Kernel:     kernel,
		Arch:       arch,
		BackupDate: time.Now().Format("2006-01-02"),
		Services:   make(map[string]bool),
	}
}

func (m *Manifest) AddService(name string, backedUp bool) {
	m.Services[name] = backedUp
}

func (m *Manifest) SetSize(size string) {
	m.Size = size
}

func (m *Manifest) Save(dir string) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	path := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	return nil
}

func Load(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	return &m, nil
}

