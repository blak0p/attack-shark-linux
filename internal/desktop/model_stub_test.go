package desktop

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/blak0p/attack-shark-linux/internal/device"
	"github.com/blak0p/attack-shark-linux/internal/macros"
	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"github.com/blak0p/attack-shark-linux/internal/transport"
)

// stubModel is a self-contained device.Model for the desktop tests. It speaks a
// toy wire format whose report IDs and acknowledgements mirror the shape the
// tests drive (report ID in byte 0, ACK 03 10 50 00 <id>) and applies the
// smallest validation and macro admission rules these tests exercise. The real
// rules and bytes are pinned by the model's own tests.
type stubModel struct{}

func testModel() device.Model { return stubModel{} }

func (stubModel) DPIOperation() mouse.CommandOperation {
	return stubOperation{id: 0x04, length: 56, encode: stubEncodeDPI}
}
func (stubModel) PollingOperation() mouse.CommandOperation {
	return stubOperation{id: 0x06, length: 9, encode: stubEncodePolling}
}
func (stubModel) LightingOperation() mouse.CommandOperation {
	return stubOperation{id: 0x05, length: 13, encode: stubEncodeLighting}
}
func (stubModel) LightingSettingsOperation() mouse.CommandOperation {
	return stubOperation{id: 0x05, length: 13, encode: stubEncodeLightingSettings}
}
func (stubModel) RemapOperation() mouse.CommandOperation {
	return stubOperation{id: 0x08, length: 59, encode: stubEncodeRemap}
}

func (stubModel) ValidateNormalSleepMinutes(minutes float64) error {
	units := minutes * 2
	if minutes < device.NormalSleepMinimumMinutes || minutes > device.NormalSleepMaximumMinutes || units != float64(int(units)) {
		return fmt.Errorf("normal sleep %v is out of range", minutes)
	}
	return nil
}

func (stubModel) ValidateResponseTime(ms int) error {
	if ms < device.ResponseTimeMinMs || ms > device.ResponseTimeMaxMs {
		return fmt.Errorf("response time %d is out of range", ms)
	}
	return nil
}

var stubLightingTemplates = map[device.LightingTemplateID]device.LightingMode{
	device.LightingTemplateStaticDPIFactoryReset: device.LightingStaticDPI,
	device.LightingTemplateOff:                   device.LightingOff,
	device.LightingTemplateFixedGreen:            device.LightingFixed,
	device.LightingTemplateBreathingGreen:        device.LightingBreathing,
	device.LightingTemplateBreathingFE5EF9:       device.LightingBreathing,
	device.LightingTemplateBreathingFF7F00:       device.LightingBreathing,
	device.LightingTemplateBreathingFFFF00:       device.LightingBreathing,
	device.LightingTemplateStaticDPIDefault:      device.LightingStaticDPI,
	device.LightingTemplateNeonOne:               device.LightingNeon,
	device.LightingTemplateNeonTwo:               device.LightingNeon,
	device.LightingTemplateColorBreathingOne:     device.LightingColorBreathing,
	device.LightingTemplateColorBreathingTwo:     device.LightingColorBreathing,
	device.LightingTemplateBreathingDPIOne:       device.LightingBreathingDPI,
	device.LightingTemplateBreathingDPITwo:       device.LightingBreathingDPI,
	device.LightingTemplateBreathingDPIThree:     device.LightingBreathingDPI,
}

func (stubModel) LightingEffects() []device.LightingEffect {
	return []device.LightingEffect{{Mode: device.LightingFixed, Label: "Fixed", DefaultTemplateID: device.LightingTemplateFixedGreen}}
}

// stubOperation validates by encoding, like a real command operation.
type stubOperation struct {
	id     byte
	length int
	encode func(any) ([]byte, error)
}

func (o stubOperation) Validate(value any) error { _, err := o.encode(value); return err }
func (o stubOperation) Encode(value any) ([]byte, error) {
	return o.encode(value)
}
func (o stubOperation) MatchesACK(report []byte) bool {
	return len(report) == 5 && report[0] == 0x03 && report[1] == 0x10 && report[2] == 0x50 && report[3] == 0 && report[4] == o.id
}

