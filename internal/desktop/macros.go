package desktop

import (
	"errors"
	"fmt"

	"github.com/blak0p/attack-shark-linux/internal/macros"
)

// WithMacroLibrary injects the single app-wide library or its startup error.
// An open failure must remain an error, never an empty replacement library.
func WithMacroLibrary(library *macros.Library, err error) Option {
	return func(s *Service) { s.macroLibrary, s.macroLibraryError = library, err }
}

func (s *Service) localMacros() (*macros.Library, error) {
	if s.macroLibraryError != nil {
		return nil, fmt.Errorf("macro library unavailable: %w", s.macroLibraryError)
	}
	if s.macroLibrary == nil {
		return nil, errors.New("macro library unavailable: not configured")
	}
	return s.macroLibrary, nil
}

// ListMacros reads app data without requiring inventory or a selected device.
func (s *Service) ListMacros() ([]macros.Macro, error) {
	library, err := s.localMacros()
	if err != nil {
		return nil, err
	}
	return library.List(), nil
}

func (s *Service) ReadMacro(id string) (macros.Macro, error) {
	library, err := s.localMacros()
	if err != nil {
		return macros.Macro{}, err
	}
	return library.Read(id)
}

func (s *Service) CreateMacro(name string, events []macros.Event) (macros.Macro, error) {
	library, err := s.localMacros()
	if err != nil {
		return macros.Macro{}, err
	}
	return library.Create(name, events)
}

func (s *Service) UpdateMacro(id, name string, events []macros.Event) (macros.Macro, error) {
	library, err := s.localMacros()
	if err != nil {
		return macros.Macro{}, err
	}
	return library.Update(id, name, events)
}

func (s *Service) DeleteMacro(id string) error {
	library, err := s.localMacros()
	if err != nil {
		return err
	}
	return library.Delete(id)
}
