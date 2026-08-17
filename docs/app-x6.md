# Official Attack Shark X6 App — Complete UI Map

> **Source**: `official-app/` (extraction of `ATTACK SHARK X6 SOFT.exe`, Inno Setup
> v6.1.0, "Attack SharkX6Mouse", X6.exe build 2025-11-10).
> **Goal**: inventory every section, tab, button, and option of the official app
> so the Linux driver (`attack_shark_linux`) knows what to replicate.
> **App languages**: `official-app/res/lan/` (CN, DE, EN, ES, IT, JP, KR, PT, RU, VN).

The `resourcetext="true"` attributes in the XML reference IDs in `lan/*.xml`; this
doc uses the English texts (`lan/EN.xml`) as the canonical names.

---

## 1. Main window (`main.xml`, 1127x760)

The main window has no top menu of its own; the system menu comes from
`traymenu.xml` and the central layout is the page `res/MS/MS_1/ms_1_page.xml`.

### 1.1 Left column — panels

Each panel is a collapsible `CheckBox` (header) + a `VerticalLayout` with its
content.

| Panel | Header (EN id) | Content |
|---|---|---|
| `ms_1_check_button` | **Button Settings** | List of remappable buttons |
| `ms_1_btn_macro` | **Macro Manager** | Button that opens the macro manager (separate window) |
| `ms_1_check_profile` | **Profile** | Profile management |
| `ms_1_check_power` | **Battery** | Battery bar + percentage |

#### Remappable buttons (`button_setting_text`)

Numbered list (image of each mouse button + assignment button):

| Control | Position on the mouse (skins/mouse/1..7.png) |
|---|---|
| `ms_1_btn_key_1` | button 1 (left) |
| `ms_1_btn_key_2` | button 2 (right) |
| `ms_1_btn_key_3` | button 3 (wheel, click) |
| `ms_1_btn_key_7` | button 4 (forward) |
| `ms_1_btn_key_8` | button 5 (backward) |
| `ms_1_btn_key_5` | button 6 |
| `ms_1_btn_key_6` | button 7 |
| `ms_1_btn_key_4` | (hidden) |
| `ms_1_btn_key_9..18` | (hidden; extra buttons if the model has them) |

> Only 7 buttons are visible by default on the X6; `key_4` and `key_9..18` are
> `visible="false"` (generic control for up to 18 buttons).

#### Profiles (`profile_text`)

- Profile list: `ms_1_list_profile`
- Action buttons:
  - `ms_1_profile_add_btn` — **Add Profile**
  - `ms_1_profile_delete_btn` — **Delete Profile**
  - `ms_1_profile_rename_btn` — **Rename Profile**
  - `ms_1_profile_import_btn` — **Import Profile**
  - `ms_1_profile_export_btn` — **Export Profile**
  - `ms_1_profile_reset_btn` — **Reset Profile**
- Name dialog: `profilename.xml` ("Input Name", max 30 chars)
- Warnings: "Keep at least one configuration file!", "Do you really want to
  delete this profile?"

#### Battery (`power_text`)

- `ms_1_progress_power` — progress bar (0-100)
- `ms_1_label_power` — "%" text
- Connection indicators: `ms_1_wired` / `ms_1_wireless` / `ms_1_BLE` (only one visible)
- States: "It needs to be charged", "Charging..."

### 1.2 Center column — mouse illustration

`Container` with `ms_1_key_control_N` (N=1..18): overlays that highlight button N
when hovering over the list. No functional controls.

### 1.3 Right column — settings (`ms_1_ver_setting`)

Collapsible panels, in order:

| Panel | Header | Content |
|---|---|---|
| `ms_1_check_dpi` | **DPI Settings** | DPI sliders per stage + Apply/Reset |
| `ms_1_check_light` | **Light Settings** | Mode, brightness, speed, color |
| `ms_1_check_prate` | **Polling Rate Settings** | 125/250/500/1000 Hz |
| `ms_1_check_mouse_attribute` | **Mouse Attribute** | Opens the Windows mouse properties dialog |
| `ms_1_check_power_setting` | **Power Management** | Light off + sleep timer |
| `ms_1_check_lift_of_distance` | **Lift Of Distance** | 1 MM / 2 MM |
| `ms_1_check_key_debounce` | **Key Response Time** | Slider 2-25 |
| `ms_1_check_ripple_control` | **Ripple Control** | Off / On |
| `ms_1_check_angle_snap` | **Angle Snap** | Off / On |
| `ms_1_check_motion_sync` | **Motion Sync** | Off / On |

---

## 2. DPI Settings (`dpi_setting_text`)

