package transport

import "testing"

func TestPreferWiredPreservesAmbiguityAndIdentities(t *testing.T) {
	dongleMatch := Match{VendorID: 0x10, ProductID: 0x01}
	wiredMatch := Match{VendorID: 0x10, ProductID: 0x02}
	otherDongle := Match{VendorID: 0x20, ProductID: 0x01}
	pair := WiredPair{Wired: wiredMatch, Dongle: dongleMatch}
	d := Candidate{VendorID: 0x10, ProductID: 0x01, Connection: Dongle, Path: "dongle", Serial: "receiver"}
	w := Candidate{VendorID: 0x10, ProductID: 0x02, Connection: Wired, Path: "cable", Serial: "mouse"}
	otherD := Candidate{VendorID: 0x20, ProductID: 0x01, Connection: Dongle, Path: "other"}
	other := Candidate{VendorID: 1, ProductID: 2, Connection: Dongle}
	for _, tc := range []struct {
		name        string
		pairs       []WiredPair
		input, want []Candidate
	}{
		{"pair", []WiredPair{pair}, []Candidate{d, w}, []Candidate{w}},
		{"reverse", []WiredPair{pair}, []Candidate{w, d}, []Candidate{w}},
		{"dongle", []WiredPair{pair}, []Candidate{d}, []Candidate{d}},
		{"cable", []WiredPair{pair}, []Candidate{w}, []Candidate{w}},
		{"multiple cables", []WiredPair{pair}, []Candidate{d, w, w}, []Candidate{d, w, w}},
		{"multiple receivers", []WiredPair{pair}, []Candidate{d, d, w}, []Candidate{d, d, w}},
		{"unrelated", []WiredPair{pair}, []Candidate{other, w}, []Candidate{other, w}},
		{"no pairs", nil, []Candidate{d, w}, []Candidate{d, w}},
		{"second pair", []WiredPair{{Wired: Match{VendorID: 0x20, ProductID: 0x02}, Dongle: otherDongle}, pair}, []Candidate{d, w}, []Candidate{w}},
		{"mismatched pair", []WiredPair{pair}, []Candidate{otherD, w}, []Candidate{otherD, w}},
		{"cross pair", []WiredPair{pair, {Wired: Match{VendorID: 0x20, ProductID: 0x02}, Dongle: otherDongle}}, []Candidate{otherD, w}, []Candidate{otherD, w}},
		{"wrong connection", []WiredPair{pair}, []Candidate{{VendorID: 0x10, ProductID: 0x01, Connection: Wired}, w}, []Candidate{{VendorID: 0x10, ProductID: 0x01, Connection: Wired}, w}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := PreferWired(tc.input, tc.pairs...)
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
