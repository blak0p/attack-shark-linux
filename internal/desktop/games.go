package desktop

import (
	"path/filepath"
	"sync"
)

const (
	GamesUnavailable ErrorCode = "games_unavailable"
	PickFailed       ErrorCode = "pick_failed"
)

// InstalledGame is one installed game or app offered by the rule picker. It
// has no JSON tags so the frontend sees PascalCase keys like every desktop type.
type InstalledGame struct {
	Name       string
	Source     string
	Folder     string
	Executable string
	IsGame     bool
}

// GameListSnapshot carries the installed games plus an optional error.
type GameListSnapshot struct {
	Games []InstalledGame
	Error Error
}

// PickResult is the outcome of a native file or folder dialog. An empty Path
// with Cancelled set means the user closed the dialog.
type PickResult struct {
	Path      string
	Name      string
	Cancelled bool
	Error     Error
}

// gameComponent holds the game catalog and picker of one Service. Its zero
// value is usable: nothing attached means games_unavailable.
type gameComponent struct {
	mu      sync.Mutex // guards catalog and picker
	pickMu  sync.Mutex // held while a native dialog is open
	catalog func() []InstalledGame
	picker  func() (string, error)
}

func (s *Service) games() *gameComponent { return &s.gameCatalog }

// AttachGameCatalog wires the installed-game lister before publication.
func (s *Service) AttachGameCatalog(list func() []InstalledGame) *Service {
	c := s.games()
	c.mu.Lock()
	c.catalog = list
	c.mu.Unlock()
	return s
}

// AttachGamePicker wires the native dialog opener before publication.
func (s *Service) AttachGamePicker(pick func() (string, error)) *Service {
	c := s.games()
	c.mu.Lock()
	c.picker = pick
	c.mu.Unlock()
	return s
}

// ListInstalledGames returns the catalog as given; Games is never nil.
func (s *Service) ListInstalledGames() GameListSnapshot {
	c := s.games()
	c.mu.Lock()
	list := c.catalog
	c.mu.Unlock()
	if list == nil {
		return GameListSnapshot{Games: []InstalledGame{}, Error: Error{Code: GamesUnavailable}}
	}
	games := append([]InstalledGame{}, list()...)
	return GameListSnapshot{Games: games}
}

// PickGameFolder opens a native dialog restricted to directories.
func (s *Service) PickGameFolder() PickResult {
	c := s.games()
	c.mu.Lock()
	picker := c.picker
	c.mu.Unlock()
	if picker == nil {
		return PickResult{Error: Error{Code: GamesUnavailable}}
	}
	if !c.pickMu.TryLock() {
		return PickResult{Error: Error{Code: PickFailed}}
	}
	defer c.pickMu.Unlock()
	path, err := picker()
	if err != nil {
		return PickResult{Error: Error{Code: PickFailed}}
	}
	if path == "" {
		return PickResult{Cancelled: true}
	}
	if !filepath.IsAbs(path) {
		return PickResult{Error: Error{Code: PickFailed}}
	}
	path = filepath.Clean(path)
	return PickResult{Path: path, Name: filepath.Base(path)}
}
