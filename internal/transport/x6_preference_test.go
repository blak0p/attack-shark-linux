package transport

import "testing"

func TestPreferX6WiredPreservesAmbiguityAndIdentities(t *testing.T) {
	d := Candidate{VendorID: 0x1d57, ProductID: 0xfa60, Connection: Dongle, Path: "dongle", Serial: "receiver"}
	w := Candidate{VendorID: 0x1d57, ProductID: 0xfa61, Connection: Wired, Path: "cable", Serial: "mouse"}
	other := Candidate{VendorID: 1, ProductID: 2, Connection: Dongle}
	for _, tc := range []struct {
		name        string
		input, want []Candidate
	}{
		{"pair", []Candidate{d, w}, []Candidate{w}},
		{"reverse", []Candidate{w, d}, []Candidate{w}},
		{"dongle", []Candidate{d}, []Candidate{d}},
		{"cable", []Candidate{w}, []Candidate{w}},
		{"multiple cables", []Candidate{d, w, w}, []Candidate{d, w, w}},
		{"multiple receivers", []Candidate{d, d, w}, []Candidate{d, d, w}},
		{"unrelated", []Candidate{other, w}, []Candidate{other, w}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := PreferX6Wired(tc.input)
			if len(got) != len(tc.want) {
				t.Fatalf("got=%v want=%v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got=%v want=%v", got, tc.want)
				}
			}
		})
	}
}
