import { createServer } from "node:http";
import { readFileSync, statSync, existsSync } from "node:fs";
import { join } from "node:path";
// Serves a fixture directory like site serve --static would: _headers applied, index.html for directories, 404.html with 404.
// opts.redirect404 makes /404.html answer 301 → /404/ instead, the way the edge's asset layer does on production
// (force-trailing-slash), so a test can serve what post mode sees.
export function serveFixture(dir, headersOverride, opts = {}) {
  const rules = parseHeaders(existsSync(join(dir, "_headers")) ? readFileSync(join(dir, "_headers"), "utf8") : "");
  const srv = createServer((req, res) => {
    let p = decodeURIComponent(new URL(req.url, "http://x").pathname);
    let fp = join(dir, p);
    if (existsSync(fp) && statSync(fp).isDirectory()) fp = join(fp, "index.html");
    for (const r of rules) if (r.match(p)) for (const [k, v] of r.headers) v === null ? res.removeHeader(k) : res.setHeader(k, v);
    // headersOverride is either a flat {header: value} object applied to every response, or
    // {path, headers} to scope it to one path only (e.g. breaking a single feed variant).
    if (headersOverride) {
      const scoped = headersOverride.path !== undefined;
      if (!scoped || headersOverride.path === p) for (const [k, v] of Object.entries(scoped ? headersOverride.headers : headersOverride)) res.setHeader(k, v);
    }
    if (p === "/404.html" && opts.redirect404) { res.statusCode = 301; res.setHeader("location", "/404/"); res.end(); return; }
    if (!existsSync(fp) || p === "/404.html") { res.statusCode = 404; res.setHeader("content-type", "text/html; charset=utf-8"); res.end(existsSync(join(dir, "404.html")) ? readFileSync(join(dir, "404.html")) : "nope"); return; }
    const ext = fp.split(".").pop();
    const types = { html: "text/html; charset=utf-8", xml: "application/xml; charset=utf-8", json: "application/json", txt: "text/plain; charset=utf-8", svg: "image/svg+xml", png: "image/png", ico: "image/x-icon", woff2: "font/woff2", webp: "image/webp", jpg: "image/jpeg", css: "text/css" };
    if (!res.getHeader("content-type")) res.setHeader("content-type", types[ext] || "application/octet-stream");
    res.end(readFileSync(fp));
  });
  return new Promise((resolve) => srv.listen(0, "127.0.0.1", () => resolve({ base: `http://127.0.0.1:${srv.address().port}`, close: () => new Promise((r) => srv.close(r)) })));
}
function parseHeaders(text) {
  const rules = [];
  for (const line of text.split("\n")) {
    if (!line.trim()) continue;
    if (!line.startsWith(" ")) { rules.push({ pattern: line.trim(), headers: [], match(p) { return this.pattern.endsWith("*") ? p.startsWith(this.pattern.slice(0, -1)) : this.pattern === p; } }); continue; }
    const l = line.trim();
    if (l.startsWith("! ")) { rules.at(-1).headers.push([l.slice(2), null]); continue; }
    const i = l.indexOf(": "); if (i > 0) rules.at(-1).headers.push([l.slice(0, i), l.slice(i + 2)]);
  }
  return rules;
}
export const ctxFor = (base, pages, extra = {}) => ({ base, mode: "ci", pages, when: "2026-09-19", link: "https://example.test/run", linkText: "CI run", ...extra });
