package hidlinux

import "testing"

func TestCompatibilityGate(t *testing.T) {
	tests := []struct {
		name string
		got  backendCapabilities
		want bool
	}{
		{
			name: "sstallion go hid supports the required passive reads",
			got: backendCapabilities{
				descriptorVisibility:   true,
				reportIDPreservation:   true,
				cancellableInterruptIN: true,
				openReadWrite:          true,
				writeAPIsExposed:       true,
			},
			want: true,
		},
		{
			name: "backend missing cancellable reads is rejected",
			got: backendCapabilities{
				descriptorVisibility:   true,
				reportIDPreservation:   true,
				cancellableInterruptIN: false,
				openReadWrite:          true,
				writeAPIsExposed:       true,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.got.compatible(); got != tt.want {
				t.Fatalf("compatible() = %t, want %t", got, tt.want)
			}
		})
	}
}
