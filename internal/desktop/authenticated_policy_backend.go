package desktop

import (
	"context"
	"encoding/base64"
	"errors"
	"reflect"

	"github.com/blak0p/attack-shark-linux/internal/udevauth"
	"github.com/blak0p/attack-shark-linux/internal/udevpolicy"
	"github.com/blak0p/attack-shark-linux/internal/update"
)

// ConfigureAuthenticatedPolicyUpdates is trusted startup composition, not a
// renderer capability. Call after the final ConfigureUpdater, before exposing
// the service. Construction performs neither network I/O nor authorization.
func ConfigureAuthenticatedPolicyUpdates(s *Service) error {
	state := s.updateState()
	if state == nil || state.updater == nil {
		return errors.New("authenticated updates require an updater")
	}
	transport, ok := state.updater.Transport.(*update.GitHubTransport)
	if !ok || transport == nil {
		return errors.New("authenticated updates require the updater's GitHub transport")
	}
	configureAuthenticatedPolicyUpdates(s, state, transport, udevpolicy.InstalledSource(), ApplyInstalledPolicy)
	return nil
}

// All backend access is through ports serialized by updateState.mu. In
// particular a failed legacy or unified check revokes previously retained wire.
type authenticatedPolicyBackend struct {
	updater    update.Updater
	transport  *update.GitHubTransport
	manifest   *update.Manifest
	checkError error
	policyOnly bool
	policy     *udevauth.Policy
	wire       []byte
	apply      func(context.Context, *udevauth.Policy, []byte) (PolicyApplyResult, error)
}

func configureAuthenticatedPolicyUpdates(s *Service, state *updateState, transport *update.GitHubTransport, source udevpolicy.Source, apply func(context.Context, *udevauth.Policy, []byte) (PolicyApplyResult, error)) {
	backend := &authenticatedPolicyBackend{updater: *state.updater, transport: transport, apply: apply}
	ConfigureUnifiedUpdates(s, backend, source)
	state.mu.Lock()
	defer state.mu.Unlock()
	state.check = backend.checkApp
}

func (b *authenticatedPolicyBackend) checkApp(ctx context.Context) (*update.VerifiedUpdate, error) {
	b.manifest, b.policy, b.wire, b.checkError = nil, nil, nil, nil
	b.policyOnly = false
	verified, err := b.updater.Check(ctx)
	if err == nil {
		manifest := verified.Manifest()
		b.manifest = &manifest
	} else if errors.Is(err, update.ErrNoUpdate) || errors.Is(err, update.ErrNoRCUpdate) {
		b.policyOnly = true
	} else {
		b.checkError = err
	}
	return verified, err
}

func (b *authenticatedPolicyBackend) Check(ctx context.Context) (*udevauth.Policy, error) {
	b.policy, b.wire = nil, nil
	if b.checkError != nil {
		return nil, b.checkError
	}
	if b.policyOnly {
		var err error
		b.manifest, err = b.updater.CheckPolicyRelease(ctx)
		if errors.Is(err, update.ErrNoUpdate) || errors.Is(err, update.ErrNoRCUpdate) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
	}
	if b.manifest == nil {
		return nil, nil
	}
	policy, wire, err := udevauth.CheckPolicyWithWire(ctx, b.transport, *b.manifest, b.updater.CurrentVersion, b.updater.PublicKey)
	if err != nil || policy == nil {
		return nil, err
	}
	b.policy, b.wire = policy, append([]byte(nil), wire...)
	copy := *policy
	copy.Bytes = append([]byte(nil), policy.Bytes...)
	copy.Previous = append([]string(nil), policy.Previous...)
	return &copy, nil
}

func (b *authenticatedPolicyBackend) Apply(ctx context.Context, candidate *udevauth.Policy) (PolicyApplyResult, error) {
	if b.policy == nil || !reflect.DeepEqual(candidate, b.policy) {
		return PolicyApplyResult{}, errors.New("policy does not match authenticated check")
	}
	public, err := base64.StdEncoding.DecodeString(b.updater.PublicKey)
	if err != nil {
		return PolicyApplyResult{}, err
	}
	policy, err := udevauth.VerifyPolicy(b.wire, b.policy.Version, public)
	if err != nil {
		return PolicyApplyResult{}, err
	}
	if !reflect.DeepEqual(&policy, b.policy) {
		return PolicyApplyResult{}, errors.New("retained policy does not match signed wire")
	}
	return b.apply(ctx, &policy, append([]byte(nil), b.wire...))
}
