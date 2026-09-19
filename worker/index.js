// herinean.com edge function. Zero client-side JS is the site's rule; this runs at the edge only.
// Hosts: production apex (assets; one analytics datapoint per HTML view, off the response path; colophon scorecard from KV);
// mta-sts (one file, everything else 404); anything else is a preview (noindex, disallow-all robots, no analytics).
// Any error here degrades to the plain asset response — the site never depends on this code to render.

const MTA_STS_HOST = "mta-sts.herinean.com";
const REFS = new Set(["li", "x", "nl", "md"]);
const BOT_UA = /bot|crawl|spider|slurp|preview|fetch|lighthouse|headless|monitor|curl|wget|python-requests|facebookexternalhit|linkedinbot|twitterbot|whatsapp|telegram|discord|slack|skype|mastodon|http\.rb|bluesky|cardyb|go-http-client|okhttp|axios|node-fetch|java\/|ia_archiver|feed/i;
const isPage = (p) => p.endsWith("/") || p.endsWith(".html");

export default {
  async fetch(request, env, ctx) {
    try {
      const url = new URL(request.url);
      const prodHost = env.PROD_HOST || "herinean.com";

      if (url.host === MTA_STS_HOST) {
        if (url.pathname === "/.well-known/mta-sts.txt") return env.ASSETS.fetch(request);
        return new Response("Not found\n", { status: 404, headers: { "content-type": "text/plain; charset=utf-8" } });
      }

      if (url.host !== prodHost) {
        if (url.pathname === "/robots.txt") {
          return new Response("User-agent: *\nDisallow: /\n", {
            status: 200,
            headers: { "content-type": "text/plain; charset=utf-8", "x-robots-tag": "noindex, nofollow" },
          });
        }
        const res = await withScorecard(request, url, charset(permanent(request, await env.ASSETS.fetch(assetRequest(request, url)))), env);
        const headers = new Headers(res.headers);
        headers.set("x-robots-tag", "noindex, nofollow");
        return new Response(res.body, { status: res.status, statusText: res.statusText, headers });
      }

      if (url.pathname === "/colophon/scorecard.json") return await scorecardJSON(env);

      const res = await withScorecard(request, url, charset(permanent(request, await env.ASSETS.fetch(assetRequest(request, url)))), env);
      // A view is a GET for a page path answered 200 or 304: revalidations are the returning readers, and a 304 carries no content-type.
      if (request.method === "GET" && (res.status === 200 || res.status === 304) && isPage(url.pathname)) {
        ctx.waitUntil(Promise.resolve().then(() => count(request, url, env)).catch(() => {}));
      }
      return res;
    } catch {
      return env.ASSETS.fetch(request); // whatever failed above, the reader gets the page
    }
  },
};

// The colophon's validator is composed below, so the asset layer must not answer its conditional requests itself.
function assetRequest(request, url) {
  if (url.pathname !== "/colophon/" || !request.headers.has("if-none-match")) return request;
  const headers = new Headers(request.headers);
  headers.delete("if-none-match");
  return new Request(request, { headers });
}

// The asset layer canonicalises paths with 307s (html_handling); the site's URLs are permanent, so readers and crawlers get a 301.
function permanent(request, res) {
  if (res.status !== 307 || !res.headers.has("location") || (request.method !== "GET" && request.method !== "HEAD")) return res;
  const headers = new Headers(res.headers);
  headers.delete("content-length");
  return new Response(null, { status: 301, headers });
}

// The asset layer sends pages and robots.txt without a charset (_headers already types llms.txt and security.txt); the Romanian pages are UTF-8 and say so.
function charset(res) {
  const ct = res.headers.get("content-type");
  if (ct !== "text/html" && ct !== "text/plain") return res;
  const headers = new Headers(res.headers);
  headers.set("content-type", ct + "; charset=utf-8");
  return new Response(res.body, { status: res.status, statusText: res.statusText, headers });
}

