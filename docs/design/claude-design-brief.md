# Claude Design brief — herinean.com

> **How this file is used**
>
> 1. **Primary path:** Claude Design → new design system → "Link code from your computer" → point it at this `docs/design/` folder. Fill the form's blurb with the first paragraph under *Brief*.
> 2. **Fallback:** paste everything below the divider as the first message of the design session.
>
> Suggested follow-ups once the first generation appears:
> - "Show me the Piece page at 390px wide, light and dark, side by side. The measure must never exceed 38rem."
> - "Propose three OFL typeface pairings, each as a full specimen: masthead, H1, meta line in small caps, body paragraph with an italic and a link, a code block, Romanian text with ș ț ă â î. Tell me which have real small caps."
> - "Make the masthead do more of the identifying work; the rest of the page should get quieter, not louder."
> - "Render the colophon table at phone width as stacked label/value rows. No horizontal scroll."
> - "Show the print rendering of the Piece page."
>
> When the result is right: save the HTML and screenshots. That's the input for the build — the export is **reference only**; the production CSS will be hand-written from it.

---

# Brief

I'm designing **herinean.com**, the personal site of Radu Herinean — a practicing CTO who ships AI in real businesses and writes about which projects are worth funding. It's a writing archive in English and Romanian for executives, boards and founders. It is *not* a developer blog, a portfolio, or a company site. The tone is a person, not a brand: no logo, no company colours, no "labs".

The site is also the subject of one of its first articles — how AI was used to build "the perfect platform for a specific purpose" — and will be audited by technical peers: zero client-side JavaScript, ~200 lines of hand-written CSS, self-hosted fonts, Lighthouse 100 on every page, WCAG AAA body contrast. **So the design must be achievable with typography, spacing, colour and hairlines alone.** Everything you propose has to survive being rewritten as plain CSS by hand.

**This is Phase 1: a visual language, not a finished site.** Produce a single self-contained HTML file (inline CSS, no JavaScript, no external requests except Google Fonts *for preview only* — production fonts will be self-hosted) that defines the system and composes the six page shapes below, each labelled with an HTML comment.

---

# The two readers

1. **The executive**, on a phone, arriving from the LinkedIn app's in-app browser. Reads 1,500–3,000 words if the page gets out of the way. Has seen a thousand Substacks and Medium posts and doesn't trust them. Needs: a face, a name, a concrete claim, and a page that reads like a well-set book.
2. **The peer CTO / technical advisor**, on a laptop, who will open view-source and the colophon. Needs to see restraint executed with precision. Nothing decorative; every choice explainable.

The test for every element: would the first reader feel *this person is serious*, and would the second reader find nothing to remove?

---

# Direction

**Editorial, warm, exact.** The neighbourhood is typographic personal sites and small presses — Butterick's *Practical Typography*, Stripe Press, *A Working Library*, Frank Chimero's site — not tech blogs. The personality comes from the type scale, the masthead, the measure, the rhythm, one accent colour, and hairline structure. Nothing else is available.

**Anti-references** (what this must not resemble): Substack and Medium defaults (Lora/Spectral + Inter, big sans headlines, pill tags, "Subscribe" boxes); Paul Graham's page (sparse *and* cheap: default font, no measure, no hierarchy); Notion-published pages; Bear Blog / Hugo theme defaults; any SaaS landing page grammar (hero, cards, gradients, icons, shadows).

---

# Hard constraints — treat as non-negotiable

