# App oficial Attack Shark X6 — Mapa completo de la UI

> **Fuente**: `official-app/` (extracción de `ATTACK SHARK X6 SOFT.exe`, Inno Setup
> v6.1.0, "Attack SharkX6Mouse", X6.exe build 2025-11-10).
> **Objetivo**: inventariar cada sección, pestaña, botón y opción de la app oficial
> para saber qué debe replicar el driver Linux (`attack_shark_linux`).
> **Idiomas de la app**: `official-app/res/lan/` (CN, DE, EN, ES, IT, JP, KR, PT, RU, YUENAN).

Los `resourcetext="true"` de los XML referencian IDs en `lan/*.xml`; este doc usa
los textos en inglés (`lan/EN.xml`) como nombre canónico.

---

## 1. Ventana principal (`main.xml`, 1127x760)

La ventana principal no tiene menú superior propio; el menú del sistema viene de
`traymenu.xml` y el layout central es la página `res/MS/MS_1/ms_1_page.xml`.

### 1.1 Columna izquierda — paneles

Cada panel es un `CheckBox` plegable (header) + un `VerticalLayout` con su
contenido.

| Panel | Header (id EN) | Contenido |
|---|---|---|
| `ms_1_check_button` | **Button Settings** | Lista de botones remapeables |
| `ms_1_btn_macro` | **Macro Manager** | Botón que abre el gestor de macros (ventana aparte) |
| `ms_1_check_profile` | **Profile** | Gestión de perfiles |
| `ms_1_check_power` | **Battery** | Barra de batería + porcentaje |

#### Botones remapeables (`button_setting_text`)

Lista numerada (imagen de cada botón del mouse + botón de asignación):

| Control | Posición en el mouse (skins/mouse/1..7.png) |
|---|---|
| `ms_1_btn_key_1` | botón 1 (izquierdo) |
| `ms_1_btn_key_2` | botón 2 (derecho) |
| `ms_1_btn_key_3` | botón 3 (rueda, click) |
| `ms_1_btn_key_7` | botón 4 (avanzar) |
| `ms_1_btn_key_8` | botón 5 (retroceder) |
| `ms_1_btn_key_5` | botón 6 |
| `ms_1_btn_key_6` | botón 7 |
| `ms_1_btn_key_4` | (oculto) |
| `ms_1_btn_key_9..18` | (ocultos; botones extra si el modelo los tiene) |

> Solo 7 botones visibles por defecto en el X6; los `key_4` y `key_9..18` están
> `visible="false"` (control genérico de hasta 18 botones).

#### Perfiles (`profile_text`)

- Lista de perfiles: `ms_1_list_profile`
- Botones de acción:
  - `ms_1_profile_add_btn` — **Add Profile**
  - `ms_1_profile_delete_btn` — **Delete Profile**
  - `ms_1_profile_rename_btn` — **Rename Profile**
  - `ms_1_profile_import_btn` — **Import Profile**
  - `ms_1_profile_export_btn` — **Export Profile**
  - `ms_1_profile_reset_btn` — **Reset Profile**
- Diálogo de nombre: `profilename.xml` ("Input Name", máx. 30 chars)
- Advertencias: "Keep at least one configuration file!", "Do you really want to
  delete this profile?"

#### Batería (`power_text`)

- `ms_1_progress_power` — barra de progreso (0-100)
- `ms_1_label_power` — texto "%"
- Indicadores de conexión: `ms_1_wired` / `ms_1_wireless` / `ms_1_BLE` (solo uno visible)
- Estados: "It needs to be charged", "Charging..."

### 1.2 Columna central — ilustración del mouse

`Container` con `ms_1_key_control_N` (N=1..18): overlays que resaltan el botón N
al pasar el mouse sobre la lista. Sin controles funcionales.

### 1.3 Columna derecha — ajustes (`ms_1_ver_setting`)

Paneles plegables, en orden:

| Panel | Header | Contenido |
|---|---|---|
| `ms_1_check_dpi` | **DPI Settings** | Sliders de DPI por stage + Apply/Reset |
| `ms_1_check_light` | **Light Settings** | Modo, brillo, velocidad, color |
| `ms_1_check_prate` | **Polling Rate Settings** | 125/250/500/1000 Hz |
| `ms_1_check_mouse_attribute` | **Mouse Attribute** | Abre diálogo de atributos de Windows |
| `ms_1_check_power_setting` | **Power Management** | Apagado de luz + sleep timer |
| `ms_1_check_lift_of_distance` | **Lift Of Distance** | 1 MM / 2 MM |
| `ms_1_check_key_debounce` | **Key Response Time** | Slider 2-25 |
| `ms_1_check_ripple_control` | **Ripple Control** | Off / On |
| `ms_1_check_angle_snap` | **Angle Snap** | Off / On |
| `ms_1_check_motion_sync` | **Motion Sync** | Off / On |

