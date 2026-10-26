//go:build !linux

package desktop

import (
	"context"
	"github.com/blak0p/attack-shark-linux/internal/udevauth"
)

type policyHelper struct{}

func installedHelper() policyHelper { return policyHelper{} }
func (policyHelper) apply(ctx context.Context, _ *udevauth.Policy, _ []byte) (PolicyApplyResult, error) {
	if err := ctx.Err(); err != nil {
		return PolicyApplyResult{}, err
	}
	return PolicyApplyResult{}, helperUnavailable()
}