func stubEncodeDPI(value any) ([]byte, error) {
	config, ok := value.(device.DPIConfig)
	if !ok {
		return nil, fmt.Errorf("DPI configuration has type %T", value)
	}
	if config.StageMask == 0 || config.LiftDistance > 1 || config.ActiveStage < 1 || config.ActiveStage > 8 {
		return nil, fmt.Errorf("invalid DPI configuration controls")
	}
	report := make([]byte, 56)
	report[0], report[5], report[7], report[24] = 0x04, config.StageMask, config.LiftDistance, config.ActiveStage
	for stage, dpi := range config.DPI {
		if dpi < 50 || dpi%50 != 0 || dpi > 3276800 {
			return nil, fmt.Errorf("stage %d DPI %d is invalid", stage+1, dpi)
		}
		raw := uint16(dpi/50 - 1)
		report[8+stage], report[16+stage] = byte(raw), byte(raw>>8)
		copy(report[25+stage*3:], config.Colors[stage][:])
	}
	return report, nil
}

func stubEncodePolling(value any) ([]byte, error) {
	rate, ok := value.(device.PollingRate)
	if !ok {
		return nil, fmt.Errorf("polling rate has type %T", value)
	}
	switch rate {
	case device.PollingRate125, device.PollingRate250, device.PollingRate500, device.PollingRate1000:
		divisor := byte(1000 / rate)
		return []byte{0x06, 0x09, 0x01, divisor, ^divisor, 0, 0, 0, 0}, nil
	}
	return nil, fmt.Errorf("unsupported polling rate %d Hz", rate)
}

func stubEncodeLighting(value any) ([]byte, error) {
	selection, ok := value.(device.LightingSelection)
	if !ok {
		return nil, fmt.Errorf("lighting selection has type %T", value)
	}
	return stubLightingReport(selection)
}

func stubLightingReport(selection device.LightingSelection) ([]byte, error) {
	mode, known := stubLightingTemplates[selection.TemplateID]
	if !known || mode != selection.Mode {
		return nil, fmt.Errorf("unsupported lighting selection %q for mode %#x", selection.TemplateID, selection.Mode)
	}
	report := make([]byte, 13)
	report[0], report[3] = 0x05, byte(selection.Mode)
	if selection.Color == "" {
		return report, nil
	}
	if selection.Mode != device.LightingFixed && selection.Mode != device.LightingBreathing {
		return nil, fmt.Errorf("lighting mode %#x does not accept a custom color", selection.Mode)
	}
	rgb, err := hex.DecodeString(strings.TrimPrefix(selection.Color, "#"))
	if err != nil || len(rgb) != 3 || !strings.HasPrefix(selection.Color, "#") {
		return nil, fmt.Errorf("lighting color %q is not #RRGGBB", selection.Color)
	}
	copy(report[6:9], rgb)
	return report, nil
}

func stubEncodeLightingSettings(value any) ([]byte, error) {
	settings, ok := value.(device.LightingSettings)
	if !ok {
		return nil, fmt.Errorf("lighting settings have type %T", value)
	}
	model := stubModel{}
	if err := model.ValidateNormalSleepMinutes(settings.NormalSleepMinutes); err != nil {
		return nil, err
	}
	if err := model.ValidateResponseTime(settings.ResponseTimeMs); err != nil {
		return nil, err
	}
	report, err := stubLightingReport(settings.LightingSelection)
	if err != nil {
		return nil, err
	}
	report[9], report[10] = byte(settings.NormalSleepMinutes*2), byte(settings.ResponseTimeMs)
	return report, nil
}

