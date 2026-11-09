// Exact browser physical identity -> raw device key code, never typed text.
// The table belongs to the connected model: the backend delivers it through
// DeviceInfo and the app installs it here. Until then nothing is admitted.
let keyCodes: Readonly<Record<string, number>> = {};

export function setDeviceKeyCodes(table: Readonly<Record<string, number>>): void {
  keyCodes = { ...table };
}

export function deviceKeyCode(identity: string): number | undefined {
  return Object.prototype.hasOwnProperty.call(keyCodes, identity) ? keyCodes[identity] : undefined;
}
