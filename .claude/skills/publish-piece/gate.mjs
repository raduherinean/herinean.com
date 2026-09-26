// The publish gate: what /publish-piece refuses before anything reaches origin. Node standard library only.
// From the repository root:
//   node .claude/skills/publish-piece/gate.mjs path   <piece.md>       the piece's ledger path
//   node .claude/skills/publish-piece/gate.mjs hash   <piece.md>       sha256:<hex> of the piece file
//   node .claude/skills/publish-piece/gate.mjs scope  [--base <ref>]   the branch changes one key's content only
//   node .claude/skills/publish-piece/gate.mjs ledger <piece.md>...    every mark the author owes is there
//   node .claude/skills/publish-piece/gate.mjs mark   [<piece.md>]     the author marks claims at a prompt
import { createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { existsSync, readdirSync, readFileSync, writeFileSync } from 'node:fs';
import { join, resolve } from 'node:path';
import { pathToFileURL } from 'node:url';
import { createInterface } from 'node:readline/promises';

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

// isSepRow reports whether a split row is a table separator (`|---|:--:|...`), shared by table() and setCells() so
// both skip and re-find rows the same way.
const isSepRow = (cells) => cells.length > 0 && cells.every((c) => /^:?-{3,}:?$/.test(c));

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
  if (hIdx < 0 || hIdx + 1 >= end || !lines[hIdx + 1].trim().startsWith('|') || !isSepRow(splitRow(lines[hIdx + 1]))) {
    throw new GateTableError(`## ${heading} has no table header`);
  }
  const headerCells = splitRow(lines[hIdx]);
  const header = headerCells.map((c) => c.toLowerCase());
  const rows = [];
  for (let i = hIdx + 2; i < end; i++) {
    const l = lines[i].trim();
    if (!l.startsWith('|')) continue; // blank, or safe prose already cleared above.
    const cells = splitRow(l);
    if (isSepRow(cells)) continue;
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

// pendingItems lists every claim and tier flag `mark` can still fix: a `source` claim with no Author, a `repo`
// claim the agent did not confirm and the author hasn't checked-and-noted, and a tier flag whose Resolution is not
// one of the two valid words for its own tier. A claim already marked ✗ is not pending — checkLedger already
// refuses it, and the fix is to the piece, not the ledger. A tier flag whose own `tier` cell is neither `work` nor
// `client` is left out too: `mark`'s prompt (see markLedger) only knows those two menus, so it cannot offer one;
// checkLedger still refuses it on its own terms.
export function pendingItems(md) {
  const items = [];
  const claims = table(md, 'Claims');
  if (claims) {
    for (const c of claims) {
      const kind = (c.kind ?? '').toLowerCase();
      const agent = (c.agent ?? '').toLowerCase();
      const author = c.author ?? '';
      const note = c.note ?? '';
      if (author === '✗') continue;
      if (kind === 'source' && author === '') items.push({ section: 'Claims', id: c['#'], row: c });
      else if (kind === 'repo' && agent !== 'confirmed' && !(author === '✓' && note !== '')) {
        items.push({ section: 'Claims', id: c['#'], row: c });
      }
    }
  }
  const tier = table(md, 'Tier');
  if (tier) {
    for (const f of tier) {
      const t = (f.tier ?? '').toLowerCase();
      const r = (f.resolution ?? '').toLowerCase();
      const ok = RESOLUTIONS[t];
      if (ok && !ok.includes(r)) items.push({ section: 'Tier', id: f['#'], row: f });
    }
  }
  return items;
}

// splitLines splits `md` into {line, eol} pairs so a rewrite can touch one line and reproduce every other line's
// own end-of-line bytes untouched — CRLF, bare LF, or (on the last line) none at all.
function splitLines(md) {
  const out = [];
  const re = /\r\n|\r|\n/g;
  let last = 0, m;
  while ((m = re.exec(md))) {
    out.push({ line: md.slice(last, m.index), eol: m[0] });
    last = re.lastIndex;
  }
  out.push({ line: md.slice(last), eol: '' });
  return out;
}

// setCells replaces exactly the row identified by `section` ('Claims' or 'Tier') and `id` (its `#` cell) with
// `patch` merged onto the row's current cells, keyed by lower-cased header name — so a header in any column order
// still lands the value in the right cell. Every other line of `md` is byte-identical to before, EOL included; the
// rewritten row is re-serialised `| c1 | c2 | … |`, with a literal `|` in any cell written `\|` (splitRow's escape,
// read back the same way by table()).
export function setCells(md, section, id, patch) {
  const parts = splitLines(md);
  const target = section.toLowerCase();
  const names = parts.map((p) => headingName(p.line));
  let start = -1;
  for (let i = 0; i < parts.length; i++) {
    if (names[i] !== null && headingLetters(names[i]) === target) { start = i; break; }
  }
  if (start < 0) throw new Error(`no ## ${section} section`);
  let end = parts.length;
  for (let i = start + 1; i < end; i++) if (names[i] !== null) { end = i; break; }
  let hIdx = -1;
  for (let i = start + 1; i < end; i++) if (parts[i].line.trim().startsWith('|')) { hIdx = i; break; }
  if (hIdx < 0) throw new Error(`## ${section} has no table header`);
  const headerCells = splitRow(parts[hIdx].line);
  const headerLower = headerCells.map((c) => c.toLowerCase());
  const idCol = headerLower.indexOf('#');
  for (let i = hIdx + 2; i < end; i++) {
    if (!parts[i].line.trim().startsWith('|')) continue;
    const cells = splitRow(parts[i].line);
    if (isSepRow(cells)) continue;
    if ((cells[idCol] ?? '') !== String(id)) continue;
    const merged = headerLower.map((h, j) => (Object.prototype.hasOwnProperty.call(patch, h) ? String(patch[h]) : (cells[j] ?? '')));
    const escaped = merged.map((c) => c.replace(/\|/g, '\\|'));
    parts[i] = { line: '| ' + escaped.join(' | ') + ' |', eol: parts[i].eol };
    return parts.map((p) => p.line + p.eol).join('');
  }
  throw new Error(`## ${section}: no row # ${id}`);
}

// markLedger drives the author's own prompt loop over every pendingItems() entry, in table order — the loop the
// `mark` CLI command runs. `ask(promptText)` is an async function that shows the prompt and returns the typed
// line (a readline `question` in the CLI; a scripted queue of answers in tests). `save(md)`, when given, is
// awaited after every completed answer, so quitting (`q`) or an interrupt loses nothing already typed.
export async function markLedger(md, ask, save) {
  let current = md;
  let marked = 0, skipped = 0, quit = false;
  const items = pendingItems(current);
  for (const item of items) {
    if (quit) break;
    if (item.section === 'Claims') {
      const row = item.row;
      const kind = (row.kind ?? '').toLowerCase();
      const agent = (row.agent ?? '').toLowerCase();
      console.log(`claim ${item.id} — ${kind}, agent: ${row.agent ?? ''}`);
      console.log(row.claim ?? '');
      console.log(row.evidence ?? '');
      let answer;
      for (;;) {
        answer = (await ask('y = ✓ checked · n = ✗ wrong · s = skip · q = quit: ')).trim().toLowerCase();
        if (['y', 'n', 's', 'q'].includes(answer)) break;
      }
      if (answer === 's') { skipped++; continue; }
      if (answer === 'q') { quit = true; break; }
      const authorMark = answer === 'y' ? '✓' : '✗';
      const noteRequired = kind === 'repo' && agent !== 'confirmed' && answer === 'y';
      let note = '', escape = null;
      for (;;) {
        const a = (await ask('note (Enter for none): ')).trim();
        if (a.toLowerCase() === 's') { escape = 's'; break; }
        if (a.toLowerCase() === 'q') { escape = 'q'; break; }
        if (noteRequired && a === '') continue;
        note = a;
        break;
      }
      if (escape === 's') { skipped++; continue; }
      if (escape === 'q') { quit = true; break; }
      current = setCells(current, 'Claims', item.id, { author: authorMark, note });
      if (save) await save(current);
      marked++;
    } else {
      const row = item.row;
      const tier = (row.tier ?? '').toLowerCase();
      console.log(`tier flag ${item.id} — ${tier}`);
      console.log(row.sentence ?? '');
      console.log(row.why ?? '');
      const valid = tier === 'work' ? ['k', 'c', 's', 'q'] : ['o', 'c', 's', 'q'];
      const promptText = tier === 'work'
        ? 'k = kept · c = changed · s = skip · q = quit: '
        : 'o = signed-off · c = changed · s = skip · q = quit: ';
      let answer;
      for (;;) {
        answer = (await ask(promptText)).trim().toLowerCase();
        if (valid.includes(answer)) break;
      }
      if (answer === 's') { skipped++; continue; }
      if (answer === 'q') { quit = true; break; }
      if (tier === 'work') {
        current = setCells(current, 'Tier', item.id, { resolution: answer === 'k' ? 'kept' : 'changed' });
      } else if (answer === 'o') {
        let who = '';
        for (;;) {
          who = (await ask('who signed off, and when: ')).trim();
          if (who !== '') break;
        }
        current = setCells(current, 'Tier', item.id, { resolution: 'signed-off', why: `${row.why ?? ''} — signed off: ${who}` });
      } else {
        current = setCells(current, 'Tier', item.id, { resolution: 'changed' });
      }
      if (save) await save(current);
      marked++;
    }
  }
  return { md: current, marked, skipped, quit };
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
const reviewRoot = () => join(resolve(git('rev-parse', '--git-common-dir')), 'review');

// pieceVerdict prints (to stderr, like the `ledger` command) every reason the gate still refuses `pieceFile`,
// given its ledger's current text, or prints `ok` to stdout when it now passes.
function pieceVerdict(pieceFile) {
  const bytes = readFileSync(pieceFile);
  const dir = ledgerDir(pieceKey(bytes)), lang = pieceLang(pieceFile);
  const lp = join(dir, `ledger-${lang}.md`), mp = join(dir, `model-pass-${lang}.md`);
  const probs = checkLedger(readFileSync(lp, 'utf8'), bytes, existsSync(mp) ? readFileSync(mp) : null);
  for (const p of probs) console.error(`gate: ${pieceFile}: ${p}`);
  if (!probs.length) console.log(`gate: ${pieceFile}: ok`);
  return probs.length ? 1 : 0;
}

// findLedgers lists every ledger-*.md under <git-common-dir>/review/*/ that pendingItems() finds at least one
// pending item in, as {path, key, lang, count}. A ledger table it cannot read is a refusal, not "no ledgers": the
// caller surfaces the error and stops rather than silently skipping it.
function findLedgers() {
  const root = reviewRoot();
  if (!existsSync(root)) return [];
  const found = [];
  for (const entry of readdirSync(root, { withFileTypes: true })) {
    if (!entry.isDirectory()) continue;
    const dir = join(root, entry.name);
    for (const file of readdirSync(dir)) {
      const m = /^ledger-([a-z]+)\.md$/.exec(file);
      if (!m) continue;
      const path = join(dir, file);
      const md = readFileSync(path, 'utf8');
      const count = pendingItems(md).length;
      if (count > 0) found.push({ path, key: entry.name, lang: m[1], count });
    }
  }
  return found;
}

async function runMark(pieceArg) {
  let ledgerPath;
  if (pieceArg) {
    const bytes = readFileSync(pieceArg);
    const key = pieceKey(bytes), lang = pieceLang(pieceArg);
    if (!key || !lang) { console.error(`gate: ${pieceArg}: not a piece (content/<en|ro>/<slug>.md with a key:)`); return 2; }
    ledgerPath = join(ledgerDir(key), `ledger-${lang}.md`);
    if (!existsSync(ledgerPath)) { console.error(`gate: no ledger at ${ledgerPath}; run /review-piece`); return 1; }
  }
  const rl = createInterface({ input: process.stdin, output: process.stdout });
  const ask = (q) => rl.question(q);
  try {
    if (!ledgerPath) {
      const found = findLedgers();
      if (found.length === 0) { console.log('nothing waits for you'); return 0; }
      found.forEach((f, i) => console.log(`${i + 1}) ${f.key} (${f.lang}): ${f.count} pending`));
      for (;;) {
        const a = (await ask('pick a ledger (or q): ')).trim().toLowerCase();
        if (a === 'q') return 0;
        const n = Number.parseInt(a, 10);
        if (Number.isInteger(n) && n >= 1 && n <= found.length) { ledgerPath = found[n - 1].path; break; }
      }
    }
    const save = (md) => writeFileSync(ledgerPath, md);
    const initial = readFileSync(ledgerPath, 'utf8');
    const { marked, skipped } = await markLedger(initial, ask, save);
    if (pieceArg) {
      console.log(`${marked} marked, ${skipped} skipped;`);
      pieceVerdict(pieceArg);
    } else {
      console.log(`${marked} marked, ${skipped} skipped; run the gate: node .claude/skills/publish-piece/gate.mjs ledger <piece>`);
    }
    return 0;
  } finally {
    rl.close();
  }
}

async function main(argv) {
  const [cmd, ...rest] = argv;
  if (cmd === 'mark' && rest.length <= 1) {
    if (!process.stdin.isTTY || !process.stdout.isTTY) {
      console.error('gate: mark needs a terminal — the author types the marks; run it in your own shell');
      return 2;
    }
    try {
      return await runMark(rest[0]);
    } catch (e) {
      console.error(`gate: ${e.message}`);
      return 1;
    }
  }
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
  console.error('usage: gate.mjs path <piece> | hash <piece> | scope [--base <ref>] | ledger <piece>... | mark [<piece>]');
  return 2;
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) process.exitCode = await main(process.argv.slice(2));
