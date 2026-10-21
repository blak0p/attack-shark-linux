package udev

import "testing"

func TestEmbeddedPolicyMatchesPackagedFile(t *testing.T) {
 if got,want:=Policy(),readFile(t,policyFile); got!=want { t.Fatal("embedded policy differs from packaged source") }
}
