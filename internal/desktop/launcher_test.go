package desktop

import (
	"errors"
	"reflect"
	"testing"
)

func TestGetLauncherPathReturnsAttachedAbsolutePath(t *testing.T) {
	snap := (&Service{}).AttachLauncherPath(func() (string, error) { return "/opt/app/x6.AppImage", nil }).GetLauncherPath()
	if snap.Path != "/opt/app/x6.AppImage" || snap.Error.Code != "" {
		t.Fatalf("snapshot = %+v", snap)
	}
}

func TestGetLauncherPathUnavailable(t *testing.T) {
	cases := map[string]*Service{
		"nothing attached": {},
		"lookup fails":     (&Service{}).AttachLauncherPath(func() (string, error) { return "", errors.New("boom") }),
		"relative path":    (&Service{}).AttachLauncherPath(func() (string, error) { return "bin/x6", nil }),
		"empty path":       (&Service{}).AttachLauncherPath(func() (string, error) { return "", nil }),
	}
	for name, svc := range cases {
		t.Run(name, func(t *testing.T) {
			snap := svc.GetLauncherPath()
			if snap.Path != "" || snap.Error.Code != LauncherUnavailable || LauncherUnavailable != "launcher_unavailable" {
				t.Fatalf("snapshot = %+v", snap)
			}
		})
	}
}

func TestLauncherPathSnapshotKeysArePascalCase(t *testing.T) {
	if got, want := keysOf(t, LauncherPathSnapshot{}), []string{"Error", "Path"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("keys = %v", got)
	}
}
