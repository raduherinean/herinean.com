---
title: "A second piece that collides on key"
date: 2026-09-07
key: pair
pillar: opportunity
summary: "This piece exists only to trigger the duplicate-key validation: two English pieces cannot share the same pairing key."
---
This piece is a deliberate fixture for the duplicate-key test: it reuses the key `pair`, which `content/en/paired.md` already claims in the same language.

The loader must reject this at validation time rather than silently letting the second piece overwrite the pairing, since two pieces cannot both be "the English half" of the same translation pair.

Nothing else about this file is unusual — it has a valid title, date, pillar, and summary — because the test should fail on the key collision specifically, not on some other missing field.
