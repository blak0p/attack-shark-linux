//go:build linux

package hidlinux

import (
	"fmt"
	"io"
	"os"
	"sync"
)

var (
	inventoryDiagnosticWriter io.Writer = os.Stderr
	inventoryDiagnosticMu     sync.Mutex
)

func inventoryDiagnosticf(format string, args ...any) {
	inventoryDiagnosticMu.Lock()
	defer inventoryDiagnosticMu.Unlock()
	_, _ = fmt.Fprintf(inventoryDiagnosticWriter, "x6_inventory "+format+"\n", args...)
}

// SetInventoryDiagnosticWriter redirects the inventory diagnostics and returns
// a function that restores the previous writer.
func SetInventoryDiagnosticWriter(w io.Writer) (restore func()) {
	inventoryDiagnosticMu.Lock()
	previous := inventoryDiagnosticWriter
	inventoryDiagnosticWriter = w
	inventoryDiagnosticMu.Unlock()
	return func() {
		inventoryDiagnosticMu.Lock()
		inventoryDiagnosticWriter = previous
		inventoryDiagnosticMu.Unlock()
	}
}
