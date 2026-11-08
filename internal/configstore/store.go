package configstore

import (
	"github.com/blak0p/attack-shark-linux/internal/device"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

)

const schemaVersion = 1

type appliedState struct {
	Version int          `json:"version"`
	DPI     device.DPIConfig `json:"dpi"`
}
type Store struct{ appliedPath, factoryPath string }

func New(appliedPath, factoryPath string) Store {
	return Store{appliedPath: appliedPath, factoryPath: factoryPath}
}
func (s Store) SaveApplied(config device.DPIConfig) error {
	return s.saveState(config, s.appliedPath)
}
func (s Store) saveState(config device.DPIConfig, path string) error {
	contents, err := json.Marshal(appliedState{Version: schemaVersion, DPI: config})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".state-*.tmp")
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
	return os.Rename(name, path)
}
func (s Store) LoadApplied() (device.DPIConfig, error) {
	state, err := s.loadState(s.appliedPath)
	if err != nil {
		return device.DPIConfig{}, err
	}
	return state.DPI, nil
}
func (s Store) LoadFactory() (device.DPIConfig, error) {
	state, err := s.loadState(s.factoryPath)
	if err == nil && state.DPI.StageMask != 0 {
		return state.DPI, nil
	}
	factory := device.DefaultDPIConfig()
	if seedErr := s.saveState(factory, s.factoryPath); seedErr != nil {
		return factory, seedErr
	}
	return factory, nil
}
func (s Store) loadState(path string) (appliedState, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return appliedState{}, err
	}
	var state appliedState
	if err := json.Unmarshal(contents, &state); err != nil {
		return appliedState{}, err
	}
	if state.Version != schemaVersion {
		return appliedState{}, fmt.Errorf("unsupported applied-state schema %d", state.Version)
	}
	return state, nil
}
