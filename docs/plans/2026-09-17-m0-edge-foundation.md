# M0 — Edge Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Four zones on Cloudflare configured as code, mail hardened on `herinean.com`, three domains redirecting, a placeholder Worker on the apex serving the full production header set, and the public repo guarded — so TLS/HSTS/internet.nl can be verified on the real domain before a line of the generator exists.

**Architecture:** OpenTofu (`infra/`) owns everything zone-level: settings, DNS, mail, DNSSEC, redirect rulesets. wrangler (`wrangler.toml`, `worker/index.js`) owns the Worker, its assets and custom domains. A bash verification script (`scripts/verify-edge.sh`) is the test suite: every task adds assertions that fail before `apply` and pass after; it becomes the seed of M2's post-deploy job.

**Tech Stack:** OpenTofu ≥ 1.8 with `cloudflare/cloudflare ~> 5.0`; wrangler 4 via `npx` (Node 22); `curl`, `dig`, `jq`, `gh`.

**Spec:** `docs/specs/2026-09-17-herinean-com-design.md` — §3 rows 9, 10, 13, 15, 16, 17; §6 (all); §11 M0.

**Status (2026-09-19):** merged to `main` as PR #1 (merge commit `b3adeb0`). Still open below: Task 9 step 4 (HSTS preload, earliest 2026-09-25); the ruleset gate was probed on 2026-09-19 (Task 2 step 2). The DMARC `p=reject` flip on `.com` landed as `4217aea` after the signed test message.

## Global Constraints

- Zones: `herinean.com` (content), `herinean.ro`, `herinean.net`, `herinean.info` (web: redirect-only). All four are Google Workspace domains of the same mailbox (`.net` is the Workspace primary, the others aliases) and **keep their MX**. All Free plan; **Workers Paid** on the account.
- TLS: minimum 1.3, HTTP/3 on, 0-RTT **off**, Always-HTTPS, HSTS `max-age=63072000; includeSubDomains; preload`.
- Injectors **off**: Email Address Obfuscation, Rocket Loader, Cloudflare Fonts, Speed Brain, Bot Fight Mode. (Mirage/Polish/Auto Minify are not on Free or no longer exist — nothing to disable.)
- Mail `.com`: MX kept (`smtp.google.com` 1, imported); SPF `v=spf1 include:_spf.google.com -all` (already so, imported); DKIM 2048 selector `google` (exists, imported); DMARC `p=reject; adkim=s; aspf=s; rua=mailto:<existing Cloudflare DMARC Management address>,mailto:dmarc@herinean.com` (via `p=none` first); MTA-STS `mode: testing` first; TLS-RPT to `dmarc@`. Other three: MX kept (imported), SPF set to `v=spf1 include:_spf.google.com -all`, DMARC `p=reject; adkim=s; aspf=s; rua=mailto:dmarc@<that domain>`. The `google-site-verification` TXT on `.net` stays unmanaged. **No plan in M0 destroys a record.**
- Redirects: `www.herinean.com` and everything on `.ro/.net/.info` → `https://herinean.com{path}{query}`, 301.
- Headers on every production response: CSP `default-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'` (placeholder has no CSS; M1 adds the hashed `style-src`), HSTS as above, `X-Content-Type-Options: nosniff`, `Referrer-Policy: strict-origin-when-cross-origin`, `Permissions-Policy` deny list, `Cross-Origin-Opener-Policy: same-origin`, `Cross-Origin-Resource-Policy: same-origin`, `X-Frame-Options: DENY`.
- No executable `<script>` anywhere; no `style=` attributes; no `/cdn-cgi/` script injection.
- Repo: public on GitHub (`origin`), private on Gitea (`gitea`); `main` requires PR + signed commits + squash merge; commits authored `radu@herinean.com`; every commit ends with the Claude co-author trailers.
- Secrets never enter the repo or the chat: the API token lives in `~/.config/herinean/cf-infra.token` (mode 600). Zone IDs, account ID and the DKIM public key are **not** secrets and are committed.
- Never `git push --force` to `main`. Never `tofu apply` without a reviewed `tofu plan`.

## Operator inputs (collected in Task 0, used everywhere)

`~/.config/herinean/m0.env` (mode 600, never committed):

```bash
export CLOUDFLARE_API_TOKEN="$(cat ~/.config/herinean/cf-infra.token)"
export CLOUDFLARE_ACCOUNT_ID="<32-hex account id: dashboard → any zone → Overview → API → Account ID>"
export ZONE_COM="<zone id>"   # dashboard → herinean.com → Overview → API → Zone ID
export ZONE_RO="<zone id>"
export ZONE_NET="<zone id>"
export ZONE_INFO="<zone id>"
export GITHUB_OWNER="raduherinean"   # the organisation; repo exists and is empty
export GITEA_REMOTE="<ssh url, e.g. git@gitea.example:radoo/herinean.com.git>"
export DKIM_TXT='"v=DKIM1; k=rsa; p=..."'   # exact API content of google._domainkey (quotes included)
```

## File map

| File | Responsibility |
|---|---|
| `README.md` | What the repo is, status, link to spec — public from the first push |
| `RUNBOOK.md` | Operations: tokens, tofu, state backup, DS records, placeholder deploy, rollback, incident notes |
| `scripts/env.sh` | Sources `~/.config/herinean/m0.env`, fails loudly if missing |
| `scripts/verify-edge.sh` | The test suite for the edge: curl/dig assertions, grows per task |
| `scripts/infra-backup.sh` | Copies tofu state to `~/.config/herinean/tfstate/<timestamp>.json` |
| `scripts/infra-import-mail.sh` | Lists the pre-existing MX/SPF/DKIM/DMARC records on all four zones and prints the `tofu import` commands |
| `scripts/deploy-placeholder.sh` | Builds `dist/` from `infra/placeholder/` and runs `wrangler deploy` |
| `infra/versions.tf` | Tofu + provider pins |
| `infra/variables.tf` | Inputs: account id, zone ids, domain names, DKIM, DMARC policy, MTA-STS id, MX set, Cloudflare DMARC report address |
| `infra/zones.auto.tfvars` | Committed values for the variables above (nothing secret) |
| `infra/settings.tf` | Zone settings for all four zones + Bot Fight Mode off |
| `infra/dns-web.tf` | CAA on all zones; proxied placeholders on `www.com` and apex+`www` of redirect zones |
| `infra/dns-mail.tf` | Mail on all four zones: MX (imported), SPF, DMARC; plus DKIM (imported), MTA-STS, TLS-RPT on `.com` |
| `infra/redirects.tf` | Dynamic-redirect rulesets |
| `infra/dnssec.tf` | DNSSEC on all zones; DS outputs |
| `infra/placeholder/` | `index.html`, `404.html`, `_headers`, `robots.txt`, `.well-known/security.txt`, `.well-known/mta-sts.txt` |
| `wrangler.toml` | Worker name, assets config, custom domains, `run_worker_first` |
| `worker/index.js` | Host routing (prod / mta-sts / preview), noindex on non-prod |
| `docs/adr/0002-workers-static-assets-over-origin.md`, `0005-workers-paid.md`, `0011-tls13-minimum.md`, `0012-infrastructure-as-code.md` | Decisions exercised by M0 |

---

### Task 0: Preflight — inputs, tools, token, zones active

**Files:**
- Create: `scripts/env.sh`
- Create: `scripts/verify-edge.sh` (skeleton with the assertion helpers)

**Interfaces:**
- Produces: `scripts/env.sh` (sourced by every later script; exports the variables listed under *Operator inputs*); `scripts/verify-edge.sh` with helpers `expect_header URL HEADER REGEX`, `expect_status URL CODE`, `expect_location URL EXPECTED`, `expect_txt NAME REGEX`, `expect_no_body URL REGEX`, `section NAME`, and a final summary that exits non-zero on any failure.

- [x] **Step 1: Confirm the dashboard prerequisites are done** (operator, from the spec §11 M0 list): four zones added, nameservers changed, Workers Paid active, Workspace DKIM generated (2048, selector `google`) and aliases `security@` + `dmarc@` created, `herinean-infra` token created (TTL 7 days), `~/.config/herinean/cf-infra.token` written with mode 600, `~/.config/herinean/m0.env` written as above.

- [x] **Step 2: Write `scripts/env.sh`**