var stubRemapActions = []device.RemapAction{
	device.RemapOff, device.RemapLeft, device.RemapRight, device.RemapMiddle, device.RemapForward, device.RemapBackward, device.RemapDoubleClick, device.RemapFire,
	device.RemapMediaPlayer, device.RemapPlayPause, device.RemapStop, device.RemapPreviousTrack, device.RemapNextTrack, device.RemapVolumeUp, device.RemapVolumeDown, device.RemapMute,
	device.RemapScrollUp, device.RemapScrollDown, device.RemapDPICycle, device.RemapDPIPlus, device.RemapDPIMinus,
	device.RemapBrowserCalculator, device.RemapBrowserEmail, device.RemapBrowserForward, device.RemapBrowserBackward, device.RemapBrowserStop,
	device.RemapBrowserMyComputer, device.RemapBrowserRefresh, device.RemapBrowserHome, device.RemapBrowserSearch,
}

func stubActionID(action device.RemapAction) byte {
	for index, candidate := range stubRemapActions {
		if candidate == action {
			return byte(index + 1)
		}
	}
	return 0
}

func stubEncodeRemap(value any) ([]byte, error) {
	config, ok := value.(device.RemapConfig)
	if !ok {
		return nil, fmt.Errorf("remap configuration has type %T", value)
	}
	if len(config.Buttons) != 7 {
		return nil, fmt.Errorf("remap configuration has %d buttons, want 7", len(config.Buttons))
	}
	report := make([]byte, 59)
	report[0] = 0x08
	for index, button := range config.Buttons {
		if button.Button != uint8(index+1) {
			return nil, fmt.Errorf("remap button %d is out of order", button.Button)
		}
		if button.Action == "" {
			if index < 5 || button.PreservedDefault == "" {
				return nil, fmt.Errorf("remap button %d has no allowed action", button.Button)
			}
			continue
		}
		id := stubActionID(button.Action)
		if id == 0 {
			return nil, fmt.Errorf("remap button %d has unsupported action %q", button.Button, button.Action)
		}
		if button.Button == 1 && id >= stubActionID(device.RemapMediaPlayer) && id <= stubActionID(device.RemapDPIMinus) {
			return nil, fmt.Errorf("remap button 1 does not support action %q", button.Action)
		}
		report[3+index*3] = id
	}
	return report, nil
}

// stubMacroDestinations maps logical buttons 1-7 to firmware destinations.
var stubMacroDestinations = [7]byte{1, 2, 3, 7, 8, 5, 6}

func stubDestination(button uint8) (byte, error) {
	if button < 1 || int(button) > len(stubMacroDestinations) {
		return 0, fmt.Errorf("unsupported macro button %d", button)
	}
	return stubMacroDestinations[button-1], nil
}

// stubMacroKeys is the keyboard subset the admission stub accepts; NumpadEnter
// is an alias of Enter.
var stubMacroKeys = map[string]string{"ShiftLeft": "ShiftLeft", "ControlLeft": "ControlLeft", "KeyA": "KeyA", "KeyD": "KeyD", "KeyF": "KeyF", "KeyS": "KeyS", "Enter": "Enter", "NumpadEnter": "Enter"}

func (stubModel) AdmitMacro(button uint8, events []macros.Event, repeat int, mode uint8) error {
	if _, err := stubDestination(button); err != nil {
		return err
	}
	if repeat < 1 || repeat > 255 || mode > 2 || len(events) == 0 || len(events) > 46 {
		return fmt.Errorf("macro is outside the admitted bounds")
	}
	held := map[string]string{}
	for _, event := range events {
		key := string(event.Type)
		if event.Type == macros.Keyboard {
			canonical, known := stubMacroKeys[event.KeyCode]
			if !known {
				return fmt.Errorf("unsupported key %q", event.KeyCode)
			}
			key += ":" + canonical
		} else if event.KeyCode != "" {
			return fmt.Errorf("unexpected key code on %s", event.Type)
		}
		if event.DelayMS < 0 || event.DelayMS >= 51200 {
			return fmt.Errorf("delay %d is out of range", event.DelayMS)
		}
		switch event.Action {
		case macros.Down:
			if _, down := held[key]; down {
				return fmt.Errorf("%s pressed twice", key)
			}
			held[key] = event.KeyCode
		case macros.Up:
			// A key must be released with the code it was pressed with.
			if code, down := held[key]; !down || code != event.KeyCode {
				return fmt.Errorf("%s released without press", key)
			}
			delete(held, key)
		default:
			return fmt.Errorf("unknown action %q", event.Action)
		}
	}
	if len(held) != 0 {
		return fmt.Errorf("macro leaves keys held")
	}
	return nil
}

