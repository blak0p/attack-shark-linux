package x6

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestServiceReadsValidatedInputInPriorityOrder(t *testing.T) {
	tests := []struct {
		name       string
		candidates []Candidate
		validate   map[string]error
		read       map[string]error
		reports    map[string][]byte
		want       Status
		wantKind   ErrorKind
		wantLog    []string
	}{
		{"dongle takes precedence", []Candidate{candidate("dongle", Dongle), candidate("wired", Wired)}, nil, nil, map[string][]byte{"dongle": {3, 0x40, 0, 0, 7}}, Status{Connection: Dongle, BatteryPercent: 70, BatteryAvailable: true}, "", []string{"enumerate", "validate:dongle", "read:dongle"}},
		{"wired follows rejected dongle", []Candidate{candidate("dongle", Dongle), candidate("wired", Wired)}, map[string]error{"dongle": errors.New("bad descriptor")}, nil, map[string][]byte{"wired": {3, 0x40, 0, 0, 5}}, Status{Connection: Wired, BatteryPercent: 50, BatteryAvailable: true}, "", []string{"enumerate", "validate:dongle", "validate:wired", "read:wired"}},
		{"no report is safe", []Candidate{candidate("dongle", Dongle)}, nil, nil, nil, Status{Connection: Dongle}, "", []string{"enumerate", "validate:dongle", "read:dongle"}},
		{"read timeout is actionable", []Candidate{candidate("dongle", Dongle)}, nil, map[string]error{"dongle": context.DeadlineExceeded}, nil, Status{}, ReadFailure, []string{"enumerate", "validate:dongle", "read:dongle"}},
		{"all descriptors rejected", []Candidate{candidate("dongle", Dongle)}, map[string]error{"dongle": errors.New("bad descriptor")}, nil, nil, Status{}, NoUsableDevice, []string{"enumerate", "validate:dongle"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeTransport{candidates: tt.candidates, validate: tt.validate, read: tt.read, reports: tt.reports}
			got, err := NewService(fake).Status(context.Background())
			if got != tt.want {
				t.Fatalf("status = %#v, want %#v", got, tt.want)
			}
			if tt.wantKind == "" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantKind != "" && !IsErrorKind(err, tt.wantKind) {
				t.Fatalf("error = %v, want kind %q", err, tt.wantKind)
			}
			if !reflect.DeepEqual(fake.log, tt.wantLog) {
				t.Fatalf("operation log = %v, want %v", fake.log, tt.wantLog)
			}
		})
	}
}

func candidate(path string, connection Connection) Candidate {
	return Candidate{Path: path, Connection: connection}
}

type fakeTransport struct {
	candidates     []Candidate
	validate, read map[string]error
	reports        map[string][]byte
	log            []string
}

func (f *fakeTransport) Enumerate(context.Context, Match) ([]Candidate, error) {
	f.log = append(f.log, "enumerate")
	return f.candidates, nil
}
func (f *fakeTransport) ValidateDescriptor(_ context.Context, c Candidate, _ InputDescriptor) (InputSource, error) {
	f.log = append(f.log, "validate:"+c.Path)
	return InputSource{Path: c.Path}, f.validate[c.Path]
}
func (f *fakeTransport) ReadInterruptIN(_ context.Context, s InputSource, accept func([]byte) bool) error {
	f.log = append(f.log, "read:"+s.Path)
	if report := f.reports[s.Path]; report != nil {
		accept(report)
	}
	return f.read[s.Path]
}
