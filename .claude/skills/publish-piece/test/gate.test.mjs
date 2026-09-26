import { test } from 'node:test';
import assert from 'node:assert/strict';
import { checkLedger, checkScope, pieceKey, pieceLang, sha256, splitRow, table } from '../gate.mjs';

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

// Fix round 1: the review found that table() stopped at the first non-`|` line after the header and took
// whatever `|` line came first as the header without checking a separator followed it — a blank or prose line
// between rows silently dropped the rows after it, and a lone data row with no header became "zero claims". The
// controller ruled the spec (the gate refuses when a mark is missing) overrides the plan's original code; these
// fixtures are the review's bypass probes, adapted to this file's `piece`.
const claimsHdr = '| # | Claim | Kind | Evidence | Agent | Author | Note |\n|---|---|---|---|---|---|---|\n';
const tierSection = '\n## Tier\n\n| # | Sentence | Tier | Why | Resolution |\n|---|---|---|---|---|\n';
const okRepoRow = '| 1 | parser | repo | a.go:1 | confirmed |  |  |\n';
const badSourceRow = '| 2 | revenue doubled | source | https://x | confirmed |  |  |\n';

test('fix round 1 (B1): a blank line between claim rows does not drop the later row', () => {
  const md = `hash: ${sha256(piece)}\n\n` + '## Claims\n\n' + claimsHdr + okRepoRow + '\n' + badSourceRow + tierSection;
  assert.match(checkLedger(md, piece, null).join('\n'), /needs the author's ✓/);
});

test('fix round 1 (B3): claims as a bullet list, no table, is refused', () => {
  const md = `hash: ${sha256(piece)}\n\n` + '## Claims\n\n- 2. revenue doubled — source — https://x — confirmed — author: (none)\n' + tierSection;
  assert.match(checkLedger(md, piece, null).join('\n'), /## Claims has no table header/);
});

test('fix round 1 (B4): a single claim row with no header row is refused', () => {
  const md = `hash: ${sha256(piece)}\n\n` + '## Claims\n\n' + badSourceRow + tierSection;
  assert.match(checkLedger(md, piece, null).join('\n'), /## Claims has no table header/);
});

test('fix round 1 (B5): a blank line between the header and the separator is refused', () => {
  const md = `hash: ${sha256(piece)}\n\n` + '## Claims\n\n| # | Claim | Kind | Evidence | Agent | Author | Note |\n\n|---|---|---|---|---|---|---|\n' + badSourceRow + tierSection;
  assert.match(checkLedger(md, piece, null).join('\n'), /## Claims has no table header/);
});

test('fix round 1 (B7): a blank line between tier rows does not drop the later, unresolved row', () => {
  const md = `hash: ${sha256(piece)}\n\n` + '## Claims\n\n' + claimsHdr + '\n## Tier\n\n| # | Sentence | Tier | Why | Resolution |\n|---|---|---|---|---|\n| 1 | s | work | w | kept |\n\n| 2 | s | client | w | kept |\n';
  assert.match(checkLedger(md, piece, null).join('\n'), /signed-off or changed/);
});

test('fix round 1 (B9): a duplicate ## Claims heading is refused', () => {
  const md = `hash: ${sha256(piece)}\n\n` + '## Claims\n\n' + claimsHdr + '\n## Claims\n\n' + claimsHdr + badSourceRow + tierSection;
  assert.match(checkLedger(md, piece, null).join('\n'), /more than one ## Claims heading/);
});

test('fix round 1: a Claims header missing a required column is refused, naming it', () => {
  const md = `hash: ${sha256(piece)}\n\n` + '## Claims\n\n| # | Claim | Kind | Evidence | Agent | Note |\n|---|---|---|---|---|---|\n| 1 | x | repo | a.go | confirmed |  |\n' + tierSection;
  assert.match(checkLedger(md, piece, null).join('\n'), /## Claims is missing column\(s\): author/);
});