- **One column.** Measure capped at **38rem** (~65–70 characters). 16px side gutters on phone, no horizontal page scroll. No sidebar, no hero, no grid of cards.
- **No decoration:** no gradients, shadows, cards, rounded pill tags, icon library, illustration, animation, or background images. Hairlines (1px rules) are the only structural device.
- **Typography:** one self-hosted **OFL** serif for body (candidates: Source Serif 4, Literata, Newsreader, Crimson Pro, Fraunces at low optical size); optionally one display serif for masthead + titles only (e.g. Instrument Serif, Fraunces high optical size). No Google-Fonts-only faces, no proprietary faces. **Must contain Romanian comma-below diacritics ș ț Ș Ț (U+0218–021B) and ă â î.** Small caps only if the face has real small caps; otherwise letter-spaced uppercase at a smaller size. Never synthesized bold/italic — if italics are used, the italic file ships.
- **Sizes:** body 18–19px on phone, line-height ≈ 1.6. A modular scale (≈1.25) — state it. Title large but not shouting (≈2.2–2.6rem on phone). Meta line small with tabular figures. `text-wrap: balance` on titles, `text-wrap: pretty` on paragraphs, hyphenation on.
- **Colour:** warm near-black on paper-white (not #000/#fff); **one** accent colour used only for links and focus rings, chosen for character (ink blue, oxblood, deep green — not default blue); visited links not purple; selection colour derived from the accent. **Dark scheme** via `prefers-color-scheme` only (no toggle): warm dark grey ground, not black; the accent re-tuned for dark. **Contrast: body ≥ 7:1 in both schemes; secondary/meta text ≥ 4.5:1.** Define everything as CSS custom properties on `:root` with a dark override block.
- **Links:** underlined, with tuned `text-decoration-thickness` and `text-underline-offset`; a designed focus ring in the accent, never the browser default.
- **Language:** UI strings exist in EN and RO; Romanian typography uses „…” quotes, English uses "…". Where Romanian appears inside an English page (an RO-only piece in the list), it is visibly a Romanian title with a small "RO" text badge — plain text, no pill.
- **Print:** navigation and footer hidden, link URLs printed after link text, black on white, sensible `@page` margins. Executives print.
- **No JavaScript.** If a component seems to need it, it's the wrong component.

---

# Page shapes to compose (in this order, each labelled)

1. **Masthead + footer** (shared). Masthead: the name "Radu Herinean" (link home), then *Writing* and a language link (*Română* / *English*). On the home page only, the masthead carries a one-line positioning statement beneath the name — draft: *"Practicing CTO. I ship AI in real businesses and write about which projects are worth funding."* Footer: *LinkedIn · RSS · Colophon* as text links, and one quiet line: *"No JavaScript, no cookies, no trackers."*
2. **Home.** Masthead with tagline → a small **portrait** (placeholder labelled "PHOTO: portrait", ~140px on phone, ~160px desktop, top-left of the About text or above it; in dark mode give it a hairline border in the accent) → three or four About paragraphs (placeholder copy below) → a section rule → **Latest**: five entries → *All writing →*.
   - **Entry format** (used everywhere): date (tabular) · pillar label (small caps) · language badge when not the page's language · title (link) · one-line summary. Show me two densities and recommend one.
3. **Writing index.** Entries grouped under year headings (`2026`, `2025`), same entry format.
4. **Piece — English.** Top: language switch link *"Citește în română →"* (only present when a translation exists) → title → meta line: `27 Sep 2026 · updated 3 Oct 2026 · 9 min read · Analysis` → body demonstrating: paragraphs, an H2 and H3, a blockquote/pull quote, a bulleted list, a table that scrolls inside its box, a code block, a figure with caption, a footnote reference and the footnotes section → a quiet line *"Also on LinkedIn · Medium"* → one-line author footer (*Radu Herinean — CTO. LinkedIn*).
5. **Piece — Romanian.** Same anatomy, Romanian UI strings (*Citește în engleză →*, `27 sept. 2026 · 9 min de citit · Guvernanță`), Romanian quotes and diacritics in the body.
6. **Colophon.** Short intro paragraph → a **scorecard table** (columns: Check · Result · Value · Verified · "Verify it yourself" link) with ~8 sample rows, one of them deliberately failing so the failure style exists → a dependency list (monospace, small) → a "Known trade-offs" list. On phone the table becomes stacked label/value rows.
7. **404.** One sentence in English and one in Romanian, links to home and index.

Also produce:
- **Type specimen block** at the top: every size in the scale with its use, in both faces, in both schemes, including a Romanian line.
- **OG image template**, 1200×630, as an HTML block at that size: paper background, title in the display face (wrapping to 3 lines max), small name + `herinean.com`, pillar label, language tag. A second variant for the home page with the portrait.
- **Favicon**: two or three SVG monogram proposals (initials or a single letter), legible at 16px, working in light and dark.

Render the Piece page at **390px** and **1280px** widths, light and dark. Render Home at both widths in light.

---

# Placeholder copy (replace at build; keep the shapes)

**Tagline (draft):** Practicing CTO. I ship AI in real businesses and write about which projects are worth funding.

**About (placeholder — real copy will state role, company, what shipped, years):**
> [Paragraph 1: current role and company, one sentence on what the company does, one on what he's responsible for.]
> [Paragraph 2: what he's been shipping — AI in production, in retail/e-commerce and agriculture — and the recurring question executives ask him.]
> [Paragraph 3: what this site is: build logs, opportunity spotting, AI and governance, analysis; each in the language of its reader.]
> [Paragraph 4: one line linking Evolution Labs as one of the things he does; contact via LinkedIn.]

**Five pieces (placeholders):**

| Date | Lang | Pillar | Title | Summary |
|---|---|---|---|---|
| 2026-10-11 | EN | Analysis | Which AI projects are worth funding in 2027 | A filter for boards: three questions that separate the projects that reach production from the ones that stay demos. |
| 2026-10-04 | RO | Guvernanță | Ce ar trebui să întrebe un board despre AI înainte de a aproba bugetul | Cinci întrebări pe care le pun consiliilor de administrație înainte de orice discuție despre buget. |
| 2026-09-27 | EN | Build log | The perfect platform for a specific purpose, built with AI | How this site was built: a 600-line generator, no JavaScript, and a scorecard it has to pass on every commit. |
| 2026-09-20 | EN | Opportunity | The unglamorous AI opportunity in retail operations | Where the money is in retail AI, and why it isn't the chatbot. |
| 2026-09-13 | RO | Analiză | De ce majoritatea pilotelor AI din CEE nu ajung în producție | Un tipar din trei ani de proiecte: problema nu e modelul, e cine deține decizia. |

**UI strings**

| EN | RO |
|---|---|
| Writing | Articole |
| Read in Romanian → / Citește în română → | Citește în engleză → |
| min read | min de citit |
| Updated | Actualizat |
| Also on LinkedIn · Medium | Și pe LinkedIn · Medium |
| Colophon | Colofon |
| All writing → | Toate articolele → |
| Latest | Recente |
| Page not found. | Pagina nu există. |

Pillars: **Build log · Opportunity · AI + governance · Analysis** (RO: *Jurnal de construcție · Oportunități · AI și guvernanță · Analiză*).

---

# What to deliver

1. One self-contained HTML file with the sections above, inline CSS, `:root` tokens with a `prefers-color-scheme: dark` block, no JavaScript.
2. A short written rationale: the typeface pairing (and whether it has real small caps and comma-below diacritics), the accent colour with contrast ratios in both schemes, the type scale, and the spacing scale.
3. Screenshots: Piece at 390/1280 × light/dark; Home at 390/1280 light; colophon at 390.

# What not to do

No cookie banner, no newsletter box, no social icons, no "share" buttons, no reading-progress bar, no estimated-reading-time icon, no author card with a round avatar and follower counts, no tags cloud, no related posts, no search, no dark-mode toggle, no comments. If it isn't text, a hairline, a portrait or the one accent, it doesn't exist.
