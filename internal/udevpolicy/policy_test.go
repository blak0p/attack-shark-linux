package udevpolicy_test

import (
 "errors"
 "io/fs"
 "os"
 "testing"

 policy "github.com/blak0p/attack-shark-linux/internal/udevpolicy"
 packaged "github.com/blak0p/attack-shark-linux/packaging/udev"
)

type source struct { mode fs.FileMode; data string; statErr, readErr error; reads int }
func (s *source) Lstat() (fs.FileMode,error) { return s.mode,s.statErr }
func (s *source) Read(limit int) ([]byte,error) { s.reads++; if limit != policy.MaxPolicyBytes+1 { panic("unbounded read") }; return []byte(s.data),s.readErr }
func TestInspect(t *testing.T) {
 old,err:=os.ReadFile("testdata/fa60-official.rules"); if err!=nil { t.Fatal(err) }
 for _,tt:=range []struct{name string; src source; want policy.State; reads int}{
 {"current",source{data:packaged.Policy()},policy.Current,1},
 {"missing",source{statErr:fs.ErrNotExist},policy.Missing,0},
 {"old",source{data:string(old)},policy.Outdated,1},
 {"group",source{data:`SUBSYSTEM=="hidraw", ATTRS{idVendor}=="1d57", ATTRS{idProduct}=="fa60", GROUP="mouse", MODE="0660"`},policy.Custom,1},
 {"extra",source{data:packaged.Policy()+"ENV{CUSTOM}=\"1\"\n"},policy.Custom,1},
 {"changed comment",source{data:string(old)+"# local\n"},policy.Custom,1},
 {"unreadable",source{readErr:fs.ErrPermission},policy.Error,1},
 {"stat denied",source{statErr:fs.ErrPermission},policy.Error,0},
 {"symlink",source{mode:fs.ModeSymlink},policy.Error,0},
 {"device",source{mode:fs.ModeDevice},policy.Error,0},
 {"directory",source{mode:fs.ModeDir},policy.Error,0},
 {"oversized",source{data:string(make([]byte,policy.MaxPolicyBytes+1))},policy.Error,1},
 } { t.Run(tt.name,func(t *testing.T){ got:=policy.Inspect(&tt.src); if got.State!=tt.want || tt.src.reads!=tt.reads { t.Fatalf("got %+v reads %d; want %s reads %d",got,tt.src.reads,tt.want,tt.reads) }; if (got.Err!=nil)!=(tt.want==policy.Error){t.Fatalf("error = %v",got.Err)}; if tt.src.readErr!=nil && !errors.Is(got.Err,tt.src.readErr){t.Fatal(got.Err)} }) }
 if policy.InstalledPath!="/etc/udev/rules.d/60-attack-shark-x6-hidraw.rules" { t.Fatal(policy.InstalledPath) }
}
