package desktop

// selectionResolver centralizes immutable binding selection and validation for
// device-specific operations. Never hold the service lock while asking the
// inventory for its selection: inventory operations have their own lock.
type selectionResolver struct{ service *Service }

func (r selectionResolver) selected() (Binding, bool) {
	r.service.mu.Lock()
	inventory := r.service.inventory
	r.service.mu.Unlock()
	if inventory == nil {
		return Binding{}, false
	}
	return inventory.Selection()
}

func (r selectionResolver) current(binding Binding) bool {
	selected, ok := r.selected()
	return ok && selected == binding
}

func (r selectionResolver) attributed(event StatusEvent) (Binding, bool) {
	selected, ok := r.selected()
	return selected, ok && selected.ID == event.ID && selected.Path == event.Path && selected.InventoryRevision == event.InventoryRevision
}
