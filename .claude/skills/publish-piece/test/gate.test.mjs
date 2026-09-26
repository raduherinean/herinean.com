import { test } from 'node:test';
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { checkLedger, checkScope, markLedger, pendingItems, pieceKey, pieceLang, setCells, sha256, splitRow, table } from '../gate.mjs';

const piece = '---\ntitle: "T"\ndate:\nkey: k\npillar: analysis\nsummary: "S"\n---\n\nBody.\n';
const row = (cells) => `| ${cells.join(' | ')} |`;
const ledger = ({ hash = sha256(piece), claims = [], tier = [], claimsHeader = '| # | Claim | Kind | Evidence | Agent | Author | Note |' } = {}) =>
  `# Review — T (en)\n\nhash: ${hash}\nrepos: /src/one\n\n## Claims\n\n${claimsHeader}\n|---|---|---|---|---|---|---|\n${claims.map(row).join('\n')}\n\nFree prose after the table.\n\n## Tier\n\n| # | Sentence | Tier | Why | Resolution |\n|---|---|---|---|---|\n${tier.map(row).join('\n')}\n\n## Editing\n\n| not | a | gate | table |\n`;

const repoOK = ['1', 'the parser rejects tabs', 'repo', 'lexer.go:42', 'confirmed', '', ''];
const sourceOK = ['2', 'revenue doubled in 2025', 'source', 'https://example.com/report — "revenue doubled"', 'confirmed', '✓', ''];
const workKept = ['1', 'our billing system runs nightly', 'work', 'names an internal system', 'kept'];

test('a complete ledger passes', () => {
  assert.deepEqual(checkLedger(ledger({ claims: [repoOK, sourceOK], tier: [workKept] }), piece, null), []);
});

test('zero claims and zero flags pass', () => {
  assert.deepEqual(checkLedger(ledger(), piece, null), []);
});

test('a stale hash is refused', () => {
  const p = checkLedger(ledger({ hash: sha256('other text') }), piece, null);
  assert.equal(p.length, 1);
  assert.match(p[0], /changed since the review/);
});