---

## 2. DPI Settings (`dpi_setting_text`)

Layout: 8 columnas verticales (`ms_1_ver_dpi_1..8`; las 7 y 8 ocultas por defecto),
cada una con:

| Control | Función |
|---|---|
| Label fijo "26000" (tope) y "50" (piso) | rango visual |
| `ms_1_slider_dpi_N` | slider vertical, min 1, max 520 (= DPI/50) |
| `ms_1_label_dpi_N` | valor mostrado (oculto, es el tooltip del slider) |
| `ms_1_edit_dpi_N` | campo editable (50–26000, máx. 5 chars) |
| `ms_1_option_dpi_step_N` | radio "paso" (grupo `ms_1_dpi_step`) — indica cuál stage edita |
| `ms_1_btn_dpi_color_N` | botón de color del stage (abre `DpiColorDialog.xml`) |

Fila inferior:

- `ms_1_btn_apply_dpi` — **Apply** (envía el report de 56 bytes)
- `ms_1_btn_reset_dpi` — **Reset DPI** (oculto `visible="false"` en el X6; en el
  X3 era visible)

> Formato del report DPI: 8 stages, cada DPI como `DPI/50 - 1` en 16-bit LE
> (lows en `[8+i]`, highs en `[16+i]`), mask de stages habilitados en `[5]`,
> stage activo en `[24]`, colores en `[25..48]`, checksum `sum([3..49])` en
> `[50..51]`. Ver `docs/protocolo-x6.md`.

### Diálogo de color (`DpiColorDialog.xml`)

- `ColorPalette` (selector HSV completo)
- Campos R/G/B (`r_edit`, `g_edit`, `b_edit` — 0-255)
- 16 colores predefinidos (`mouse_dpi_color_1..16`)
- `color_show_control` — preview
- `confirm_btn` / `cancel_btn`

---

## 3. Light Settings (`light_setting_text`)

Página `ms_1_lightpage.xml` (también hay un combo reducido en `ms_1_page.xml`):

| Control | Función |
|---|---|
| `ms_1_light_mode_combo` | modo de luz (12 modos; el combo BLE tiene solo 6) |
| `ms_1_brightness_slider` | brillo 1-8 |
| `ms_1_breathing_slider` | velocidad de respiración 1-8 |
| `ms_1_preview_control` | preview del color |
| `ms_1_color_btn1..16` | 16 colores predefinidos |
| `ms_1_r_edit` / `ms_1_g_edit` / `ms_1_b_edit` | RGB manual 0-255 |
| `ms_1_light_color_pallet` | selector de color HSV |
| `ms_1_synchronize_btn` | "Synchronize" (oculto) |
| `ms_1_sleep_timer_slider` | sleep timer de luz (oculto en el X6, `visible="false"`) |

### Modos de luz (`ms_lightmode1..12`)

| # | Nombre EN |
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

> En `ms_1_page.xml` el combo principal solo muestra modos 1-7 (`8..12` ocultos);
> la página dedicada `ms_1_lightpage.xml` los muestra todos.

---

## 4. Polling Rate (`prate_setting_text`)

4 opciones (grupo `ms_1_prate_group`), cada una con su skin:

| Opción | Nombre | Hz |
|---|---|---|
| `ms_1_btn_prate_1` | Power Saving | 125 |
| `ms_1_btn_prate_2` | Office | 250 |
| `ms_1_btn_prate_3` | Gaming | 500 |
| `ms_1_btn_prate_4` | E-sports | 1000 |

`ms_1_label_prate` muestra el nombre del seleccionado. El índice se guarda en
`config[0x938]` (0-3).

---

## 5. Mouse Attribute (`mouse_attribute_setting_text`)

- `ms_1_btn_mouse_attribute` — botón "Open Mouse Attribute" con icono de Windows.
  Abre el diálogo de propiedades del mouse de Windows (punteros, botones, etc.).
  Es solo un acceso a configuración del SO, **no es configuración del X6**.

---

## 6. Power Management (`power_setting_text`)

| Control | Función |
|---|---|
| `ms_1_slider_light_off` | min. de inactividad para apagar la luz (1-60) |
| `ms_1_label_light_off` | etiqueta del valor |
| `ms_1_slider_sleep_timer` | min. para dormir el mouse (1-60) |
| `ms_1_label_sleep_timer` | etiqueta del valor |
| `ms_1_check_movemode` | **Move Wake** (despertar al mover) — `visible="false"` en el X6 |

Valores por defecto (init): `0x924=8` (luz off), `0x92c=10` (sleep), `0x930=1`
(wake), `0x928=6`, `0x958=4`.

