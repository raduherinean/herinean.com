import lighthouse from "lighthouse";
import desktopConfig from "lighthouse/core/config/desktop-config.js";
import { launch } from "chrome-launcher";
import { chromium } from "playwright";
import { row } from "./row.mjs";

export const name = "Lighthouse";
export const modes = ["ci", "post"];

const CATS = ["performance", "accessibility", "best-practices", "seo"];
const median = (xs) => { const s = [...xs].sort((a, b) => a - b); return s[Math.floor(s.length / 2)]; };

// Row 1: every sitemap page (never /404.html), mobile + desktop, 3 runs, median per category, all
// four = 100. CI runs this against the local `site serve --static` build (gzip on), never a
// preview host. Sharded: `ctx.shard` ("i/n") slices the page list; `ctx.formFactor` restricts to
// one form factor (default both). One row per form factor per shard invocation; merge.mjs folds
// the shards (named "Lighthouse [<mobile|desktop> i/n]") back into a single "Lighthouse" row.
export async function run(ctx) {
  const [i, n] = ctx.shard.split("/").map(Number);
  const all = ctx.pages.filter((p) => p.path !== "/404.html").map((p) => p.path);
  const mine = all.filter((_, k) => k % n === i - 1);
  const ffs = ctx.formFactor ? [ctx.formFactor] : ["mobile", "desktop"];
  const checkFor = (ff) => `${ctx.mode === "post" ? name + " (production)" : name} [${ff} ${i}/${n}]`;
  // An empty shard (n greater than the page count) must not pass silently — merge.mjs folds this
  // shard's `pass` straight into the merged row, so a quietly-green empty shard would hide a real
  // gap instead of failing loud (the bench's "empty audit scope is a red row" convention).
  if (mine.length === 0) {
    return ffs.map((ff) => ({ ...row(checkFor(ff), false, `shard ${i}/${n} has no pages (${all.length} in the sitemap)`, ctx), pages: 0, worst: 0 }));
  }
  const chrome = await launch({ chromePath: chromium.executablePath(), chromeFlags: ["--headless=new", "--no-sandbox", "--disable-gpu"] });
  const rows = [];
  try {
    for (const ff of ffs) {
      const problems = [], failingAudits = [], scores = {};
      let worst = 100;
      for (const path of mine) {
        const runs = [];
        for (let r = 0; r < 3; r++) {
          const res = await lighthouse(
            ctx.base + path,
            {
              port: chrome.port,
              output: "json",
              logLevel: "error",
              onlyCategories: CATS,
              formFactor: ff,
              screenEmulation: ff === "desktop" ? desktopConfig.settings.screenEmulation : undefined,
              throttling: ff === "desktop" ? desktopConfig.settings.throttling : undefined,
            },
            ff === "desktop" ? desktopConfig : undefined,
          );
          runs.push(Object.fromEntries(CATS.map((c) => [c, Math.round((res.lhr.categories[c].score ?? 0) * 100)])));
          // Only the first run's audit list is recorded — the three runs agree on which audits are
          // binary failures far more often than on the exact score, and one list is enough to point
          // at the offending audit ids from the detail artifact.
          if (r === 0) for (const [id, a] of Object.entries(res.lhr.audits)) if (a.score !== null && a.score < 1 && a.scoreDisplayMode === "binary") failingAudits.push(`${path} ${id}`);
        }
        const med = Object.fromEntries(CATS.map((c) => [c, median(runs.map((x) => x[c]))]));
        scores[path] = med;
        const failing = CATS.filter((c) => med[c] < 100);
        if (failing.length) problems.push(`${path}: ${failing.map((c) => `${c} ${med[c]}`).join(", ")}`);
        worst = Math.min(worst, ...Object.values(med));
      }
      const value = problems.length ? problems.join("; ") : `${mine.length} pages ${ff}: 100/100/100/100`;
      rows.push({ ...row(checkFor(ff), problems.length === 0, value, ctx, { failingAudits, scores }), pages: mine.length, worst });
    }
  } finally {
    await chrome.kill();
  }
  return rows;
}
