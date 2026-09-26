# M2b — Authoring: four skills, drafts that commit, one-commit publish: design

**Status (2026-09-26):** designed; each section reviewed adversarially before approval, then the whole document against the north star, the repository's rules and the working specs and settings (findings folded in: corrections, the LinkedIn post, public skill examples, gate tests in CI, the spec amendments list, trailers, client sign-off). Implementation plan: next.

**Parent spec:** `docs/specs/2026-09-17-herinean-com-design.md` — §8 (authoring and publishing), §10 (the AI disclosure), §11 (launch criteria), §13 (layout). **Sibling:** `docs/specs/2026-09-19-m2a-ci-bench-scorecard-design.md`, whose §2.2 listed M2b's scope. That list is now delivered in three parts: **M2b** (this document) — the path a piece takes from draft to `main`; **M2c** — the weekly job, the uptime monitor, the build id on the wire, CodeQL and OpenSSF Scorecard; **M2d** — the display-face probe and preload, the Android fallback face, the Newsreader name table, Worker tests under workerd. M2b stacks on M2a's branch; nothing in it needs M2a's first Actions run except the auto-merge in §7.

## 1. Goal

A piece goes from `site new` to a merged `Publish:` commit — and a published piece to a merged `Correct:` commit — without a bypassed hook, without a draft reaching the public repository, and without a claim reaching a reader that neither an agent nor the author has checked. The disclosure on the colophon describes what the skills do; the skills are written so that it stays true.

## 2. Scope

### 2.1 M2b delivers

1. **Generator:** `site check --draft`, `site stamp [--updated]`, `site link`, a `check` rule tying piece dates to `launched:`, a host rule for `linkedin:` and `medium:`, and `site serve` keeping the last good build.
2. **The pre-commit hook** runs `check --draft` on `piece/*` branches, and the gate's tests when `.claude/` changes; CI's `build` job runs the gate's tests too (a one-line addition to M2a's `ci.yml`).
3. **Four skills** in `.claude/skills/`: `review-piece`, `translate-piece`, `publish-piece` (with a bundled, tested gate), `link-piece`.
4. **`.claude/CLAUDE.md`**, public.
5. **Docs:** RUNBOOK "Writing and publishing"; spec amendments as one commit.

### 2.2 Not in M2b

Everything listed for M2c and M2d above. The AI disclosure in `site.yaml` is unchanged (decision 6).

### 2.3 Ordering

M2b lands before the first `Publish:` pull request (parent spec §11: the fact-check pass must exist before launch). The acceptance run (§10) is the first parked piece, English and Romanian.

## 3. Decisions taken in the design (with the alternatives rejected)

1. **Judgement in prompts, gates in code.** What a validator can decide is Go (`check --draft`, `stamp`, the date and host rules) or a tested script (`gate.mjs`); what only judgement can decide is a `SKILL.md`. Rejected: prompts only (the Bucharest date and "no PR without the author's marks" would rest on the model reading carefully) and scripts only (more shell, which the agents then have to reason about anyway).
2. **Three-way split of the old M2b list** (see the header). Rejected: one spec (the weekly half would be designed against CI not yet proven on GitHub), and a two-way split.
3. **The fact-check agent reads and judges every claim; the author checks by hand every claim no repository can settle, and signs off.** The agent fetches each cited source and reports what it actually says beside its verdict; the author's column is separate and gates publication (§6). Rejected: a checklist without fetching (loses the agent's read of each source before the author checks it) and agent verdicts the author merely approves (the disclosure says the author checks by hand).
4. **The ledger is fully private.** It lives in the shared git directory, never in a commit, never in a pull request. Rejected: the claims table in the public PR description, and a public sources page per piece.
5. **Publishing pushes one commit, not the draft branch.** A pull request's commits stay public after a squash; the draft history stays on the private remote (§6).
6. **The disclosure stays as the author wrote it.** Every sentence remains true under this design. It does not mention that an agent also reads the web sources before the author checks them; the author's call, recorded here because spec §10 asks the disclosure to state *exactly* how AI is used and the public skill shows the agent's read.
7. **The author writes the LinkedIn post; the model assists under the one-sentence rule.** A model-written post under the author's name would be AI writing the disclosure does not cover, and the disclosure matching `/review-piece` is a launch criterion. Rejected: the model drafts the post, disclosed or not.
8. **Skill files carry invented examples only.** They are public; nothing from a draft, a ledger or the author's work goes into a `SKILL.md` or `glossary.md` as an example.

## 4. Drafts, dates and the hook