```bash
#!/usr/bin/env bash
# Source this: `. scripts/env.sh`. Loads operator inputs for M0 from outside the repo.
set -euo pipefail
ENV_FILE="${HOME}/.config/herinean/m0.env"
if [ ! -r "$ENV_FILE" ]; then
  echo "missing $ENV_FILE — see docs/plans/2026-09-17-m0-edge-foundation.md 'Operator inputs'" >&2
  return 1 2>/dev/null || exit 1
fi
# shellcheck disable=SC1090
. "$ENV_FILE"
for v in CLOUDFLARE_API_TOKEN CLOUDFLARE_ACCOUNT_ID ZONE_COM ZONE_RO ZONE_NET ZONE_INFO GITHUB_OWNER GITEA_REMOTE DKIM_TXT; do
  [ -n "${!v:-}" ] || { echo "$v is empty in $ENV_FILE" >&2; return 1 2>/dev/null || exit 1; }
done
export TF_VAR_account_id="$CLOUDFLARE_ACCOUNT_ID"
```

- [x] **Step 3: Write the `scripts/verify-edge.sh` skeleton**

```bash
#!/usr/bin/env bash
# Edge verification suite. Every M0 task appends a section. Exit 1 if anything fails.
set -uo pipefail
FAIL=0; PASS=0
section() { printf '\n== %s\n' "$1"; }
ok()   { PASS=$((PASS+1)); printf '  ok   %s\n' "$1"; }
bad()  { FAIL=$((FAIL+1)); printf '  FAIL %s\n' "$1"; }
hdrs() { curl -sS -o /dev/null -D - --max-time 15 "$@" 2>/dev/null; }
# expect_header URL HEADER REGEX   — header value (case-insensitive name) matches regex
expect_header() {
  local v; v=$(hdrs "$1" | awk -v h="$2" 'BEGIN{IGNORECASE=1} tolower($1)==tolower(h":"){sub(/^[^:]*: */,""); sub(/\r$/,""); print; exit}')
  if printf '%s' "$v" | grep -Eq -- "$3"; then ok "$1 $2: $v"; else bad "$1 $2 expected /$3/ got '${v:-<absent>}'"; fi
}
# expect_status URL CODE
expect_status() {
  local c; c=$(curl -sS -o /dev/null -w '%{http_code}' --max-time 15 "$1" 2>/dev/null)
  if [ "$c" = "$2" ]; then ok "$1 → $c"; else bad "$1 expected $2 got $c"; fi
}
# expect_location URL EXPECTED  — first response is a redirect to EXPECTED
expect_location() {
  local l; l=$(hdrs "$1" | awk 'BEGIN{IGNORECASE=1} tolower($1)=="location:"{print $2; exit}' | tr -d '\r')
  if [ "$l" = "$2" ]; then ok "$1 → $l"; else bad "$1 expected Location $2 got '${l:-<absent>}'"; fi
}
# expect_txt NAME REGEX — some TXT record at NAME matches (via 1.1.1.1)
expect_txt() {
  local t; t=$(dig +short TXT "$1" @1.1.1.1 | tr -d '"' | tr '\n' ' ')
  if printf '%s' "$t" | grep -Eq -- "$2"; then ok "TXT $1 ~ /$2/"; else bad "TXT $1 expected /$2/ got '${t:-<none>}'"; fi
}
# expect_no_body URL REGEX — body must NOT match
expect_no_body() {
  if curl -sS --max-time 15 "$1" 2>/dev/null | grep -Eq -- "$2"; then bad "$1 body matches forbidden /$2/"; else ok "$1 body free of /$2/"; fi
}
# expect_dig NAME TYPE REGEX — dig +short output matches
expect_dig() {
  local r; r=$(dig +short "$2" "$1" @1.1.1.1 | tr '\n' ' ')
  if printf '%s' "$r" | grep -Eq -- "$3"; then ok "$2 $1 ~ /$3/"; else bad "$2 $1 expected /$3/ got '${r:-<none>}'"; fi
}

section "preflight"
for t in curl dig jq; do command -v "$t" >/dev/null && ok "$t present" || bad "$t missing"; done

# --- task sections are appended below this line ---

printf '\n%d passed, %d failed\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ]
```

- [x] **Step 4: Verify the token and that all four zones are active**

```bash
. scripts/env.sh
curl -sS -H "Authorization: Bearer $CLOUDFLARE_API_TOKEN" https://api.cloudflare.com/client/v4/user/tokens/verify | jq -r '.result.status'
for z in $ZONE_COM $ZONE_RO $ZONE_NET $ZONE_INFO; do
  curl -sS -H "Authorization: Bearer $CLOUDFLARE_API_TOKEN" "https://api.cloudflare.com/client/v4/zones/$z" | jq -r '"\(.result.name) \(.result.status) \(.result.plan.name)"'
done
for d in herinean.com herinean.ro herinean.net herinean.info; do echo "$d NS: $(dig +short NS $d @1.1.1.1 | tr '\n' ' ')"; done
```
Expected: `active`; four lines `herinean.<tld> active Free Website`; NS lines showing `*.ns.cloudflare.com`. If a zone is `pending`, stop — nameservers haven't propagated; nothing in Tasks 3–9 can be verified until they have.

- [x] **Step 5: Tools** — `tofu version` (install if missing: `curl -fsSL https://get.opentofu.org/install-opentofu.sh | sh -s -- --install-method standalone`, then re-run), `node --version` (need 22.x; install via the distro or `nvm install 22`), `gh auth status`.

- [x] **Step 6: Run the suite skeleton**

Run: `chmod +x scripts/*.sh && scripts/verify-edge.sh`
Expected: preflight section with three `ok` lines, `3 passed, 0 failed`.

- [x] **Step 7: Commit** (on a branch — `main` is protected from Task 2 onward, so start the discipline now)

```bash
git checkout -b infra/m0
git add scripts/env.sh scripts/verify-edge.sh
git commit -m "$(cat <<'MSG'
M0: operator env loader and edge verification skeleton

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
MSG
)"
```

---

### Task 1: Commit signing, README, remotes, first push

**Files:**
- Create: `README.md`

**Interfaces:**
- Produces: signed history on `main`; remotes `origin` (GitHub, public) and `gitea` (private).

- [x] **Step 1: SSH signing key**

```bash
ssh-keygen -t ed25519 -C radu@herinean.com -f ~/.ssh/herinean_signing -N ""
git config gpg.format ssh
git config user.signingkey ~/.ssh/herinean_signing.pub
git config commit.gpgsign true
git config gpg.ssh.allowedSignersFile ~/.config/git/allowed_signers
mkdir -p ~/.config/git && printf 'radu@herinean.com %s\n' "$(cat ~/.ssh/herinean_signing.pub)" >> ~/.config/git/allowed_signers
```
Then in GitHub → Settings → SSH and GPG keys → **New SSH key → Key type: Signing Key** → paste `~/.ssh/herinean_signing.pub`. Also Settings → Emails → add and verify `radu@herinean.com`.

- [x] **Step 2: Re-sign the existing commits** (one on `main`, one on `infra/m0`)

```bash
git checkout main
git rebase --root --exec 'git commit --amend --no-edit -S'
git checkout infra/m0 && git rebase main
git log --show-signature --oneline main | head
```
Expected: each commit shows `Good "git" signature for radu@herinean.com with ED25519 key`.

- [x] **Step 3: Write `README.md`**

```markdown
# herinean.com

The personal site of Radu Herinean — writing for executives, boards and founders about which AI projects are worth funding. English and Romanian.

**Status:** M0 (edge foundation). Nothing is published yet.

This repository is the whole platform: a bespoke Go static-site generator, a ~80-line Cloudflare Worker, the zone configuration as code, and the CI that audits every commit against a public scorecard. The design is in [`docs/specs/2026-09-17-herinean-com-design.md`](docs/specs/2026-09-17-herinean-com-design.md); decisions are in `docs/adr/`; operations in [`RUNBOOK.md`](RUNBOOK.md).

Code: MIT. Writing: CC BY-NC-ND 4.0.
```

- [x] **Step 4: Commit README on `main`** (the last direct commit to `main` — protection lands in Task 2)

```bash
git checkout main
git add README.md
git commit -m "$(cat <<'MSG'
README

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
MSG
)"
git checkout infra/m0 && git rebase main
```

- [x] **Step 5: Wire the remotes and push `main`** (both repos already exist and are empty) — done: `gitea` (private) and `origin` (public) both wired; `main` on both.

