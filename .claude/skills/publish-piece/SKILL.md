---
name: publish-piece
description: Publish a reviewed piece, or a correction to a published one — gate, stamp, check, one author-written commit on a branch built from origin/main, push, pull request. Re-stamps an open publish PR. Rehearses without committing. Only when the author asks.
argument-hint: "[--rehearse <base>]"
disable-model-invocation: true
---

# /publish-piece

Invoking this skill is the author's ask to reach `origin`. Nothing is pushed before the author has typed the commit body.

## Rules

1. Push only the publish branch (`publish/<slug>` or `correct/<slug>`), never `piece/<slug>`: a pull request keeps every commit public forever, and the draft history belongs on the private remote.
2. The commit carries the author's body and no trailer. The pull request carries no AI attribution line: the colophon's disclosure covers the process.
3. Stop at the first failed step; say what failed and how to fix it.

## Steps — from the piece's worktree, on `piece/<slug>`

1. **Clean tree:** `git status --porcelain` prints nothing; `git fetch origin`.
2. **Base:** `BASE=origin/main`; with `--rehearse <base>`, `BASE=<base>`.
3. **Scope:** `node .claude/skills/publish-piece/gate.mjs scope --base "$BASE"` — it prints the piece files, or refuses.
4. **Gate:** `node .claude/skills/publish-piece/gate.mjs ledger <each piece file>`. For a translation (a `model-pass-<lang>.md` beside its ledger), also show how much the author changed: `git diff --no-index --stat <model pass> <piece>`.
5. **Mode:** for each piece file, `git cat-file -e "$BASE:<file>"` fails → a publication; succeeds → a correction. Mixed → stop: publish and correct in separate runs.
6. **Binary:** `go build -tags nodynamic -o .cache/site ./cmd/site` and `SITE=$PWD/.cache/site`.
7. **Publish worktree:** `<slug>` is the first piece file's; `B=publish/<slug>` (a correction: `correct/<slug>`; a rehearsal: `rehearse/<slug>`); `W=.claude/worktrees/$(echo "$B" | tr / -)`.
   - `git rev-parse --verify --quiet "$B"` fails → `git worktree add -b "$B" "$W" "$BASE"`.
   - It succeeds and `gh pr view "$B" --json state -q .state` prints `OPEN` → a **re-stamp**: `git worktree add "$W" "$B"`.
   - It succeeds without an open PR → stop and ask the author (a leftover branch).
8. **Copy:** each piece file to `$W/<file>`; when `assets/img/<key>/` exists, `rm -rf "$W/assets/img/<key>" && mkdir -p "$W/assets/img" && cp -r assets/img/<key> "$W/assets/img/"`.
9. **Stamp:** in `$W`, `"$SITE" stamp <file>` for a publication, `"$SITE" stamp --updated <file>` for a correction.
10. **Verify:** in `$W`, `SOURCE_DATE_EPOCH=$(date +%s) "$SITE" check && SOURCE_DATE_EPOCH=$(date +%s) "$SITE" build && "$SITE" check --dist`. (The epoch is now: `$W`'s last commit predates today's stamp.)
11. **Rehearsal stops here:** show `git -C "$W" status --short` and the stamped dates; `git worktree remove --force "$W"`; `git branch -D "$B"`; say "rehearsal complete — nothing committed or pushed".
12. **The author's commit:** show the file list, `git -C "$W" diff --stat` and the stamped dates.
    - A publication or correction: ask *"Write the commit body in your words — it becomes the commit on main."* and wait. Typing it is the confirmation. Title: `Publish: <title>` or `Correct: <title>`, `<title>` from the piece's front matter (for a pair, the English title). Write title, a blank line and the body to a file outside the repository; `git -C "$W" add -A && git -C "$W" commit -F <that file>`. No trailer. The repository's git config signs it.
    - A re-stamp: `git -C "$W" add -A && git -C "$W" commit --amend --no-edit` — the author's message is kept; the PR stays one commit.
13. **Push:** `git -C "$W" push -u origin "$B"`; a re-stamp: `git -C "$W" push --force-with-lease origin "$B"`.
14. **Pull request** (not for a re-stamp): `gh pr create --base main --head "$B" --title "<commit title>" --body-file <file>` with the body

        Preview: CI comments the preview URL on this pull request once the build is up.
        Stamped <date> (Europe/Bucharest). Merge today, or re-run /publish-piece to re-stamp.
        Merge with squash: the commit on main is the author's.

15. **Clean up:** `git worktree remove "$W"` (the local branch stays, for a re-stamp). Tell the author: the PR's URL, the stamped date, and that `piece/<slug>` is untouched on the private remote.

## After the merge
Before the launch, the merge publishes the CI rows and deploys nothing; the launch commit re-stamps every piece dated before `launched:` (`site check` refuses them otherwise). After the launch, the piece is live a few minutes after the merge, once the audit is green. Then the author posts on LinkedIn, and `/link-piece` adds the post's URL.
