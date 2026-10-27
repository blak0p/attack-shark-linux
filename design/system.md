# Design system — Attack Shark X6 Configurator

Synced from `frontend/src/styles/tokens.css`, `frontend/src/styles.css`, `frontend/src/styles/controls.css` and `frontend/src/styles/profiles.css`. Dark only (`color-scheme: dark`), GNOME / libadwaita look.

## Colors
| Token | Value | Usage | Source |
|---|---|---|---|
| `--accent` | #1d9e75 | primary buttons, switch on, hover border | tokens.css |
| `--bg` | #1e1e1c | window and dialog background | tokens.css |
| `--rail` | #292a27 | side rail | tokens.css |
| `--surface` | #30312e | body, inputs, buttons | tokens.css |
| `--raised` | #393a36 | hovered controls, panels | tokens.css |
| `--line` | #4a4b46 | borders, row dividers | tokens.css |
| `--text` | #f2f1ed | primary text | tokens.css |
| `--muted` | #b6b7b1 | secondary text, labels | tokens.css |
| `--dim` | #8b8c86 | tertiary text | tokens.css |
| `--ok` | #55c590 | success | tokens.css |
| `--danger` | #e05f65 | errors | tokens.css |
| `--accent-subtle` | #244b3e | badges | tokens.css |
| `--accent-on-primary` | #09261d | text on accent | tokens.css |
| `--border-strong` | #55564f | dialog borders | tokens.css |
| `--focus-ring` | #77dabe | focus outline (2px, offset 2px) | tokens.css |
| switch off | #676862 | switch track off | controls.css |
| danger text | #f4a5a5 | destructive action text | profiles.css |

## Typography
`system-ui, -apple-system, BlinkMacSystemFont, "Cantarell", "Segoe UI", sans-serif`; base 14px/1.4 (styles.css). Dialog title 20px/600, body 13px, helper text 12px (muted, line-height 1.6), small button 11px, badge 10px (profiles.css).

## Spacing and radius
`--space-1..6` = 4, 8, 12, 16, 20, 24px. Radii: `--radius-sm` 6, `--radius-md` 9, `--radius-lg` 13, `--radius-window` 18. Controls use 8px, dialogs 16px, toasts 12px, pills 20px.

## Shadows
`--shadow` 0 12px 36px #0005; dialog 0 24px 80px #0008.

## Motion
`--motion-fast` 120ms, `--motion-standard` 180ms, `--motion-deliberate` 240ms, `--ease-standard` cubic-bezier(0.2,0.8,0.2,1). Respect `prefers-reduced-motion`.

## Component rules
- Button: surface background, 1px `--line` border, 8px radius, padding 8px 12px; hover border `--accent` and `--raised` background; disabled opacity .45 (profiles.css).
- Primary button: `--accent` background, `--accent-on-primary` text, weight 600.
- Small row action: 11px, padding 6px 9px, transparent background.
- Switch: 38×22px track, 16px white knob, on = `--accent` (controls.css).
- Row: flex, space-between, padding 16px 0, 1px `--line` top border.
- Badge: `--accent-subtle` background, 10px, 20px radius, padding 3px 8px.
- Dialog: `--bg` background, 1px `--border-strong`, 16px radius, padding 24px, max 560px.
- Input: same as button, full width.