```bash
. scripts/env.sh
git checkout main
git remote add origin git@github.com:raduherinean/herinean.com.git
git remote add gitea "$GITEA_REMOTE"
git push -u origin main
git push gitea main
git push -u gitea infra/m0                 # work branches go to gitea only
gh repo edit raduherinean/herinean.com --enable-squash-merge --enable-merge-commit=false --enable-rebase-merge=false --delete-branch-on-merge --enable-auto-merge --description "Personal site — bespoke Go generator, Cloudflare edge as code, CI-enforced scorecard" --homepage https://herinean.com
```
Expected: `https://github.com/raduherinean/herinean.com` shows the README, spec, brief, plan; commits show **Verified**.

---

### Task 2: Branch protection ruleset

**Files:** none in the repo (GitHub configuration; recorded in `RUNBOOK.md` in Task 10).

- [x] **Step 1: Create the ruleset** — done: ruleset `main` (id 23683194, active, no bypass actors) with deletion, non-fast-forward, required signatures and pull request. Merge methods changed on 2026-09-19 to `merge` + `squash`: build milestones merge with a merge commit so their signed history stays on `main`; pieces still squash as `Publish: <title>` (spec §8).

```bash
. scripts/env.sh
gh api -X POST "repos/raduherinean/herinean.com/rulesets" --input - <<'JSON'
{
  "name": "main",
  "target": "branch",
  "enforcement": "active",
  "conditions": { "ref_name": { "include": ["refs/heads/main"], "exclude": [] } },
  "rules": [
    { "type": "deletion" },
    { "type": "non_fast_forward" },
    { "type": "required_signatures" },
    { "type": "pull_request", "parameters": {
        "required_approving_review_count": 0,
        "dismiss_stale_reviews_on_push": false,
        "require_code_owner_review": false,
        "require_last_push_approval": false,
        "required_review_thread_resolution": false,
        "allowed_merge_methods": ["squash"] } }
  ]
}
JSON
```
(M2 adds `required_status_checks` once the CI job names exist.)

- [x] **Step 2: Verify the gate works** — done 2026-09-19: an unsigned empty commit pushed at `main` from a temporary ref was declined (`GH013`) naming both rules, pull request required and verified signatures; `main` unchanged.

```bash
git checkout main && git commit --allow-empty -m "protection probe" && git push origin main; echo "exit=$?"
git reset --hard origin/main
```
Expected: push **rejected** with a rules message; `exit` non-zero. The reset discards the probe.

---

### Task 3: OpenTofu scaffold and provider auth

**Files:**
- Create: `infra/versions.tf`, `infra/variables.tf`, `infra/zones.auto.tfvars`, `scripts/infra-backup.sh`
- Modify: `.gitignore` (already ignores `/infra/.terraform/` and `/infra/*.tfstate*`; keep `.terraform.lock.hcl` tracked)

**Interfaces:**
- Produces: variables `account_id`, `zones` (map short→id), `domains` (map short→fqdn), `redirect_zones` (set), `mx` (map host→priority), `dkim_txt`, `dmarc_policy`, `mta_sts_id`, `dmarc_cf_rua` — used by every later `.tf` file.

- [x] **Step 1: `infra/versions.tf`**

```hcl
terraform {
  required_version = ">= 1.8.0"
  required_providers {
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 5.0"
    }
  }
}

# Auth: CLOUDFLARE_API_TOKEN from the environment (scripts/env.sh). Never in files.
provider "cloudflare" {}
```

- [x] **Step 2: `infra/variables.tf`**

```hcl
variable "account_id" {
  type        = string
  description = "Cloudflare account id (TF_VAR_account_id from scripts/env.sh)"
}

variable "zones" {
  type        = map(string)
  description = "short name -> zone id"
}

variable "domains" {
  type = map(string)
  default = {
    com  = "herinean.com"
    ro   = "herinean.ro"
    net  = "herinean.net"
    info = "herinean.info"
  }
}

variable "redirect_zones" {
  type    = set(string)
  default = ["ro", "net", "info"]
}

variable "mx" {
  type        = map(number)
  description = "Google Workspace MX hosts -> priority, identical on all four zones"
  default     = { "smtp.google.com" = 1 }
}

variable "dmarc_cf_rua" {
  type        = string
  description = "Cloudflare DMARC Management report address already in _dmarc.herinean.com (mailto:<id>@dmarc-reports.cloudflare.net)"
}

variable "dkim_txt" {
  type        = string
  description = "google._domainkey TXT value (public key; not secret)"
}

variable "dmarc_policy" {
  type    = string
  default = "none" # flipped to "reject" in Task 6 step 8
}

variable "mta_sts_id" {
  type    = string
  default = "20260917T000000Z" # bump whenever the policy file changes
}
```

- [x] **Step 3: `infra/zones.auto.tfvars`** — zone ids from `m0.env`; `dmarc_cf_rua` from `dig +short TXT _dmarc.herinean.com` (the `mailto:…@dmarc-reports.cloudflare.net` part):

```hcl
zones = {
  com  = "<ZONE_COM>"
  ro   = "<ZONE_RO>"
  net  = "<ZONE_NET>"
  info = "<ZONE_INFO>"
}

dmarc_cf_rua = "mailto:<id>@dmarc-reports.cloudflare.net"

dkim_txt = "\"v=DKIM1; k=rsa; p=<as returned by the API, quotes included>\""  # the record already exists; scripts/env.sh carries the exact API value in DKIM_TXT
```

- [x] **Step 4: `scripts/infra-backup.sh`**

```bash
#!/usr/bin/env bash
# Copies tofu state out of the repo. State holds zone ids and public DNS data only, but losing it costs an hour of imports.
set -euo pipefail
cd "$(dirname "$0")/../infra"
dest="${HOME}/.config/herinean/tfstate"; mkdir -p "$dest"; chmod 700 "$dest"
[ -f terraform.tfstate ] || { echo "no state yet"; exit 0; }
cp terraform.tfstate "$dest/$(date -u +%Y%m%dT%H%M%SZ).json"
ls -1 "$dest" | tail -3
```

- [x] **Step 5: Init and prove auth with an empty plan**

```bash
. scripts/env.sh && cd infra && tofu init && tofu plan
```
Expected: `No changes. Your infrastructure matches the configuration.` (no resources yet). A 403/10000 error here means the token is wrong or lacks a permission — fix the token before continuing.

- [x] **Step 6: Commit**

```bash
cd .. && git add infra/versions.tf infra/variables.tf infra/zones.auto.tfvars infra/.terraform.lock.hcl scripts/infra-backup.sh
git commit -m "$(cat <<'MSG'
M0: OpenTofu scaffold for the four zones

Provider pinned to cloudflare ~> 5.0; token from the environment only.
Zone ids, MX set and DKIM public key are committed on purpose: none of
them is secret and the edge is meant to be readable.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
MSG
)"
git push gitea infra/m0
```

---

### Task 4: Zone settings — TLS, HTTP/3, HSTS, injectors off

**Files:**
- Create: `infra/settings.tf`
- Modify: `scripts/verify-edge.sh` (append section)

**Interfaces:**
- Consumes: `var.zones`.
- Produces: `cloudflare_zone_setting.s[...]`, `cloudflare_zone_setting.hsts[...]`, `cloudflare_bot_management.b[...]`.

- [x] **Step 1: Append the failing assertions** (before the summary lines in `scripts/verify-edge.sh`)

```bash
section "zone settings (API)"
cf() { curl -sS -H "Authorization: Bearer $CLOUDFLARE_API_TOKEN" "https://api.cloudflare.com/client/v4$1"; }
for z in $ZONE_COM $ZONE_RO $ZONE_NET $ZONE_INFO; do
  for pair in min_tls_version:1.3 tls_1_3:on http3:on 0rtt:off always_use_https:on ipv6:on email_obfuscation:off rocket_loader:off fonts:off speed_brain:off browser_check:on; do
    s=${pair%%:*}; want=${pair##*:}
    got=$(cf "/zones/$z/settings/$s" | jq -r '.result.value // empty')
    [ "$got" = "$want" ] && ok "$z $s=$got" || bad "$z $s expected $want got '${got:-<none>}'"
  done
  h=$(cf "/zones/$z/settings/security_header" | jq -c '.result.value.strict_transport_security | {enabled,max_age,include_subdomains,preload}')
  [ "$h" = '{"enabled":true,"max_age":63072000,"include_subdomains":true,"preload":true}' ] && ok "$z hsts $h" || bad "$z hsts got $h"
  bfm=$(cf "/zones/$z/bot_management" | jq -r '.result.fight_mode')
  [ "$bfm" = "false" ] && ok "$z bot fight mode off" || bad "$z bot fight mode = $bfm"
done
```

