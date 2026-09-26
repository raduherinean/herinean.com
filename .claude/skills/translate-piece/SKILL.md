---
name: translate-piece
description: Write the first-pass translation of a piece into the other language (en ↔ ro) as a new file with the same key, for the author to rewrite. Use when the author asks for a translation.
argument-hint: content/<from>/<slug>.md <to>
---

# /translate-piece

Writes one new file: the model's first pass. The author rewrites it — the colophon says translations "start from a model's pass and are rewritten by me".

## Rules

1. This is the only skill that creates a file in `content/`, and only this one file, once. If the target exists, stop: never overwrite the author's rewrite.
2. The untouched pass is saved privately beside the ledger, so `/publish-piece` can refuse a translation nobody rewrote.
3. The examples here and in `glossary.md` are invented or are house terms. Never quote a draft into this repository.

## Steps

1. Read the source piece and `glossary.md` (beside this file).
2. Propose the target slug: lowercase ASCII words joined by `-`; Romanian letters without diacritics (`ș`→`s`, `ț`→`t`, `ă`/`â`→`a`, `î`→`i`). Ask the author to confirm or change it — a published URL never changes. The target is `content/<to>/<slug>.md`. If it exists, stop.
3. Translate:
   - Idiomatic, not literal: a reader of the target language should not feel the source language underneath. Keep the argument, its order, the facts and the author's register; write new sentences.
   - Front matter: the same `key` and `pillar`; `title` and `summary` translated, the summary at most 160 characters; `date:` empty; no `linkedin:`, `medium:` or `updated:`.
   - Follow `glossary.md`.
   - Quotations: translated in the body; the footnote keeps the original inside `<span lang="…">…</span>`.
   - Footnotes keep their sources and URLs; translate their prose.
   - Figures, dates and names cross unchanged (their format follows the glossary).
4. Write the file. Then save the same bytes as the model pass:
   `LEDGER=$(node .claude/skills/publish-piece/gate.mjs path content/<to>/<slug>.md)`;
   `mkdir -p "$(dirname "$LEDGER")"`;
   `cp content/<to>/<slug>.md "$(dirname "$LEDGER")/model-pass-<to>.md"`.
5. `go build -tags nodynamic -o .cache/site ./cmd/site && .cache/site check --draft` — report every problem, and the summary's length.
6. Tell the author: the new file, the model pass's path, the glossary rules applied, and every term you were unsure of, each with a proposed `glossary.md` line. Next: the author rewrites the file; then `/review-piece` on it.

## When the author corrects a pass
Propose the line to add to `glossary.md`: a term and its translation, or a rule. Terms and rules only — never a sentence from a draft.
