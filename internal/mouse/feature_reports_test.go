package mouse

import "testing"

func TestHIDFactsAcceptsOnlyDeclaredFeatureReports(t *testing.T) {
	facts := HIDFacts{FeatureReports: []FeatureReport{{ID: 0x04, Length: 3}, {ID: 0x06, Length: 2}}}
	for _, tt := range []struct {
		name    string
		payload []byte
		want    bool
	}{
		{"declared first", []byte{4, 0, 0}, true},
		{"declared second", []byte{6, 0}, true},
		{"declared ID wrong length", []byte{4, 0}, false},
		{"undeclared ID", []byte{9, 0, 0}, false},
		{"empty", nil, false},
	} {
		if got := facts.AcceptsFeatureReport(tt.payload); got != tt.want {
			t.Fatalf("%s: AcceptsFeatureReport(%x) = %t, want %t", tt.name, tt.payload, got, tt.want)
		}
	}
	if (HIDFacts{}).AcceptsFeatureReport([]byte{4, 0, 0}) {
		t.Fatal("facts without feature reports admitted a payload")
	}
}