- [x] **Step 2: Run to see them fail**

Run: `. scripts/env.sh && scripts/verify-edge.sh`
Expected: `FAIL` lines for `min_tls_version` (default 1.0/1.2), `0rtt`, `email_obfuscation` (default on), hsts, and possibly others.

- [x] **Step 3: `infra/settings.tf`**

```hcl
locals {
  # Free-plan zone settings. Mirage/Polish are Pro+, Auto Minify no longer exists: nothing to disable there.
  zone_settings = {
    always_use_https         = "on"
    min_tls_version          = "1.3"
    tls_1_3                  = "on"
    http3                    = "on"
    "0rtt"                   = "off"
    ipv6                     = "on"
    brotli                   = "on"
    opportunistic_encryption = "on"
    ssl                      = "strict"
    browser_check            = "on"
    # injectors — every one of these adds a script or a resource to the page
    email_obfuscation = "off"
    rocket_loader     = "off"
    fonts             = "off"
    speed_brain       = "off"
  }
  zone_setting_pairs = merge([
    for z, id in var.zones : {
      for s, v in local.zone_settings : "${z}.${s}" => { zone_id = id, setting = s, value = v }
    }
  ]...)
}

resource "cloudflare_zone_setting" "s" {
  for_each   = local.zone_setting_pairs
  zone_id    = each.value.zone_id
  setting_id = each.value.setting
  value      = each.value.value
}

resource "cloudflare_zone_setting" "hsts" {
  for_each   = var.zones
  zone_id    = each.value
  setting_id = "security_header"
  value = {
    strict_transport_security = {
      enabled            = true
      max_age            = 63072000
      include_subdomains = true
      preload            = true
      nosniff            = true
    }
  }
}

resource "cloudflare_bot_management" "b" {
  for_each   = var.zones
  zone_id    = each.value
  fight_mode = false
}
```
Provider note: if `tofu validate` rejects the object under `value` for `security_header`, the provider version wants `value = jsonencode({ strict_transport_security = { … } })`; use that form and keep the same content. If `apply` answers `not entitled` for any setting on the Free plan, delete that key from `zone_settings` — the feature cannot be on.

- [x] **Step 4: Plan, review, apply**

```bash
cd infra && tofu validate && tofu plan -out m0-settings.plan
```
Expected: `Plan: 64 to add` (14 settings × 4 zones + 4 HSTS + 4 bot management; a few fewer if a setting had to be dropped). Read the plan. Then:
```bash
tofu apply m0-settings.plan && rm m0-settings.plan && ../scripts/infra-backup.sh && cd ..
```

- [x] **Step 5: Verify**

Run: `scripts/verify-edge.sh`
Expected: the *zone settings* section all `ok`. Run `tofu plan` again: `No changes`. (A perpetual diff on `security_header` means the provider normalises the object — match its form and re-plan.)

- [x] **Step 6: Commit**

```bash
git add infra/settings.tf scripts/verify-edge.sh
git commit -m "$(cat <<'MSG'
M0: zone settings — TLS 1.3 minimum, HTTP/3, HSTS preload, injectors off

Email obfuscation, Rocket Loader, Cloudflare Fonts, Speed Brain and Bot
Fight Mode all add scripts or resources to pages. Off on all four zones.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
MSG
)"
git push gitea infra/m0
```

---

### Task 5: Web DNS, CAA and redirects

**Files:**
- Create: `infra/dns-web.tf`, `infra/redirects.tf`
- Modify: `scripts/verify-edge.sh`

**Interfaces:**
- Consumes: `var.zones`, `var.domains`, `var.redirect_zones`.
- Produces: proxied placeholder records (`192.0.2.1` / `100::`) on `www.herinean.com` and on apex + `www` of the redirect zones; CAA on all zones; rulesets `cloudflare_ruleset.redirect[...]`, `cloudflare_ruleset.www`. The `herinean.com` apex record is **not** created here — the Workers custom domain creates it in Task 8.

- [x] **Step 1: Append failing assertions**

```bash
section "redirects and CAA"
for d in herinean.ro herinean.net herinean.info; do
  expect_location "https://$d/writing/x/?ref=li" "https://herinean.com/writing/x/?ref=li"
  expect_location "https://www.$d/" "https://herinean.com/"
  expect_location "http://$d/" "https://$d/"
  expect_header "https://$d/" strict-transport-security 'max-age=63072000; includeSubDomains; preload'
done
expect_location "https://www.herinean.com/a/?b=1" "https://herinean.com/a/?b=1"
for d in herinean.com herinean.ro herinean.net herinean.info; do
  expect_dig "$d" CAA 'issue "letsencrypt.org"'
  expect_dig "$d" CAA 'iodef "mailto:security@herinean.com"'
done
```

- [x] **Step 2: Run — expect failures** (`curl` errors on the redirect zones since nothing answers; CAA absent).

- [x] **Step 3: `infra/dns-web.tf`**

```hcl
locals {
  # CAs used by Cloudflare Universal SSL, per Cloudflare's docs; iodef reports to the security alias.
  caa = [
    { tag = "issue", value = "letsencrypt.org" },
    { tag = "issue", value = "pki.goog; cansignhttpexchanges=yes" },
    { tag = "issue", value = "ssl.com" },
    { tag = "issuewild", value = "letsencrypt.org" },
    { tag = "issuewild", value = "pki.goog; cansignhttpexchanges=yes" },
    { tag = "issuewild", value = "ssl.com" },
    { tag = "iodef", value = "mailto:security@herinean.com" },
  ]
  caa_pairs = merge([
    for z, id in var.zones : {
      for i, c in local.caa : "${z}.${i}" => { zone_id = id, name = var.domains[z], tag = c.tag, value = c.value }
    }
  ]...)

  # Hosts that exist only to redirect. Proxied placeholders per Cloudflare's own guidance.
  web_hosts = merge(
    { for z in var.redirect_zones : "${z}.apex" => { zone = z, name = var.domains[z] } },
    { for z in var.redirect_zones : "${z}.www" => { zone = z, name = "www.${var.domains[z]}" } },
    { "com.www" = { zone = "com", name = "www.herinean.com" } },
  )
}

resource "cloudflare_dns_record" "caa" {
  for_each = local.caa_pairs
  zone_id  = each.value.zone_id
  name     = each.value.name
  type     = "CAA"
  ttl      = 1
  data = {
    flags = 0
    tag   = each.value.tag
    value = each.value.value
  }
}

resource "cloudflare_dns_record" "web_a" {
  for_each = local.web_hosts
  zone_id  = var.zones[each.value.zone]
  name     = each.value.name
  type     = "A"
  content  = "192.0.2.1"
  proxied  = true
  ttl      = 1
}

resource "cloudflare_dns_record" "web_aaaa" {
  for_each = local.web_hosts
  zone_id  = var.zones[each.value.zone]
  name     = each.value.name
  type     = "AAAA"
  content  = "100::"
  proxied  = true
  ttl      = 1
}
```

- [x] **Step 4: `infra/redirects.tf`**

```hcl
# Everything on the redirect-only zones → https://herinean.com, path and query preserved.
resource "cloudflare_ruleset" "redirect" {
  for_each = var.redirect_zones
  zone_id  = var.zones[each.key]
  name     = "redirect to herinean.com"
  kind     = "zone"
  phase    = "http_request_dynamic_redirect"
  rules = [{
    description = "301 to herinean.com, keep path and query"
    expression  = "true"
    action      = "redirect"
    enabled     = true
    action_parameters = {
      from_value = {
        status_code           = 301
        preserve_query_string = true
        target_url = {
          expression = "concat(\"https://herinean.com\", http.request.uri.path)"
        }
      }
    }
  }]
}

# www.herinean.com → apex. Trailing-slash normalisation is the Worker's job (assets html_handling).
resource "cloudflare_ruleset" "www" {
  zone_id = var.zones["com"]
  name    = "www to apex"
  kind    = "zone"
  phase   = "http_request_dynamic_redirect"
  rules = [{
    description = "301 www to apex, keep path and query"
    expression  = "http.host eq \"www.herinean.com\""
    action      = "redirect"
    enabled     = true
    action_parameters = {
      from_value = {
        status_code           = 301
        preserve_query_string = true
        target_url = {
          expression = "concat(\"https://herinean.com\", http.request.uri.path)"
        }
      }
    }
  }]
}
```

