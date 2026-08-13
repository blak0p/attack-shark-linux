package mouse

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
