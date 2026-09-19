---
title: "Why I'm writing my own static-site generator"
date: 2026-09-05
key: pair
pillar: build-log
summary: "Frameworks solve problems I don't have and hide the ones I do. So the generator behind this site is a few hundred lines of Go."
linkedin: https://www.linkedin.com/feed/update/urn:li:activity:7123456789012345678
---
Every time I start a personal site, I reach for a static-site generator, spend an afternoon fighting its plugin system, and end up with a build I don't fully understand. This time I decided to write the generator instead of choosing one.

The requirements turned out to be small: parse Markdown with front matter, validate a handful of fields, render two languages, and produce plain HTML with no client-side JavaScript. None of that needs a plugin ecosystem or a templating language of its own — it needs a parser, a couple of structs, and tests that fail loudly when a piece is missing something.

Writing it myself also means the failure modes are mine to fix. When a build breaks because a translation is missing or an image has no alt text, the error points at the exact file and line, because I wrote the code that produces that message.

The next few posts in this build log will walk through the pieces as they land: content loading, image processing, and the render pipeline that turns all of this into the site you're reading right now.