- [x] **Step 5: Plan, review, apply, backup** — `cd infra && tofu validate && tofu plan -out m0-web.plan` (expected: 28 CAA + 7 A + 7 AAAA + 4 rulesets = 46 to add), `tofu apply m0-web.plan && rm m0-web.plan && ../scripts/infra-backup.sh && cd ..`.

- [x] **Step 6: Verify** — `scripts/verify-edge.sh`. Universal SSL for a new zone can take up to 15 minutes to issue; until then the HTTPS assertions on the redirect zones fail with a TLS error — wait, re-run. Expected: section all `ok`, including the HSTS header on the 301 responses (zone-level HSTS applies to Cloudflare-generated redirects).

- [x] **Step 7: Commit and push to gitea** — message: `M0: CAA, redirect placeholders and 301 rulesets for www and the three redirect zones` with the trailers.

---

### Task 6: Mail — hardened on all four Workspace domains

**Files:**
- Create: `infra/dns-mail.tf`, `scripts/infra-import-mail.sh`
- Modify: `scripts/verify-edge.sh`, `infra/zones.auto.tfvars` (DMARC flip at step 8)

**Interfaces:**
- Consumes: `var.zones`, `var.domains`, `var.redirect_zones`, `var.mx`, `var.dkim_txt`, `var.dmarc_policy`, `var.mta_sts_id`, `var.dmarc_cf_rua`.
- Produces: `cloudflare_dns_record.mx["<zone>.<host>"]`, `spf[zone]`, `dmarc[zone]`, `com_dkim`, `com_mta_sts`, `com_tlsrpt`.

- [x] **Step 1: Append failing assertions**

```bash
section "mail DNS"
for d in herinean.com herinean.ro herinean.net herinean.info; do
  expect_dig "$d" MX '^1 smtp\.google\.com\.'
  expect_txt "$d" 'v=spf1 include:_spf\.google\.com -all'
  expect_txt "_dmarc.$d" 'v=DMARC1; p=(none|reject); adkim=s; aspf=s; rua=mailto:'
done
expect_txt google._domainkey.herinean.com 'v=DKIM1; k=rsa; p='
expect_txt _dmarc.herinean.com 'dmarc-reports\.cloudflare\.net.*mailto:dmarc@herinean\.com'
expect_txt _mta-sts.herinean.com 'v=STSv1; id='
expect_txt _smtp._tls.herinean.com 'v=TLSRPTv1; rua=mailto:dmarc@herinean\.com'
expect_txt herinean.net 'google-site-verification='     # pre-existing, must survive untouched
```

- [x] **Step 2: Run — expect** SPF on `.ro/.net/.info` (currently `~all` or legacy), every `_dmarc` except `.com`'s shape, MTA-STS and TLS-RPT to fail; MX rows already pass.

- [x] **Step 3: `scripts/infra-import-mail.sh`** — prints `tofu import` commands for the records that already exist on all four zones, so tofu adopts them instead of colliding or duplicating.

```bash
#!/usr/bin/env bash
# Prints `tofu import` commands for pre-existing mail records on all four zones. Run from repo root before the first apply of dns-mail.tf.
set -euo pipefail
. "$(dirname "$0")/env.sh"
cf() { curl -sS -H "Authorization: Bearer $CLOUDFLARE_API_TOKEN" "https://api.cloudflare.com/client/v4$1"; }
for pair in com:$ZONE_COM ro:$ZONE_RO net:$ZONE_NET info:$ZONE_INFO; do
  z=${pair%%:*}; id=${pair##*:}
  cf "/zones/$id/dns_records?per_page=200" | jq -r --arg z "$z" --arg id "$id" '
    .result[] as $r
    | if ($r.type=="MX" and ($r.name|test("^herinean\\.")))
        then "cd infra && tofu import '"'"'cloudflare_dns_record.mx[\"\($z).\($r.content)\"]'"'"' \($id)/\($r.id) && cd .."
      elif ($r.type=="TXT" and ($r.name|test("^herinean\\.")) and ($r.content|test("v=spf1")))
        then "cd infra && tofu import '"'"'cloudflare_dns_record.spf[\"\($z)\"]'"'"' \($id)/\($r.id) && cd .."
      elif ($r.type=="TXT" and ($r.name|test("^_dmarc\\.")))
        then "cd infra && tofu import '"'"'cloudflare_dns_record.dmarc[\"\($z)\"]'"'"' \($id)/\($r.id) && cd .."
      elif ($r.type=="TXT" and $r.name=="google._domainkey.herinean.com")
        then "cd infra && tofu import cloudflare_dns_record.com_dkim \($id)/\($r.id) && cd .."
      else empty end'
done | sort -u
```

- [x] **Step 4: `infra/dns-mail.tf`**

```hcl
# All four domains are Google Workspace domains of one mailbox (.net primary, the rest aliases). Every one keeps its MX.
# Everything else is hardened: strict SPF, DMARC reject with strict alignment; DKIM, MTA-STS and TLS-RPT on .com (the sending domain).

locals {
  mx_pairs = merge([
    for z, id in var.zones : {
      for host, prio in var.mx : "${z}.${host}" => { zone_id = id, name = var.domains[z], host = host, prio = prio }
    }
  ]...)
  # .com reports to Cloudflare DMARC Management and to the mailbox; the others report to dmarc@ on themselves (same inbox via the alias).
  dmarc_rua = { for z, d in var.domains : z => (z == "com" ? "${var.dmarc_cf_rua},mailto:dmarc@${d}" : "mailto:dmarc@${d}") }
}

resource "cloudflare_dns_record" "mx" {
  for_each = local.mx_pairs
  zone_id  = each.value.zone_id
  name     = each.value.name
  type     = "MX"
  content  = each.value.host
  priority = each.value.prio
  ttl      = 1
}

resource "cloudflare_dns_record" "spf" {
  for_each = var.zones
  zone_id  = each.value
  name     = var.domains[each.key]
  type     = "TXT"
  content  = "\"v=spf1 include:_spf.google.com -all\""
  ttl      = 1
}

resource "cloudflare_dns_record" "dmarc" {
  for_each = var.zones
  zone_id  = each.value
  name     = "_dmarc.${var.domains[each.key]}"
  type     = "TXT"
  # .com goes through p=none → verified → reject (var.dmarc_policy); the alias domains are reject from the start: nothing sends as them.
  content  = "\"v=DMARC1; p=${each.key == "com" ? var.dmarc_policy : "reject"}; adkim=s; aspf=s; rua=${local.dmarc_rua[each.key]}\""
  ttl      = 1
}

resource "cloudflare_dns_record" "com_dkim" {
  zone_id = var.zones["com"]
  name    = "google._domainkey.${var.domains["com"]}"
  type    = "TXT"
  content = var.dkim_txt # already in the API's quoted form
  ttl     = 1
}

resource "cloudflare_dns_record" "com_mta_sts" {
  zone_id = var.zones["com"]
  name    = "_mta-sts.${var.domains["com"]}"
  type    = "TXT"
  content = "\"v=STSv1; id=${var.mta_sts_id}\""
  ttl     = 1
}

resource "cloudflare_dns_record" "com_tlsrpt" {
  zone_id = var.zones["com"]
  name    = "_smtp._tls.${var.domains["com"]}"
  type    = "TXT"
  content = "\"v=TLSRPTv1; rua=mailto:dmarc@herinean.com\""
  ttl     = 1
}
```
Provider note on TXT quoting: the API returns TXT `content` quoted (`"\"…\""`), and provider 5.x expects the same. If a `tofu plan` right after `apply` shows a perpetual diff on a TXT record, switch that record to the other form; keep all TXT records consistent.

- [x] **Step 5: Import the pre-existing records, then plan**

```bash
scripts/infra-import-mail.sh            # read the commands it prints: 4 MX, 4 SPF, 1 DMARC (.com), 1 DKIM expected
scripts/infra-import-mail.sh | bash     # run them
cd infra && tofu plan -out m0-mail.plan && cd ..
```
Expected plan: 4 MX **no change**; `.com` SPF no change, the other three SPF **update in-place** (→ `-all`); `.com` DMARC update in-place (adds `adkim/aspf` and `dmarc@`; policy stays `none`); DKIM no change; **adds**: 3 DMARC records, MTA-STS, TLS-RPT. **Zero `destroy`.** If any destroy appears, stop and fix the import — never apply.

