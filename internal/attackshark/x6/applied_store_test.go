package x6

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/configstore"
)

func TestAppliedStateIsSavedOnlyAfterAcknowledgementAndRestoredOnRestart(t *testing.T) {
	dir := t.TempDir()
	appliedPath := filepath.Join(dir, "applied.json")
	store := configstore.New(appliedPath, filepath.Join(dir, "factory.json"))
	config := DefaultDPIConfig()
	config.DPI[0] = 1600
	command := &ackCommand{beforeAcknowledgement: func() {
		if _, err := os.Stat(appliedPath); !os.IsNotExist(err) {
			t.Fatalf("applied state exists before acknowledgement: %v", err)
		}
	}}

	if err := NewCommandService(command).ApplyAndPersist(context.Background(), config, store); err != nil {
		t.Fatalf("ApplyAndPersist() error = %v", err)
	}
	restarted, err := store.LoadApplied()
	if err != nil || restarted.DPI[0] != 1600 || len(command.report) != 56 {
		t.Fatalf("restarted state = %#v, %v; report length = %d; want documented 56-byte baseline", restarted, err, len(command.report))
	}
}

type ackCommand struct {
	report                []byte
	beforeAcknowledgement func()
}

func (f *ackCommand) SendAndAwait(_ context.Context, report []byte, keepReading func([]byte) bool) error {
	f.report = append([]byte(nil), report...)
	f.beforeAcknowledgement()
	if keepReading([]byte{0x03, 0x10, 0x50, 0x00, 0x04}) {
		return os.ErrInvalid
	}
	return nil
}
