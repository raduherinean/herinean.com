import { readFileSync } from "node:fs";
import { row, summary } from "./row.mjs";
export const name = "Links";
export const modes = ["ci"];
// Row 4: every same-origin href/src resolves (fail); external links are fetched with a timeout and only warn (allowlist bench/links-allow.txt).
export async function run(ctx) {
  const problems = [], warnings = [];
  const allow = readFileSync(new URL("../links-allow.txt", import.meta.url), "utf8").split("\n").map((s) => s.trim()).filter((s) => s && !s.startsWith("#"));
  const internal = new Map(), external = new Map();
  for (const p of ctx.pages) {
    for (const el of p.$("a[href], img[src], link[href], source[srcset], img[srcset]").toArray()) {
      const attr = p.$(el).attr("href") || p.$(el).attr("src") || p.$(el).attr("srcset") || "";
      for (const raw of attr.split(",").map((s) => s.trim().split(" ")[0]).filter(Boolean)) {
        if (raw.startsWith("#") || raw.startsWith("mailto:") || raw.startsWith("data:")) continue;
        const u = new URL(raw, ctx.base + p.path);
        if (u.origin === new URL(ctx.base).origin || u.host === "herinean.com") (internal.get(u.pathname) || internal.set(u.pathname, []).get(u.pathname)).push(p.path);
        else if (!allow.some((a) => u.href.startsWith(a))) (external.get(u.href) || external.set(u.href, []).get(u.href)).push(p.path);
      }
    }
  }
  for (const [path, from] of internal) {
    const res = await fetch(ctx.base + path, { method: "HEAD", redirect: "manual" });
    if (res.status !== 200) problems.push(`${path} → ${res.status} (from ${from[0]})`);
  }
  await Promise.all([...external].map(async ([href, from]) => {
    try {
      const res = await fetch(href, { method: "GET", redirect: "follow", signal: AbortSignal.timeout(10000), headers: { "user-agent": "Mozilla/5.0 (compatible; herinean-bench link check)" } });
      if (res.status >= 400 && res.status !== 403 && res.status !== 429) warnings.push(`${href} → ${res.status} (from ${from[0]})`);
    } catch (e) { warnings.push(`${href}: ${e.name} (from ${from[0]})`); }
  }));
  const value = summary(`${internal.size} internal links resolve; ${external.size} external checked${warnings.length ? `, ${warnings.length} unreachable (warning): ${warnings.slice(0, 2).join("; ")}` : ""}`, problems);
  return [row(name, problems.length === 0, value, ctx, { problems, warnings })];
}