---

## 7. Lift Of Distance (`lift_of_distance_text`)

2 opciones (grupo `ms_1_lift_of_distance`):

| Opción | Valor |
|---|---|
| `ms_1_option_lift_of_distance_1` | 1 MM |
| `ms_1_option_lift_of_distance_2` | 2 MM |

Se guarda en `config[0x948]` (report `[7]`).

---

## 8. Key Response Time (`key_debounce_text`)

- `ms_1_slider_key_debounce` — slider 2-25 (ms)
- `ms_1_label_key_debounce` — valor

---

## 9. Ripple Control (`ripple_control_text`)

2 opciones (grupo `ms_1_ripple_control`): **Off** / **On**.

---

## 10. Angle Snap (`angle_snap_text`)

2 opciones (grupo `ms_1_angle_snap`): **Off** / **On**.

---

## 11. Motion Sync (`motion_sync_text`)

2 opciones (grupo `ms_1_motion_sync`): **Off** / **On**.

---

## 12. Macro Manager (`macro_manager_text`)

Ventana aparte (`ms_1_macropage.xml`, 723x500).

### 12.1 Panel izquierdo

- `ms_1_macro_list` — lista de macros (columna "Macro Name")
- `ms_1_macro_key1_btn..key5_btn` — insertar acción de mouse (Left/Right/Middle/
  Forward/Backward), tab oculto `ms_1_macro_mouse_event_tab`

### 12.2 Barra superior

| Control | Función |
|---|---|
| `ms_1_macro_name_edit` | nombre de la macro (máx. 20 chars) |
| `ms_1_macro_add_btn` | **Add Macro** |
| `ms_1_macro_delete_btn` | **Delete Macro** |
| `ms_1_macro_insert_mouse_btn` | toggle **Mouse Action** |
| `ms_1_macro_record_btn` | toggle **Recording** (graba acciones del usuario) |

### 12.3 Lista de acciones

`ms_1_macro_action_list` con columnas **Key** / **Action** / **Delay(ms)**.

Tipos de acción (tab `ms_1_macro_action_edit_tab`, 3 tabs):

1. **Record Delay** (`ms_1_record_delay_btn`):
   - `ms_1_default_delay_btn` — "ms Default Delay" (campo `ms_1_default_delay_edit`, default 10)
   - `ms_1_no_delay_btn` — "Min Delay"
2. **Key** — `ms_1_macro_action_edit` (tecla) + `ms_1_macro_delay_edit` (delay)
3. **Mouse Action** — `ms_1_macro_mouse_action_combo` (L/R/M/Forward/Backward
   Button) + `ms_1_macro_mouseaction_delay_edit`

### 12.4 Botones de edición (derecha)

| Control | Función |
|---|---|
| `ms_1_macro_action_edit_check` | toggle **Edit** |
| `ms_1_macro_up_btn` | **Move Up** |
| `ms_1_macro_down_btn` | **Move Down** |
| `ms_1_macro_delete_action_btn` | **Delete** |
| `ms_1_macro_clear_action_btn` | **Clear All** |

### 12.5 Guardar / exportar / importar

- `ms_1_macro_save_btn` — **Save**
- `ms_1_macro_cancel_btn` — **Cancel**
- `ms_1_macro_export_btn` — **Export**
- `ms_1_macro_import_btn` — **Import**
- Menú de contexto de macro (`macromenu.xml`): **Load** / **Save**

### 12.6 Diálogo "Select A Macro" (`selectmacro.xml`)

Al asignar una macro a un botón:

- `list_macro` — lista de macros
- `edit_loop_time` — **Loop Times** (1-999)
- Modos de repetición (grupo `repeat_mode_group`):
  - `repeat_1` — "The Number Of Time To Play"
  - `repeat_2` — "Any Key Press To Stop Playing"
  - `repeat_3` — "Press And Hold, Release Stop"

> La acción "Select A Macro" del menú de botones abre este diálogo.

---

## 13. Menú de asignación de botones (`ms_1_menu.xml`)

Al hacer clic en un botón remapeable se abre este menú contextual:

| Acción | id EN | Observación |
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
| `ms_1_shortcut` | Assign A Shortcut | abre `shortcut.xml` |
| `ms_1_select_macro` | Select A Macro | abre `selectmacro.xml` |
| `ms_1_scroll_up` | Scroll Up | |
| `ms_1_scroll_down` | Scroll Down | |
| `ms_1_titl_right` | Scroll to Right | `visible="false"` |
| `ms_1_titl_left` | Scroll to Left | `visible="false"` |
| `ms_1_button_off` | Button Off | |

