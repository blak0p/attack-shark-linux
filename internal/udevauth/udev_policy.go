package udevauth

import (
	"context"
	"encoding/base64"
	"errors"

	"github.com/blak0p/attack-shark-linux/internal/update"
)

// CheckPolicy authenticates a policy attached to a signed backend-selected
// release, including a same-version release. A nil policy and nil error means
// legacy unavailability, never successful policy authentication. All other
// transport/authentication failures return an error and no policy.
// It performs no filesystem, process, device or privilege operations. Callers
// must not expose manifest, current version or key selection to renderers.
func CheckPolicy(ctx context.Context, transport *update.GitHubTransport, manifest update.Manifest, current, key string) (*Policy, error) {
	policy, _, err := CheckPolicyWithWire(ctx, transport, manifest, current, key)
	return policy, err
}

// CheckPolicyWithWire returns the authenticated policy and a detached copy of
// the exact signed envelope received in one fetch. Only 404 returns nil, nil,
// nil; all failures return no policy or wire. This is a trusted-Go boundary,
// never a renderer API or an unsigned reserialization for helper input.
func CheckPolicyWithWire(ctx context.Context, transport *update.GitHubTransport, manifest update.Manifest, current, key string) (*Policy, []byte, error) {
	version, err := update.PolicyVersion(manifest, current, key)
	if err != nil {
		return nil, nil, err
	}
	if transport == nil {
		return nil, nil, errors.New("policy transport is required")
	}
	wire, err := transport.PolicyEnvelope(ctx, version, MaxEnvelopeBytes)
	if errors.Is(err, update.ErrPolicyUnavailable) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	public, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return nil, nil, err
	}
	policy, err := VerifyPolicy(wire, version, public)
	if err != nil {
		return nil, nil, err
	}
	return &policy, append([]byte(nil), wire...), nil
}
