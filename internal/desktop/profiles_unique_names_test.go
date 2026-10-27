package desktop

import "testing"

func TestProfileDuplicateNameMapsToDuplicateCode(t *testing.T) {
	s, _, _ := switchTestService(t, t.TempDir(), &recordingSwitchCommand{})
	source := s.GetProfiles().ActiveID
	s.CreateProfile("Work")
	if got := s.CreateProfile("Work").Error.Code; got != DuplicateProfile {
		t.Fatal(got)
	}
	if got := s.DuplicateProfile(source, "Work").Error.Code; got != DuplicateProfile {
		t.Fatal(got)
	}
	if got := s.RenameProfile(source, "Work").Error.Code; got != DuplicateProfile {
		t.Fatal(got)
	}
	if got := s.CreateProfile("").Error.Code; got != InvalidProfile {
		t.Fatal(got)
	}
}
