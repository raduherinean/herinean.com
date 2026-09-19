---
title: "Colophon"
summary: "How herinean.com is built and audited: the scorecard, the stack, the dependencies, the trade-offs."
---
This page is the site's test suite, written down. Every claim below is checked on each commit; the build fails loudly rather than quietly. If a row says fail, it is failing right now and the fix is queued, not hidden.

The platform is a Go program of a few hundred lines that turns Markdown into these pages, a Cloudflare Worker of about eighty lines at the edge, and the zone configuration as code. There is no server, no database, no JavaScript in the browser, no cookie, and no third-party request. The whole thing is public: [github.com/raduherinean/herinean.com](https://github.com/raduherinean/herinean.com), including the [design specification](https://github.com/raduherinean/herinean.com/blob/main/docs/specs/2026-09-17-herinean-com-design.md) and the decisions that shaped it.

**Analytics.** Page views are counted at the edge with the path, language, referring host, a `?ref=` tag and country. Not stored: IP address, user agent, full referrer, or anything that could count readers rather than reads. Views include some bots; the numbers compare pieces, they don't measure reach.

**Known trade-offs.** No comments (discussion is on LinkedIn), no search (the index is small and browsers find text), no dark-mode toggle (the OS decides), no unique-visitor counts (see above), no tags or related posts, no embeds of any kind (they need scripts), TLS 1.3 only (SSL Labs withholds A+ from 1.3-only servers — see ADR-0011 — while internet.nl scores 100%), and one vendor at the edge (the runbook covers the day it fails). URLs never change and pieces are never deleted; corrections carry an "updated" date.

**How the writing is made.** Every piece here starts as a draft written by an AI model from my own notes and working history, then goes to an agent with repository access that checks each technical claim against the code, the commits and the logs and marks it confirmed, wrong or unverifiable. I edit what comes back and decide what ships. It's the same shape as any review I'd defend at work — the machine does the volume, a second machine checks the facts, a human signs. Anything wrong that survived all three is mine.