- [x] **Step 6: Apply, backup, verify**

```bash
cd infra && tofu apply m0-mail.plan && rm m0-mail.plan && ../scripts/infra-backup.sh && cd .. && scripts/verify-edge.sh
```
Expected: *mail DNS* section all `ok` (`.com` DMARC matches `p=none` for now).

- [x] **Step 7: Prove the chain on `.com`** (operator): send a message from `radu@herinean.com` to any Gmail address, open it, ⋮ → **Show original**. Expected: `SPF: PASS`, `DKIM: 'PASS' with domain herinean.com`, `DMARC: 'PASS'`. If DKIM shows `gappssmtp.com` or `herinean.net`, Google Admin → Gmail → Authenticate email → herinean.com → **Start authentication** first, wait an hour, resend.

- [x] **Step 8: Flip `.com` DMARC to reject**

Edit `infra/zones.auto.tfvars`: add `dmarc_policy = "reject"`. Then `cd infra && tofu plan -out m0-dmarc.plan` (expected: 1 update in-place), `tofu apply m0-dmarc.plan && rm m0-dmarc.plan && ../scripts/infra-backup.sh && cd .. && scripts/verify-edge.sh`. Expected: `_dmarc.herinean.com` now `p=reject`.

- [x] **Step 9: Commit and push to gitea** — two commits: `M0: mail — strict SPF and DMARC on all four domains; MTA-STS and TLS-RPT on .com` (after step 6) and `M0: DMARC p=reject on .com after SPF/DKIM verified` (after step 8), both with the trailers. Done: `da9e752` and `4217aea` (the `.com` reject flip followed a signed test message on 2026-09-18).

---

### Task 7: DNSSEC

**Files:**
- Create: `infra/dnssec.tf`
- Modify: `scripts/verify-edge.sh`

**Interfaces:**
- Consumes: `var.zones`.
- Produces: `cloudflare_zone_dnssec.d[z]`, output `ds_records`.

- [x] **Step 1: Append failing assertions**

```bash
section "DNSSEC"
for d in herinean.com herinean.ro herinean.net herinean.info; do
  expect_dig "$d" DS '^[0-9]+ 13 2 [0-9A-F]+'
  ad=$(dig +dnssec +noall +comments A "$d" @1.1.1.1 | grep -o 'flags:[^;]*' )
  printf '%s' "$ad" | grep -q ' ad' && ok "$d AD flag set" || bad "$d not validated (flags: $ad)"
done
```

- [x] **Step 2: `infra/dnssec.tf`**

```hcl
resource "cloudflare_zone_dnssec" "d" {
  for_each = var.zones
  zone_id  = each.value
  status   = "active"
}

output "ds_records" {
  description = "Paste each DS record at that domain's registrar. Format: keytag algorithm digest_type digest"
  value       = { for z, r in cloudflare_zone_dnssec.d : var.domains[z] => r.ds }
}
```

- [x] **Step 3: Apply and read the DS records**

```bash
cd infra && tofu plan -out m0-dnssec.plan && tofu apply m0-dnssec.plan && rm m0-dnssec.plan && tofu output ds_records && ../scripts/infra-backup.sh && cd ..
```

- [x] **Step 4: Registrar side** (operator): at each registrar, DNSSEC → add DS record → paste the four fields (key tag, algorithm `13`, digest type `2`, digest). `.ro` at the RoTLD registrar supports it; `.com/.net/.info` at theirs. Propagation: minutes to a few hours.

- [x] **Step 5: Verify** — `scripts/verify-edge.sh`; expected DS present and `AD` flag on all four. Cross-check one with `https://dnsviz.net/d/herinean.com/dnssec/` (all green).

- [x] **Step 6: Commit and push to gitea** — `M0: DNSSEC on all four zones` with the trailers. Done: `c0a39b8`.

---

### Task 8: Placeholder Worker on the apex with the production header set

**Files:**
- Create: `wrangler.toml`, `worker/index.js`, `scripts/deploy-placeholder.sh`, `infra/placeholder/index.html`, `infra/placeholder/404.html`, `infra/placeholder/_headers`, `infra/placeholder/robots.txt`, `infra/placeholder/.well-known/security.txt`, `infra/placeholder/.well-known/mta-sts.txt`
- Modify: `scripts/verify-edge.sh`

**Interfaces:**
- Produces: Worker `herinean-com` with assets binding `ASSETS`; custom domains `herinean.com` and `mta-sts.herinean.com`; the header set that M1's `edge` package must reproduce byte-for-byte (plus `style-src`).

- [x] **Step 1: Append failing assertions**

```bash
section "apex headers and routing"
U=https://herinean.com
expect_status "$U/" 200
expect_header "$U/" content-security-policy "^default-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'$"
expect_header "$U/" strict-transport-security '^max-age=63072000; includeSubDomains; preload$'
expect_header "$U/" x-content-type-options '^nosniff$'
expect_header "$U/" referrer-policy '^strict-origin-when-cross-origin$'
expect_header "$U/" permissions-policy 'camera=\(\)'
expect_header "$U/" cross-origin-opener-policy '^same-origin$'
expect_header "$U/" cross-origin-resource-policy '^same-origin$'
expect_header "$U/" x-frame-options '^DENY$'
expect_header "$U/" x-robots-tag '^noindex$'                     # placeholder only; M1 removes it on production
expect_no_body "$U/" '<script|cdn-cgi|style='
expect_status "$U/nope/" 404
expect_location "$U/404" "$U/404/"                                # force-trailing-slash (404.html is a real asset)
expect_location "http://herinean.com/" "$U/"
expect_status "$U/.well-known/security.txt" 200
expect_header "$U/.well-known/security.txt" content-type '^text/plain'
expect_status "https://mta-sts.herinean.com/.well-known/mta-sts.txt" 200
expect_status "https://mta-sts.herinean.com/" 404
h3=$(curl -sS --http3-only -o /dev/null -w '%{http_version}' --max-time 15 "$U/" 2>/dev/null || true)
[ "$h3" = "3" ] && ok "HTTP/3 negotiated" || bad "HTTP/3 not negotiated (got '${h3:-error}'; curl needs http3 support — see RUNBOOK)"
tls12=$(curl -sS --tls-max 1.2 -o /dev/null -w '%{http_code}' --max-time 15 "$U/" 2>&1 || true)
printf '%s' "$tls12" | grep -Eq 'alert|handshake|error|000' && ok "TLS 1.2 refused" || bad "TLS 1.2 accepted ($tls12)"
```

- [x] **Step 2: Run — expect** everything in the section to fail (apex serves a Cloudflare error today).

- [x] **Step 3: `worker/index.js`** — M0 version; M1 adds analytics and the colophon rewrite to this same file.

```js
// herinean.com edge function. Zero client-side JS is the site's rule; this runs at the edge only.
// Hosts: production apex; mta-sts (one file, everything else 404); anything else is a preview (noindex, no analytics).

const PROD_HOST = "herinean.com";
const MTA_STS_HOST = "mta-sts.herinean.com";

export default {
  async fetch(request, env) {
    const url = new URL(request.url);

    if (url.host === MTA_STS_HOST) {
      if (url.pathname === "/.well-known/mta-sts.txt") return env.ASSETS.fetch(request);
      return new Response("Not found\n", { status: 404, headers: { "content-type": "text/plain; charset=utf-8" } });
    }

    if (url.host !== PROD_HOST) {
      if (url.pathname === "/robots.txt") {
        return new Response("User-agent: *\nDisallow: /\n", {
          status: 200,
          headers: { "content-type": "text/plain; charset=utf-8", "x-robots-tag": "noindex, nofollow" },
        });
      }
      const res = await env.ASSETS.fetch(request);
      const headers = new Headers(res.headers);
      headers.set("x-robots-tag", "noindex, nofollow");
      return new Response(res.body, { status: res.status, statusText: res.statusText, headers });
    }

    return env.ASSETS.fetch(request);
  },
};
```

- [x] **Step 4: `wrangler.toml`**