Layout: 8 vertical columns (`ms_1_ver_dpi_1..8`; columns 7 and 8 hidden by
default), each with:

| Control | Function |
|---|---|
| Fixed label "26000" (top) and "50" (bottom) | visual range |
| `ms_1_slider_dpi_N` | vertical slider, min 1, max 520 (= DPI/50) |
| `ms_1_label_dpi_N` | shown value (hidden; it is the slider tooltip) |
| `ms_1_edit_dpi_N` | editable field (50–26000, max 5 chars) |
| `ms_1_option_dpi_step_N` | "step" radio (group `ms_1_dpi_step`) — marks which stage is being edited |
| `ms_1_btn_dpi_color_N` | stage color button (opens `DpiColorDialog.xml`) |

Bottom row:

- `ms_1_btn_apply_dpi` — **Apply** (sends the 56-byte report)
- `ms_1_btn_reset_dpi` — **Reset DPI** (`visible="false"` on the X6; it was
  visible on the X3)

> DPI report format: 8 stages, each DPI as `DPI/50 - 1` in 16-bit LE (lows in
> `[8+i]`, highs in `[16+i]`), enabled-stages mask in `[5]`, active stage in
> `[24]`, colors in `[25..48]`, checksum `sum([3..49])` in `[50..51]`. See
> `docs/protocol-x6.md`.

### Color dialog (`DpiColorDialog.xml`)

- `ColorPalette` (full HSV picker)
- R/G/B fields (`r_edit`, `g_edit`, `b_edit` — 0-255)
- 16 predefined colors (`mouse_dpi_color_1..16`)
- `color_show_control` — preview
- `confirm_btn` / `cancel_btn`

---

## 3. Light Settings (`light_setting_text`)

Page `ms_1_lightpage.xml` (there is also a reduced combo in `ms_1_page.xml`):

| Control | Function |
|---|---|
| `ms_1_light_mode_combo` | light mode (12 modes; the BLE combo has only 6) |
| `ms_1_brightness_slider` | brightness 1-8 |
| `ms_1_breathing_slider` | breathing speed 1-8 |
| `ms_1_preview_control` | color preview |
| `ms_1_color_btn1..16` | 16 predefined colors |
| `ms_1_r_edit` / `ms_1_g_edit` / `ms_1_b_edit` | manual RGB 0-255 |
| `ms_1_light_color_pallet` | HSV color picker |
| `ms_1_synchronize_btn` | "Synchronize" (hidden) |
| `ms_1_sleep_timer_slider` | light sleep timer (hidden on the X6, `visible="false"`) |

### Light modes (`ms_lightmode1..12`)

| # | EN name |
|---|---|
| 1 | LED Off |
| 2 | Static |
| 3 | Breathing |
| 4 | Neon |
| 5 | Color Breathing |
| 6 | Static DPI |
| 7 | Breathing DPI |
| 8 | Rainbow Wave |
| 9 | Lightning |
| 10 | Static Mixed Color |
| 11 | Marquee |
| 12 | Marquee 2 |

> In `ms_1_page.xml` the main combo only shows modes 1-7 (`8..12` hidden); the
> dedicated page `ms_1_lightpage.xml` shows them all.

---

## 4. Polling Rate (`prate_setting_text`)

4 options (group `ms_1_prate_group`), each with its own skin:

| Option | Name | Hz |
|---|---|---|
| `ms_1_btn_prate_1` | Power Saving | 125 |
| `ms_1_btn_prate_2` | Office | 250 |
| `ms_1_btn_prate_3` | Gaming | 500 |
| `ms_1_btn_prate_4` | E-sports | 1000 |

`ms_1_label_prate` shows the selected name. The index is stored in
`config[0x938]` (0-3).

---

## 5. Mouse Attribute (`mouse_attribute_setting_text`)

- `ms_1_btn_mouse_attribute` — "Open Mouse Attribute" button with a Windows icon.
  Opens the Windows mouse properties dialog (pointers, buttons, etc.). It is
  just OS configuration access, **not X6 configuration**.

---

## 6. Power Management (`power_setting_text`)

| Control | Function |
|---|---|
| `ms_1_slider_light_off` | inactivity minutes before the light turns off (1-60) |
| `ms_1_label_light_off` | value label |
| `ms_1_slider_sleep_timer` | minutes before the mouse sleeps (1-60) |
| `ms_1_label_sleep_timer` | value label |
| `ms_1_check_movemode` | **Move Wake** (wake on movement) — `visible="false"` on the X6 |

Default values (init): `0x924=8` (light off), `0x92c=10` (sleep), `0x930=1`
(wake), `0x928=6`, `0x958=4`.

---

## 7. Lift Of Distance (`lift_of_distance_text`)

