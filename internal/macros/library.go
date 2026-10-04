// Package macros stores an app-wide local macro library, independent of devices.
// Events express local mouse semantics; delay_ms is a nonnegative local duration,
// not a claim about firmware units, minimums, encoding, or playback support.
// A future device encoder must validate its own supported boundary. Share one
// Library instance across the app; separate instances/processes are not coordinated.
package macros

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// EventType identifies the local input involved in an event.
type EventType string

const (
	MouseLeft  EventType = "mouse_left"
	MouseRight EventType = "mouse_right"
)

// Action describes a press or release, not a firmware opcode.
type Action string

const (
	Down Action = "down"
	Up   Action = "up"
)

type Event struct {
	Type    EventType `json:"type"`
	Action  Action    `json:"action"`
	DelayMS int64     `json:"delay_ms"`
}

type Macro struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Events []Event `json:"events"`
}

var ErrNotFound = errors.New("macro not found")

const schemaVersion = 1

type document struct {
	Version int     `json:"version"`
	Macros  []Macro `json:"macros"`
}

// Library serializes all reads and atomic persistence within this instance.
// Successful mutations replace the file before becoming visible in memory.
// File data is synced before rename; directory-entry crash durability is not promised.
type Library struct {
	mu     sync.RWMutex
	path   string
	macros []Macro
}

// Open loads a versioned file. A missing file is an empty library; any other
// read, schema, or validation error leaves the file untouched and returns no library.
func Open(path string) (*Library, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("macro file path is blank")
	}
	lib := &Library{path: path, macros: []Macro{}}
	contents, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return lib, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read macros: %w", err)
	}
	var doc document
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&doc); err != nil {
		return nil, fmt.Errorf("decode macros: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, errors.New("macro file contains trailing data")
	}
	if doc.Version != schemaVersion {
		return nil, fmt.Errorf("unsupported macro schema %d", doc.Version)
	}
	seen := make(map[string]bool)
	for i, m := range doc.Macros {
		if strings.TrimSpace(m.ID) == "" {
			return nil, fmt.Errorf("macro %d: blank ID", i)
		}
		if seen[m.ID] {
			return nil, fmt.Errorf("macro %d: duplicate ID %q", i, m.ID)
		}
		seen[m.ID] = true
		if err := validate(m.Name, m.Events); err != nil {
			return nil, fmt.Errorf("macro %d: %w", i, err)
		}
		lib.macros = append(lib.macros, clone(m))
	}
	return lib, nil
}

func validate(name string, events []Event) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("macro name is blank")
	}
	for i, event := range events {
		if event.Type != MouseLeft && event.Type != MouseRight {
			return fmt.Errorf("event %d: unknown type %q", i, event.Type)
		}
		if event.Action != Down && event.Action != Up {
			return fmt.Errorf("event %d: unknown action %q", i, event.Action)
		}
		if event.DelayMS < 0 {
			return fmt.Errorf("event %d: delay_ms is negative", i)
		}
	}
	return nil
}

func clone(m Macro) Macro {
	m.Events = append([]Event{}, m.Events...)
	return m
}

func (l *Library) List() []Macro {
	l.mu.RLock()
	defer l.mu.RUnlock()
	result := make([]Macro, len(l.macros))
	for i, m := range l.macros {
		result[i] = clone(m)
	}
	return result
}

func (l *Library) Read(id string) (Macro, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	for _, m := range l.macros {
		if m.ID == id {
			return clone(m), nil
		}
	}
	return Macro{}, ErrNotFound
}

// Create permits empty drafts and duplicate names, but never duplicate IDs.
func (l *Library) Create(name string, events []Event) (Macro, error) {
	if err := validate(name, events); err != nil {
		return Macro{}, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	var id string
	for {
		var random [16]byte
		if _, err := rand.Read(random[:]); err != nil {
			return Macro{}, fmt.Errorf("generate macro ID: %w", err)
		}
		id = hex.EncodeToString(random[:])
		collision := false
		for _, m := range l.macros {
			if m.ID == id {
				collision = true
				break
			}
		}
		if !collision {
			break
		}
	}
	m := clone(Macro{ID: id, Name: name, Events: events})
	next := append(append([]Macro{}, l.macros...), m)
	if err := l.persist(next); err != nil {
		return Macro{}, err
	}
	l.macros = next
	return clone(m), nil
}

// Update replaces the name and ordered events while preserving identity.
func (l *Library) Update(id, name string, events []Event) (Macro, error) {
	if err := validate(name, events); err != nil {
		return Macro{}, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	for i, m := range l.macros {
		if m.ID != id {
			continue
		}
		updated := clone(Macro{ID: id, Name: name, Events: events})
		next := append([]Macro{}, l.macros...)
		next[i] = updated
		if err := l.persist(next); err != nil {
			return Macro{}, err
		}
		l.macros = next
		return clone(updated), nil
	}
	return Macro{}, ErrNotFound
}

func (l *Library) Delete(id string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	for i, m := range l.macros {
		if m.ID != id {
			continue
		}
		next := append([]Macro{}, l.macros[:i]...)
		next = append(next, l.macros[i+1:]...)
		if err := l.persist(next); err != nil {
			return err
		}
		l.macros = next
		return nil
	}
	return ErrNotFound
}

func (l *Library) persist(macros []Macro) error {
	contents, err := json.Marshal(document{Version: schemaVersion, Macros: macros})
	if err != nil {
		return err
	}
	directory := filepath.Dir(l.path)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return fmt.Errorf("create macro directory: %w", err)
	}
	file, err := os.CreateTemp(directory, ".macros-*.tmp")
	if err != nil {
		return fmt.Errorf("create macro temporary file: %w", err)
	}
	// CreateTemp uses 0600. Cleanup also covers write, sync, close and rename errors.
	defer os.Remove(file.Name())
	if _, err = file.Write(contents); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("write macros: %w", err)
	}
	if err := os.Rename(file.Name(), l.path); err != nil {
		return fmt.Errorf("replace macros: %w", err)
	}
	return nil
}
