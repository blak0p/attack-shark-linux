// Independent rows from the official converter evidence (the Go side pins the same
// 104 identities in macrocodec/x6keys_test.go). Tests install it the way the app
// installs the table the backend delivers through DeviceInfo.
export const expected: Record<string, number> = {};
for (let i = 0; i < 26; i++) expected[`Key${String.fromCharCode(65 + i)}`] = 4 + i;
for (let i = 1; i <= 9; i++) expected[`Digit${i}`] = 0x1d + i;
expected.Digit0 = 0x27;
["Enter", "Escape", "Backspace", "Tab", "Space", "Minus", "Equal", "BracketLeft", "BracketRight", "Backslash"].forEach((key, i) => expected[key] = 0x28 + i);
["Semicolon", "Quote", "Backquote", "Comma", "Period", "Slash", "CapsLock"].forEach((key, i) => expected[key] = 0x33 + i);
for (let i = 1; i <= 12; i++) expected[`F${i}`] = 0x39 + i;
["PrintScreen", "ScrollLock", "Pause", "Insert", "Home", "PageUp", "Delete", "End", "PageDown", "ArrowRight", "ArrowLeft", "ArrowDown", "ArrowUp", "NumLock", "NumpadDivide", "NumpadMultiply", "NumpadSubtract", "NumpadAdd"].forEach((key, i) => expected[key] = 0x46 + i);
for (let i = 1; i <= 9; i++) expected[`Numpad${i}`] = 0x58 + i;
Object.assign(expected, { Numpad0: 0x62, NumpadDecimal: 0x63, ContextMenu: 0x65, NumpadEnter: 0x28 });
["ControlLeft", "ShiftLeft", "AltLeft", "MetaLeft", "ControlRight", "ShiftRight", "AltRight", "MetaRight"].forEach((key, i) => expected[key] = 0xe0 + i);
