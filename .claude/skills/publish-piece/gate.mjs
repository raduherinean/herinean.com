// The publish gate: what /publish-piece refuses before anything reaches origin. Node standard library only.
// From the repository root:
//   node .claude/skills/publish-piece/gate.mjs path   <piece.md>       the piece's ledger path
//   node .claude/skills/publish-piece/gate.mjs hash   <piece.md>       sha256:<hex> of the piece file
//   node .claude/skills/publish-piece/gate.mjs scope  [--base <ref>]   the branch changes one key's content only
//   node .claude/skills/publish-piece/gate.mjs ledger <piece.md>...    every mark the author owes is there
import { createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { existsSync, readFileSync } from 'node:fs';
import { join, resolve } from 'node:path';
import { pathToFileURL } from 'node:url';

export const sha256 = (bytes) => 'sha256:' + createHash('sha256').update(bytes).digest('hex');

// pieceKey reads `key:` from a piece's front matter; '' when absent.
export function pieceKey(src) {
  const fm = /^---\r?\n([\s\S]*?)\r?\n---\r?\n/.exec(String(src));
  const k = fm && /^key:[ \t]*["']?([^"'\s]+)["']?[ \t]*\r?$/m.exec(fm[1]);
  return k ? k[1] : '';
}

// pieceLang reads the language from content/<lang>/<slug>.md; '' when the path is not a piece.
export function pieceLang(path) {
  const m = /(?:^|\/)content\/(en|ro)\/[a-z0-9-]+\.md$/.exec(path);
  return m ? m[1] : '';
}

// splitRow splits one Markdown table row into trimmed cells; `\|` is a literal pipe inside a cell.
export function splitRow(line) {
  let s = line.trim();
  if (s.startsWith('|')) s = s.slice(1);
  const cells = [];
  let cur = '';
  for (let i = 0; i < s.length; i++) {
    if (s[i] === '\\' && s[i + 1] === '|') { cur += '|'; i++; continue; }
    if (s[i] === '|') { cells.push(cur.trim()); cur = ''; continue; }
    cur += s[i];
  }
  if (cur.trim() !== '') cells.push(cur.trim());
  return cells;
}

// GateTableError marks a `## <heading>` section the gate found but could not read as a table. The gate refuses
// rather than guess, so it does not silently drop or misread: a blank or prose line between rows, a bullet list
// standing in for a table, a lone data row with no header, a GFM row written without a leading `|` (outer pipes
// are optional in GFM, but the gate is not a renderer), a row hidden behind `>` or `- `, a fake table pasted above
// the real header, a lazy line with no pipe sitting directly under a row, or a duplicate heading disguised by
// extra whitespace, decoration (`## **Claims**`) or extension (`## Claims (continued)`) of its name — each of
// these would let a mark the author owes go unseen.
// checkLedger turns this into a refusal message; it is not a "no section" (that stays `table() === null`, since
// the ledger template always writes header + separator, even with zero rows).
class GateTableError extends Error {}

// headingName reads a `##`-level ATX heading's raw name (the part after `##` and before an optional trailing
// `##`), or null when the line is not such a heading. Whitespace variants (`##  Claims`, `##\tClaims ##`) are all
// still headings; the name itself is normalised for comparison in headingLetters, not here.
function headingName(line) {
  const m = /^##[ \t]+(.*?)(?:[ \t]+#+)?[ \t]*$/.exec(line.trim());
  return m ? m[1] : null;
}

// headingLetters strips everything but letters and lower-cases what remains, so `**Claims**`, `Claims (continued)`
// and `Claims` all compare as text starting with `claims` — a heading can't dodge duplicate detection by
// decorating or extending its name.
const headingLetters = (name) => name.replace(/[^a-zA-Z]/g, '').toLowerCase();

// hasUnescapedPipe reports whether `s` contains a `|` not preceded by the same `\|` escape splitRow honours —
// i.e. whether it looks like a table row GFM would render even without a leading `|`.
function hasUnescapedPipe(s) {
  for (let i = 0; i < s.length; i++) {
    if (s[i] === '\\' && s[i + 1] === '|') { i++; continue; }
    if (s[i] === '|') return true;
  }
  return false;
}

// table returns the rows of the first Markdown table under `## <heading>` (keyed by lower-cased header), or null
// when the heading is absent. The section runs from just after the (single, exact) `## <heading>` to the next
// `##`-level heading of any name, or EOF.
//
// Before anything else, every non-blank line in that whole range (not just from the header down — a bypass could
// be pasted in above the real header) must either start with `|`, or be safe prose: no unescaped `|` (GFM would
// still render `2 | claim | ...` as a row even with no leading `|`), and not sitting directly under a `|` line
// with no blank line between (GFM's lazy continuation reads that as part of the row above too). Either violation
// throws GateTableError, naming the exact line.
//
// Only once that holds does table() look for the header: the section's first `|` line, which must be followed,
// on the very next source line, by a separator row — otherwise, or if the section has no `|` line at all, this
// throws too. A later `|` line that repeats the header or is itself a separator is skipped; anything else becomes
// a row, even a second, differently-shaped table — it then fails whatever value or column check applies to it.
//
// `## <heading>` matching by letters-only name (see headingLetters) more than once — including a heading whose
// letters-only name merely starts with the target's, such as `## Claims (continued)` under `## Claims` — also
// throws: a duplicate can't be silently picked between. The first, real heading must still match exactly.
export function table(md, heading) {
  const lines = md.split(/\r?\n/);
  const target = heading.toLowerCase();
  const names = lines.map((l) => headingName(l));
  let start = -1, exact = 0, loose = 0;
  for (let i = 0; i < lines.length; i++) {
    if (names[i] === null) continue;
    const letters = headingLetters(names[i]);
    if (letters === target) { exact++; if (start < 0) start = i; }
    else if (letters.startsWith(target)) loose++;
  }
  if (exact === 0) return null;
  if (exact > 1 || loose > 0) throw new GateTableError(`more than one ## ${heading} heading`);

  let end = lines.length;
  for (let i = start + 1; i < end; i++) if (names[i] !== null) { end = i; break; }

  let afterPipe = false;
  for (let i = start + 1; i < end; i++) {
    const l = lines[i].trim();
    if (l === '') { afterPipe = false; continue; }
    if (l.startsWith('|')) { afterPipe = true; continue; }
    if (hasUnescapedPipe(l)) {
      throw new GateTableError(`## ${heading} line ${i + 1}: a line with | must be a table row starting with |; move prose out of this section or write the pipe as \\|`);
    }
    if (afterPipe) {
      throw new GateTableError(`## ${heading} line ${i + 1}: directly under the table, a line is read as a row; leave a blank line or write it as | … |`);
    }
    afterPipe = false;
  }

  let hIdx = -1;
  for (let i = start + 1; i < end; i++) if (lines[i].trim().startsWith('|')) { hIdx = i; break; }
  const isSep = (cells) => cells.length > 0 && cells.every((c) => /^:?-{3,}:?$/.test(c));
  if (hIdx < 0 || hIdx + 1 >= end || !lines[hIdx + 1].trim().startsWith('|') || !isSep(splitRow(lines[hIdx + 1]))) {
    throw new GateTableError(`## ${heading} has no table header`);
  }
  const headerCells = splitRow(lines[hIdx]);
  const header = headerCells.map((c) => c.toLowerCase());
  const rows = [];
  for (let i = hIdx + 2; i < end; i++) {
    const l = lines[i].trim();
    if (!l.startsWith('|')) continue; // blank, or safe prose already cleared above.
    const cells = splitRow(l);
    if (isSep(cells)) continue;
    const norm = cells.map((c) => c.toLowerCase());
    if (norm.length === header.length && norm.every((c, j) => c === header[j])) continue;
    rows.push(Object.fromEntries(header.map((h, j) => [h, cells[j] ?? ''])));
  }
  Object.defineProperty(rows, 'header', { value: header, enumerable: false });
  return rows;
}

const AGENT = ['confirmed', 'wrong', 'unverifiable', 'blocked'];
const RESOLUTIONS = { work: ['kept', 'changed'], client: ['signed-off', 'changed'] };
const CLAIMS_COLUMNS = ['#', 'claim', 'kind', 'evidence', 'agent', 'author', 'note'];
const TIER_COLUMNS = ['#', 'sentence', 'tier', 'why', 'resolution'];

// readSection is checkLedger's front door onto table(): "no section" and "section present but unreadable" (a
// GateTableError, or a header missing one of `columns`) are different refusals, but both leave nothing to
// iterate, so both push one message onto `probs` and hand back `[]`.
function readSection(ledger, heading, columns, probs) {
  let rows;
  try {
    rows = table(ledger, heading);
  } catch (e) {
    probs.push(e.message);
    return [];
  }
  if (rows === null) { probs.push(`the ledger has no ## ${heading} section`); return []; }
  const missing = columns.filter((c) => !rows.header.includes(c));
  if (missing.length) { probs.push(`## ${heading} is missing column(s): ${missing.join(', ')}`); return []; }
  return rows;
}

// checkLedger returns every reason the gate refuses one piece, given the ledger's text, the piece's bytes and the
// translation's saved model pass (null when the piece is not a translation).
export function checkLedger(ledger, piece, modelPass) {
  const probs = [];
  const h = /^hash:[ \t]*(sha256:[0-9a-f]{64})[ \t]*$/m.exec(ledger);
  if (!h) probs.push('the ledger has no hash: line');
  else if (h[1] !== sha256(piece)) probs.push('the piece changed since the review (the hash differs); re-run /review-piece');
  const claims = readSection(ledger, 'Claims', CLAIMS_COLUMNS, probs);
  for (const c of claims) {
    const id = `claim ${c['#'] || '?'}`;
    const kind = (c.kind ?? '').toLowerCase(), agent = (c.agent ?? '').toLowerCase(), author = c.author ?? '', note = c.note ?? '';
    if (kind !== 'repo' && kind !== 'source') { probs.push(`${id}: kind ${JSON.stringify(c.kind)} is not repo or source`); continue; }
    if (!AGENT.includes(agent)) probs.push(`${id}: agent verdict ${JSON.stringify(c.agent)} is not one of ${AGENT.join(', ')}`);
    if (!['✓', '✗', ''].includes(author)) probs.push(`${id}: author mark ${JSON.stringify(author)} is not ✓, ✗ or empty`);
    else if (author === '✗') probs.push(`${id}: the author marked it wrong; fix the piece and re-run /review-piece`);
    else if (kind === 'source' && author !== '✓') probs.push(`${id}: a source claim needs the author's ✓ (checked by hand against the source)`);
    else if (kind === 'repo' && agent !== 'confirmed' && !(author === '✓' && note !== '')) probs.push(`${id}: a repository claim the agent did not confirm needs the author's ✓ and a note`);
  }
  const tier = readSection(ledger, 'Tier', TIER_COLUMNS, probs);
  for (const f of tier) {
    const id = `tier flag ${f['#'] || '?'}`;
    const t = (f.tier ?? '').toLowerCase(), r = (f.resolution ?? '').toLowerCase();
    const ok = RESOLUTIONS[t];
    if (!ok) probs.push(`${id}: tier ${JSON.stringify(f.tier)} is not work or client`);
    else if (!ok.includes(r)) probs.push(`${id}: a ${t} flag resolves as ${ok.join(' or ')}, not ${JSON.stringify(f.resolution ?? '')}`);
  }
  if (modelPass !== null && Buffer.compare(Buffer.from(modelPass), Buffer.from(piece)) === 0) {
    probs.push('the translation is the untouched model pass; rewrite it first');
  }
  return probs;
}

// checkScope returns every reason a branch is not a piece branch. `changes` are `git diff --name-status` entries
// ({status: first letter, path: last path}) against the base; `keyOf(path)` reads a changed piece's key.
export function checkScope(changes, keyOf) {
  const probs = [];
  const keys = new Set();
  const pieces = [];
  for (const { status, path } of changes) {
    if (status === 'D') { probs.push(`${path}: deleted; published URLs never change and pieces are never deleted`); continue; }
    if (status !== 'A' && status !== 'M') { probs.push(`${path}: renamed or copied; published URLs never change`); continue; }
    const img = /^assets\/img\/([a-z0-9-]+)\/.+$/.exec(path);
    if (pieceLang(path)) {
      pieces.push(path);
      const k = keyOf(path);
      if (k) keys.add(k); else probs.push(`${path}: no key: in the front matter`);
    } else if (img) keys.add(img[1]);
    else probs.push(`${path}: a piece branch carries content/<lang>/<slug>.md and assets/img/<key>/ only`);
  }
  if (pieces.length === 0) probs.push('no piece file changed');
  if (keys.size > 1) probs.push(`more than one key: ${[...keys].sort().join(', ')}`);
  return { probs, pieces };
}

const git = (...args) => execFileSync('git', args, { encoding: 'utf8' }).trim();
const ledgerDir = (key) => join(resolve(git('rev-parse', '--git-common-dir')), 'review', key);

function main(argv) {
  const [cmd, ...rest] = argv;
  if ((cmd === 'path' || cmd === 'hash') && rest.length === 1) {
    const bytes = readFileSync(rest[0]);
    if (cmd === 'hash') { console.log(sha256(bytes)); return 0; }
    const key = pieceKey(bytes), lang = pieceLang(rest[0]);
    if (!key || !lang) { console.error(`gate: ${rest[0]}: not a piece (content/<en|ro>/<slug>.md with a key:)`); return 2; }
    console.log(join(ledgerDir(key), `ledger-${lang}.md`));
    return 0;
  }
  if (cmd === 'scope' && (rest.length === 0 || (rest.length === 2 && rest[0] === '--base'))) {
    const base = rest[1] ?? 'origin/main';
    const out = git('diff', '--name-status', `${base}...HEAD`);
    const changes = out ? out.split('\n').map((l) => { const f = l.split('\t'); return { status: f[0][0], path: f[f.length - 1] }; }) : [];
    const { probs, pieces } = checkScope(changes, (p) => pieceKey(readFileSync(p)));
    for (const p of probs) console.error(`gate: scope: ${p}`);
    if (probs.length) return 1;
    for (const p of pieces) console.log(p);
    return 0;
  }
  if (cmd === 'ledger' && rest.length > 0) {
    let refused = 0;
    for (const file of rest) {
      const piece = readFileSync(file);
      const dir = ledgerDir(pieceKey(piece)), lang = pieceLang(file);
      const lp = join(dir, `ledger-${lang}.md`), mp = join(dir, `model-pass-${lang}.md`);
      const probs = existsSync(lp)
        ? checkLedger(readFileSync(lp, 'utf8'), piece, existsSync(mp) ? readFileSync(mp) : null)
        : [`no ledger at ${lp}; run /review-piece`];
      for (const p of probs) console.error(`gate: ${file}: ${p}`);
      if (probs.length) refused++; else console.log(`gate: ${file}: ok`);
    }
    return refused ? 1 : 0;
  }
  console.error('usage: gate.mjs path <piece> | hash <piece> | scope [--base <ref>] | ledger <piece>...');
  return 2;
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) process.exitCode = main(process.argv.slice(2));
