package main

import "testing"

func TestCloseActionCoversEveryCombination(t *testing.T) {
	cases := []struct {
		trayOn, host bool
		want         closeDecision
	}{
		{false, false, closeQuit},
		{false, true, closeQuit},
		{true, true, closeHide},
		{true, false, closeMinimise},
	}
	for _, c := range cases {
		if got := closeAction(c.trayOn, c.host); got != c.want {
			t.Errorf("closeAction(%v, %v) = %v, want %v", c.trayOn, c.host, got, c.want)
		}
	}
}

type fakeTray struct{ destroyed int }

func (f *fakeTray) Destroy() { f.destroyed++ }

type trayHarness struct {
	host    bool
	created []*fakeTray
	hostN   int
	manager *trayManager
}

func newTrayHarness(host bool) *trayHarness {
	h := &trayHarness{host: host}
	h.manager = newTrayManager(
		func() bool { h.hostN++; return h.host },
		func() trayHandle { f := &fakeTray{}; h.created = append(h.created, f); return f },
	)
	return h
}

func TestTrayManagerCreatesOnlyWhenOnAndHostPresent(t *testing.T) {
	h := newTrayHarness(true)
	h.manager.Apply(false)
	if len(h.created) != 0 {
		t.Fatalf("tray created while off")
	}
	h.manager.Apply(true)
	h.manager.Apply(true)
	if len(h.created) != 1 {
		t.Fatalf("created = %d, want 1 (idempotent)", len(h.created))
	}
}

func TestTrayManagerDestroysWhenTurnedOffAndRecreates(t *testing.T) {
	h := newTrayHarness(true)
	h.manager.Apply(true)
	h.manager.Apply(false)
	if h.created[0].destroyed != 1 {
		t.Fatalf("destroyed = %d, want 1", h.created[0].destroyed)
	}
	h.manager.Apply(true)
	if len(h.created) != 2 {
		t.Fatalf("created = %d, want 2", len(h.created))
	}
}

func TestTrayManagerNoHostCreatesNothingAndMinimises(t *testing.T) {
	h := newTrayHarness(false)
	h.manager.Apply(true)
	if len(h.created) != 0 {
		t.Fatal("tray created without a host")
	}
	if got := h.manager.OnClose(); got != closeMinimise {
		t.Fatalf("OnClose = %v, want minimise", got)
	}
}

func TestTrayManagerOnCloseRechecksHostEachTime(t *testing.T) {
	h := newTrayHarness(false)
	h.manager.Apply(true)
	before := h.hostN
	h.manager.OnClose()
	h.host = true
	if got := h.manager.OnClose(); got != closeHide {
		t.Fatalf("OnClose after host appeared = %v, want hide", got)
	}
	if h.hostN < before+2 {
		t.Fatalf("host checked %d times, want a check per close", h.hostN-before)
	}
	if len(h.created) != 1 {
		t.Fatalf("tray should be created once the host appears, created = %d", len(h.created))
	}
}

func TestTrayManagerOnCloseQuitsWhenOffUsingCurrentValue(t *testing.T) {
	h := newTrayHarness(true)
	h.manager.Apply(true)
	if got := h.manager.OnClose(); got != closeHide {
		t.Fatalf("on: %v", got)
	}
	h.manager.Apply(false)
	if got := h.manager.OnClose(); got != closeQuit {
		t.Fatalf("off after on: %v, want quit", got)
	}
}

func TestStartWithoutWindow(t *testing.T) {
	cases := []struct {
		trayMode, trayOn, host, want bool
	}{
		{true, true, true, true},   // login start with a working tray: no window
		{true, false, true, false}, // setting was turned off since: show the window
		{true, true, false, false}, // no tray host: never invisible
		{false, true, true, false}, // a normal launch always opens the window
	}
	for _, c := range cases {
		if got := startWithoutWindow(c.trayMode, c.trayOn, c.host); got != c.want {
			t.Errorf("startWithoutWindow(%+v) = %v, want %v", c, got, c.want)
		}
	}
}
