# `site-ok` fixture

A minimal *built* site, used by `bench/test/*.test.mjs` so module tests never depend on the Go
binary or a live server. It was built by `site build` from this repository's own content at the
time (`go build -tags nodynamic -o .cache/site ./cmd/site && .cache/site build`), copying real
`dist/` output; "Refreshing" below regenerates it from the current tree:

- `index.html`, `ro/index.html`, `404.html`, `_headers`, `robots.txt`, `llms.txt`,
  `.well-known/security.txt`, `feed.xml`, `feed.en.xml`, `feed.ro.xml`, `feed.json`,
  `favicon.svg`, `favicon.ico`, `apple-touch-icon.png`, `fonts/*.woff2` — copied byte-for-byte.
- `sitemap.xml` — hand-written, same shape as the real one, listing only `https://herinean.com/`
  and `https://herinean.com/ro/` with their hreflang alternates and lastmod (the real sitemap
  also lists `/writing/`, `/privacy/`, `/colophon/` and their `/ro/` counterparts, which this
  fixture deliberately omits).
- `og/home-en*.png`, `og/home-ro*.png`, `img/home/portrait*.{webp,jpg}` — **not** byte-copies.
  The real files are ~300 KB combined, which alone would blow the fixture's size budget. These
  are generated placeholders at the exact same pixel dimensions and format as the real assets
  (og: 1200×630 PNG; portrait: 320×361 and 480×542 WebP/JPEG) so any module that checks
  dimensions, content-type, or "does this URL resolve" still gets a correct answer — just not
  real photographic content. Regenerate with Pillow if a future module needs different pixel
  content (e.g. a real face for a manual visual check):
  ```
  python3 -c "from PIL import Image; Image.new('RGB', (1200, 630), (250, 248, 244)).save('bench/fixtures/site-ok/og/home-en.3b3118bf.png', optimize=True)"
  ```

Excluded on purpose: `writing/`, `privacy/`, `colophon/`, `ro/articole/`, `ro/confidentialitate/`
— the fixture only needs to exercise `/` and `/ro/`; a test that needs a piece page or a stub for
one of these paths writes it into its own temporary copy (see `addPiece` and `stubPage` in
`bench/test/`).

`/` and `/ro/` reference each other's `hreflang` (verify with `grep hreflang
bench/fixtures/site-ok/ro/index.html`) — this is what lets the i18n test's "broken hreflang"
mutation (renaming `hreflang="en"` to `hreflang="de"` on `/ro/`) actually break a pair.

## Refreshing

```
go build -tags nodynamic -o .cache/site ./cmd/site
.cache/site build
cp dist/index.html dist/ro/index.html dist/404.html dist/_headers dist/robots.txt dist/llms.txt \
   dist/feed.xml dist/feed.en.xml dist/feed.ro.xml dist/feed.json dist/favicon.svg dist/favicon.ico \
   dist/apple-touch-icon.png bench/fixtures/site-ok/... # (see the file list above for destinations)
cp dist/fonts/*.woff2 bench/fixtures/site-ok/fonts/
cp dist/.well-known/security.txt bench/fixtures/site-ok/.well-known/
```
Then hand-edit `sitemap.xml` to keep only the `/` and `/ro/` entries, and regenerate the `og/*`
and `img/home/*` placeholders (above) if the real filenames' content hashes changed. Check the
total with `du -sh bench/fixtures/site-ok` — keep it under 300 KB.