// A view is a navigation a person made: speculative loads (Chrome's prefetch/prerender, Safari's and Firefox's
// Purpose: prefetch) and non-document fetches (an <iframe> embed, a fetch() from someone else's page) are not.
function speculative(request) {
  const purpose = (request.headers.get("sec-purpose") || request.headers.get("purpose") || "").toLowerCase();
  if (purpose.includes("prefetch") || purpose.includes("prerender")) return true;
  const dest = request.headers.get("sec-fetch-dest");
  return dest !== null && dest !== "document";
}

// count writes: path, lang, referrer host, ref, country, 1. Never IP, user agent or the full referrer (spec §6.3; the privacy page says exactly this).
function count(request, url, env) {
  const ua = request.headers.get("user-agent");
  if (!env.VIEWS || !ua || BOT_UA.test(ua) || speculative(request)) return;
  const tag = url.searchParams.get("ref");
  const ref = tag === null ? "none" : REFS.has(tag) ? tag : "other";
  let refHost = "";
  try {
    const r = request.headers.get("referer");
    if (r) refHost = new URL(r).hostname;
  } catch {}
  if (refHost === url.host) refHost = ""; // internal navigation is not a referral
  const path = url.pathname.slice(0, 96); // index limit
  env.VIEWS.writeDataPoint({
    blobs: [path, path.startsWith("/ro/") ? "ro" : "en", refHost, ref, (request.cf && request.cf.country) || ""],
    doubles: [1],
    indexes: [path],
  });
}

// If-None-Match may carry a list and weak forms; the entity tag is what has to match.
const matches = (inm, etag) => (inm || "").split(",").map((s) => s.trim().replace(/^W\//, "")).includes(etag.replace(/^W\//, ""));

// With no fragment to compose, the asset's own validator is still honoured: the conditional header was stripped upstream.
function unchanged(request, res) {
  const etag = res.headers.get("etag");
  if (!etag || !matches(request.headers.get("if-none-match"), etag)) return res;
  const headers = new Headers(res.headers);
  headers.delete("content-length");
  return new Response(null, { status: 304, headers });
}

// withScorecard replaces <table data-scorecard> on the colophon with the KV fragment (rendered and validated by CI, spec §7).
// The ETag becomes asset etag + fragment hash (the key's `etag` metadata, written by scripts/scorecard-publish.sh), so conditional
// requests still get 304s (row 17) — one KV read for the fragment and its validator together.
// No KV value, or any failure: the built-in table (the same rows as of the last build) stays, but the asset's own ETag still answers a 304.
async function withScorecard(request, url, res, env) {
  if (url.pathname !== "/colophon/" || res.status !== 200) return res;
  if (!env.SCORECARD) return unchanged(request, res);
  try {
    const { value: html, metadata } = await env.SCORECARD.getWithMetadata("scorecard");
    if (!html) return unchanged(request, res);
    const tag = (metadata && metadata.etag) || "kv";
    const headers = new Headers(res.headers);
    headers.delete("content-length");
    const etag = `W/"${(res.headers.get("etag") || "").replace(/^W\/|"/g, "")}-${tag}"`;
    headers.set("etag", etag);
    if (matches(request.headers.get("if-none-match"), etag)) return new Response(null, { status: 304, headers });
    return new HTMLRewriter()
      .on("table[data-scorecard]", { element(el) { el.replace(html, { html: true }); } })
      .transform(new Response(res.body, { status: res.status, headers }));
  } catch {
    return unchanged(request, res);
  }
}

async function scorecardJSON(env) {
  const body = env.SCORECARD ? await env.SCORECARD.get("scorecard.json").catch(() => null) : null;
  if (!body) return new Response("Not found\n", { status: 404, headers: { "content-type": "text/plain; charset=utf-8" } });
  return new Response(body, { headers: { "content-type": "application/json; charset=utf-8", "cache-control": "public, max-age=300", "access-control-allow-origin": "*", "x-content-type-options": "nosniff" } });
}
