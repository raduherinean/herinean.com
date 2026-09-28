import { row, summary } from "./row.mjs";
export const name = "Social previews";
export const modes = ["ci"];
const meta = ($, sel) => $(`meta[property="${sel}"], meta[name="${sel}"]`).attr("content");
// Row 7: og:* + twitter:card=summary_large_image on every page; og:image is a 1200×630 PNG < 200 KB at a content-hashed URL.
export async function run(ctx) {
  const problems = [];
  const images = new Map();
  for (const p of ctx.pages.filter((p) => p.path !== "/404.html")) {
    for (const k of ["og:title", "og:description", "og:url", "og:image", "og:type", "og:locale"]) if (!meta(p.$, k)) problems.push(`${p.path}: no ${k}`);
    if (meta(p.$, "twitter:card") !== "summary_large_image") problems.push(`${p.path}: twitter:card is ${meta(p.$, "twitter:card")}`);
    const img = meta(p.$, "og:image");
    if (!img) continue;
    // Pages carry absolute production URLs even when CI serves them from 127.0.0.1 (the fetch
    // below goes to ctx.base + pathname for that reason), so "own origin" is the page's canonical
    // origin, not ctx.base's: an og:image hosted anywhere else is a card that breaks when that
    // host does, and a preview scraper that is not this site's to reason about.
    const canonical = p.$('link[rel="canonical"]').attr("href");
    let origin; try { origin = new URL(canonical).origin; } catch { origin = null; }
    let imgOrigin; try { imgOrigin = new URL(img).origin; } catch { problems.push(`${p.path}: og:image ${img} is not an absolute URL`); continue; }
    if (!origin) problems.push(`${p.path}: no canonical to hold og:image ${img} against`);
    else if (imgOrigin !== origin) problems.push(`${p.path}: og:image ${img} is not on the site's own origin (${origin})`);
    images.set(img, p.path);
  }
  for (const [img, from] of images) {
    const u = new URL(img);
    if (!/^\/og\/[A-Za-z0-9._-]+\.[0-9a-f]{8}\.png$/.test(u.pathname)) problems.push(`${from}: og:image ${u.pathname} is not a hashed /og/ PNG`);
    const res = await fetch(ctx.base + u.pathname);
    if (!res.ok) { problems.push(`${from}: og:image ${u.pathname} → ${res.status}`); continue; }
    const buf = Buffer.from(await res.arrayBuffer());
    if (buf.length >= 200 * 1024) problems.push(`${from}: og:image ${buf.length} bytes ≥ 200 KB`);
    if (buf.readUInt32BE(16) !== 1200 || buf.readUInt32BE(20) !== 630) problems.push(`${from}: og:image is ${buf.readUInt32BE(16)}×${buf.readUInt32BE(20)}, want 1200×630`);
  }
  return [row(name, problems.length === 0, summary(`${ctx.pages.length - 1} pages, ${images.size} OG images: 1200×630 PNG < 200 KB, hashed URLs on the site's own origin`, problems), ctx, problems)];
}
