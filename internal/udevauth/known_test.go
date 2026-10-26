package udevauth_test

import (
	"reflect"
	"sort"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/udevauth"
)

func TestKnownOfficialHistoryIsIndependent(t *testing.T) {
	known := udevauth.KnownOfficialDigests()
	want := []string{udevauth.PreviousSHA256, udevauth.InitialUnifiedSHA256}
	sort.Strings(want)
	if !reflect.DeepEqual(known, want) {
		t.Fatalf("known history=%v", known)
	}
	known[0] = "modified"
	if !reflect.DeepEqual(udevauth.KnownOfficialDigests(), want) {
		t.Fatal("caller mutated signing history")
	}
}
