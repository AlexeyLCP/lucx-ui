# LucX-UI branding proposals

Three hand-drawn SVG logo variants (issue feedback: Mr. Zaic, 26.11). Vector on
purpose: crisp at favicon size, a few hundred bytes, no font shipping (system
sans-serif stack).

| File                     | Motif                                                             |
| ------------------------ | ----------------------------------------------------------------- |
| `variant-1-lx-hex.svg`   | Hexagonal badge, bold **L** + X as two crossing light beams       |
| `variant-2-wordmark.svg` | `lucx` wordmark with a laser-X and a thin ray underneath (banner) |
| `variant-3-shield.svg`   | Shield with a tunnel-perspective X and a glowing core             |

Open them side by side in any browser to compare. Dark theme matches the panel
(`#0e1621` base, cyan `#22d3ee` accent — the panel's existing palette).

Wiring one in later = replace the favicon in `frontend/public/icons/` with a
48-px raster export of the chosen SVG and adjust the sidebar mark if wanted —
a one-commit change once a variant wins.