- **`site check --draft`** runs the loader's existing draft mode (the one `site serve` uses): the fields `site new` leaves blank — title, date, pillar, summary — are warnings on stderr, exit 0. Every other rule still fails with `file:line` (cedilla diacritics, summary over 160 characters, a `draft:` field, a date set in the future, a malformed `updated:` or `linkedin:`). Missing author inputs stay exit 3.
- **The hook** runs `site check --draft` when HEAD is a `piece/*` branch and plain `site check` otherwise, a detached HEAD included. CI keeps plain `check`, so a dateless piece cannot reach `main`.
- **`site stamp [--updated] <file>`** writes `date:` (or, with `--updated`, `updated:`) = today in Europe/Bucharest and prints `old → new`; run again, it re-stamps. It writes a source file, not build output: the rule that wall-clock time never enters `dist/` holds. `--updated` is for corrections (§6): `datePublished` and the feed's `pubDate` stay, per parent spec §8 step 8.
- **Rule: no piece dated before the launch.** When `site.yaml` has `launched:`, a piece whose `date` is earlier fails `check` with "dated before the launch; `site stamp` it". The launch commit therefore re-stamps the pieces merged before launch, and `datePublished` never names a day on which the site was not public.
- **Rule: discussion hosts.** `linkedin:` must parse to host `www.linkedin.com`, `medium:` to `medium.com`, compared exactly (not by prefix).
- **`site link <file> <url>`** sets `linkedin:` or `medium:` by the URL's host (the rule above), on a piece whose `date` is set. With `site stamp`, it is how a skill changes a piece's front matter: by running a command, never by editing the file.
- **`site serve` keeps the last good build.** A failing rule prints its problems with `file:line` and the server keeps serving the previous good render, instead of answering 500 on every page. With no good build yet, it answers 500 with the problem list.

## 5. `/review-piece` and `/translate-piece`

**Ledger location:** `$(git rev-parse --git-common-dir)/review/<key>/` — inside the shared git directory: shared by every worktree, never committed, survives `git worktree remove`. Per language: `ledger-<lang>.md`, `linkedin-<lang>.md`, and for a translation `model-pass-<lang>.md`. The ledger header records the SHA-256 of the piece file as reviewed and the repositories used, as local paths given by the author at the first run. The skills print the path.

**Ledger format** (what `gate.mjs` parses; everything else in the file is free prose for the author):

```
hash: sha256:<hex of the piece file as reviewed>
repos: <path>, <path>

## Claims
| # | Claim | Kind | Evidence | Agent | Author | Note |
  Kind   = repo | source
  Agent  = confirmed | wrong | unverifiable | blocked
  Author = ✓ | ✗ | (empty)

## Tier
| # | Sentence | Tier | Why | Resolution |
  Tier       = work | client
  Resolution = kept | changed | signed-off | (empty)
               client flags: signed-off (who and when in Why) or changed; never kept
```

**`/review-piece content/<lang>/<slug>.md`**, on a `piece/*` branch, re-runnable. Its first rule: **it never writes to `content/`** — front-matter fields change only through `site stamp` and `site link`. This holds by instruction; the hash in the ledger makes an edit the author did not make visible. Passes, in order:

