import * as cheerio from "cheerio";
export async function fetchPage(base, path) {
  const res = await fetch(base + path, { redirect: "manual", headers: { "user-agent": "herinean-bench" } });
  const html = await res.text();
  return { url: base + path, path, status: res.status, headers: res.headers, html, $: cheerio.load(html) };
}
export async function loadPages(base, { include404 = false } = {}) {
  const sm = await fetch(base + "/sitemap.xml");
  if (!sm.ok) throw new Error(`sitemap.xml: ${sm.status}`);
  const locs = [...(await sm.text()).matchAll(/<loc>([^<]+)<\/loc>/g)].map((m) => new URL(m[1]).pathname);
  const paths = [...new Set(locs)].sort();
  if (include404) paths.push("/404.html");
  return Promise.all(paths.map((p) => fetchPage(base, p)));
}
export const sitemapPaths = (pages) => pages.filter((p) => p.path !== "/404.html");
