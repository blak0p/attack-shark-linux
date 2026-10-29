package main

import (
	_ "embed"
	"sync"

	"github.com/godbus/dbus/v5"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/blak0p/attack-shark-linux/internal/configstore"
	"github.com/blak0p/attack-shark-linux/internal/desktop"
)

type closeDecision int

const (
	closeQuit closeDecision = iota
	closeHide
	closeMinimise
)

// trayIcon is the app icon (packaging/icons/attack-shark-x6.svg) rendered at 64x64;
// the Linux tray needs PNG bytes.
//
//go:embed tray-icon.png
var trayIcon []byte

func (d closeDecision) String() string {
	switch d {
	case closeHide:
		return "hide"
	case closeMinimise:
		return "minimise"
	default:
		return "quit"
	}
}

// closeAction decides what closing the window does. Without a tray host the
// window is minimised so the app never becomes invisible.
func closeAction(trayOn, hostAvailable bool) closeDecision {
	switch {
	case !trayOn:
		return closeQuit
	case hostAvailable:
		return closeHide
	default:
		return closeMinimise
	}
}

// startWithoutWindow decides whether a launch creates no window at all. Only the
// login start (--tray) can skip it, and only when the tray setting is still on
// and a tray host exists, so the app is never invisible and unrecoverable.
func startWithoutWindow(trayMode, trayOn, hostAvailable bool) bool {
	return trayMode && trayOn && hostAvailable
}

const statusNotifierWatcher = "org.kde.StatusNotifierWatcher"

// trayHostAvailable reports whether a StatusNotifier host owns its well-known
// name on the session bus. Any D-Bus error counts as no host.
func trayHostAvailable() bool {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return false
	}
	defer conn.Close()
	var has bool
	err = conn.BusObject().Call("org.freedesktop.DBus.NameHasOwner", 0, statusNotifierWatcher).Store(&has)
	return err == nil && has
}

type trayHandle interface{ Destroy() }

// trayManager owns the tray lifecycle and the close decision. The setting and
// the host are read at the time of use, never captured at startup.
type trayManager struct {
	mu      sync.Mutex
	on      bool
	hasHost func() bool
	create  func() trayHandle
	handle  trayHandle
}

func newTrayManager(hasHost func() bool, create func() trayHandle) *trayManager {
	return &trayManager{hasHost: hasHost, create: create}
}

// Apply records the setting and creates or destroys the tray to match it.
func (m *trayManager) Apply(on bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.on = on
	m.syncLocked(on && m.hasHost())
}

// OnClose returns what to do for a window close, re-checking the host and
// creating the tray if a host has appeared since it was last missing.
func (m *trayManager) OnClose() closeDecision {
	m.mu.Lock()
	defer m.mu.Unlock()
	host := m.on && m.hasHost()
	m.syncLocked(host)
	return closeAction(m.on, host)
}

func (m *trayManager) syncLocked(want bool) {
	switch {
	case want && m.handle == nil:
		m.handle = m.create()
	case !m.on && m.handle != nil:
		m.handle.Destroy()
		m.handle = nil
	}
}

// lazyWindow creates the main window on first use, so a tray-only start holds
// no WebKit view until the user opens the app.
type lazyWindow struct {
	mu     sync.Mutex
	window *application.WebviewWindow
	create func() *application.WebviewWindow
}

func (l *lazyWindow) get() *application.WebviewWindow {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.window == nil {
		l.window = l.create()
	}
	return l.window
}

// installTray wires the tray into the running app. It is thin Wails glue and
// is only compile-checked; it has not been exercised on a real desktop.
func installTray(app *application.App, windows *lazyWindow, service *desktop.Service) *trayManager {
	open := func() {
		window := windows.get()
		window.Show()
		window.UnMinimise()
		window.Focus()
	}
	manager := newTrayManager(trayHostAvailable, func() trayHandle {
		menu := app.NewMenu()
		menu.Add("Open").OnClick(func(*application.Context) { open() })
		menu.Add("Quit").OnClick(func(*application.Context) { app.Quit() })
		tray := app.SystemTray.New()
		tray.SetIcon(trayIcon)
		tray.SetTooltip("Attack Shark X6 Configurator")
		tray.SetMenu(menu)
		tray.OnClick(open)
		return tray
	})
	service.AddWatcherChangeHandler(func(s configstore.WatcherSettings) { manager.Apply(s.Tray) })
	manager.Apply(service.GetWatcherSettings().Tray)
	return manager
}

// hookClose applies the tray close decision to window.
func hookClose(window *application.WebviewWindow, manager *trayManager) {
	window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		switch manager.OnClose() {
		case closeHide:
			event.Cancel()
			window.Hide()
		case closeMinimise:
			event.Cancel()
			window.Minimise()
		}
	})
}
