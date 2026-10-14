package macros

import (
	"embed"
	"encoding/json"
)

// Official normal-key converter table, PE SHA256
// 154b825d700231f980883376f332fe78b93d295aa5ce8ef839ab41341aba8f6e,
// converter 0x401780, 112 rows at 0x41cef0 (file offset 0x1b4f0).
// Browser codes name physical positions, not layout-dependent typed text.
// NumpadEnter is an inferential VK_RETURN alias, NOT VK_SEPARATOR (0x58).
// OEM8 and VK_SEPARATOR have no demonstrated browser physical identity.
//
//go:embed x6keys.json
var officialKeyFiles embed.FS

var x6KeyboardCodes = func() map[string]byte {
	data, err := officialKeyFiles.ReadFile("x6keys.json")
	if err != nil {
		panic(err)
	}
	var keys map[string]byte
	if err := json.Unmarshal(data, &keys); err != nil {
		panic(err)
	}
	return keys
}()
