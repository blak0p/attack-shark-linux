package configstore

import (
	"errors"
	"fmt"
)

// ErrProfileNotFound means no profile matches the reference.
var ErrProfileNotFound = errors.New("profile not found")

// ErrAmbiguousProfile means several profiles share the referenced name.
var ErrAmbiguousProfile = errors.New("profile name is ambiguous")

// AmbiguousProfileError carries the IDs of every profile with the name.
type AmbiguousProfileError struct {
	Name string
	IDs  []string
}

func (e *AmbiguousProfileError) Error() string {
	return fmt.Sprintf("%v: %q matches %d profiles", ErrAmbiguousProfile, e.Name, len(e.IDs))
}

func (e *AmbiguousProfileError) Is(target error) bool { return target == ErrAmbiguousProfile }

// ResolveProfileRef maps a profile name (exact, case-sensitive) to its ID.
// A raw profile ID of an existing profile is accepted when no name matches.
func ResolveProfileRef(profiles []ProfileSummary, ref string) (string, error) {
	var ids []string
	for _, p := range profiles {
		if p.Name == ref {
			ids = append(ids, p.ID)
		}
	}
	switch len(ids) {
	case 1:
		return ids[0], nil
	case 0:
	default:
		return "", &AmbiguousProfileError{Name: ref, IDs: ids}
	}
	if validProfileID(ref) {
		for _, p := range profiles {
			if p.ID == ref {
				return ref, nil
			}
		}
	}
	return "", fmt.Errorf("%w: %q", ErrProfileNotFound, ref)
}
