package update

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripper func(*http.Request) (*http.Response, error)

func (f roundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestGitHubTransportListsManifestAssetsWithInjectedClient(t *testing.T) {
	client := &http.Client{Transport: roundTripper(func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case "https://api.github.com/repos/blak0p/attack-shark-linux/releases?per_page=100":
			return response(`[{"assets":[{"name":"update-manifest.json","browser_download_url":"https://github.com/blak0p/attack-shark-linux/releases/download/v1/update-manifest.json"}]}]`), nil
		case "https://github.com/blak0p/attack-shark-linux/releases/download/v1/update-manifest.json":
			return response(`{"version":"1.2.0","url":"https://github.com/blak0p/attack-shark-linux/releases/download/v1/app.AppImage","sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","signature":"signature"}`), nil
		default:
			t.Fatalf("unexpected URL = %s", r.URL)
			return nil, nil
		}
	})}
	transport := NewGitHubTransport(client)
	manifests, err := transport.Manifests(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(manifests) != 1 || manifests[0].Version != "1.2.0" {
		t.Fatalf("manifests = %#v", manifests)
	}
}

func TestGitHubTransportBoundsEveryRequestLifetime(t *testing.T) {
	transport := NewGitHubTransport(&http.Client{Transport: roundTripper(func(r *http.Request) (*http.Response, error) {
		if _, ok := r.Context().Deadline(); !ok {
			t.Fatal("GitHub request must have a deadline")
		}
		return response(`[]`), nil
	})})
	if _, err := transport.Manifests(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestGitHubTransportDownloadsOnlyHTTPSWithRequestContext(t *testing.T) {
	type contextKey struct{}
	key := contextKey{}
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), key, "caller value"))
	defer cancel()
	transport := NewGitHubTransport(&http.Client{Transport: roundTripper(func(r *http.Request) (*http.Response, error) {
		if got := r.Context().Value(key); got != "caller value" {
			t.Fatalf("request context value = %v, want caller value", got)
		}
		if _, ok := r.Context().Deadline(); !ok {
			t.Fatal("request did not retain a bounded deadline")
		}
		return response("AppImage"), nil
	})})
	body, err := transport.Download(ctx, "https://github.com/blak0p/attack-shark-linux/releases/download/v1/app.AppImage")
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	contents, _ := io.ReadAll(body)
	if string(contents) != "AppImage" {
		t.Fatalf("contents = %q", contents)
	}
	if _, err := transport.Download(ctx, "http://example.invalid/file"); err == nil {
		t.Fatal("HTTP download unexpectedly accepted")
	}
	if _, err := transport.Download(ctx, "https://example.invalid/file"); err == nil {
		t.Fatal("non-GitHub download unexpectedly accepted")
	}
}

func TestGitHubTransportPropagatesCallerCancellationAndReleasesDownloadTimeout(t *testing.T) {
	requestStarted := make(chan struct{})
	var requestContext context.Context
	transport := NewGitHubTransport(&http.Client{Transport: roundTripper(func(r *http.Request) (*http.Response, error) {
		requestContext = r.Context()
		close(requestStarted)
		return response("AppImage"), nil
	})})
	caller, cancelCaller := context.WithCancel(context.Background())
	body, err := transport.Download(caller, "https://github.com/blak0p/attack-shark-linux/releases/download/v1/app.AppImage")
	if err != nil {
		t.Fatal(err)
	}
	<-requestStarted
	cancelCaller()
	select {
	case <-requestContext.Done():
	case <-time.After(time.Second):
		t.Fatal("caller cancellation did not reach the transport request")
	}
	if err := body.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestGitHubTransportReleasesRequestTimeoutWhenDownloadBodyCloses(t *testing.T) {
	var requestContext context.Context
	transport := NewGitHubTransport(&http.Client{Transport: roundTripper(func(r *http.Request) (*http.Response, error) {
		requestContext = r.Context()
		return response("AppImage"), nil
	})})
	body, err := transport.Download(context.Background(), "https://github.com/blak0p/attack-shark-linux/releases/download/v1/app.AppImage")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadAll(body); err != nil {
		t.Fatal(err)
	}
	select {
	case <-requestContext.Done():
	case <-time.After(time.Second):
		t.Fatal("download request timeout was not released after its body finished")
	}
	if err := body.Close(); err != nil {
		t.Fatal(err)
	}
}

func response(body string) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}
