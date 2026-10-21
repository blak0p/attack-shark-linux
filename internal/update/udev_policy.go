package update

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

var ErrPolicyUnavailable = errors.New("release has no signed udev policy")

// PolicyVersion validates same-channel, non-older release eligibility and its
// signature. The returned version is the exact artifact version (without v).
func PolicyVersion(m Manifest, current, key string) (string, error) {
	installed, err := channelForVersion(current)
	if err != nil {
		return "", err
	}
	candidate, err := channelForVersion(m.Version)
	if err != nil {
		return "", err
	}
	comparison, err := CompareVersions(m.Version, current)
	if err != nil {
		return "", err
	}
	if installed != candidate || comparison < 0 {
		return "", errors.New("policy release must be same-channel and not older")
	}
	if err := VerifyReleaseManifest(m, key); err != nil {
		return "", err
	}
	version := strings.TrimPrefix(m.Version, "v")
	// Do not derive an authority from an arbitrary signed download host, query,
	// alias or tag. Only the canonical repository/tag directory is supported.
	prefix := "https://github.com/blak0p/attack-shark-linux/releases/download/v" + version + "/"
	if !strings.HasPrefix(m.URL, prefix) || strings.ContainsAny(strings.TrimPrefix(m.URL, prefix), "/?#%") {
		return "", errors.New("policy requires a canonical release AppImage URL")
	}
	return version, nil
}

// PolicyEnvelope fetches only the fixed canonical artifact. Authentication and
// size limits belong to the caller; 404 alone denotes legacy unavailability.
func (t *GitHubTransport) PolicyEnvelope(ctx context.Context, version string, limit int64) ([]byte, error) {
	if _, err := parseVersion(version); err != nil || strings.HasPrefix(version, "v") {
		return nil, errors.New("invalid policy version")
	}
	if limit <= 0 || limit > 1<<20 {
		return nil, errors.New("invalid policy fetch bound")
	}
	request, cancel, err := httpsRequest(ctx, "https://github.com/blak0p/attack-shark-linux/releases/download/v"+version+"/udev-policy.txt")
	if err != nil {
		return nil, err
	}
	defer cancel()
	response, err := t.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return nil, ErrPolicyUnavailable
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("policy download returned %s", response.Status)
	}
	wire, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(wire)) > limit {
		return nil, errors.New("policy envelope exceeds fetch bound")
	}
	return wire, nil
}
