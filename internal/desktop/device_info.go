package desktop

// DeviceInfo describes the model behind the service so the frontend shows its
// name and admits only macro keys the device can bind. It has no JSON tags so
// the frontend sees PascalCase keys like every desktop type.
type DeviceInfo struct {
	Name     string
	KeyCodes map[string]uint8
}

// DeviceInfo returns the injected model's display name and macro key table.
func (s *Service) DeviceInfo() DeviceInfo {
	return DeviceInfo{Name: s.model.Name(), KeyCodes: s.model.KeyCodes()}
}
