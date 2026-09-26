package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const (
	githubReleasesURL    = "https://api.github.com/repos/blak0p/attack-shark-linux/releases?per_page=100"
	githubRequestTimeout = 15 * time.Second
)

type githubRelease struct {
	Assets []githubAsset `json:"assets"`
}
type githubAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

// GitHubTransport is the concrete, bounded HTTPS boundary for published release assets.
// Eligibility remains entirely in Updater.Check.
type GitHubTransport struct{ client *http.Client }

type cancellableBody struct {
	io.ReadCloser
	cancel context.CancelFunc
	once   sync.Once
}

func (body *cancellableBody) Read(p []byte) (int, error) {
	n, err := body.ReadCloser.Read(p)
	if err != nil {
		body.once.Do(body.cancel)
	}
	return n, err
}

func (body *cancellableBody) Close() error {
	err := body.ReadCloser.Close()
	body.once.Do(body.cancel)
	return err
}

func NewGitHubTransport(client *http.Client) *GitHubTransport {
	if client == nil {
		client = &http.Client{Timeout: githubRequestTimeout}
	}
	return &GitHubTransport{client: client}
}

func (t *GitHubTransport) Manifests(ctx context.Context) ([]Manifest, error) {
	var releases []githubRelease
	if err := t.getJSON(ctx, githubReleasesURL, &releases); err != nil {
		return nil, fmt.Errorf("list GitHub releases: %w", err)
	}
	var manifests []Manifest
	for _, release := range releases {
		for _, asset := range release.Assets {
			if asset.Name != "update-manifest.json" {
				continue
			}
			var manifest Manifest
			if err := t.getJSON(ctx, asset.URL, &manifest); err != nil {
				return nil, fmt.Errorf("fetch update manifest: %w", err)
			}
			manifests = append(manifests, manifest)
		}
	}
	return manifests, nil
}

func (t *GitHubTransport) Download(ctx context.Context, rawURL string) (io.ReadCloser, error) {
	request, cancel, err := httpsRequest(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	response, err := t.client.Do(request)
	if err != nil {
		cancel()
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		response.Body.Close()
		cancel()
		return nil, fmt.Errorf("GitHub download returned %s", response.Status)
	}
	return &cancellableBody{ReadCloser: response.Body, cancel: cancel}, nil
}

func (t *GitHubTransport) getJSON(ctx context.Context, rawURL string, target any) error {
	request, cancel, err := httpsRequest(ctx, rawURL)
	if err != nil {
		return err
	}
	defer cancel()
	response, err := t.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return fmt.Errorf("GitHub returned %s", response.Status)
	}
	return json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(target)
}

func httpsRequest(ctx context.Context, rawURL string) (*http.Request, context.CancelFunc, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" || (parsed.Host != "api.github.com" && parsed.Host != "github.com") {
		return nil, nil, errors.New("GitHub transport requires an HTTPS GitHub URL")
	}
	bounded, cancel := context.WithTimeout(ctx, githubRequestTimeout)
	request, err := http.NewRequestWithContext(bounded, http.MethodGet, rawURL, nil)
	if err != nil {
		cancel()
		return nil, nil, err
	}
	return request, cancel, nil
}
