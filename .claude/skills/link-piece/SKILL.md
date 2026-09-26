---
name: link-piece
description: Add the LinkedIn (or Medium) discussion URL to a published piece — one commit on a branch built from origin/main, a pull request, auto-merge when the repository allows it. Only when the author asks.
argument-hint: content/<lang>/<slug>.md <url>
disable-model-invocation: true
---

# /link-piece

Invoking this skill is the author's ask to reach `origin`.

## Rules

- The piece must already be on `origin/main`.
- The front matter changes only through `site link`; nothing else in the file changes. It is not a correction: `updated:` stays as it is.
- The commit carries no trailer.

## Steps — from any worktree of the repository

1. `git fetch origin`; `git cat-file -e origin/main:<file>` — if it fails, stop: the piece is not published.
2. `B=link/<slug>`; `W=.claude/worktrees/link-<slug>`; `git worktree add -b "$B" "$W" origin/main`.
3. `go build -tags nodynamic -o .cache/site ./cmd/site && SITE=$PWD/.cache/site`; then `(cd "$W" && "$SITE" link <file> <url> && SOURCE_DATE_EPOCH=$(date +%s) "$SITE" check)` — `site link` refuses any host but `www.linkedin.com` and `medium.com`.
4. `git -C "$W" commit -am "Link: <title> → LinkedIn"` (or `→ Medium`), `<title>` from the front matter. No trailer.
5. `git -C "$W" push -u origin "$B"`; `gh pr create --base main --head "$B" --title "Link: <title> → LinkedIn" --body "Adds the discussion link. The page changes, so the full audit runs."`.
6. Auto-merge only when both hold: `gh api repos/{owner}/{repo} --jq .allow_auto_merge` prints `true`, and `gh api repos/{owner}/{repo}/rulesets --jq '.[].id'` lists a ruleset whose rules (`gh api repos/{owner}/{repo}/rulesets/<id> --jq '.rules[].type'`) include `required_status_checks`. Then `gh pr merge "$B" --auto --squash`. Otherwise tell the author: "auto-merge is not set up; merge it yourself once CI is green."
7. `git worktree remove "$W"`. Tell the author the PR's URL and whether it will auto-merge.
