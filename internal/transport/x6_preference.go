package transport

// PreferX6Wired suppresses only the dongle in an eligible one-cable/one-dongle
// pair. Callers must validate candidates first. It neither merges identities nor
// resolves multiple devices of either USB mode; raw discovery stays unchanged.
func PreferX6Wired(eligible []Candidate) []Candidate {
	if len(eligible) != 2 {
		return eligible
	}
	wired, dongle := -1, -1
	for i, c := range eligible {
		switch {
		case c.VendorID == X6WiredMatch().VendorID && c.ProductID == X6WiredMatch().ProductID && c.Connection == Wired:
			wired = i
		case c.VendorID == X6Match().VendorID && c.ProductID == X6Match().ProductID && c.Connection == Dongle:
			dongle = i
		}
	}
	if wired >= 0 && dongle >= 0 {
		return []Candidate{eligible[wired]}
	}
	return eligible
}
