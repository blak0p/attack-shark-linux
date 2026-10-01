package desktop

import (
	"context"

	"github.com/blak0p/attack-shark-linux/internal/x6"
)

// listenerComponent owns listener attachment and frontend event emission.
// Service.mu protects attachment fields; never emit while holding it.
type listenerComponent struct {
	listener StatusListener
	events   EventSink
}

func (c *listenerComponent) attach(s *Service, listener StatusListener, events EventSink) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c.listener, c.events = listener, events
}

func (c *listenerComponent) start(ctx context.Context, s *Service) {
	s.mu.Lock()
	listener := c.listener
	s.mu.Unlock()
	if listener == nil {
		return
	}
	go func() {
		_ = listener.Listen(ctx, s.handleStatusEvent)
	}()
}

// The listener callback is serialized by Listen; only the state lock is needed
// while folding each report.
func (c *listenerComponent) handleStatusEvent(s *Service, event x6.StatusEvent) {
	battery, stage := statusDelta(event)
	s.mu.Lock()
	inventory := s.inventory
	devices := len(s.inventoryDevices)
	s.mu.Unlock()
	if inventory != nil {
		selected, ok := (selectionResolver{s}).selected()
		if !ok || devices != 1 {
			return
		}
		c.handleAttributedStatusEvent(s, StatusEvent{ID: selected.ID, Path: selected.Path, InventoryRevision: selected.InventoryRevision, Connection: string(event.Connection), Battery: battery, ActiveStage: stage})
		return
	}
	c.foldStatusEvent(s, s.legacy, StatusEvent{Connection: string(event.Connection), Battery: battery, ActiveStage: stage}, "x6:status")
}

func (c *listenerComponent) handleAttributedStatusEvent(s *Service, event StatusEvent) {
	s.mu.Lock()
	inventory := s.inventory
	s.mu.Unlock()
	if inventory == nil {
		return
	}
	selected, ok := (selectionResolver{s}).attributed(event)
	if !ok {
		return
	}
	s.mu.Lock()
	state := s.states[event.ID]
	s.mu.Unlock()
	if state == nil {
		return
	}
	if validStage(event.ActiveStage) {
		s.cancelSync(selected)
	}
	c.foldStatusEvent(s, state, event, "mouse:status")
	if validStage(event.ActiveStage) {
		c.emitConfiguration(s, selected, state)
	}
}

func (c *listenerComponent) foldStatusEvent(s *Service, state *deviceState, event StatusEvent, eventName string) {
	state.mu.Lock()
	state.connection = x6.Connection(event.Connection)
	if event.Battery != nil {
		value := *event.Battery
		state.battery = &value
	}
	if validStage(event.ActiveStage) {
		state.pending = state.applied
		stage := *event.ActiveStage
		state.observedStage = &stage
		state.observedDPI = mappedDPI(state.applied, stage)
	}
	state.mu.Unlock()
	c.emit(s, eventName, event)
}

func (c *listenerComponent) emit(s *Service, name string, payload any) {
	s.mu.Lock()
	sink := c.events
	s.mu.Unlock()
	if sink != nil {
		sink.Emit(name, payload)
	}
}

func (c *listenerComponent) emitConfiguration(s *Service, binding Binding, state *deviceState) {
	s.mu.Lock()
	sink := c.events
	s.mu.Unlock()
	if sink != nil {
		sink.Emit("mouse:configuration", ConfigurationEvent{Binding: binding, Snapshot: snapshotOf(state)})
	}
}
