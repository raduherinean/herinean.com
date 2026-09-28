# herinean.com

This repository is the whole of herinean.com: a static site generator in Go, the edge Worker, the infrastructure as code, and the audit that scores every build. It is public, and so are these instructions.

**North star.** Anyone examining the site, its hosting or its stack scores it at the maximum on every measurable axis, continuously — and the platform is the least platform that scores perfect: nothing is added that a validator or a reader would not miss.

## Commands

    go build -tags nodynamic -o .cache/site ./cmd/site   build the CLI (never `go run`: it hides exit code 3)
    .cache/site build                     content/ + assets/ + templates/ + i18n/ → dist/
    .cache/site check [--draft|--dist]    validate the sources (--draft: the blanks `site new` leaves are warnings) or dist/
    .cache/site serve [--static]          local preview; --static serves dist/ as built (what CI audits)
    .cache/site new <en|ro> <slug>        scaffold a piece
    .cache/site stamp [--updated] <file>  today's date (Europe/Bucharest) into date: (or updated:)
    .cache/site link <file> <url>         linkedin: or medium:, by the URL's host
    .cache/site scorecard …               the colophon's scorecard fragment (CI)
    scripts/bench.sh                      the CI audit, locally

Every Go command takes `-tags nodynamic`. `site check` exit 3 means author inputs are missing: the pre-commit hook allows it, CI does not.

## Rules

- Every change keeps the scorecard green (`docs/specs/2026-09-17-herinean-com-design.md` §3). Before a commit: `gofmt`, `go vet`, `go tool staticcheck ./...`, `go test ./...`; the pre-commit hook (installed by `scripts/setup.sh`) runs what the staged files need.
- Commits are signed. A message says what changed and, when the diff does not show it, why. It ends with exactly `Co-Authored-By: Claude` — no model name, no version, no session link — and this holds for subagents too, over any trailer their own harness suggests. `Publish:`, `Correct:` and `Link:` commits carry no trailer: they are the author's acts on an article.
- Two remotes: a private one for drafts (every `piece/*` branch) and `origin` on GitHub, public. `origin` is reached only through `/publish-piece` and `/link-piece`, or by the owner.
- A `piece/*` branch carries content only: `content/<lang>/<slug>.md` files sharing one key, and `assets/img/<key>/`.
- The model never edits `content/` by hand. Front-matter fields change through `site stamp` and `site link`; `/translate-piece` creates one new file, once. Everything else in a piece is the author's.
- Review ledgers are private: they live in the shared git directory (`node .claude/skills/publish-piece/gate.mjs path <piece>`) and never enter a commit, a pull request or this repository. Only the author marks them, at `gate.mjs mark` in their own terminal; agents never run `mark`.
- Examples in skills and docs are invented. Nothing from a draft, a ledger or the author's work goes into a file here as an example.
- URLs never change and pieces are never deleted: a correction sets `updated:`; a retraction is a note at the top.
- No private host, local path or session link in any file.

## Where things are

- Design: `docs/specs/2026-09-17-herinean-com-design.md`, milestone designs beside it; plans: `docs/plans/`; decisions: `docs/adr/`.
- Operations: `RUNBOOK.md`. The audit: `bench/` and its README. The skills: `.claude/skills/`.
