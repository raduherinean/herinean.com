# Claude Design export — review against the brief (2026-09-18)

Source: Claude Design project "Phase 1 design system" (`7eb631db-a572-4b87-8246-568a740685d2`), file `herinean-phase1.html`, saved under `claude-design-export/` with renders made here with Playwright (the design tool's own screenshots exceed the read limit).

## Verdict
On brief. Everything below is reproducible in hand-written CSS with no JavaScript. Adopted as the visual reference for M1a (templates, CSS) and M1b (fonts).

## System (from the export's legend, verified)
- Tokens (light / dark): paper `#FAF8F4` / `#191714`; paper-2 `#F1EDE6` / `#221F1B`; ink `#1C1A17` / `#EAE4DA`; ink-2 `#5C574F` / `#A69C8E`; rule `#DCD5C9` / `#38332C`; accent (oxblood) `#7D2B22` / `#E2907E`; visited `#5B322C` / `#C29C93`; selection `#EBD8D1` / `#4A2E27`.
- Contrast (computed here): body 16.4:1 light, 14.2:1 dark (AAA); meta 6.8 / 6.6 (AA, above the 4.5 floor for secondary text); accent 8.8 / 7.3; visited 10.2 / 7.2.
- Type: body Source Serif 4 (OFL, true italic, ș ț ă â î covered, no real small caps → labels are letter-spaced uppercase); display Newsreader (OFL) for masthead and titles only; mono IBM Plex Mono (OFL) for code and the dependency list. Scale 1.25 from 19 px: 12.2 / 15.2 / 19 / 23.8 / 29.7 / 37.1 / 46.4 / 58. Line-height 1.6, measure 38rem.
- Spacing on a 0.5rem base: 0.5 / 0.85 / 1.05 / 1.6 / 2.2 / 3rem. One 1px hairline (`--rule`) is the only structural device.
- Links: underline .055em thick, offset .16em, thicker on hover; focus ring 2px accent, offset 3px; selection tinted from the accent.
- Entry format: date · PILLAR · (RO) badge / title / one-line summary; density B (with summaries) recommended and adopted.
- Colophon table stacks at ≤34rem into label/value rows via `td[data-label]::before` — CSS only.
- Portrait: hairline border in light, accent border in dark.
- Print: nav/footer hidden, `a[href^=http]::after{content:" <" attr(href) ">"}`, `@page{margin:18mm 16mm}`.
- Favicon: option B (paper lowercase "h" on oxblood) — reads at 16 px in both schemes; ships as a single SVG path.
- OG: paper ground, pillar label top-left, language top-right, Newsreader title (≤ 3 lines), name bottom-left, `herinean.com` in mono bottom-right; home variant carries the portrait.

## Deviations to apply in the build (adversarial pass)
1. **Data tables at phone width.** The export's `@media (max-width:34rem){div[data-scroll]{overflow-x:visible}}` is meant for the stacking colophon table but also affects content tables, which then clip (see `renders/piece-390-light-2.png`, the cost column). Production: every content table sits in a scroll container with `overflow-x:auto` at all widths; only `table[data-stack]` stacks.
2. **Code blocks** must have `overflow-x:auto` explicitly (`pre`), plus `tab-size:2`; the render suggests clipping.
3. **Fonts vs. the weight budget (spec §3 row 14: fonts ≤ 100 KB total, one preloaded).** Three variable families with italics would be ~200 KB. Production ships static instances, subset to Latin + Latin Extended-A + U+0218–021B: Source Serif 4 Regular + Italic (~30 KB each), Newsreader Medium (~30 KB); **code uses the system mono stack** (`ui-monospace, "Cascadia Mono", "SF Mono", Menlo, Consolas, monospace`) instead of shipping IBM Plex Mono. ≈ 90 KB, body Regular preloaded. Decision recorded here; M1b executes.
4. `body{background:#8B867E}` and the container-query (`cqw`) sizing are artboard devices, not production. Production: `:root` tokens + one `prefers-color-scheme: dark` override, no `data-scheme` attribute; `clamp()` with `vw` for the gutter, title and portrait.
5. Google Fonts `<link>`s exist for the preview only; production is self-hosted with `font-display: swap` and metric-matched fallbacks (Georgia for both serifs) — M1b.
6. The OG renderer (Go) vendors Newsreader Medium and Source Serif 4 Regular TTFs; the domain line uses the mono fallback rendering from Source Serif 4 if no mono is vendored (or vendor IBM Plex Mono Regular for OG only — server-side, no weight cost).
7. The RO badge is plain text; the build adds `lang="ro"` and a visually-hidden full name, per spec §9.