```toml
name = "herinean-com"
main = "worker/index.js"
compatibility_date = "2026-09-01"
workers_dev = false
preview_urls = true

routes = [
  { pattern = "herinean.com", custom_domain = true },
  { pattern = "mta-sts.herinean.com", custom_domain = true },
]

[assets]
directory = "./dist"
binding = "ASSETS"
html_handling = "force-trailing-slash"
not_found_handling = "404-page"
# The Worker runs only for page routes. Images, OG cards, feeds and machine files come straight from the asset layer.
run_worker_first = ["/*", "!/img/*", "!/og/*", "!/feed.*", "!/sitemap.xml", "!/favicon*", "!/apple-touch-icon*"]
```

- [x] **Step 5: Placeholder assets**

`infra/placeholder/index.html`:
```html
<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex">
<title>Radu Herinean</title>
</head>
<body>
<p>herinean.com — coming soon.</p>
</body>
</html>
```

`infra/placeholder/404.html`:
```html
<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex">
<title>Not found</title>
</head>
<body>
<p>Page not found. <a href="/">Home</a></p>
</body>
</html>
```

`infra/placeholder/_headers`:
```
/*
  Content-Security-Policy: default-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'
  Strict-Transport-Security: max-age=63072000; includeSubDomains; preload
  X-Content-Type-Options: nosniff
  Referrer-Policy: strict-origin-when-cross-origin
  Permissions-Policy: accelerometer=(), camera=(), geolocation=(), gyroscope=(), magnetometer=(), microphone=(), payment=(), usb=(), interest-cohort=()
  Cross-Origin-Opener-Policy: same-origin
  Cross-Origin-Resource-Policy: same-origin
  X-Frame-Options: DENY
  X-Robots-Tag: noindex
  Cache-Control: public, max-age=0, must-revalidate

/.well-known/security.txt
  Content-Type: text/plain; charset=utf-8

/.well-known/mta-sts.txt
  Content-Type: text/plain; charset=utf-8
```

`infra/placeholder/robots.txt`:
```
User-agent: *
Disallow: /
```

`infra/placeholder/.well-known/security.txt`:
```
Contact: mailto:security@herinean.com
Expires: 2027-09-01T00:00:00.000Z
Preferred-Languages: en, ro
Canonical: https://herinean.com/.well-known/security.txt
```

`infra/placeholder/.well-known/mta-sts.txt` (covers both Workspace MX shapes; `testing` until TLS-RPT reports look clean, then `enforce` and bump `mta_sts_id`):
```
version: STSv1
mode: testing
mx: smtp.google.com
mx: aspmx.l.google.com
mx: *.aspmx.l.google.com
max_age: 86400
```

- [x] **Step 6: `scripts/deploy-placeholder.sh`**

```bash
#!/usr/bin/env bash
# Deploys the M0 placeholder: dist/ is rebuilt from infra/placeholder/ and pushed with wrangler.
# M1 replaces this with `site build && wrangler deploy` from CI.
set -euo pipefail
cd "$(dirname "$0")/.."
. scripts/env.sh
rm -rf dist && cp -r infra/placeholder dist
npx --yes wrangler@4 deploy
```

- [x] **Step 7: Local check first**

```bash
rm -rf dist && cp -r infra/placeholder dist && npx --yes wrangler@4 dev --port 8787 &
sleep 8
curl -sSI http://localhost:8787/ | grep -Ei 'content-security-policy|x-frame-options|x-robots'
curl -sS -o /dev/null -w '%{http_code}\n' http://localhost:8787/nope/
kill %1
```
Expected: the CSP, `X-Frame-Options: DENY`, `X-Robots-Tag: noindex` lines; `404`.

- [x] **Step 8: Deploy**

Run: `scripts/deploy-placeholder.sh`
Expected: wrangler prints the custom domains `herinean.com` and `mta-sts.herinean.com` and creates their DNS records. If it refuses because a DNS record for the apex already exists, delete that record in the dashboard (it is not tofu-managed) and re-run.