2 options (group `ms_1_lift_of_distance`):

| Option | Value |
|---|---|
| `ms_1_option_lift_of_distance_1` | 1 MM |
| `ms_1_option_lift_of_distance_2` | 2 MM |

Stored in `config[0x948]` (report `[7]`).

---

## 8. Key Response Time (`key_debounce_text`)

- `ms_1_slider_key_debounce` — slider 2-25 (ms)
- `ms_1_label_key_debounce` — value

---

## 9. Ripple Control (`ripple_control_text`)

2 options (group `ms_1_ripple_control`): **Off** / **On**.

---

## 10. Angle Snap (`angle_snap_text`)

2 options (group `ms_1_angle_snap`): **Off** / **On**.

---

## 11. Motion Sync (`motion_sync_text`)

2 options (group `ms_1_motion_sync`): **Off** / **On**.

---

## 12. Macro Manager (`macro_manager_text`)

Separate window (`ms_1_macropage.xml`, 723x500).

### 12.1 Left panel

- `ms_1_macro_list` — macro list ("Macro Name" column)
- `ms_1_macro_key1_btn..key5_btn` — insert mouse action (Left/Right/Middle/
  Forward/Backward), hidden tab `ms_1_macro_mouse_event_tab`

### 12.2 Top bar

| Control | Function |
|---|---|
| `ms_1_macro_name_edit` | macro name (max 20 chars) |
| `ms_1_macro_add_btn` | **Add Macro** |
| `ms_1_macro_delete_btn` | **Delete Macro** |
| `ms_1_macro_insert_mouse_btn` | **Mouse Action** toggle |
| `ms_1_macro_record_btn` | **Recording** toggle (records user actions) |

### 12.3 Action list

`ms_1_macro_action_list` with columns **Key** / **Action** / **Delay(ms)**.

Action types (tab `ms_1_macro_action_edit_tab`, 3 tabs):

1. **Record Delay** (`ms_1_record_delay_btn`):
   - `ms_1_default_delay_btn` — "ms Default Delay" (field `ms_1_default_delay_edit`, default 10)
   - `ms_1_no_delay_btn` — "Min Delay"
2. **Key** — `ms_1_macro_action_edit` (key) + `ms_1_macro_delay_edit` (delay)
3. **Mouse Action** — `ms_1_macro_mouse_action_combo` (L/R/M/Forward/Backward
   Button) + `ms_1_macro_mouseaction_delay_edit`

### 12.4 Edit buttons (right)

| Control | Function |
|---|---|
| `ms_1_macro_action_edit_check` | **Edit** toggle |
| `ms_1_macro_up_btn` | **Move Up** |
| `ms_1_macro_down_btn` | **Move Down** |
| `ms_1_macro_delete_action_btn` | **Delete** |
| `ms_1_macro_clear_action_btn` | **Clear All** |

### 12.5 Save / export / import

- `ms_1_macro_save_btn` — **Save**
- `ms_1_macro_cancel_btn` — **Cancel**
- `ms_1_macro_export_btn` — **Export**
- `ms_1_macro_import_btn` — **Import**
- Macro context menu (`macromenu.xml`): **Load** / **Save**

### 12.6 "Select A Macro" dialog (`selectmacro.xml`)

When assigning a macro to a button:

- `list_macro` — macro list
- `edit_loop_time` — **Loop Times** (1-999)
- Repeat modes (group `repeat_mode_group`):
  - `repeat_1` — "The Number Of Time To Play"
  - `repeat_2` — "Any Key Press To Stop Playing"
  - `repeat_3` — "Press And Hold, Release Stop"

> The "Select A Macro" action in the button menu opens this dialog.

---

## 13. Button assignment menu (`ms_1_menu.xml`)

Clicking a remappable button opens this context menu:

| Action | EN id | Notes |
|---|---|---|
| `ms_1_click` | Left Click | |
| `ms_1_menu` | Right Click | |
| `ms_1_scroll` | Middle Button | |
| `ms_1_forward` | Forward | |
| `ms_1_backward` | Backward | |
| `ms_1_double_click` | Double Click | |
| `ms_1_fire_button` | Fire Button | |
| `ms_1_easy_aim` | Easy Aim | |
| `ms_1_led_loop` | LED Loop | `visible="false"` |
| `ms_1_shortcut` | Assign A Shortcut | opens `shortcut.xml` |
| `ms_1_select_macro` | Select A Macro | opens `selectmacro.xml` |
| `ms_1_scroll_up` | Scroll Up | |
| `ms_1_scroll_down` | Scroll Down | |
| `ms_1_titl_right` | Scroll to Right | `visible="false"` |
| `ms_1_titl_left` | Scroll to Left | `visible="false"` |
| `ms_1_button_off` | Button Off | |

