package macros

import "testing"

func TestEmptyDraftReloadAndCreateCopies(t *testing.T) {
	lib, path := openTest(t)
	draft, err := lib.Create("draft", nil)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := loaded.Read(draft.ID)
	if err != nil || len(got.Events) != 0 {
		t.Fatalf("empty draft: %v %v", got, err)
	}
	events := []Event{{Type: MouseRight, Action: Down, DelayMS: 1}}
	created, err := lib.Create("click", events)
	if err != nil {
		t.Fatal(err)
	}
	events[0].DelayMS = 2
	created.Events[0].DelayMS = 3
	got, _ = lib.Read(created.ID)
	if got.Events[0].DelayMS != 1 {
		t.Fatal("create exposed event storage")
	}
}

func TestDeterministicValidation(t *testing.T) {
	cases := []struct {
		name   string
		events []Event
		want   string
	}{
		{" ", []Event{{Type: "bad"}}, "macro name is blank"},
		{"name", []Event{{Type: "bad", Action: "bad", DelayMS: -1}}, `event 0: unknown type "bad"`},
		{"name", []Event{{Type: MouseLeft, Action: "bad", DelayMS: -1}}, `event 0: unknown action "bad"`},
		{"name", []Event{{Type: MouseLeft, Action: Down, DelayMS: -1}}, "event 0: delay_ms is negative"},
		{"name", []Event{{Type: MouseRight, Action: Up}, {Type: "bad"}}, `event 1: unknown type "bad"`},
	}
	for _, tc := range cases {
		err := validate(tc.name, tc.events)
		if err == nil || err.Error() != tc.want {
			t.Fatalf("want %q, got %v", tc.want, err)
		}
	}
}
