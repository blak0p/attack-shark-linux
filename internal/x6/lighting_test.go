package x6

import (
	"bytes"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
	protocol "github.com/blak0p/attack-shark-linux/internal/protocol/x6"
)

func TestLightingOperationAdaptsOnlyValidatedSelections(t *testing.T) {
	selection := LightingSelection{Mode: LightingBreathing, TemplateID: LightingTemplateBreathingFF7F00}
	operation := NewLightingOperation()
	var _ mouse.CommandOperation = operation

	if err := operation.Validate(selection); err != nil {
		t.Fatalf("Validate(%#v) error = %v", selection, err)
	}
	got, err := operation.Encode(selection)
	want, wantErr := protocol.NewLightingOperation(protocol.LightingSelection(selection))
	if err != nil || wantErr != nil || !bytes.Equal(got, want) {
		t.Fatalf("Encode(%#v) = % x, %v; want % x, %v", selection, got, err, want, wantErr)
	}
	if err := operation.Validate(LightingSelection{Mode: LightingFixed, TemplateID: LightingTemplateBreathingGreen}); err == nil {
		t.Fatal("Validate() accepted an unsupported mode/template pair")
	}
	if !operation.MatchesACK([]byte{0x03, 0x10, 0x50, 0x00, 0x05}) || operation.MatchesACK([]byte{0x03, 0x10, 0x50, 0x00, 0x06}) {
		t.Fatal("MatchesACK() did not isolate the lighting acknowledgement")
	}
}
