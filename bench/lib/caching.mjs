import * as cheerio from "cheerio";
import { row, summary } from "./row.mjs";
export const name = "Caching (production)";
export const modes = ["post"];

// Pure decision function over plain fetch results (no Headers objects, no network) — unit-tested
// with canned inputs.
export function cachingProblems(r) {
  const problems = [];
  if (r.home.status !== 200) problems.push(`/: ${r.home.status}`);
  else {
    if (r.home.cacheControl !== "public, max-age=0, must-revalidate") problems.push(`/: cache-control = ${r.home.cacheControl ?? "<absent>"}`);
    if (!r.home.etag) problems.push("/: no etag");
    else if (r.home.notModifiedStatus !== 304) problems.push(`/: If-None-Match round trip got ${r.home.notModifiedStatus ?? "<no response>"}, want 304`);
  }
  if (!r.font.found) problems.push("/: no preloaded font link found");
  else if (r.font.status !== 200 || r.font.cacheControl !== "public, max-age=31536000, immutable") problems.push(`font: ${r.font.status} cache-control = ${r.font.cacheControl ?? "<absent>"}`);
  if (!r.ogImage.found) problems.push("/: no og:image found");
  else if (r.ogImage.status !== 200 || r.ogImage.cacheControl !== "public, max-age=31536000, immutable") problems.push(`og:image: ${r.ogImage.status} cache-control = ${r.ogImage.cacheControl ?? "<absent>"}`);
  if (r.feed.status !== 200 || r.feed.cacheControl !== "public, max-age=300" || r.feed.contentType !== "application/rss+xml; charset=utf-8") problems.push(`/feed.xml: ${r.feed.status} cache-control=${r.feed.cacheControl ?? "<absent>"} content-type=${r.feed.contentType ?? "<absent>"}`);
  if (r.securityTxt.status !== 200 || r.securityTxt.contentType !== "text/plain; charset=utf-8") problems.push(`/.well-known/security.txt: ${r.securityTxt.status} ${r.securityTxt.contentType ?? "<absent>"}`);
  if (r.colophon.status !== 200) problems.push(`/colophon/: ${r.colophon.status}`);
  else {
    if (!r.colophon.etag || !/^W\/"[0-9a-f]+-[0-9a-f]{8}"$/.test(r.colophon.etag)) problems.push(`/colophon/: etag = ${r.colophon.etag ?? "<absent>"}, want the composed W/"<hex>-<8hex>" form`);
    if (r.colophon.notModifiedStatus !== 304) problems.push(`/colophon/: If-None-Match round trip got ${r.colophon.notModifiedStatus ?? "<no response>"}, want 304`);
  }
  return problems;
}

const get = (url, headers) => fetch(url, { redirect: "manual", headers });

// Row 17: cache-control per path class, etag round trips (If-None-Match -> 304 on `/` and the
// composed `/colophon/` etag), immutable fonts/OG image, short-lived feed, plain-text security.txt.
// Fetches `/` itself rather than relying on ctx.pages — in post mode against the placeholder
// there may be no sitemap at all (see bench/lib/pages.mjs), so this row must stand on its own.
export async function run(ctx) {
  const home = await get(ctx.base + "/");
  const homeEtag = home.headers.get("etag");
  const home304 = homeEtag ? await get(ctx.base + "/", { "if-none-match": homeEtag }) : null;
  const homeHtml = home.status === 200 ? await home.text() : "";
  const $ = cheerio.load(homeHtml);

  const fontHref = $('link[rel="preload"][as="font"]').attr("href");
  const font = fontHref ? await get(new URL(fontHref, ctx.base).href) : null;

  const ogContent = $('meta[property="og:image"]').attr("content");
  const ogPath = ogContent ? new URL(ogContent, ctx.base).pathname : null;
  const ogImage = ogPath ? await get(ctx.base + ogPath) : null;

  const feed = await get(ctx.base + "/feed.xml");
  const securityTxt = await get(ctx.base + "/.well-known/security.txt");

  const colophon = await get(ctx.base + "/colophon/");
  const colophonEtag = colophon.headers.get("etag");
  const colophon304 = colophonEtag ? await get(ctx.base + "/colophon/", { "if-none-match": colophonEtag }) : null;

  const responses = {
    home: { status: home.status, cacheControl: home.headers.get("cache-control"), etag: homeEtag, notModifiedStatus: home304?.status ?? null },
    font: { found: !!fontHref, status: font?.status ?? null, cacheControl: font?.headers.get("cache-control") ?? null },
    ogImage: { found: !!ogPath, status: ogImage?.status ?? null, cacheControl: ogImage?.headers.get("cache-control") ?? null },
    feed: { status: feed.status, cacheControl: feed.headers.get("cache-control"), contentType: feed.headers.get("content-type") },
    securityTxt: { status: securityTxt.status, contentType: securityTxt.headers.get("content-type") },
    colophon: { status: colophon.status, etag: colophonEtag, notModifiedStatus: colophon304?.status ?? null },
  };
  const problems = cachingProblems(responses);
  return [row(name, problems.length === 0, summary("/ + colophon etag round trips 304; font/og:image immutable 1y; feed 5 min; security.txt text/plain", problems), ctx, responses)];
}
