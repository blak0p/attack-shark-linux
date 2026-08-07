package x6

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestEncodeDPIReportValidatesAndEncodesComplete52ByteReport(t *testing.T) {
	valid := DefaultDPIConfig()
	report, err := EncodeDPIReport(valid)
	if err != nil {
		t.Fatalf("EncodeDPIReport() error = %v", err)
	}
	if len(report) != 52 || report[0] != 0x04 || report[1] != 0x38 || report[24] != 4 || report[50] != 0x0e || report[51] != 0x74 {
		t.Fatalf("report = %x, want captured 52-byte vector", report)
	}

	invalid := valid
	invalid.DPI[0] = 75
	if _, err := EncodeDPIReport(invalid); !IsErrorKind(err, InvalidDPI) {
		t.Fatalf("invalid DPI error = %v, want %q", err, InvalidDPI)
	}
}

func TestApplyDPIWaitsForMatchingACKAndPersistsOnlyAfterSuccess(t *testing.T) {
	for _, tt := range []struct {
		name    string
		reports [][]byte
		readErr error
		wantErr ErrorKind
		wantLog []string
	}{
		{"ignores heartbeat before matching ACK", [][]byte{{0x03, 0x10, 0x40, 0x01, 0x0a}, {0x03, 0x10, 0x50, 0x00, 0x04}}, nil, "", []string{"write", "read", "save"}},
		{"wrong ACK does not persist", [][]byte{{0x03, 0x10, 0x50, 0x00, 0x05}}, nil, AckFailure, []string{"write", "read"}},
		{"read failure does not persist", nil, errors.New("timeout"), AckFailure, []string{"write", "read"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			log := []string{}
			transport := &commandFake{reports: tt.reports, readErr: tt.readErr, log: &log}
			store := &storeFake{log: &log}
			err := NewCommandService(transport).ApplyAndPersist(context.Background(), DefaultDPIConfig(), store)
			if tt.wantErr == "" && err != nil {
				t.Fatalf("ApplyAndPersist() error = %v", err)
			}
			if tt.wantErr != "" && !IsErrorKind(err, tt.wantErr) {
				t.Fatalf("ApplyAndPersist() error = %v, want %q", err, tt.wantErr)
			}
			if !reflect.DeepEqual(log, tt.wantLog) {
				t.Fatalf("operations = %v, want %v", log, tt.wantLog)
			}
		})
	}
}

type commandFake struct {
	reports [][]byte
	readErr error
	log     *[]string
}

func (f *commandFake) SendAndAwait(_ context.Context, report []byte, accept func([]byte) bool) error {
	if len(report) != 52 {
		return errors.New("unexpected report length")
	}
	*f.log = append(*f.log, "write", "read")
	for _, report := range f.reports {
		if !accept(report) {
			break
		}
	}
	return f.readErr
}

type storeFake struct{ log *[]string }

func (s *storeFake) SaveApplied(DPIConfig) error { *s.log = append(*s.log, "save"); return nil }
