package configstore_test

import (
	"errors"
	"reflect"
	"sort"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/configstore"
)

const (
	idA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	idB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	idC = "cccccccccccccccccccccccccccccccc"
)

func TestResolveProfileRef(t *testing.T) {
	list := []configstore.ProfileSummary{{ID: idA, Name: "Work"}, {ID: idB, Name: "Game"}, {ID: idC, Name: idA}}
	tests := []struct {
		name    string
		profs   []configstore.ProfileSummary
		ref     string
		want    string
		wantErr error
	}{
		{"exact name", list, "Game", idB, nil},
		{"case differs", list, "game", "", configstore.ErrProfileNotFound},
		{"whitespace not trimmed", list, " Work", "", configstore.ErrProfileNotFound},
		{"existing hex id", list, idB, idB, nil},
		{"hex id not listed", list, "dddddddddddddddddddddddddddddddd", "", configstore.ErrProfileNotFound},
		{"name wins over another profile's id", list, idA, idC, nil},
		{"empty ref", list, "", "", configstore.ErrProfileNotFound},
		{"empty list", nil, "Work", "", configstore.ErrProfileNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := configstore.ResolveProfileRef(tc.profs, tc.ref)
			if !errors.Is(err, tc.wantErr) || got != tc.want {
				t.Fatalf("got (%q, %v), want (%q, %v)", got, err, tc.want, tc.wantErr)
			}
		})
	}
}

func TestResolveProfileRefAmbiguous(t *testing.T) {
	list := []configstore.ProfileSummary{{ID: idA, Name: "Dup"}, {ID: idB, Name: "Other"}, {ID: idC, Name: "Dup"}}
	got, err := configstore.ResolveProfileRef(list, "Dup")
	if got != "" || !errors.Is(err, configstore.ErrAmbiguousProfile) {
		t.Fatalf("got (%q, %v)", got, err)
	}
	var amb *configstore.AmbiguousProfileError
	if !errors.As(err, &amb) {
		t.Fatalf("not an AmbiguousProfileError: %v", err)
	}
	ids := append([]string(nil), amb.IDs...)
	sort.Strings(ids)
	if !reflect.DeepEqual(ids, []string{idA, idC}) {
		t.Fatalf("ids = %v", ids)
	}
}
