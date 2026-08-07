package hidlinux

// backendCapabilities records evidence required before a HID adapter may join
// the read-only transport boundary.
type backendCapabilities struct {
	descriptorVisibility   bool
	reportIDPreservation   bool
	cancellableInterruptIN bool
	openReadWrite          bool
	writeAPIsExposed       bool
}

func (c backendCapabilities) compatible() bool {
	return c.descriptorVisibility &&
		c.reportIDPreservation &&
		c.cancellableInterruptIN
}