Submenú **DPI**:
- `ms_1_dpi_cycle` — DPI Cycle
- `ms_1_dpi_up` — DPI +
- `ms_1_dpi_down` — DPI -

Submenú **Multimedia** (`media_function_text`):
- Media Player / Play/Pause / Stop / Previous Track / Next Track / Volume + / Volume - / Mute

Submenú **Browser** (`browser_function_text`):
- Browser Home / Favorites / Forward / Backward / Stop / Refresh / Search /
  Email / Calculator / My Computer

Submenú **Shortcut** (`shortcut_function_text`):
- Cut / Copy / Paste / Open / Save / Find / Redo / Select All / Print /
  Close Window / Swap Windows / Show Desktop / Run Command / Lock PC / Screen Capture

### Diálogo "Assign A Shortcut" (`shortcut.xml`)

- `edit_key` — tecla
- `check_flag_1..4` — modificadores **Ctrl / Shift / Alt / Win**

> El menú también permite "Custom Key Combination" (mismo diálogo shortcut.xml).

---

## 14. Otros diálogos

### `dpimsg.xml` — OSD de DPI

Overlay que muestra el DPI actual al cambiar con el botón físico
(`dpi_label`, tamaño 200x50, fondo semitransparente). El mouse hace DPI Cycle
onboard; la app muestra el valor.

### `msg.xml` — diálogo de confirmación genérico

Título `warning_title` + cuerpo `warning` + **OK** / **Cancel**. Usado para:
"Delete this profile?", "Delete this macro?", "At least one button must be Left
Click!", "No Device Connected!", "restore to default?", errores de nombre.

### `msgtips.xml` — tooltip

`label_tips` (fondo azul 0x00B5E2).

### `about.xml`

`about_title`, `version` ("Version: V1.0"), `copyright`, botón `exit`.

---

## 15. Menú de bandeja (`traymenu.xml`)

- **Configuration** (`configuration_text`) — abre la ventana principal
- **Exit** (`exit_text`)

---

## 16. Config interna (offsets usados por la app, en `X6.exe`)

El struct de configuración que la app sincroniza con el dongle (report de 56 B):

| Offset | Campo | Init |
|---|---|---|
| `0x8b4` | ? (report `[3]`... no, `[3]` viene de `0x944`) | 0 |
| `0x904..0x920` | mask stages 1-8 habilitados | 1,1,1,1,1,1,0,0 |
| `0x8c4..0x8e0` | DPI/50 por stage (8) | 0x10,0x18,0x20,0x40,0x70,0x208,0,0 |
| `0x8e4..0x900` | DPI/50 (copia B) | idem |
| `0x934` | stage activo (0-indexed) | 1 |
| `0x938` | polling rate (0-3) | 3 |
| `0x940` | report `[4]` | 0 |
| `0x944` | report `[3]` y `[6]` | 0 |
| `0x948` | lift of distance (1 MM=0?/2 MM=1) | 1 |
| `0x958` | debounce/luz off? (ms/2) | 4 |
| `0x924` | luz off timer | 8 |
| `0x928` | ? | 6 |
| `0x92c` | sleep timer | 10 |
| `0x930` | wake mode | 1 |
| `0x960..0x97c` | colores por stage (dword) | ff,ff00,ff0000,ffff,ffff00,ff00ff,40ff,ffffff |
| `0x95c` | color picker activo | ff00 |
| `0x980..0x99c` | paleta de colores | ... |

> Los offsets marcados con `?` requieren confirmación adicional con el decompilado
> completo de `FUN_00413940` (wrapper HID) y las funciones de sincronización.

---

## 17. Referencia cruzada con el protocolo

El report de 56 B (builder `FUN_004143a0`) que la app envía al aplicar:

| Byte | Campo | Fuente config |
|---|---|---|
| 0-1 | `04 38` header | fijo |
| 2 | `01` subcomando | fijo |
| 3 | polling/lift? | `0x944` |
| 4 | ? | `0x940` |
| 5 | mask stages 1-8 | `0x904..0x920` |
| 6 | polling/lift? | `0x944` |
| 7 | lift of distance | `0x948` |
| 8+i | DPI low `(DPI/50 - 1)` | `0x8c4+i*4` |
| 16+i | DPI high | `0x8c4+i*4` |
| 24 | stage activo + 1 | `0x934` |
| 25+i*3 | color i (R) | `0x960+i*4` |
| 26+i*3 | color i (G) | |
| 27+i*3 | color i (B) | |
| 49 | `1` indicador | fijo |
| 50-51 | checksum `sum([3..49])` BE | calculado |

Envío: SET_REPORT feature `0x0304`, 56 bytes, interfaz 2. Estado/ACK por interrupt
IN `0x83` (ACK de configuración: `03 10 50 00 04`).
