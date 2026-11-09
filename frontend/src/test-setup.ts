import "./i18n";
import { setDeviceKeyCodes } from "./macros/device-keys";
import { expected } from "./macros/test-key-table";

// The app installs the table the backend delivers; tests install the audited one.
setDeviceKeyCodes(expected);
