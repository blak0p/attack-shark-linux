package x6

import (
	"context"
	"errors"
	"testing"

	protocol "github.com/blak0p/attack-shark-linux/internal/protocol/x6"
	"github.com/blak0p/attack-shark-linux/internal/transport"
)

func TestX6FacadesMatchGenericTransportAndPureProtocol(t *testing.T) {
	if Match(transport.X6Match()) != (Match{VendorID: 0x1D57, ProductID: 0xFA60}) {
		t.Fatalf("X6 transport match does not preserve generic identity")
	}
	config := DefaultDPIConfig()
	config.DPI[0] = 1600
	got, err := EncodeDPIReport(config)
	want, protocolErr := protocol.EncodeDPIReport(protocol.DPIConfig(config))
	if err != nil || protocolErr != nil || string(got) != string(want) {
		t.Fatalf("DPI façade output = % x, %v; protocol = % x, %v", got, err, want, protocolErr)
	}
}

func TestApplyAndPersistRequiresMatchingACKBeforeSaving(t *testing.T) {
	config := DefaultDPIConfig()
	command := &commandFake{reports: [][]byte{{0x03, 0x10, 0x50, 0x00, 0x05}, {0x03, 0x10, 0x50, 0x00, 0x04}}}
	store := &storeFake{}
	if err := NewCommandService(command).ApplyAndPersist(context.Background(), config, store); err != nil {
		t.Fatalf("ApplyAndPersist() error = %v", err)
	}
	if command.calls != 1 || len(command.sent) != 52 || !store.saved || !command.acknowledgedBeforeSave {
		t.Fatalf("apply calls=%d report=%d saved=%t ack-before-save=%t", command.calls, len(command.sent), store.saved, command.acknowledgedBeforeSave)
	}
}

func TestApplyAndPersistDoesNotSaveWhenAcknowledgementFails(t *testing.T) {
	command := &commandFake{err: errors.New("timeout")}
	store := &storeFake{}
	err := NewCommandService(command).ApplyAndPersist(context.Background(), DefaultDPIConfig(), store)
	if !IsErrorKind(err, AckFailure) || store.saved {
		t.Fatalf("ApplyAndPersist() error = %v, saved = %t; want ACK failure without persistence", err, store.saved)
	}
}

type commandFake struct {
	reports                [][]byte
	err                    error
	calls                  int
	sent                   []byte
	acknowledgedBeforeSave bool
}

func (f *commandFake) SendAndAwait(_ context.Context, report []byte, keepReading func([]byte) bool) error {
	f.calls++
	f.sent = append([]byte(nil), report...)
	for _, response := range f.reports {
		if !keepReading(response) {
			f.acknowledgedBeforeSave = true
			break
		}
	}
	return f.err
}

type storeFake struct{ saved bool }

func (f *storeFake) SaveApplied(DPIConfig) error { f.saved = true; return nil }
