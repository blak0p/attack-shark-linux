package main

import (
	"context"
	"strings"
	"testing"

	"github.com/alejandro/attack-shark-linux/internal/x6"
)

type statusFunc func(context.Context) (x6.Status, error)

func (f statusFunc) Status(ctx context.Context) (x6.Status, error) { return f(ctx) }

func TestRunRendersStatus(t *testing.T) {
	var output strings.Builder
	service := statusFunc(func(context.Context) (x6.Status, error) {
		return x6.Status{Connection: x6.Wired}, nil
	})
	if got := run([]string{"status"}, service, &output); got != 0 || !strings.Contains(output.String(), "connection: wired\n") {
		t.Fatalf("run() = (%d, %q), want wired success", got, output.String())
	}
}