func (m stubModel) ValidateMacroAssignments(pending device.RemapConfig, specs []device.MacroSpec) error {
	_, err := m.MacroAssignmentBatch(pending, specs)
	return err
}

// stubMacroChunk is the decoded form of one stub upload chunk.
type stubMacroChunk struct {
	Destination byte
	Repeat      int
	Mode        uint8
	Events      []macros.Event
}

func (m stubModel) MacroAssignmentBatch(pending device.RemapConfig, specs []device.MacroSpec) (mouse.Batch, error) {
	setup, err := stubEncodeRemap(pending)
	if err != nil {
		return mouse.Batch{}, err
	}
	var chunks [][]byte
	for _, spec := range specs {
		destination, err := stubDestination(spec.Button)
		if err != nil {
			return mouse.Batch{}, err
		}
		if err := m.AdmitMacro(spec.Button, spec.Events, spec.Repeat, spec.Mode); err != nil {
			return mouse.Batch{}, err
		}
		setup[3+int(destination-1)*3] = 0x12
		chunk, err := json.Marshal(stubMacroChunk{Destination: destination, Repeat: spec.Repeat, Mode: spec.Mode, Events: spec.Events})
		if err != nil {
			return mouse.Batch{}, err
		}
		chunks = append(chunks, chunk)
	}
	return mouse.Batch{Setup: setup, Chunks: chunks}, nil
}

// stubProfile registers the stub model under the test device identity.
type stubProfile struct{ match transport.Match }

func newStubProfile() mouse.Profile {
	return stubProfile{match: transport.Match{VendorID: 0x1D57, ProductID: 0xFA60}}
}
func newStubWiredProfile() mouse.Profile {
	return stubWiredProfile{stubProfile{match: transport.Match{VendorID: 0x1D57, ProductID: 0xFA61}}}
}

func (p stubProfile) ID() string                   { return "attack-shark-x6" }
func (p stubProfile) Match() transport.Match       { return p.match }
func (stubProfile) AllowsSeriallessIdentity() bool { return true }
func (stubProfile) Codec() mouse.Codec             { return stubCodec{} }
func (stubProfile) HIDFacts() mouse.HIDFacts {
	return mouse.HIDFacts{
		StatusInput:             transport.InputDescriptor{InterfaceNumber: 2, UsagePage: 1, Usage: 0x80, EndpointAddress: 0x83},
		ConfigurationReportSize: 56,
		Connection:              transport.Dongle,
		FeatureReports:          []mouse.FeatureReport{{ID: 0x04, Length: 56}, {ID: 0x06, Length: 9}, {ID: 0x05, Length: 13}, {ID: 0x08, Length: 59}},
	}
}

type stubWiredProfile struct{ stubProfile }

func (p stubWiredProfile) HIDFacts() mouse.HIDFacts {
	facts := p.stubProfile.HIDFacts()
	facts.Connection = transport.Wired
	return facts
}

type stubCodec struct{}

func (stubCodec) Validate(configuration any) error {
	_, err := stubEncodeDPI(configuration)
	return err
}
func (stubCodec) Encode(configuration any) ([]byte, error) {
	return stubEncodeDPI(configuration)
}
func (stubCodec) DecodeStatus([]byte) (any, bool) { return nil, false }
func (stubCodec) MatchesACK(report []byte) bool {
	return stubOperation{id: 0x04}.MatchesACK(report)
}
func (stubCodec) Defaults() any { return device.DefaultDPIConfig() }
