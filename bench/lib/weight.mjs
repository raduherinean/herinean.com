import { brotliCompressSync, constants } from "node:zlib";
import { row, summary } from "./row.mjs";
export const name = "Weight";
export const modes = ["ci"];
const br = (buf) => brotliCompressSync(buf, { params: { [constants.BROTLI_PARAM_QUALITY]: 5 } }).length; // Cloudflare's default level is in this range
// Row 14: HTML+CSS ≤ 30 KB brotli; fonts ≤ 100 KB, exactly one preloaded; first view ≤ 6 requests and ≤ 150 KB; 0 bytes of executable JS.
export async function run(ctx) {
  const problems = [], per = [];
  const pages = ctx.pages.filter((p) => p.path !== "/404.html");
  const fontURLs = new Set();
  for (const p of pages) for (const m of p.html.matchAll(/url\((\/fonts\/[^)]+)\)/g)) fontURLs.add(m[1]);
  let fontBytes = 0;
  for (const u of fontURLs) fontBytes += (await (await fetch(ctx.base + u)).arrayBuffer()).byteLength;
  if (fontBytes > 100 * 1024) problems.push(`fonts total ${fontBytes} bytes > 100 KB`);
  const browser = await ctx.browser();
  const bctx = await browser.newContext({ viewport: { width: 390, height: 844 } });
  try {
    for (const p of pages) {
      const html = br(Buffer.from(p.html));
      if (html > 30 * 1024) problems.push(`${p.path}: HTML+CSS ${html} bytes brotli > 30 KB`);
      const preloads = p.$('link[rel="preload"][as="font"]').length;
      if (preloads !== 1) problems.push(`${p.path}: ${preloads} font preloads, want 1`);
      const page = await bctx.newPage();
      const reqs = [], pending = [];
      let seen = 0, failed = 0;
      // Every response is accounted for: the promise that reads its body is collected, not fired-and-forgotten,
      // so a slow or failing read can never be silently missing from reqs when the page closes.
      page.on("response", (r) => {
        seen++;
        pending.push((async () => {
          let body;
          try {
            body = await r.body();
          } catch {
            failed++;
            return;
          }
          const ct = r.headers()["content-type"] || "";
          reqs.push({ url: r.url(), bytes: /^(text\/|application\/(json|xml|rss|feed)|image\/svg)/.test(ct) ? br(body) : body.length, js: /javascript/.test(ct) });
        })());
      });
      try {
        await page.goto(ctx.base + p.path, { waitUntil: "networkidle" });
        await Promise.all(pending);
      } finally {
        await page.close();
      }
      const accounted = reqs.length;
      if (seen - accounted !== 0) problems.push(`${p.path}: ${seen - accounted} responses could not be measured`);
      const total = reqs.reduce((t, r) => t + r.bytes, 0);
      if (reqs.length > 6) problems.push(`${p.path}: ${reqs.length} requests > 6`);
      if (total > 150 * 1024) problems.push(`${p.path}: first view ${total} bytes > 150 KB`);
      if (reqs.some((r) => r.js)) problems.push(`${p.path}: JavaScript fetched`);
      per.push({ path: p.path, html, requests: reqs.length, total, failed });
    }
  } finally {
    await bctx.close();
  }
  const worst = per.reduce((w, x) => (x.total > w.total ? x : w), per[0]);
  return [row(name, problems.length === 0, summary(`worst first view ${worst.path}: ${worst.requests} requests, ${(worst.total / 1024).toFixed(1)} KB brotli-equivalent; fonts ${(fontBytes / 1024).toFixed(1)} KB, one preload; 0 bytes JS`, problems), ctx, per)];
}
