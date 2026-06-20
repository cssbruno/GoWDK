# GOWDK Page Theme

## Theme Goal

The page theme should align with the WDK logo and the wider Go tooling feel:
cyan-blue energy, dark ink outlines, precise developer-workspace spacing, and
light technical backgrounds. The site should feel like a calm Go developer
tool, not a green SaaS dashboard or a generic dark terminal.

## Palette

| Token | Value | Use |
| --- | --- | --- |
| `--ink` | `#10232b` | Main text, icon fill, code background, strong outlines. |
| `--paper` | `#eef8fc` | Page background and header glass color. |
| `--surface` | `#fbfcfb` | Panels, notices, buttons, cards. |
| `--surface-soft` | `#e6f6fb` | Navigation hover and quiet UI fills. |
| `--muted` | `#506872` | Body copy, secondary labels, metadata. |
| `--line` | `#bed6df` | Borders and section dividers. |
| `--accent` | `#007d9c` | Primary actions, active states, eyebrow labels. |
| `--accent-strong` | `#00add8` | Go-blue highlights and technical background lines. |
| `--accent-soft` | `#d9f3fb` | Active tab fills and soft emphasis backgrounds. |
| `--code` | `#10232b` | Code and generated-output panels. |
| `--code-text` | `#d9f8ff` | Code text on dark surfaces. |

## Logo-Derived Anchors

- Cyan fill: approximately `#8ad3dd`.
- Muted teal shadow/fill: approximately `#6c9fa4`.
- Dark outline: approximately `#101615` to `#283232`.
- Warm light detail: approximately `#e5e3de`.

The UI should use the darker teal `--accent` for readable text and buttons,
while reserving the brighter logo cyan for soft fills and small highlights.

## Usage Rules

- Use `--accent` for primary buttons, active tabs, links that need emphasis,
  and eyebrow labels.
- Use `--accent-soft` for selected or active backgrounds.
- Keep large backgrounds on `--paper` with subtle Go-blue grid or angled line
  treatments; avoid saturated cyan page sections.
- Keep code surfaces dark with cyan-tinted text so they echo the logo outline
  and fill without becoming decorative.
- Avoid green accents unless a future product state requires success/error
  semantics.
- Keep cards and panels restrained: `8px` radius, thin `--line` borders, and
  shadows only for overlays such as the cookie notice.

## Accessibility

Primary action text must stay white on `--accent`. Body text should use
`--ink` or `--muted`; do not place `--muted` on `--accent-soft` for critical
labels unless contrast is checked.