1. **Mechanical:** `site check --draft`; word count against the 1,500–3,000 band (reported, never a failure: parent spec §2 describes the reader, not a rule); every footnote defined and referenced; no embeds; every foreign-language fragment inside a `<span lang>` (spec row 3's in-piece rule, handed over by M2a); Romanian diacritics beyond the cedilla rule `check` already enforces.
2. **Tier, by judgement:** own project; generalised lesson from work (no names, no numbers); named client (sign-off first, parent spec §2). Each flag quotes the sentence, names its tier and says why; each needs the author's resolution before publication — *kept* or *changed* for a work flag, *signed-off* or *changed* for a client flag.
3. **Shape:** situation → trade-off → decision → what I'd change, as a note; the template is not a rule.
4. **Editing, under the one-sentence rule:** flags, questions and suggestions keyed by line; a reformulation is at most one sentence, written as an exact *before → after*; the author applies what they accept.
5. **Fact-check, by a fresh agent** that has not seen the editing pass. It extracts every checkable claim, footnoted or not. *Repository claims:* confirmed / wrong / unverifiable, with the `file:line` or commit. *Source claims:* the cited URL fetched, the passage quoted, the date read, the agent's verdict — or *blocked* when the site refuses the fetch, in which case it looks for the primary document or a second outlet and records which it used. The author's column is empty until the author fills it.
6. **LinkedIn post, written by the author:** the skill proposes the angle, three candidate opening lines and the points worth carrying over from the piece; the author writes the post in `linkedin-<lang>.md`; the skill then checks it (native and complete, not a teaser; ≤ 3,000 characters; the link with `?ref=li` kept for the first comment) and may propose reformulations of single sentences, as in pass 4.

Re-runs keep the author's marks on claims whose text is unchanged; a changed claim resets to unchecked. The final run after the last edit is therefore cheap, and it is required (§6).

**`/translate-piece content/<from>/<slug>.md <to>`** writes one new file, `content/<to>/<slug>.md`: same `key` and pillar; title, summary and body translated; `date` and `linkedin` empty. This is the one exception to "never writes to `content/`", stated in both skills: it creates the file once and refuses if it exists. The target slug is proposed and the author decides it (URLs never change after publication). Idiomatic, not literal; a public `glossary.md` in the skill carries the house rules and terms (Romanian: plural address, comma-below diacritics, *incumbent* → *jucătorul consacrat*) and grows as the author corrects passes — terms and rules only, never sentences from a draft (decision 8). Quotations are translated in the body; the footnote keeps the original inside `<span lang>`. It runs `site check --draft` and reports the summary length. The untouched pass is saved as `model-pass-<to>.md`. The translation then gets its own `/review-piece`; its fact-check matches each claim to the source-language ledger and confirms every figure, date and name crossed unchanged.

## 6. `/publish-piece`

On `piece/<slug>` with a clean tree:

1. **Scope:** the branch's changes against `origin/main` may touch only `content/*/` files sharing one `key` and `assets/img/<key>/`; anything else is refused.
2. **Gate** — `node .claude/skills/publish-piece/gate.mjs` (Node standard library only; no package), per changed piece file: the ledger exists; its hash equals the file's; every source claim carries the author's ✓; every repository claim is confirmed or carries the author's ✓ with a note; every tier flag is resolved, and no client flag is *kept*; a translation differs from its model pass (and the skill shows how much the author changed).
3. **Mode:** a piece file absent from `origin/main` is a **publication**; one present there is a **correction** (parent spec §8 step 8 — a fix, or a retraction note at the top). The mode sets the stamp, the branch prefix and the commit title below.
4. **Stamp and verify, on a publish branch:** `publish/<slug>` (or `correct/<slug>`) from `origin/main`; the piece files and images copied from the piece branch; `site stamp` on each new piece file, `site stamp --updated` on each corrected one; `site check`, `site build`, `site check --dist`. The piece branch is never stamped and stays a draft. For a correction, the gate's hash and marks cover the changed claims; unchanged claims keep their marks from the publication's ledger.
5. **The author's commit:** the skill shows the file list and diff stat and asks for the commit body in the author's words; typing it is the confirmation before anything reaches `origin`. One signed commit, `Publish: <title>` or `Correct: <title>`, that body, no trailer: like `Link:`, these are the author's acts on an article, and the colophon's disclosure covers the process (site commits keep the bare `Co-Authored-By: Claude`).
6. **Push and PR:** push the publish branch (never `piece/<slug>`); `gh pr create`. The description says where the preview will appear and "merge today, or re-run `/publish-piece` to re-stamp". It carries no AI attribution line: the colophon's disclosure covers the process.
7. **Re-stamp:** with an open PR, `/publish-piece` re-runs the gate, re-stamps, amends the single commit (re-signed) and pushes with `--force-with-lease` to the publish branch (the `main` ruleset does not cover it); the PR stays one commit, so with the repository's squash settings (title: commit or PR title; message: commit messages) the squash commit carries the author's message; GitHub appends the PR number to the title and signs the squash commit with its own key.
8. **Merge:** squash (the repository allows merge and squash). Before launch the merge publishes CI rows and does not deploy; §4's launch rule re-stamps the piece on launch day.
9. **Rehearsal:** `/publish-piece --rehearse <base>` runs steps 1–4 against `<base>` instead of `origin/main` and stops before the commit; nothing is committed or pushed. The acceptance run uses it, because `origin/main` has no M2b until M2b merges.

## 7. `/link-piece`

`/link-piece content/<lang>/<slug>.md <url>` runs `site link` (§4) on `link/<slug>` from `origin/main`: `site check`, one signed commit `Link: <title> → LinkedIn` (or Medium), no trailer (§6 step 5), push, PR. Not a correction: `updated:` is untouched. The link changes a live page (a second link labelled "LinkedIn" beside the byline's), so the PR takes the full audit like any other. **Auto-merge** (`gh pr merge --auto --squash`) is used only when the repository has a required status check and auto-merge enabled (§12); until then the skill opens the PR and leaves the merge to the author, and says so.

## 8. `.claude/CLAUDE.md`

Public, written as if clients read it. The north star in two sentences; the commands (`site build`, `check [--draft|--dist]`, `serve [--static]`, `stamp`, `link`, `new`, `scorecard`); commits are signed and end with a bare `Co-Authored-By: Claude`, which applies to subagents too, over any default trailer their harness suggests — except `Publish:`, `Correct:` and `Link:` commits, which carry none; skill files use invented examples only; `origin` is reached only through `/publish-piece` and `/link-piece` or by the owner; `piece/*` branches carry content only; the model never edits `content/` by hand — front-matter fields are set by `site stamp` and `site link`, and `/translate-piece` creates one new file, once; ledgers are private and not in the repository; every change keeps the scorecard green; where the spec, ADRs, RUNBOOK and plans are. It names no private host.

## 9. Layout

```
.claude/CLAUDE.md
.claude/skills/review-piece/SKILL.md
.claude/skills/translate-piece/SKILL.md   glossary.md
.claude/skills/publish-piece/SKILL.md     gate.mjs   test/ (node --test, fixture ledgers)
.claude/skills/link-piece/SKILL.md
```

`.gitignore` already excludes only `/.claude/worktrees/`.

## 10. Testing and acceptance

- **Go:** `check --draft` (blank fields warn, every other rule fails); `stamp` and `stamp --updated` (Bucharest date across the UTC midnight, re-stamp, `date` untouched by `--updated`); `link` (field by host, replace, refuse a look-alike host and an unpublished piece); the launch rule; the host rule (exact host, a look-alike refused); `serve` keeping the last good build and answering 500 with no good build.
- **Gate:** `node --test` over fixture ledgers — one passing fixture and one failing fixture per rule (stale hash, missing ✓ on a source claim, unresolved tier flag, a client flag marked *kept*, wrong repository claim without a note, translation equal to its model pass); run by CI's `build` job and by the hook.
- **Hook:** the branch switch is shell; it is exercised by committing a dateless draft on a `piece/*` branch (passes with warnings) and on another branch (refused).
- **Acceptance:** the first parked piece. `/review-piece` on the English and the Romanian file (the September ledger kept as reference and moved into the new location); `/publish-piece --rehearse m2b/design` run up to the body prompt, with the gate shown refusing a stale hash and a missing ✓ first. Pushing the publish branch is the author's call at that prompt.

## 11. Docs

- RUNBOOK: "Writing and publishing" — the four skills, where ledgers live, re-stamping, corrections, what the gate refuses; a backup line for the ledgers in the pattern of `scripts/infra-backup.sh` (a timestamped copy under `~/.config/herinean/`, not a remote), because the ledger is the evidence behind the disclosure's process claim and otherwise lives on one disk.
- Spec amendments, one commit, written against the text after `colophon/one-ai-disclosure` and M2a are merged: §4.2 `linkedin:`/`medium:` hosts; §5's verb list gains `stamp [--updated]`, `link` and `check --draft`, and its `serve` line keeps the last good build; §5.2's "empty `date`" gains "(a warning under `--draft`)" and the list gains the launch-date and host rules; §8 step 3 names `/review-piece`'s passes as built (the LinkedIn post written by the author); step 4 describes the publish branch and the correction mode; step 8 names `/publish-piece` as the correction path; step 5 becomes "Squash-merge: the commit is already the author's `Publish: <title>`. Live a few minutes after the merge, once the audit is green (ADR-0015)." — the sentence about GitHub pre-filling the drafts' messages goes, because there are none; §11's M2 bullet names the three parts; §13's `cmd/site/` line lists the verbs and its `.claude/` line the layout of §9.

## 12. Waits for Radu

- After M2a's first Actions run: add a required status check on the gate job to the `main` ruleset, and enable auto-merge in the repository settings. Until both exist, pull requests can merge with CI red (M2a's deploy still waits for a green audit, so red never goes live) and `/link-piece` does not auto-merge.
- The pushes at the acceptance run's body prompt.

## 13. Decisions for Radu (made in the design without a question; revertable)

1. Squash is the only merge method the skills assume; the repository also allows merge commits, kept for milestone branches whose commits are the story.
2. The ledger is Markdown with two fixed tables (§5) rather than YAML: the author reads and marks it by hand, and the gate needs only the two tables.
