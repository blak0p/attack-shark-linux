package configstore

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
)

const deviceSchemaVersion = 2

var ErrIncompatibleRecord = errors.New("incompatible device record")
var ErrTransientPath = errors.New("transient path cannot be persisted")
var ErrMigrationNoTarget = errors.New("migration has no target")
var ErrMigrationSelectionRequired = errors.New("migration selection required")
var ErrMigrationCancelled = errors.New("migration cancelled")
var ErrMigrationTargetExists = errors.New("migration target already exists")

type DeviceRecord struct {
	Profile       string          `json:"profile"`
	Model         string          `json:"model"`
	ConfigVersion int             `json:"configVersion"`
	Configuration json.RawMessage `json:"configuration"`
}
type migrationRecord struct{ SourceHash, TargetKey, Status string }
type deviceEnvelope struct {
	Version    int                        `json:"version"`
	Devices    map[string]DeviceRecord    `json:"devices"`
	Migrations map[string]migrationRecord `json:"migrations"`
}

type DeviceStore struct {
	path string
	mu   sync.Mutex
}

func NewDeviceStore(path string) *DeviceStore { return &DeviceStore{path: path} }

func (s *DeviceStore) Save(id mouse.DeviceID, profile, model string, configVersion int, configuration any) error {
	if err := id.Validate(); err != nil {
		return err
	}
	contents, err := json.Marshal(configuration)
	if err != nil {
		return err
	}
	if hasPath(contents) {
		return ErrTransientPath
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	envelope, err := s.read()
	if err != nil {
		return err
	}
	key := id.Key()
	if current, exists := envelope.Devices[key]; exists && current.Profile != profile {
		return ErrIncompatibleRecord
	}
	envelope.Devices[key] = DeviceRecord{Profile: profile, Model: model, ConfigVersion: configVersion, Configuration: contents}
	return s.write(envelope)
}

func (s *DeviceStore) Load(id mouse.DeviceID, profile string, destination any) error {
	if err := id.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	envelope, err := s.read()
	if err != nil {
		return err
	}
	record, ok := envelope.Devices[id.Key()]
	if !ok {
		return os.ErrNotExist
	}
	if record.Profile != profile {
		return ErrIncompatibleRecord
	}
	return json.Unmarshal(record.Configuration, destination)
}

type MigrationTarget struct {
	ID             mouse.DeviceID
	Profile, Model string
}
type MigrationStatus string

const (
	MigrationImported        MigrationStatus = "imported"
	MigrationAlreadyImported MigrationStatus = "already_imported"
)

type MigrationResult struct {
	Status                MigrationStatus
	SourceHash, TargetKey string
}

func (s *DeviceStore) MigrateV1(appliedPath, factoryPath string, targets []MigrationTarget, cancelled bool) (MigrationResult, error) {
	if cancelled {
		return MigrationResult{}, ErrMigrationCancelled
	}
	if len(targets) == 0 {
		return MigrationResult{}, ErrMigrationNoTarget
	}
	if len(targets) != 1 {
		return MigrationResult{}, ErrMigrationSelectionRequired
	}
	target := targets[0]
	if err := target.ID.Validate(); err != nil {
		return MigrationResult{}, err
	}
	applied, err := readV1(appliedPath)
	if err != nil {
		return MigrationResult{}, err
	}
	factory, err := readV1(factoryPath)
	if err != nil {
		return MigrationResult{}, err
	}
	if hasPath(applied.DPI) {
		return MigrationResult{}, ErrTransientPath
	}
	sum := sha256.Sum256(append(append([]byte(nil), applied.Contents...), factory.Contents...))
	hash := hex.EncodeToString(sum[:])
	s.mu.Lock()
	defer s.mu.Unlock()
	envelope, err := s.read()
	if err != nil {
		return MigrationResult{}, err
	}
	if previous, ok := envelope.Migrations[hash]; ok && previous.TargetKey == target.ID.Key() && previous.Status == string(MigrationImported) {
		if record, ok := envelope.Devices[target.ID.Key()]; ok {
			if normalized, ok := normalizeMigratedConfiguration(record.Configuration); ok {
				record.Configuration = normalized
				envelope.Devices[target.ID.Key()] = record
				if err := s.write(envelope); err != nil {
					return MigrationResult{}, err
				}
			}
		}
		return MigrationResult{Status: MigrationAlreadyImported, SourceHash: hash, TargetKey: target.ID.Key()}, nil
	}
	if _, exists := envelope.Devices[target.ID.Key()]; exists {
		return MigrationResult{}, ErrMigrationTargetExists
	}
	if err := backup(appliedPath, applied.Contents); err != nil {
		return MigrationResult{}, err
	}
	if err := backup(factoryPath, factory.Contents); err != nil {
		return MigrationResult{}, err
	}
	envelope.Devices[target.ID.Key()] = DeviceRecord{Profile: target.Profile, Model: target.Model, ConfigVersion: 1, Configuration: applied.DPI}
	envelope.Migrations[hash] = migrationRecord{SourceHash: hash, TargetKey: target.ID.Key(), Status: string(MigrationImported)}
	if err := s.write(envelope); err != nil {
		return MigrationResult{}, err
	}
	return MigrationResult{Status: MigrationImported, SourceHash: hash, TargetKey: target.ID.Key()}, nil
}

func (s *DeviceStore) read() (deviceEnvelope, error) {
	contents, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return deviceEnvelope{Version: deviceSchemaVersion, Devices: map[string]DeviceRecord{}, Migrations: map[string]migrationRecord{}}, nil
	}
	if err != nil {
		return deviceEnvelope{}, err
	}
	var envelope deviceEnvelope
	if err := json.Unmarshal(contents, &envelope); err != nil {
		return deviceEnvelope{}, err
	}
	if envelope.Version != deviceSchemaVersion {
		return deviceEnvelope{}, fmt.Errorf("unsupported device-state schema %d", envelope.Version)
	}
	if envelope.Devices == nil {
		envelope.Devices = map[string]DeviceRecord{}
	}
	if envelope.Migrations == nil {
		envelope.Migrations = map[string]migrationRecord{}
	}
	return envelope, nil
}
func (s *DeviceStore) write(envelope deviceEnvelope) error {
	contents, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(s.path), ".devices-*.tmp")
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
	if err := os.Rename(name, s.path); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(s.path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

type legacyState struct {
	Contents json.RawMessage
	DPI      json.RawMessage `json:"dpi"`
}

func readV1(path string) (legacyState, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return legacyState{}, err
	}
	state, err := readLegacyContents(contents)
	if err != nil {
		return legacyState{}, err
	}
	state.Contents = contents
	return state, nil
}
func backup(path string, contents []byte) error {
	file, err := os.OpenFile(path+".v1.bak", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if os.IsExist(err) {
			existing, readErr := os.ReadFile(path + ".v1.bak")
			if readErr == nil && bytes.Equal(existing, contents) {
				return nil
			}
		}
		return err
	}
	_, err = file.Write(contents)
	if syncErr := file.Sync(); err == nil {
		err = syncErr
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	return err
}
func normalizeMigratedConfiguration(contents json.RawMessage) (json.RawMessage, bool) {
	var wrapper struct {
		Applied json.RawMessage `json:"applied"`
		Factory json.RawMessage `json:"factory"`
	}
	if err := json.Unmarshal(contents, &wrapper); err != nil || len(wrapper.Applied) == 0 || len(wrapper.Factory) == 0 {
		return nil, false
	}
	applied, err := readLegacyContents(wrapper.Applied)
	if err != nil {
		return nil, false
	}
	if _, err := readLegacyContents(wrapper.Factory); err != nil {
		return nil, false
	}
	return applied.DPI, true
}
func readLegacyContents(contents []byte) (legacyState, error) {
	var state legacyState
	var version struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(contents, &version); err != nil {
		return legacyState{}, err
	}
	if err := json.Unmarshal(contents, &state); err != nil {
		return legacyState{}, err
	}
	if version.Version != schemaVersion || len(state.DPI) == 0 {
		return legacyState{}, fmt.Errorf("unsupported legacy schema %d", version.Version)
	}
	return state, nil
}
func hasPath(contents []byte) bool {
	return strings.Contains(strings.ToLower(string(contents)), `"path"`)
}
