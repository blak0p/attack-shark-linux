package configstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/alejandro/attack-shark-linux/internal/x6"
)

const schemaVersion = 1

type appliedState struct {
	Version int          `json:"version"`
	DPI     x6.DPIConfig `json:"dpi"`
}
type Store struct{ appliedPath, factoryPath string }

func New(appliedPath, factoryPath string) Store {
	return Store{appliedPath: appliedPath, factoryPath: factoryPath}
}

func (s Store) SaveApplied(config x6.DPIConfig) error {
	contents, err := json.Marshal(appliedState{Version: schemaVersion, DPI: config})
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(s.appliedPath), ".applied-*.tmp")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	if _, err = file.Write(contents); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, s.appliedPath)
}

func (s Store) LoadApplied() (x6.DPIConfig, error) {
	contents, err := os.ReadFile(s.appliedPath)
	if err != nil {
		return x6.DPIConfig{}, err
	}
	var state appliedState
	if err := json.Unmarshal(contents, &state); err != nil {
		return x6.DPIConfig{}, err
	}
	if state.Version != schemaVersion {
		return x6.DPIConfig{}, fmt.Errorf("unsupported applied-state schema %d", state.Version)
	}
	return state.DPI, nil
}
