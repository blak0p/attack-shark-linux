//go:build linux

package hidlinux

import (
	"bytes"
	"testing"
)

func TestSetInventoryDiagnosticWriterRedirectsAndRestores(t *testing.T) {
	var logs bytes.Buffer
	restore := SetInventoryDiagnosticWriter(&logs)
	inventoryDiagnosticf("event=%s", "probe")
	restore()
	if got := logs.String(); got != "x6_inventory event=probe\n" {
		t.Fatalf("redirected output = %q", got)
	}
	inventoryDiagnosticMu.Lock()
	defer inventoryDiagnosticMu.Unlock()
	if inventoryDiagnosticWriter == &logs {
		t.Fatal("writer was not restored")
	}
}
