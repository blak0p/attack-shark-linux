package desktop

import (
	"context"
	"fmt"

	"github.com/blak0p/attack-shark-linux/internal/udevauth"
	"github.com/blak0p/attack-shark-linux/internal/update"
)

// ApplyInstalledPolicy is a trusted-Go execution boundary, not a Service method.
// The backend must supply a VerifyPolicy-authenticated candidate and its exact
// signed wire. This adapter does not choose keys or authenticate releases; the
// installed helper independently verifies the wire before touching policy.
// No filesystem inspection or authorization occurs until this explicit call.
func ApplyInstalledPolicy(ctx context.Context, candidate *udevauth.Policy, wire []byte) (PolicyApplyResult, error) {
	return installedHelper().apply(ctx, candidate, wire)
}

func validHelperInput(p *udevauth.Policy, wire []byte) bool {
	if p == nil || len(p.Version) == 0 || len(p.Version) > 80 || len(p.Bytes) == 0 || len(p.Bytes) > udevauth.MaxPolicyBytes || len(wire) == 0 || len(wire) > udevauth.MaxEnvelopeBytes {
		return false
	}
	_, err := update.CompareVersions(p.Version, p.Version)
	return err == nil
}

func helperUnavailable() error {
	return fmt.Errorf("USB policy updates unavailable: use a trusted release installer --setup-usb-permissions; a safe root-owned helper and /usr/bin/pkexec are required")
}
