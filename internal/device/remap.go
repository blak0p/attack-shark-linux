package device

type RemapAction string

const (
	RemapOff               RemapAction = "off"
	RemapLeft              RemapAction = "left"
	RemapRight             RemapAction = "right"
	RemapMiddle            RemapAction = "middle"
	RemapBackward          RemapAction = "backward"
	RemapForward           RemapAction = "forward"
	RemapDoubleClick       RemapAction = "double_click"
	RemapFire              RemapAction = "fire"
	RemapMediaPlayer       RemapAction = "media_player"
	RemapPlayPause         RemapAction = "play_pause"
	RemapStop              RemapAction = "stop"
	RemapPreviousTrack     RemapAction = "previous_track"
	RemapNextTrack         RemapAction = "next_track"
	RemapVolumeUp          RemapAction = "volume_up"
	RemapVolumeDown        RemapAction = "volume_down"
	RemapMute              RemapAction = "mute"
	RemapScrollUp          RemapAction = "scroll_up"
	RemapScrollDown        RemapAction = "scroll_down"
	RemapDPICycle          RemapAction = "dpi_cycle"
	RemapDPIPlus           RemapAction = "dpi_plus"
	RemapDPIMinus          RemapAction = "dpi_minus"
	RemapBrowserCalculator RemapAction = "browser_calculator"
	RemapBrowserEmail      RemapAction = "browser_email"
	RemapBrowserForward    RemapAction = "browser_forward"
	RemapBrowserBackward   RemapAction = "browser_backward"
	RemapBrowserStop       RemapAction = "browser_stop"
	RemapBrowserMyComputer RemapAction = "browser_my_computer"
	RemapBrowserRefresh    RemapAction = "browser_refresh"
	RemapBrowserHome       RemapAction = "browser_home"
	RemapBrowserSearch     RemapAction = "browser_search"
)

type RemapButton struct {
	Button           uint8
	Action           RemapAction
	PreservedDefault string
}

type RemapConfig struct{ Buttons []RemapButton }

func DefaultRemapConfig() RemapConfig {
	return RemapConfig{Buttons: []RemapButton{
		{Button: 1, Action: RemapLeft}, {Button: 2, Action: RemapRight}, {Button: 3, Action: RemapMiddle},
		{Button: 4, Action: RemapForward}, {Button: 5, Action: RemapBackward},
		{Button: 6, PreservedDefault: "DPI+"}, {Button: 7, PreservedDefault: "DPI-"},
	}}
}
