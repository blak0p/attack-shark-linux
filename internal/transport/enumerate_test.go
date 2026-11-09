package transport

import (
	"context"
	"testing"
)

type singleBackend struct {
	passiveTransportStub
	matches []Match
}

func (b *singleBackend) Enumerate(_ context.Context, m Match) ([]Candidate, error) {
	b.matches = append(b.matches, m)
	return []Candidate{{Path: "single"}}, nil
}

type multiBackend struct {
	singleBackend
	got []Match
}

func (b *multiBackend) EnumerateMatches(_ context.Context, m ...Match) ([]Candidate, error) {
	b.got = m
	return []Candidate{{Path: "multi"}}, nil
}

func TestEnumerateAllUsesMultiEnumeratorWhenImplemented(t *testing.T) {
	a, b := Match{VendorID: 1, ProductID: 2}, Match{VendorID: 1, ProductID: 3}
	backend := &multiBackend{}
	got, err := EnumerateAll(context.Background(), backend, a, b)
	if err != nil || len(got) != 1 || got[0].Path != "multi" {
		t.Fatalf("EnumerateAll() = %v, %v", got, err)
	}
	if len(backend.got) != 2 || backend.got[0] != a || backend.got[1] != b || len(backend.matches) != 0 {
		t.Fatalf("matches = %v, single calls = %v", backend.got, backend.matches)
	}
}

func TestEnumerateAllFallsBackToFirstMatch(t *testing.T) {
	a, b := Match{VendorID: 1, ProductID: 2}, Match{VendorID: 1, ProductID: 3}
	backend := &singleBackend{}
	got, err := EnumerateAll(context.Background(), backend, a, b)
	if err != nil || len(got) != 1 || got[0].Path != "single" {
		t.Fatalf("EnumerateAll() = %v, %v", got, err)
	}
	if len(backend.matches) != 1 || backend.matches[0] != a {
		t.Fatalf("fallback matches = %v, want only %v", backend.matches, a)
	}
}