- [x] **Step 9: Verify** — `scripts/verify-edge.sh`. Expected: *apex headers and routing* all `ok`. If the HTTP/3 line fails only because local `curl` lacks HTTP/3, confirm with `https://http3check.net/?host=herinean.com` and note the curl limitation in the RUNBOOK. Then `cd infra && tofu plan && cd ..`: **No changes** (the custom-domain records live outside tofu — confirm tofu doesn't try to touch them).

- [x] **Step 10: Preview URL behaves** — `npx --yes wrangler@4 versions upload` prints a `*.workers.dev` preview URL; `curl -sSI <url>/ | grep -i x-robots-tag` → `noindex, nofollow`; `curl -sS <url>/robots.txt` → `Disallow: /`.

- [x] **Step 11: Commit and push to gitea** — `M0: placeholder Worker on the apex with the production header set` with the trailers. (`dist/` is gitignored.)

---

### Task 9: TLS spike, external audits, HSTS preload submission, ADR-0011

**Files:**
- Create: `docs/adr/0011-tls13-minimum.md`

- [x] **Step 1: External audits** (operator, browser): securityheaders.com → `https://securityheaders.com/?q=herinean.com&followRedirects=on` (expect **A+**); SSL Labs → `https://www.ssllabs.com/ssltest/analyze.html?d=herinean.com` (expect **A+**, protocols: TLS 1.3 only); internet.nl → `https://internet.nl/site/herinean.com/` (record the percentage and every non-green item); Observatory → `https://developer.mozilla.org/en-US/observatory/analyze?host=herinean.com` (expect **A+**). Save the four result URLs.

- [x] **Step 2: Decide TLS.** If internet.nl is 100% with TLS 1.3 only: keep. If it is below 100% *because of* TLS (not because of the placeholder's `noindex` or a missing `security.txt` field), set `min_tls_version = "1.2"` in `infra/settings.tf`, apply, re-test, and record which cipher items changed. Either way the decision and the evidence go in the ADR.

- [x] **Step 3: `docs/adr/0011-tls13-minimum.md`**

```markdown
# ADR-0011 — Minimum TLS version

**Status:** accepted 2026-09-__ · **Spec:** §6.1, §3 row 10/11

## Context
The scorecard requires SSL Labs A+ and internet.nl 100%. A TLS 1.3-only edge is the cleanest configuration, but some corporate middleboxes still speak only 1.2, and internet.nl scores 1.2 cipher lists.

## Decision
Minimum TLS <1.3 | 1.2>, because: <internet.nl result with 1.3-only: __%; with 1.2: __%; SSL Labs: __>. Result URLs: <…>.

## Consequences
- Readers on clients without TLS 1.3 <cannot connect | connect over 1.2 with modern ciphers>.
- Re-verified weekly by `verify.yml`; revisit if a real reader reports being blocked.
```

- [ ] **Step 4: HSTS preload** (open; the placeholder has served the header since 2026-09-17, so the earliest submission is 2026-09-25) — after the four zones have served the header for at least a week and `scripts/verify-edge.sh` is green: submit each domain at `https://hstspreload.org/` (status: pending → preloaded over the following weeks). Record the submission dates in the RUNBOOK. This is irreversible in practice; the spec accepts that.

- [x] **Step 5: Commit and push to gitea** — `M0: ADR-0011 minimum TLS version with audit evidence` with the trailers.

---

### Task 10: RUNBOOK, remaining ADRs, PR to `main`

**Files:**
- Create: `RUNBOOK.md`, `docs/adr/0002-workers-static-assets-over-origin.md`, `docs/adr/0005-workers-paid.md`, `docs/adr/0012-infrastructure-as-code.md`

- [x] **Step 1: `RUNBOOK.md`**

```markdown
# RUNBOOK — herinean.com

Everything operational lives here. If it isn't here, it isn't a supported operation.

## Accounts and secrets
- Cloudflare: one account, hardware-key 2FA. Zones: herinean.com (content), herinean.ro / .net / .info (web redirect-only). All four are Google Workspace domains of one mailbox and keep their MX. Workers Paid.
- Tokens: `herinean-infra` (operator; 7-day TTL; recreate with the same permissions when it expires) in `~/.config/herinean/cf-infra.token` (mode 600). `herinean-ci` (GitHub Actions; Workers Scripts Edit, Workers KV Storage Edit, Account Analytics Read, Account Settings Read) as the repo secret `CLOUDFLARE_API_TOKEN`. Rotate: create new → update file/secret → delete old.
- Operator inputs: `~/.config/herinean/m0.env` (see `docs/plans/2026-09-17-m0-edge-foundation.md`).
- GitHub: `main` is protected by the ruleset "main" (PR, signed commits, squash only, no force-push, no deletion). Commits are SSH-signed with `~/.ssh/herinean_signing`; the public key is registered as a Signing Key.
- Remotes: `origin` = GitHub (public; `main` + publish PRs), `gitea` = private (all work branches).

## Edge as code (OpenTofu, `infra/`)
1. `. scripts/env.sh`
2. `cd infra && tofu plan -out x.plan` — read every line.
3. `tofu apply x.plan && rm x.plan && ../scripts/infra-backup.sh`
State is local (`infra/terraform.tfstate`, gitignored); backups in `~/.config/herinean/tfstate/`. Lost state: `tofu import` each resource (ids from the API); ~1 hour.
Not in tofu: the apex `herinean.com` and `mta-sts.herinean.com` DNS records (created by the Workers custom domains), zones themselves, DS records at registrars.

## DNSSEC
`cd infra && tofu output ds_records` → paste at each registrar. Verify: `dig +dnssec A herinean.com @1.1.1.1` shows the `ad` flag; https://dnsviz.net/d/herinean.com/dnssec/.

## Mail
All four domains: Workspace MX, SPF `include:_spf.google.com -all`, DMARC `p=reject; adkim=s; aspf=s` (reports to dmarc@ on each domain; .com also to Cloudflare DMARC Management). herinean.com additionally: DKIM selector `google` (2048), MTA-STS (`mode: testing` → `enforce`: edit `infra/placeholder/.well-known/mta-sts.txt` or M1's `static/`, bump `mta_sts_id` in `infra/zones.auto.tfvars`, apply, deploy), TLS-RPT to dmarc@. Change the MX set only via `var.mx`.

## Worker
- M0 placeholder: `scripts/deploy-placeholder.sh`. M1+: CI deploys on merge to `main`.
- Roll back: `npx wrangler@4 rollback` (previous version) — or `npx wrangler@4 deployments list` then `rollback <id>`.
- Preview: `npx wrangler@4 versions upload` prints a noindex preview URL.
- Custom domains are declared in `wrangler.toml`; wrangler creates their DNS records.

## Verification
`scripts/verify-edge.sh` — the edge test suite (settings, redirects, CAA, mail DNS, DNSSEC, apex headers, HTTP/3, TLS). Run after any infra change. Local `curl` needs HTTP/3 support for that one check; otherwise use https://http3check.net/.
External: securityheaders.com, SSL Labs, internet.nl, Mozilla Observatory, dnsviz. HSTS preload submitted: <dates>.

## Incidents
- Cloudflare-wide outage: nothing to do; the site has no origin. Site recovery elsewhere: `site build` → any static host with the `_headers` equivalents; DNS TTL is 1 (auto).
- Under attack: "Under Attack" mode serves a JS challenge and breaks the zero-JS promise for the duration. Prefer a WAF rate-limit rule first.
- Billing failure on Workers Paid drops the account to Free quotas (100k req/day). Billing alert is set; fix the card.
- Domains: auto-renew and registrar lock on all four; weekly job warns at 60 days to expiry (M2).
```

- [x] **Step 2: ADRs** (short; the spec §12 carries the full reasoning)

`docs/adr/0002-workers-static-assets-over-origin.md`:
```markdown
# ADR-0002 — Cloudflare Workers static assets, no origin server
**Status:** accepted 2026-09-17 · **Spec:** §6
## Context
A static site of a few dozen pages for a global audience, judged on TTFB and on what it costs to run.
## Decision
Serve `dist/` from Cloudflare Workers static assets with a ~80-line edge function; no origin, no Coolify.
## Consequences
Nothing to patch or back up; global edge; `_headers`; one vendor (see RUNBOOK "Cloudflare-wide outage"). Coolify's only possible future role is a newsletter host.
```

`docs/adr/0005-workers-paid.md`:
```markdown
# ADR-0005 — Workers Paid instead of the free tier
**Status:** accepted 2026-09-17 · **Spec:** §6.1, §6.5
## Context
The free tier hard-fails at 100k Worker requests/day; one good LinkedIn day plus bots would serve errors.
## Decision
Workers Paid ($5/month) from day one; billing alert set.
## Consequences
The site cannot fail on quota. Cost line: $5/month beyond domains and Workspace.
```

`docs/adr/0012-infrastructure-as-code.md`:
```markdown
# ADR-0012 — Zone configuration as code (OpenTofu); Worker via wrangler
**Status:** accepted 2026-09-17 · **Spec:** §6.4
## Context
Four zones, TLS settings, mail records, redirects and injector switches configured by hand are unreviewable and drift silently.
## Decision
`infra/` (OpenTofu, `cloudflare ~> 5.0`) owns zone settings, DNS, mail, DNSSEC and redirect rulesets, applied from the operator's machine with a short-lived token; local state, backed up. wrangler owns the Worker, assets and custom domains. `scripts/verify-edge.sh` checks observable behaviour.
## Consequences
The entire edge is readable in the public repo. Zone ids, account id and the DKIM public key are committed (not secrets). Drift shows up in `tofu plan` and in the weekly verification.
```

- [x] **Step 3: Commit, push to both remotes, open the PR, merge** — done: PR #1 merged 2026-09-19 as merge commit `b3adeb0`.

```bash
git add RUNBOOK.md docs/adr/
git commit -m "$(cat <<'MSG'
M0: RUNBOOK and ADRs 0002, 0005, 0012

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
MSG
)"
git push gitea infra/m0
git push -u origin infra/m0
gh pr create --title "M0: edge foundation — zones as code, mail hardening, placeholder Worker" --body "$(cat <<'BODY'
Four zones configured as OpenTofu (`infra/`): TLS 1.3 minimum, HTTP/3, HSTS preload, every script-injecting feature off, CAA, DNSSEC, strict SPF and DMARC on all four Workspace domains, DKIM/MTA-STS/TLS-RPT on herinean.com, 301 rulesets for www and the three redirect zones. Placeholder Worker on the apex serving the production header set. `scripts/verify-edge.sh` is green; external audits recorded in ADR-0011.

Spec: docs/specs/2026-09-17-herinean-com-design.md §6, §11.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

https://claude.ai/code/session_01CyBKVKXNbCCfWDPgYiMTKf
BODY
)"
gh pr merge --squash --delete-branch
git checkout main && git pull origin main && git push gitea main
```
Expected: one squash commit `M0: edge foundation …` on `main`, **Verified**; `scripts/verify-edge.sh` still green from `main`.

---

## Self-review

**Spec coverage (M0 scope):** §6.1 zones/TLS/mail/injectors → Tasks 4–6; §6.1 `workers.dev`/preview → Task 8; §6.2 headers → Task 8 (all but `style-src`, which needs M1's CSS); §6.4 IaC → Tasks 3–7; §6.5 cost → operator step + ADR-0005; §3 rows 9, 10, 12, 13, 15, 16 (security.txt, robots), 17 (cache header present) → `verify-edge.sh` sections; §11 M0 dashboard list → Task 0 step 1 and Task 1; repo guardrails (§7) minus CI-dependent checks → Tasks 1–2; HSTS preload and TLS spike → Task 9. Deferred to M1/M2 on purpose: CI token, Search Console, uptime monitor, weekly RDAP/expiry, Dependabot/CodeQL/Scorecard, `style-src` hash, removal of `X-Robots-Tag: noindex` on production.

**Placeholder scan:** the only bracketed values are operator inputs (zone ids, owner, Gitea URL, DKIM value) and ADR-0011's evidence fields, which are filled by the spike's results.

**Type consistency:** `var.zones`/`var.domains`/`var.redirect_zones`/`var.mx`/`var.dkim_txt`/`var.dmarc_policy`/`var.mta_sts_id`/`var.dmarc_cf_rua` are declared in Task 3 and used with those exact names in Tasks 4–7; resource names referenced by the import script (`mx["<zone>.<host>"]`, `spf["<zone>"]`, `dmarc["<zone>"]`, `com_dkim`) match `dns-mail.tf`; the `ASSETS` binding name in `wrangler.toml` matches `env.ASSETS` in the Worker; helper names in `verify-edge.sh` (`expect_header`, `expect_status`, `expect_location`, `expect_txt`, `expect_no_body`, `expect_dig`, `section`, `ok`, `bad`) are defined once in Task 0 and used as such.