**DPI** submenu:
- `ms_1_dpi_cycle` — DPI Cycle
- `ms_1_dpi_up` — DPI +
- `ms_1_dpi_down` — DPI -

**Multimedia** submenu (`media_function_text`):
- Media Player / Play/Pause / Stop / Previous Track / Next Track / Volume + / Volume - / Mute

**Browser** submenu (`browser_function_text`):
- Browser Home / Favorites / Forward / Backward / Stop / Refresh / Search /
  Email / Calculator / My Computer

**Shortcut** submenu (`shortcut_function_text`):
- Cut / Copy / Paste / Open / Save / Find / Redo / Select All / Print /
  Close Window / Swap Windows / Show Desktop / Run Command / Lock PC / Screen Capture

### "Assign A Shortcut" dialog (`shortcut.xml`)

- `edit_key` — key
- `check_flag_1..4` — **Ctrl / Shift / Alt / Win** modifiers

> The menu also offers "Custom Key Combination" (same shortcut.xml dialog).

---

## 14. Other dialogs

### `dpimsg.xml` — DPI OSD

Overlay that shows the current DPI when changing it with the physical button
(`dpi_label`, size 200x50, semi-transparent background). The mouse does DPI
Cycle onboard; the app displays the value.

### `msg.xml` — generic confirmation dialog

Title `warning_title` + body `warning` + **OK** / **Cancel**. Used for:
"Delete this profile?", "Delete this macro?", "At least one button must be Left
Click!", "No Device Connected!", "restore to default?", name errors.

### `msgtips.xml` — tooltip

`label_tips` (blue background 0x00B5E2).

### `about.xml`

`about_title`, `version` ("Version: V1.0"), `copyright`, `exit` button.

---

## 15. Tray menu (`traymenu.xml`)

- **Configuration** (`configuration_text`) — opens the main window
- **Exit** (`exit_text`)

---

## 16. Internal config (offsets used by the app, in `X6.exe`)

The configuration struct the app syncs with the dongle (56-byte report):

| Offset | Field | Init |
|---|---|---|
| `0x8b4` | ? (report `[3]`... no, `[3]` comes from `0x944`) | 0 |
| `0x904..0x920` | enabled-stages mask 1-8 | 1,1,1,1,1,1,0,0 |
| `0x8c4..0x8e0` | DPI/50 per stage (8) | 0x10,0x18,0x20,0x40,0x70,0x208,0,0 |
| `0x8e4..0x900` | DPI/50 (copy B) | same |
| `0x934` | active stage (0-indexed) | 1 |
| `0x938` | polling rate (0-3) | 3 |
| `0x940` | report `[4]` | 0 |
| `0x944` | report `[3]` and `[6]` | 0 |
| `0x948` | lift of distance (1 MM=0?/2 MM=1) | 1 |
| `0x958` | debounce/light off? (ms/2) | 4 |
| `0x924` | light off timer | 8 |
| `0x928` | ? | 6 |
| `0x92c` | sleep timer | 10 |
| `0x930` | wake mode | 1 |
| `0x960..0x97c` | per-stage colors (dword) | ff,ff00,ff0000,ffff,ffff00,ff00ff,40ff,ffffff |
| `0x95c` | active color picker | ff00 |
| `0x980..0x99c` | color palette | ... |

> The offsets marked with `?` need further confirmation against the full
> decompilation of `FUN_00413940` (HID wrapper) and the sync functions.

---

## 17. Cross-reference with the protocol

The 56-byte report (builder `FUN_004143a0`) the app sends on Apply:

| Byte | Field | Config source |
|---|---|---|
| 0-1 | `04 38` header | fixed |
| 2 | `01` subcommand | fixed |
| 3 | polling/lift? | `0x944` |
| 4 | ? | `0x940` |
| 5 | enabled-stages mask 1-8 | `0x904..0x920` |
| 6 | polling/lift? | `0x944` |
| 7 | lift of distance | `0x948` |
| 8+i | DPI low `(DPI/50 - 1)` | `0x8c4+i*4` |
| 16+i | DPI high | `0x8c4+i*4` |
| 24 | active stage + 1 | `0x934` |
| 25+i*3 | color i (R) | `0x960+i*4` |
| 26+i*3 | color i (G) | |
| 27+i*3 | color i (B) | |
| 49 | `1` indicator | fixed |
| 50-51 | checksum `sum([3..49])` BE | computed |

Sending: SET_REPORT feature `0x0304`, 56 bytes, interface 2. Status/ACK via
interrupt IN `0x83` (config ACK: `03 10 50 00 04`).