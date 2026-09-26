---
name: review-piece
description: Review a draft piece before publication — mechanical checks, tier check, shape, an editing pass under the one-sentence rule, a fact-check by a fresh agent, and help with the LinkedIn post the author writes. Use on a piece/* branch when the author asks for a review.
argument-hint: content/<en|ro>/<slug>.md
---

# /review-piece

Reviews one piece and writes a private ledger. The author decides everything; this skill finds, checks and proposes.

## Rules

1. **Never write to `content/`.** Propose; the author applies. A reformulation is at most one sentence, written as an exact *before → after*; never a rewritten paragraph. The ledger records the piece's hash, so an edit the author did not make shows.
2. The ledger never enters a commit, a pull request or any public place. It lives in the shared git directory.
3. The examples in this file are invented. Never quote a draft or a ledger into a file in this repository.

## Before the passes

- The branch is `piece/*`; the argument is `content/<en|ro>/<slug>.md`.
- `LEDGER=$(node .claude/skills/publish-piece/gate.mjs path <piece>)` and `mkdir -p "$(dirname "$LEDGER")"`.
- If `$LEDGER` exists, read it: this is a re-run (see "Re-runs").
- On the first run, ask the author which repositories the piece is about — this one, and any other it names — and their local paths. They go on the `repos:` line and stay private.
- `go build -tags nodynamic -o .cache/site ./cmd/site`.

## The passes, in order

### 1. Mechanical
- `.cache/site check --draft` — report every problem with its `file:line`.
- Words in the body, front matter and footnote definitions excluded:
  `awk '/^---$/{fm++; next} fm>=2 && !/^\[\^[^]]+\]:/' <piece> | wc -w` — report it against 1,500–3,000. Reported, never a failure.
- Every footnote reference `[^x]` has a definition `[^x]:`, and every definition is referenced.
- No embeds: no `<iframe>`, `<script>`, `<video>`, `<object>`, `<embed>`. A video is a self-hosted thumbnail linking out.
- Every fragment in a language other than the piece's sits inside `<span lang="…">…</span>` — an English phrase in a Romanian piece, a Romanian name of an institution in an English one. List each unmarked fragment with its line.
- Romanian: ș ț Ș Ț with comma below (`check` already refuses the cedilla forms); list words that look like they are missing ă, â or î.

### 2. Tier, by judgement
An **own project** needs no approval. A **generalised lesson from work** carries no names and no numbers. A **named client** needs the client's sign-off first. Read every sentence that touches work other than the author's own projects; flag each one that a person at that organisation could read as a disclosure. Quote it, name the tier (`work` or `client`), say why in one line. There is no list of forbidden names: this is judgement.
Invented example: *"Our warehouse system loses a day of orders at every quarter-end."* — tier `work`: an internal system and a failure an employer's board would recognise.

### 3. Shape
The editorial template is situation → trade-off → decision → what I'd change. Say which parts the piece has and which it lacks, as a note. It is not a rule.

### 4. Editing, under the one-sentence rule
Flags, questions and suggestions, each keyed by line: an unclear sentence, a strong claim without support, a term used two ways, a repetition, a claim that needs a hedge. A proposed reformulation is one sentence, as *before → after*, copied exactly so that applying it is a paste.

### 5. Fact-check, by a fresh agent
Dispatch a new subagent that has not seen passes 1–4. Give it the piece's path, the repositories and their paths, the Claims table header below, and these instructions:
- Extract every checkable claim, footnoted or not: figures, dates, names, quotations, what code does, what a commit changed.
- **Repository claims** — the code or its history can settle them: read the code, the commits, the logs. Agent `confirmed`, `wrong` or `unverifiable`; Evidence is `path:line` or a commit hash.
- **Source claims** — everything else: fetch the cited URL. Evidence is the URL, the passage quoted exactly, and the date the page gives. Agent `confirmed`, `wrong`, `unverifiable`, or `blocked` when the site refuses the fetch — then find the primary document or a second outlet that states the same thing, and say in Evidence which one was used.
- Leave Author and Note empty. Never edit the piece.
Put the claims it returns into the Claims table.

For a translation (a `model-pass-<lang>.md` sits beside the ledger), also give the agent the source-language ledger (`ledger-<other lang>.md`): it matches each claim to its original, and every figure, date and name must cross unchanged — a number or date written in the target language's format per `.claude/skills/translate-piece/glossary.md` (for example 1,500 → 1.500) is the same figure, not a mismatch. A mismatch is `wrong`, with both versions in Evidence.

### 6. LinkedIn post — the author writes it
Propose, in the ledger's LinkedIn section: the angle in one line, three candidate opening lines, and the points from the piece worth carrying over. The author writes the post in `linkedin-<lang>.md` beside the ledger. When that file exists, check it: native and complete, not a teaser; at most 3,000 characters (`wc -m`); the piece's URL with `?ref=li` kept for the first comment, not in the post. Propose single-sentence reformulations only, as in pass 4.

## The ledger

Write or update `$LEDGER` in this shape (the gate reads the `hash:` line and the two tables; the rest is for the author):

    # Review — <title> (<lang>)

    hash: <output of: node .claude/skills/publish-piece/gate.mjs hash <piece>>
    repos: <path>, <path>

    ## Claims

    | # | Claim | Kind | Evidence | Agent | Author | Note |
    |---|---|---|---|---|---|---|

    ## Tier

    | # | Sentence | Tier | Why | Resolution |
    |---|---|---|---|---|

    ## Mechanical
    ## Shape
    ## Editing
    ## LinkedIn
    ## Rounds

- Kind: `repo` or `source`. Agent: `confirmed`, `wrong`, `unverifiable`, `blocked`.
- Author: `✓`, `✗` or empty — only the author fills it. Resolution: `kept` or `changed` for a `work` flag; `signed-off` (who and when, in Why) or `changed` for a `client` flag — only the author fills it.
- A `|` inside a cell is written `\|`.
- Each of the two gate tables is one block: the header, the separator, then the rows, with no blank line, prose or `|`-bearing text between them or elsewhere inside `## Claims` and `## Tier`. Notes go in the Note column or in the later sections. The gate refuses a table it cannot read whole.
- `## Rounds` gets one dated line per run: what changed since the last run.

## Re-runs
Re-extract the claims and flags from the current text. Keep the author's Author, Note and Resolution on claims and flags whose text is unchanged; a changed claim or flag starts empty again. Update the `hash:` line. The last run must follow the last edit: `/publish-piece` refuses a ledger whose hash is not the piece's.

## What to tell the author
The ledger's path; claims by verdict; claims waiting for the author's ✓; tier flags waiting for a resolution; the mechanical problems; that nothing in `content/` was changed. To mark, they run `node .claude/skills/publish-piece/gate.mjs mark` in their own terminal — it prompts through everything still waiting for them and writes the cells itself. Agents never run `mark`: it refuses without a terminal, because the marks are the author's alone.
