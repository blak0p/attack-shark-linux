package configstore

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
)

var (
	ErrInvalidProfile   = errors.New("invalid local profile")
	ErrProtectedProfile = errors.New("cannot delete active or last profile")
)

// ProfileSummary is local metadata, not an onboard hardware profile.
type ProfileSummary struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt"`
}
type ProfileSnapshot struct {
	Profiles []ProfileSummary `json:"profiles"`
	ActiveID string           `json:"activeID"`
	Notice   string           `json:"notice"`
}
type LocalProfile struct {
	Version       int             `json:"version"`
	ID            string          `json:"id"`
	Configuration json.RawMessage `json:"configuration"`
}
type profileIndex struct {
	Version         int    `json:"version"`
	DeviceKey       string `json:"deviceKey"`
	HardwareProfile string `json:"hardwareProfile"`
	ProfileSnapshot
}

// ProfileStore serializes operations within this instance. Use one instance per
// application root; concurrent processes writing the same root are unsupported.
// Configuration is an opaque JSON object: callers validate model-specific schema
// and supply all settings, including lighting and macro assignments.
type ProfileStore struct {
	root string
	mu   sync.Mutex
}

func NewProfileStore(root string) *ProfileStore { return &ProfileStore{root: root} }
func (s *ProfileStore) directory(id mouse.DeviceID, binding string) (string, error) {
	if err := id.Validate(); err != nil {
		return "", err
	}
	if strings.TrimSpace(binding) == "" {
		return "", ErrInvalidProfile
	}
	sum := sha256.Sum256([]byte(id.Key()))
	return filepath.Join(s.root, hex.EncodeToString(sum[:])), nil
}
func validProfileID(id string) bool {
	b, err := hex.DecodeString(id)
	return err == nil && len(b) == 16 && id == strings.ToLower(id)
}
func validProfileName(name string) bool {
	if !utf8.ValidString(name) || strings.TrimSpace(name) != name || name == "" || utf8.RuneCountInString(name) > 80 || strings.ContainsAny(name, "/\\") || name == "." || name == ".." {
		return false
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
func validConfiguration(raw json.RawMessage) bool {
	return json.Valid(raw) && strings.HasPrefix(strings.TrimSpace(string(raw)), "{")
}
func (s *ProfileStore) load(id mouse.DeviceID, binding string) (profileIndex, string, error) {
	dir, err := s.directory(id, binding)
	if err != nil {
		return profileIndex{}, dir, err
	}
	raw, err := os.ReadFile(filepath.Join(dir, "index.json"))
	if err != nil {
		return profileIndex{}, dir, err
	}
	var index profileIndex
	if err = json.Unmarshal(raw, &index); err != nil {
		return index, dir, err
	}
	if index.Version != 1 || index.DeviceKey != id.Key() || index.HardwareProfile != binding || len(index.Profiles) == 0 {
		return index, dir, ErrIncompatibleRecord
	}
	seen := map[string]bool{}
	active := false
	for _, p := range index.Profiles {
		if !validProfileID(p.ID) || !validProfileName(p.Name) || seen[p.ID] {
			return index, dir, ErrInvalidProfile
		}
		seen[p.ID] = true
		if _, err = time.Parse(time.RFC3339Nano, p.CreatedAt); err != nil {
			return index, dir, ErrInvalidProfile
		}
		if p.ID == index.ActiveID {
			active = true
		}
		if _, err = readLocalProfile(dir, p.ID); err != nil {
			return index, dir, err
		}
	}
	if !active {
		return index, dir, ErrInvalidProfile
	}
	return index, dir, nil
}
func readLocalProfile(dir, id string) (LocalProfile, error) {
	var p LocalProfile
	if !validProfileID(id) {
		return p, ErrInvalidProfile
	}
	raw, err := os.ReadFile(filepath.Join(dir, id+".json"))
	if err != nil {
		return p, err
	}
	if err = json.Unmarshal(raw, &p); err != nil {
		return p, err
	}
	if p.Version != 1 || p.ID != id || !validConfiguration(p.Configuration) {
		return p, ErrInvalidProfile
	}
	return p, nil
}
func writeProfileIndex(dir string, index profileIndex) error {
	return writeProfileJSON(filepath.Join(dir, "index.json"), index)
}

// Existing atomic writers are methods tied to incompatible envelopes. This
// writer follows their temp/write/sync/rename pattern and syncs the directory.
func writeProfileJSON(path string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".profile-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(raw); err == nil {
		err = f.Sync()
	}
	if e := f.Close(); err == nil {
		err = e
	}
	if err != nil {
		return err
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return err
	}
	d, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
func newProfileSummary(name string) (ProfileSummary, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return ProfileSummary{}, err
	}
	return ProfileSummary{ID: hex.EncodeToString(b), Name: name, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}, nil
}

// Bootstrap only seeds a genuinely absent device directory. Existing or damaged
// data is never replaced; legacy loading and backup belong to the caller.
func (s *ProfileStore) Bootstrap(id mouse.DeviceID, binding string, seed json.RawMessage) (ProfileSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	index, dir, err := s.load(id, binding)
	if err == nil {
		return index.ProfileSnapshot, nil
	}
	if !os.IsNotExist(err) {
		return ProfileSnapshot{}, err
	}
	if _, statErr := os.Stat(dir); !os.IsNotExist(statErr) {
		return ProfileSnapshot{}, fmt.Errorf("profile directory already exists: %w", ErrInvalidProfile)
	}
	if !validConfiguration(seed) {
		return ProfileSnapshot{}, ErrInvalidProfile
	}
	p, err := newProfileSummary("Automatic profile")
	if err != nil {
		return ProfileSnapshot{}, err
	}
	index = profileIndex{Version: 1, DeviceKey: id.Key(), HardwareProfile: binding, ProfileSnapshot: ProfileSnapshot{Profiles: []ProfileSummary{p}, ActiveID: p.ID, Notice: fmt.Sprintf("Automatic profile created at %s from local settings. No mouse settings were changed.", p.CreatedAt)}}
	if err = writeProfileJSON(filepath.Join(dir, p.ID+".json"), LocalProfile{1, p.ID, seed}); err == nil {
		err = writeProfileIndex(dir, index)
	}
	return index.ProfileSnapshot, err
}
func (s *ProfileStore) List(id mouse.DeviceID, binding string) (ProfileSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	index, _, err := s.load(id, binding)
	return index.ProfileSnapshot, err
}
func (s *ProfileStore) Read(id mouse.DeviceID, binding, profileID string) (LocalProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	index, dir, err := s.load(id, binding)
	if err != nil {
		return LocalProfile{}, err
	}
	if profilePosition(index, profileID) < 0 {
		return LocalProfile{}, ErrInvalidProfile
	}
	return readLocalProfile(dir, profileID)
}
func profilePosition(index profileIndex, id string) int {
	for i, p := range index.Profiles {
		if p.ID == id {
			return i
		}
	}
	return -1
}

// Create clones the active saved payload and activates it, without hardware I/O.
func (s *ProfileStore) Create(id mouse.DeviceID, binding, name string) (ProfileSnapshot, error) {
	return s.clone(id, binding, "", name, true)
}

// Duplicate clones a saved payload without changing the active pointer.
func (s *ProfileStore) Duplicate(id mouse.DeviceID, binding, sourceID, name string) (ProfileSnapshot, error) {
	return s.clone(id, binding, sourceID, name, false)
}
func (s *ProfileStore) clone(id mouse.DeviceID, binding, sourceID, name string, activate bool) (ProfileSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !validProfileName(name) {
		return ProfileSnapshot{}, ErrInvalidProfile
	}
	index, dir, err := s.load(id, binding)
	if err != nil {
		return ProfileSnapshot{}, err
	}
	if activate {
		sourceID = index.ActiveID
	}
	if profilePosition(index, sourceID) < 0 {
		return ProfileSnapshot{}, ErrInvalidProfile
	}
	source, err := readLocalProfile(dir, sourceID)
	if err != nil {
		return ProfileSnapshot{}, err
	}
	p, err := newProfileSummary(name)
	if err != nil {
		return ProfileSnapshot{}, err
	}
	// Publish the complete file before making it reachable from metadata.
	if err = writeProfileJSON(filepath.Join(dir, p.ID+".json"), LocalProfile{1, p.ID, source.Configuration}); err != nil {
		return ProfileSnapshot{}, err
	}
	index.Profiles = append(index.Profiles, p)
	if activate {
		index.ActiveID = p.ID
	}
	return index.ProfileSnapshot, writeProfileIndex(dir, index)
}
func (s *ProfileStore) Rename(id mouse.DeviceID, binding, profileID, name string) (ProfileSnapshot, error) {
	return s.update(id, binding, func(index *profileIndex) error {
		if !validProfileName(name) {
			return ErrInvalidProfile
		}
		pos := profilePosition(*index, profileID)
		if pos < 0 {
			return ErrInvalidProfile
		}
		index.Profiles[pos].Name = name
		return nil
	})
}

// Switch changes only local metadata; the caller owns device application.
func (s *ProfileStore) Switch(id mouse.DeviceID, binding, profileID string) (ProfileSnapshot, error) {
	return s.update(id, binding, func(index *profileIndex) error {
		if profilePosition(*index, profileID) < 0 {
			return ErrInvalidProfile
		}
		index.ActiveID = profileID
		return nil
	})
}
func (s *ProfileStore) DismissNotice(id mouse.DeviceID, binding string) (ProfileSnapshot, error) {
	return s.update(id, binding, func(index *profileIndex) error { index.Notice = ""; return nil })
}
func (s *ProfileStore) update(id mouse.DeviceID, binding string, change func(*profileIndex) error) (ProfileSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	index, dir, err := s.load(id, binding)
	if err != nil {
		return ProfileSnapshot{}, err
	}
	if err = change(&index); err != nil {
		return ProfileSnapshot{}, err
	}
	return index.ProfileSnapshot, writeProfileIndex(dir, index)
}
func (s *ProfileStore) SaveActive(id mouse.DeviceID, binding string, configuration json.RawMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !validConfiguration(configuration) {
		return ErrInvalidProfile
	}
	index, dir, err := s.load(id, binding)
	if err != nil {
		return err
	}
	return writeProfileJSON(filepath.Join(dir, index.ActiveID+".json"), LocalProfile{1, index.ActiveID, configuration})
}
func (s *ProfileStore) Delete(id mouse.DeviceID, binding, profileID string) (ProfileSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	index, dir, err := s.load(id, binding)
	if err != nil {
		return ProfileSnapshot{}, err
	}
	pos := profilePosition(index, profileID)
	if pos < 0 {
		return ProfileSnapshot{}, ErrInvalidProfile
	}
	if len(index.Profiles) == 1 || index.ActiveID == profileID {
		return ProfileSnapshot{}, ErrProtectedProfile
	}
	index.Profiles = append(index.Profiles[:pos], index.Profiles[pos+1:]...)
	// Unpublish before removing the now unreachable file. A crash may leave an
	// orphan, never an index pointing at a missing profile.
	if err = writeProfileIndex(dir, index); err != nil {
		return ProfileSnapshot{}, err
	}
	return index.ProfileSnapshot, os.Remove(filepath.Join(dir, profileID+".json"))
}
