# Attack Shark X6 Linux Configurator — User Guide

This guide provides a comprehensive walkthrough of the **Attack Shark Linux** desktop configurator for the Attack Shark X6 wireless gaming mouse (`VID:PID 1d57:fa60`). The application is built with Go + [Wails v3](https://wails.io) and React + Vite, communicating directly with the hardware through Linux `/dev/hidraw` devices without background root daemons, kernel driver modifications, or proprietary Windows services.

---

## Table of Contents

- [Overview & Workspace Layout](#overview--workspace-layout)
- [Performance View](#performance-view)
- [Lighting View](#lighting-view)
- [Controls View](#controls-view)
- [Button Remapping View](#button-remapping-view)
- [Device & Maintenance View](#device--maintenance-view)
- [Emergency Recovery](#emergency-recovery)

---

## Overview & Workspace Layout

The user interface follows GNOME HIG design principles with an uncluttered dark theme, instant visual feedback, and keyboard-friendly navigation.

### Global Workspace Elements

- **Navigation Rail (Left Sidebar)**: Switch between the five core configuration panels:
  - **◉ Performance**: DPI stages, polling rate, and normal sleep timeout.
  - **☼ Lighting**: RGB effect modes, speed variants, and color presets.
  - **⌁ Controls**: Sensor enhancements (Angle snap, Ripple control, Lift-off distance) and key response debounce.
  - **↺ Button remapping**: 7-button customization with staging and draft confirmation.
  - **▣ Device**: Hardware status, live battery monitoring, device inventory, and factory reset.
- **Top Bar**: Displays the active mouse profile name (**Attack Shark X6**) and a device selector when multiple eligible receivers are available.
- **Operation Feedback (Bottom Right)**: One shared spinner shows `Working…` during device changes, library operations, reset, and updates. Confirmations disappear after three seconds: `Applied` for acknowledged settings, `Saved` for local library saves or persistence retries, and `Sent` for macro transport. Startup snapshots do not trigger success notices. Errors remain visible with a `Retry` action when available; a local-save retry does not resend settings to the mouse. Macro transport is not proof of physical playback or device persistence; expand **Device details** for that distinction.
- **Connection & Battery Card (Bottom Left)**: Live indicator showing device availability (`Device available` / `Device unavailable`) and current battery level percentage (`Battery 100%`). Hardware battery heartbeats and physical DPI button presses are pushed live from the mouse dongle to the UI.
- **Auto-Sync vs. Draft Confirmation**:
  - Sliders and toggles in **Performance**, **Lighting**, and **Controls** are debounced and automatically applied to the hardware after one second of user inactivity.
  - **Button Remapping** uses an explicit draft workflow requiring confirmation before sending new bindings to the hardware, preventing accidental lockouts.

### Local macro feedback

Saving, importing, and deleting macros change only the local library. The shared spinner shows work in progress; confirmations disappear after three seconds. Errors remain visible, including device errors that do not prevent a successful local save.

---

## Performance View

The **Performance** view manages sensor resolution, USB reporting frequency, and energy-saving sleep timers.

![Performance View](assets/screenshots/ui-performance.png)

### 1. Active DPI Stage & Stage Tuning

- **Active DPI Stage Selector**: Displays the active DPI stage (Stages 1 through 6) with individual DPI values and color indicators. Clicking any stage immediately activates it on the mouse. Pressing the physical DPI buttons on the mouse also updates the active stage in real time.
- **Granular DPI Slider**: Adjusts the resolution of the currently selected stage from **50 DPI** to **26,000 DPI** in precise **50-DPI increments**.
- **Factory DPI Stages**:
  - Stage 1: 800 DPI
  - Stage 2: 1,200 DPI
  - Stage 3: 1,600 DPI
  - Stage 4: 3,200 DPI
  - Stage 5: 5,600 DPI
  - Stage 6: 26,000 DPI

### 2. Polling Rate

Controls how frequently the mouse reports position updates to the host computer:

- **Selectable Rates**: `125 Hz` (8 ms), `250 Hz` (4 ms), `500 Hz` (2 ms), and `1,000 Hz` (1 ms).
- **Default**: `1,000 Hz` for maximum responsiveness and tracking accuracy in competitive gaming.
- **Battery Impact**: Lower rates (e.g., 250 Hz or 500 Hz) reduce CPU wakeups and conserve wireless battery life during productivity or travel use.
- **Feedback**: Uses the shared `Working…` / `Applied` feedback. The selected rate stays visible in its control; local-save failures remain visible separately.

### 3. Normal Sleep

Configures how long the mouse remains active during idle periods before entering low-power sleep mode:

- **Adjustment Range**: **0.5 minutes (30 seconds)** to **60 minutes**, adjusted via slider in 0.5-minute increments.
- **Wake Behavior**: The mouse wakes immediately upon motion or click input.
- **Default**: `0.5 minutes` (30 seconds) for optimal wireless power efficiency.

---

## Lighting View

The **Lighting** view configures the RGB LED lighting strip and illuminated logo on the Attack Shark X6.

![Lighting View](assets/screenshots/ui-lighting.png)

### 1. Effect Selection

- **Hardware Effect Modes**: Choose from firmware-supported lighting patterns such as `Fixed` (steady color), breathing, dynamic wave/streaming, and off. Effect names and capabilities are queried directly from the device firmware.

### 2. Speed Variants & Color Templates

- **Animation Speed Slider**: For animated lighting effects, adjust the animation speed step-by-step.
- **Color Presets**: Select from device-supported color swatches (e.g., Neon Green, Magenta, Orange, Yellow) or effect-controlled multi-color modes.
- **Feedback**: Uses the same progress indicator and brief `Applied` confirmation as other settings.

---

## Controls View

The **Controls** view provides fine-grained control over sensor tracking characteristics and switch debounce timings.

![Controls View](assets/screenshots/ui-controls.png)

### 1. Mouse Features (Sensor Enhancements)

- **Angle Snap** (Switch):
  - *Align sensor movement to straight lines.*
  - When enabled, the sensor firmware suppresses slight angular deviations, snapping horizontal and vertical sweeps to straight axes.
  - Useful for CAD drafting, drawing, or horizontal tracking drills; recommended **OFF** for standard FPS gameplay.
- **Ripple Control** (Switch):
  - *Smooth sensor jitter at high DPI values.*
  - Applies a smoothing filter to eliminate high-frequency sensor noise and erratic cursor jitter at very high resolutions (typically above 5,000 DPI).
  - Keeps cursor tracking clean without introducing perceptible input lag at normal DPI ranges.
- **Lift-Off Distance (LOD)** (Dropdown):
  - *Sensor tracking cutoff height.*
  - **1 mm** (Default): Low cutoff height. Stops tracking almost immediately when the mouse is slightly raised, eliminating cursor drift during aggressive pick-up-and-swipe mouse resets.
  - **2 mm**: Higher tracking cutoff. Suitable for uneven or textured mouse pads that require a wider optical focal range.

### 2. Key Response Time (Debounce)

- **Adjustable Debounce Slider**: Configures the microswitch debouncing window from **2 ms** to **25 ms** in 1 ms steps.
- **Default**: `8 ms`.
- **Tradeoffs**:
  - *Lower values (2–4 ms)*: Minimize click registration latency for maximum reaction speed in competitive gaming.
  - *Higher values (8–16 ms)*: Provide greater electrical switch debounce margin, preventing accidental double-clicking on aging switches.
- **Value & Retry**: The chosen milliseconds stay visible beside the title. Progress and confirmation use the shared feedback region; local-save failures retain their retry action.

---

## Button Remapping View

The **Button remapping** view configures the physical buttons of the mouse. To prevent accidental loss of mouse control, button remapping uses a **draft and confirmation workflow**.

![Button Remapping View](assets/screenshots/ui-button-remapping.png)

### 1. Remappable Physical Buttons

All 7 physical mouse controls can be assigned custom actions:

1. **Button 1**: Left Click (*Multimedia and Mouse Controls actions are unavailable; Basic and Browser actions are available*)
2. **Button 2**: Right Click
3. **Button 3**: Middle Click (Scroll wheel click)
4. **Button 4**: Forward (Front thumb button)
5. **Button 5**: Backward (Rear thumb button)
6. **Button 6**: DPI+ (Top button behind the scroll wheel)
7. **Button 7**: DPI− (Lower top button behind DPI+)

### 2. Available Action Library

Each button can be assigned to actions grouped into four distinct categories:

| Category | Actions |
|---|---|
| **Basic** | Left, Right, Middle, Forward, Backward, Double Click, Fire, Off |
| **Multimedia** | Media Player, Play/Pause, Stop, Previous Track, Next Track, Volume Up, Volume Down, Mute |
| **Mouse Controls** | Scroll Up, Scroll Down, DPI Cycle, DPI+, DPI− |
| **Browser** | Calculator, Email, Forward, Backward, Stop, My Computer, Refresh, Home, Search |

Browser Favorites is unavailable because its captured ID is uncertain. The Browser Forward and Backward actions are distinct from the Basic thumb-button Forward and Backward actions.

### Macro playback modes

Choose **Macro** in a button's action menu, select a compatible saved mouse-click macro, then choose its playback mode:

| Mode | Intended behavior |
|---|---|
| **Repeat a set number of times** (default) | Run the sequence 1–255 times. |
| **Repeat until pressed again** | Start on press; press again to stop. |
| **Repeat while button is held** | Repeat while the assigned button is held; release to stop. |

The two continuous modes hide the fixed repeat count and stage a repeat value of 1. Mode belongs to the button assignment, not the saved macro library; reopening the dialog restores the staged mode. **Confirm** only stages locally; **Apply remap** sends the assignment.

Physical playback remains unverified, including continuous stop-on-second-press and release-to-stop behavior. Transport confirmation is not proof of playback or device persistence. Keyboard events, delays, and expanded click counts are not added by these modes.

### 3. Draft Confirmation Workflow

Unlike performance sliders that auto-apply after a delay, button remapping uses an explicit confirmation model:

1. **Drafting**: Select new actions for any button using the dropdown menus. The summary text updates live to reflect the staged assignment.
2. **Reviewing**: A draft status indicator (`Remap draft pending confirmation`) signals that changes have not yet been sent to the mouse.
3. **Applying**: Click **Apply remap** (or press `Enter`) to compile and write the 59-byte report over hidraw. The applied assignment advances only after the device acknowledges it. The status updates to `Button remapping applied`.
4. **Discarding**: Click **Discard remap** (or press `Escape`) to cancel unapplied changes and restore the currently active configuration.

---

## Device & Maintenance View

The **Device** view provides hardware diagnostics, multi-device management, and reset operations.

![Device View](assets/screenshots/ui-device.png)

### 1. Status Information

- **Application version**: Installed version of the configurator (e.g., `1.2.1`).
- **Interface**: Active physical interface through which the mouse is communicating (`dongle` for 2.4 GHz wireless receiver or `wired` for USB connection).
- **Battery charge**: Real-time remaining battery capacity percentage (`100%`).
- **Serial identifier**: Hardware serial identifier reported by the dongle or `unavailable` for serialless dongles.

### 2. Device Inventory

Displays all detected Attack Shark devices connected to the host system. When multiple receivers or mice are attached, the configurator lists each device path, interface type, and eligibility status, allowing seamless profile switching.

### 3. In-App Factory Reset

If custom settings cause unintended behavior, you can restore factory defaults directly from the UI:

1. Click **Reset to factory** in the Reset card.
2. An inline confirmation dialog appears:
   > *This resets DPI, lighting/sleep/key response time, polling, and button remapping on the selected device.*
3. Click **Confirm factory reset** to write the captured defaults across four supported lanes, or **Cancel factory reset** to abort without sending a reset.

Reset selects **DPI stage 2**, **Static DPI lighting**, **0.5-minute normal sleep**, **4-ms key response time**, **1000-Hz polling**, and the default button layout. DPI values and stage colors retain the documented six-stage baseline. Ordinary application defaults remain stage 4 and 8 ms; reset uses a separate capture-backed baseline.

Only after all four lanes acknowledge are stale local profiles cleared and the displayed DPI, polling, lighting, sleep, debounce, and remapping snapshots refreshed. A failure does not report success or replace the displayed settings with defaults; hardware may be partially changed. This is not a verified complete device-memory/EEPROM erase or onboard macro deletion.

---

### 4. Updates

Startup silently checks AppImage and USB permissions together. An upper-right
**Updates** notice appears only when an update is available or a request fails;
there is no loading or up-to-date panel. It labels AppImage, USB permissions (udev),
or both. Choose **Apply updates** explicitly; expand **Update details** for the
explanation. USB eligibility comes from backend-verified policy contents, not a
release version or a changed manifest alone. No Device navigation is needed.
Failures stay visible, with **Check for updates** for explicit reinspection and
**Relaunch app** when applicable. The notice scrolls on smaller screens.
Checking never applies anything; silence is not proof of current USB permissions.

**Apply updates** approves the unfinished eligible operations together. Missing or
recognized official outdated USB rules use one system authorization dialog; custom
rules are preserved and current rules are not overwritten. The app never collects
a password. AppImage replacement remains user-local. Older backends retain the
legacy **Update and restart** banner.

A confirmed USB-only success shows **USB permissions updated.** for three seconds,
then hides the finished notice. Errors and still-needed AppImage installation or
restart/relaunch actions remain visible.

Outcomes are independent: a written policy can still have a reload/activation
error, and a replaced AppImage does not prove it relaunched. Inspect each displayed
error before retrying. Explicit retries do not repeat completed writes; **Relaunch
app** requests restart while keeping any policy error visible. Reconnect the mouse
if needed. Missing helper or authorization-agent errors require the separate
[trusted setup procedure](linux-usb-prerequisites.md#one-time-authenticated-usb-permissions-setup),
not running the app as root. The UI does not run that procedure.

**Safe user-led validation before installation:** use the upper-right Updates window, record the
affected targets (if a notice appears), expand Update details, and check that reviewing/checking causes no system
authorization or apply. Do not click Apply updates or run setup until you choose to
authorize host changes. After a separately authorized update, record individual
outcomes and reconnect/test the mouse; do not treat a reload error as success.

---

## Emergency Recovery

If an applied button remapping disables left-click or renders the mouse unnavigable in graphical environments, you do not need the GUI to recover. Attack Shark Linux includes a headless, command-line recovery tool.

### Running the Headless Reset

Execute the reset command from your terminal:

```sh
attack-shark-linux reset
```

If using the user-local AppImage:

```sh
cd ~/.local/share/attack-shark-x6
./attack-shark-linux-x86_64.AppImage reset
```

### What Emergency Reset Does

1. **No Window**: Executes completely headlessly without loading the Wails application or React frontend.
2. **Hardware Target Discovery**: Discovers and validates the connected Attack Shark X6 hidraw node using `/dev/hidraw*`.
3. **Synchronous Factory Write**: Directly writes the documented factory configuration:
   - DPI: Default 6-stage curve (800, 1200, 1600, 3200, 5600, 26000), active stage 2
   - Lighting: Captured Static DPI reset template (distinct from the ordinary Static DPI template)
   - Polling rate: 1,000 Hz
   - Normal sleep: 0.5 minutes (30 seconds)
   - Sensor features: Angle snap OFF, Ripple control OFF, LOD 1 mm
   - Key response time: 4 ms
   - Button mapping: Default 7-button layout (Button 1: Left, Button 2: Right, etc.)
   The lane order is DPI (`0x04`), lighting/sleep/debounce (`0x05`), polling (`0x06`), then remapping (`0x08`); each requires an ACK and a failure stops later lanes. Complete memory erasure and onboard macro deletion are unverified.
4. **Atomic Profile Cleanup**: Only after all four ACKs, cleans and replaces the application state directory (`~/.config/attack-shark-linux/`) with clean defaults so corrupt state is not re-applied upon launching the GUI again.
5. **Unprivileged Operation**: Uses your existing local-seat udev permissions (`TAG+="uaccess"`); **do not run as root**.