test('a missing hash line or section is refused', () => {
  assert.match(checkLedger('## Claims\n\n## Tier\n', piece, null).join('\n'), /no hash: line/);
  assert.match(checkLedger(`hash: ${sha256(piece)}\n\n## Tier\n`, piece, null).join('\n'), /no ## Claims/);
  assert.match(checkLedger(`hash: ${sha256(piece)}\n\n## Claims\n`, piece, null).join('\n'), /no ## Tier/);
});

test('a source claim needs the author\'s check mark', () => {
  const noMark = ['2', 'revenue doubled', 'source', 'https://example.com', 'confirmed', '', ''];
  assert.match(checkLedger(ledger({ claims: [noMark] }), piece, null).join('\n'), /needs the author's ✓/);
});

test('a blocked source the author checked by hand passes', () => {
  const blocked = ['3', 'the ruling cites clause 4', 'source', 'blocked; the court PDF used', 'blocked', '✓', 'read the PDF'];
  assert.deepEqual(checkLedger(ledger({ claims: [blocked] }), piece, null), []);
});

test('an author ✗ is refused', () => {
  const wrong = ['2', 'revenue doubled', 'source', 'https://example.com', 'wrong', '✗', ''];
  assert.match(checkLedger(ledger({ claims: [wrong] }), piece, null).join('\n'), /marked it wrong/);
});

test('an unconfirmed repository claim needs the author\'s ✓ and a note', () => {
  const unver = ['1', 'the cache halves latency', 'repo', 'no benchmark in the repo', 'unverifiable', '✓', ''];
  assert.match(checkLedger(ledger({ claims: [unver] }), piece, null).join('\n'), /needs the author's ✓ and a note/);
  unver[6] = 'measured by hand, numbers in the footnote';
  assert.deepEqual(checkLedger(ledger({ claims: [unver] }), piece, null), []);
});

test('a wrong repository claim with the author\'s ✓ still needs a note; adding one passes', () => {
  const wrongRepo = ['1', 'the cache halves latency', 'repo', 'no benchmark in the repo', 'wrong', '✓', ''];
  assert.match(checkLedger(ledger({ claims: [wrongRepo] }), piece, null).join('\n'), /needs the author's ✓ and a note/);
  wrongRepo[6] = 'measured by hand, numbers in the footnote';
  assert.deepEqual(checkLedger(ledger({ claims: [wrongRepo] }), piece, null), []);
});

test('tier flags: unresolved refused; a client flag kept refused; signed-off passes', () => {
  assert.match(checkLedger(ledger({ tier: [['1', 's', 'work', 'why', '']] }), piece, null).join('\n'), /kept or changed/);
  assert.match(checkLedger(ledger({ tier: [['1', 's', 'client', 'why', 'kept']] }), piece, null).join('\n'), /signed-off or changed/);
  assert.deepEqual(checkLedger(ledger({ tier: [['1', 's', 'client', 'signed off by the client, 2026-10-01', 'signed-off']] }), piece, null), []);
});

test('a translation equal to its model pass is refused', () => {
  assert.match(checkLedger(ledger(), piece, piece).join('\n'), /untouched model pass/);
  assert.deepEqual(checkLedger(ledger(), piece, piece.replace('Body.', 'Text.')), []);
});

test('columns in any order, escaped pipes, capitals', () => {
  const md = ledger({
    claimsHeader: '| Author | # | Kind | Agent | Claim | Evidence | Note |',
    claims: [['✓', '1', 'Source', 'Confirmed', 'a \\| b split', 'https://example.com', '']],
  });
  assert.deepEqual(checkLedger(md, piece, null), []);
  assert.equal(table(md, 'Claims')[0].claim, 'a | b split');
});

test('splitRow keeps empty cells and honours \\|', () => {
  assert.deepEqual(splitRow('| a \\| b | c |  |'), ['a | b', 'c', '']);
});

test('pieceKey and pieceLang', () => {
  assert.equal(pieceKey(piece), 'k');
  assert.equal(pieceKey(piece.replaceAll('\n', '\r\n')), 'k');
  assert.equal(pieceKey('no front matter'), '');
  assert.equal(pieceLang('content/ro/un-titlu.md'), 'ro');
  assert.equal(pieceLang('content/fr/x.md'), '');
});

const keyOf = (p) => ({ 'content/en/a-piece.md': 'k', 'content/ro/o-piesa.md': 'k', 'content/en/other.md': 'j' })[p] ?? '';

test('scope: one key\'s pieces and images pass', () => {
  const r = checkScope([{ status: 'A', path: 'content/en/a-piece.md' }, { status: 'M', path: 'content/ro/o-piesa.md' }, { status: 'A', path: 'assets/img/k/sub/chart.svg' }], keyOf);
  assert.deepEqual(r.probs, []);
  assert.deepEqual(r.pieces, ['content/en/a-piece.md', 'content/ro/o-piesa.md']);
});

test('scope: anything else, two keys, or no piece is refused', () => {
  assert.match(checkScope([{ status: 'A', path: 'content/en/a-piece.md' }, { status: 'M', path: 'templates/piece.html' }], keyOf).probs.join('\n'), /templates\/piece.html: a piece branch carries/);
  assert.match(checkScope([{ status: 'M', path: 'content/en/_home.md' }], keyOf).probs.join('\n'), /_home.md: a piece branch carries/);
  assert.match(checkScope([{ status: 'A', path: 'content/en/a-piece.md' }, { status: 'A', path: 'content/en/other.md' }], keyOf).probs.join('\n'), /more than one key: j, k/);
  assert.match(checkScope([{ status: 'A', path: 'assets/img/k/x.png' }], keyOf).probs.join('\n'), /no piece file changed/);
});

test('scope: deletions and renames are refused', () => {
  assert.match(checkScope([{ status: 'D', path: 'content/en/a-piece.md' }], keyOf).probs.join('\n'), /deleted; published URLs never change/);
  assert.match(checkScope([{ status: 'R', path: 'content/en/a-piece.md' }], keyOf).probs.join('\n'), /renamed or copied/);
});

// table() must not silently drop rows: stopping at the first non-`|` line after the header, or taking whatever
// `|` line came first as the header without checking a separator followed it, would let a blank or prose line
// between rows drop the rows after it, and let a lone data row with no header read as "zero claims". The gate
// refuses instead — the spec requires every mark the author owes to be visible, never silently skipped.
const claimsHdr = '| # | Claim | Kind | Evidence | Agent | Author | Note |\n|---|---|---|---|---|---|---|\n';
const tierSection = '\n## Tier\n\n| # | Sentence | Tier | Why | Resolution |\n|---|---|---|---|---|\n';
const okRepoRow = '| 1 | parser | repo | a.go:1 | confirmed |  |  |\n';
const badSourceRow = '| 2 | revenue doubled | source | https://x | confirmed |  |  |\n';

test('a blank line between claim rows does not drop the later row', () => {
  const md = `hash: ${sha256(piece)}\n\n` + '## Claims\n\n' + claimsHdr + okRepoRow + '\n' + badSourceRow + tierSection;
  assert.match(checkLedger(md, piece, null).join('\n'), /needs the author's ✓/);
});

test('claims written as a bullet list, not a table, is refused', () => {
  const md = `hash: ${sha256(piece)}\n\n` + '## Claims\n\n- 2. revenue doubled — source — https://x — confirmed — author: (none)\n' + tierSection;
  assert.match(checkLedger(md, piece, null).join('\n'), /## Claims has no table header/);
});

test('a single claim row with no header row is refused', () => {
  const md = `hash: ${sha256(piece)}\n\n` + '## Claims\n\n' + badSourceRow + tierSection;
  assert.match(checkLedger(md, piece, null).join('\n'), /## Claims has no table header/);
});

test('a blank line between the header and the separator is refused', () => {
  const md = `hash: ${sha256(piece)}\n\n` + '## Claims\n\n| # | Claim | Kind | Evidence | Agent | Author | Note |\n\n|---|---|---|---|---|---|---|\n' + badSourceRow + tierSection;
  assert.match(checkLedger(md, piece, null).join('\n'), /## Claims has no table header/);
});

test('a blank line between tier rows does not drop the later, unresolved row', () => {
  const md = `hash: ${sha256(piece)}\n\n` + '## Claims\n\n' + claimsHdr + '\n## Tier\n\n| # | Sentence | Tier | Why | Resolution |\n|---|---|---|---|---|\n| 1 | s | work | w | kept |\n\n| 2 | s | client | w | kept |\n';
  assert.match(checkLedger(md, piece, null).join('\n'), /signed-off or changed/);
});

test('a duplicate ## Claims heading is refused', () => {
  const md = `hash: ${sha256(piece)}\n\n` + '## Claims\n\n' + claimsHdr + '\n## Claims\n\n' + claimsHdr + badSourceRow + tierSection;
  assert.match(checkLedger(md, piece, null).join('\n'), /more than one ## Claims heading/);
});

test('a Claims header missing a required column is refused, naming it', () => {
  const md = `hash: ${sha256(piece)}\n\n` + '## Claims\n\n| # | Claim | Kind | Evidence | Agent | Note |\n|---|---|---|---|---|---|\n| 1 | x | repo | a.go | confirmed |  |\n' + tierSection;
  assert.match(checkLedger(md, piece, null).join('\n'), /## Claims is missing column\(s\): author/);
});

// GFM makes a table row's outer pipes optional, so a row written without a leading `|` (or hidden behind `>` or
// `- `) still renders as a row in any Markdown viewer — table() must not silently skip a line that doesn't start
// with `|`, or that row's marks become invisible to the gate. Separately, the duplicate-heading check must not be
// dodged by a whitespace or trailing-`#` variant of `## Claims`/`## Tier`.

test('a GFM row with no leading | is refused, not silently dropped', () => {
  const md = `hash: ${sha256(piece)}\n\n` + '## Claims\n\n' + claimsHdr + okRepoRow + '2 | revenue doubled | source | https://x | confirmed |  |\n' + tierSection;
  // the message below is the one this file's later tests also check for, on the same line.
  assert.match(checkLedger(md, piece, null).join('\n'), /a line with \| must be a table row starting with \|/);
});

test('a blockquoted "> | …" row is refused, not silently dropped', () => {
  const md = `hash: ${sha256(piece)}\n\n` + '## Claims\n\n' + claimsHdr + okRepoRow + '> | 2 | revenue doubled | source | https://x | confirmed |  |  |\n' + tierSection;
  assert.match(checkLedger(md, piece, null).join('\n'), /a line with \| must be a table row starting with \|/);
});

test('"##  Claims" (two spaces) is still a duplicate ## Claims heading', () => {
  const md = `hash: ${sha256(piece)}\n\n` + '## Claims\n\n' + claimsHdr + '\n##  Claims\n\n' + claimsHdr + badSourceRow + tierSection;
  assert.match(checkLedger(md, piece, null).join('\n'), /more than one ## Claims heading/);
});

test('a pipeless GFM row in ## Tier is refused, not silently dropped', () => {
  const tierHdr = '| # | Sentence | Tier | Why | Resolution |\n|---|---|---|---|---|\n';
  const md = `hash: ${sha256(piece)}\n\n` + '## Claims\n\n' + claimsHdr + '\n## Tier\n\n' + tierHdr + '2 | s | client | w | kept\n';
  assert.match(checkLedger(md, piece, null).join('\n'), /a line with \| must be a table row starting with \|/);
});

// The pipeless-row check must not only run from the header down: a whole pipeless table pasted above the real
// header would otherwise go unexamined; a lazy line with no pipe at all, sitting directly under a real row with
// no blank line between, would otherwise pass, though GFM still renders it as part of that row; and the
// duplicate-heading check must not be dodged by decorating the name (`## **Claims**`) or extending it
// (`## Claims (continued)`), not just varying its whitespace. The rule: every non-blank line anywhere in a
// ## Claims/## Tier section either starts with `|`, or is safe prose (no unescaped `|`, and not immediately below
// a `|` line with no blank line between); and a later heading counts as a duplicate whenever its letters-only,
// lower-cased name equals or starts with the target's.

test('a pipeless fake table above the real header is refused, not ignored', () => {
  const md = `hash: ${sha256(piece)}\n\n` + '## Claims\n\nClaim | # | Kind | Evidence | Agent | Author | Note\n---|---|---|---|---|---|---\nrevenue doubled | 2 | source | https://x | confirmed |  | \n\n' + claimsHdr + okRepoRow + tierSection;
  assert.match(checkLedger(md, piece, null).join('\n'), /a line with \| must be a table row starting with \|/);
});

test('a lazy line with no pipe directly under a row is refused', () => {
  const md = `hash: ${sha256(piece)}\n\n` + '## Claims\n\n' + claimsHdr + okRepoRow + 'revenue doubled, source, unchecked\n' + tierSection;
  assert.match(checkLedger(md, piece, null).join('\n'), /directly under the table, a line is read as a row/);
});

test('"## Claims (continued)" is still a duplicate ## Claims heading', () => {
  const md = `hash: ${sha256(piece)}\n\n` + '## Claims\n\n' + claimsHdr + okRepoRow + '\n## Claims (continued)\n\n' + claimsHdr + badSourceRow + tierSection;
  assert.match(checkLedger(md, piece, null).join('\n'), /more than one ## Claims heading/);
});

test('prose containing an unescaped | between tables is refused', () => {
  const md = `hash: ${sha256(piece)}\n\n` + '## Claims\n\n' + claimsHdr + okRepoRow + '\nClaim 1 checked at v1.2 | v1.3.\n' + tierSection;
  assert.match(checkLedger(md, piece, null).join('\n'), /## Claims line \d+: a line with \| must be a table row starting with \|; move prose out of this section or write the pipe as \\\|/);
});

// --- gate.mjs mark: pendingItems, setCells, markLedger ---------------------------------------------------------

test('pendingItems: pending claims and flags, in table order; ✗ and confirmed/resolved rows are not pending', () => {
  const unmarkedSource = ['1', 'revenue doubled', 'source', 'https://example.com', 'confirmed', '', ''];
  const confirmedRepo = ['2', 'the parser rejects tabs', 'repo', 'lexer.go:42', 'confirmed', '', ''];
  const unverRepoNoNote = ['3', 'the cache halves latency', 'repo', 'no benchmark', 'unverifiable', '✓', ''];
  const wrongClaim = ['4', 'revenue tripled', 'source', 'https://example.com', 'wrong', '✗', ''];
  const md = ledger({
    claims: [unmarkedSource, confirmedRepo, unverRepoNoNote, wrongClaim],
    tier: [
      ['1', 'an unresolved work sentence', 'work', 'names an internal system', ''],
      ['2', 'a client sentence', 'client', 'names a client', 'kept'],
      ['3', 'a resolved client sentence', 'client', 'names a client', 'signed-off'],
    ],
  });
  const items = pendingItems(md);
  assert.deepEqual(items.map((i) => [i.section, i.id]), [
    ['Claims', '1'],
    ['Claims', '3'],
    ['Tier', '1'],
    ['Tier', '2'],
  ]);
});

test('pendingItems surfaces an unreadable table as an error, never "nothing pending"', () => {
  assert.throws(() => pendingItems('## Claims\n\nrevenue doubled | 2 | source | https://x | confirmed |  |\n'), /table row starting with \|/);
});

test('setCells changes only the target row; every other byte of the file is unchanged', () => {
  const claim1 = ['1', 'revenue doubled', 'source', 'https://example.com', 'confirmed', '', ''];
  const claim2 = ['2', 'the parser rejects tabs', 'repo', 'lexer.go:42', 'confirmed', '', ''];
  const md = ledger({ claims: [claim1, claim2] });
  const updated = setCells(md, 'Claims', '1', { author: '✓', note: 'checked' });
  assert.equal(table(updated, 'Claims')[0].author, '✓');
  assert.equal(table(updated, 'Claims')[0].note, 'checked');
  assert.equal(table(updated, 'Claims')[1].author, '');
  const before = md.split('\n');
  const after = updated.split('\n');
  assert.equal(after.length, before.length);
  const diffLines = before.map((l, i) => (l === after[i] ? null : i)).filter((i) => i !== null);
  assert.deepEqual(diffLines.length, 1);
});

test('setCells escapes | in a written cell; table() reads it back unescaped', () => {
  const claim1 = ['1', 'revenue doubled', 'source', 'https://example.com', 'confirmed', '', ''];
  const md = ledger({ claims: [claim1] });
  const updated = setCells(md, 'Claims', '1', { note: 'a | b' });
  assert.match(updated, /a \\\| b/);
  assert.equal(table(updated, 'Claims')[0].note, 'a | b');
});

test('setCells keeps CRLF on every line of a CRLF ledger', () => {
  const claim1 = ['1', 'revenue doubled', 'source', 'https://example.com', 'confirmed', '', ''];
  const md = ledger({ claims: [claim1] }).replaceAll('\n', '\r\n');
  const updated = setCells(md, 'Claims', '1', { author: '✓' });
  const lines = updated.split('\n');
  for (let i = 0; i < lines.length - 1; i++) assert.ok(lines[i].endsWith('\r'), `line ${i} lost its CR`);
  assert.equal(table(updated, 'Claims')[0].author, '✓');
});

test('setCells: columns in a non-default header order', () => {
  const md = ledger({
    claimsHeader: '| Author | # | Kind | Agent | Claim | Evidence | Note |',
    claims: [['', '1', 'source', 'confirmed', 'revenue doubled', 'https://example.com', '']],
  });
  const updated = setCells(md, 'Claims', '1', { author: '✓', note: 'checked by hand' });
  assert.equal(table(updated, 'Claims')[0].author, '✓');
  assert.equal(table(updated, 'Claims')[0].note, 'checked by hand');
});

const scripted = (answers) => {
  let i = 0;
  return async () => {
    if (i >= answers.length) throw new Error('markLedger asked more questions than the script has answers');
    return answers[i++];
  };
};

test('markLedger: answering y (and required notes/sign-off) satisfies the gate', async () => {
  const md = ledger({
    claims: [
      ['1', 'revenue doubled', 'source', 'https://example.com', 'confirmed', '', ''],
      ['2', 'the cache halves latency', 'repo', 'no benchmark', 'unverifiable', '', ''],
    ],
    tier: [['1', 'a client sentence', 'client', 'names a client', '']],
  });
  const answers = ['y', '', 'y', 'measured by hand', 'o', 'the client, 2026-10-01'];
  const { md: result, marked, skipped, quit } = await markLedger(md, scripted(answers));
  assert.equal(marked, 3);
  assert.equal(skipped, 0);
  assert.equal(quit, false);
  assert.deepEqual(checkLedger(result, piece, null), []);
});

test('markLedger: n writes an author ✗', async () => {
  const md = ledger({ claims: [['1', 'revenue doubled', 'source', 'https://example.com', 'confirmed', '', '']] });
  const { md: result, marked } = await markLedger(md, scripted(['n', '']));
  assert.equal(marked, 1);
  assert.equal(table(result, 'Claims')[0].author, '✗');
});

test('markLedger: s skips, leaving the cell empty', async () => {
  const md = ledger({ claims: [['1', 'revenue doubled', 'source', 'https://example.com', 'confirmed', '', '']] });
  const { md: result, marked, skipped } = await markLedger(md, scripted(['s']));
  assert.equal(marked, 0);
  assert.equal(skipped, 1);
  assert.equal(table(result, 'Claims')[0].author, '');
});

test('markLedger: q stops and keeps earlier answers', async () => {
  const md = ledger({
    claims: [
      ['1', 'revenue doubled', 'source', 'https://example.com', 'confirmed', '', ''],
      ['2', 'the parser rejects tabs', 'source', 'https://example.com', 'confirmed', '', ''],
    ],
  });
  const { md: result, marked, quit } = await markLedger(md, scripted(['y', '', 'q']));
  assert.equal(marked, 1);
  assert.equal(quit, true);
  assert.equal(table(result, 'Claims')[0].author, '✓');
  assert.equal(table(result, 'Claims')[1].author, '');
});

test('markLedger: a repo-unconfirmed y with an empty note re-prompts, then accepts', async () => {
  const md = ledger({ claims: [['1', 'the cache halves latency', 'repo', 'no benchmark', 'unverifiable', '', '']] });
  const { md: result, marked } = await markLedger(md, scripted(['y', '', '  ', 'noted, footnote 3']));
  assert.equal(marked, 1);
  assert.equal(table(result, 'Claims')[0].author, '✓');
  assert.equal(table(result, 'Claims')[0].note, 'noted, footnote 3');
});

test('markLedger: a client flag "o" appends the sign-off to Why', async () => {
  const md = ledger({ tier: [['1', 'a client sentence', 'client', 'names a client', '']] });
  const { md: result, marked } = await markLedger(md, scripted(['o', 'the client, 2026-10-01']));
  assert.equal(marked, 1);
  const row = table(result, 'Tier')[0];
  assert.equal(row.resolution, 'signed-off');
  assert.equal(row.why, 'names a client — signed off: the client, 2026-10-01');
});

test('markLedger: an invalid answer re-prompts', async () => {
  const md = ledger({ claims: [['1', 'revenue doubled', 'source', 'https://example.com', 'confirmed', '', '']] });
  const { marked } = await markLedger(md, scripted(['x', 'y', '']));
  assert.equal(marked, 1);
});

test('CLI: mark refuses without a terminal', () => {
  const gatePath = new URL('../gate.mjs', import.meta.url).pathname;
  const result = spawnSync(process.execPath, [gatePath, 'mark'], { input: '', encoding: 'utf8' });
  assert.equal(result.status, 2);
  assert.match(result.stderr, /mark needs a terminal — the author types the marks; run it in your own shell/);
});
