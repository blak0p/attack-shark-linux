package x6

import (
	"bytes"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
	protocol "github.com/blak0p/attack-shark-linux/internal/protocol/x6"
)

func TestPollingOperationAdaptsTypedProtocolContract(t *testing.T) {
	operation := NewPollingOperation()
	var _ mouse.CommandOperation = operation

	for _, rate := range []protocol.PollingRate{protocol.PollingRate125, protocol.PollingRate1000} {
		if err := operation.Validate(rate); err != nil {
			t.Fatalf("Validate(%d) error = %v", rate, err)
		}
		got, err := operation.Encode(rate)
		want, wantErr := protocol.EncodePollingReport(rate)
		if err != nil || wantErr != nil || !bytes.Equal(got, want) {
			t.Fatalf("Encode(%d) = % x, %v; want % x, %v", rate, got, err, want, wantErr)
		}
	}
	if err := operation.Validate(protocol.PollingRate(999)); err == nil {
		t.Fatal("Validate() accepted unsupported polling rate")
	}
	if !operation.MatchesACK([]byte{0x03, 0x10, 0x50, 0x00, 0x06}) || operation.MatchesACK([]byte{0x03, 0x10, 0x50, 0x00, 0x04}) {
		t.Fatal("MatchesACK() did not isolate the polling acknowledgement")
	}
}
