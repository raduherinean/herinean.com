// herinean.com edge function. Zero client-side JS is the site's rule; this runs at the edge only.
// Hosts: production apex; mta-sts (one file, everything else 404); anything else is a preview (noindex, no analytics).

const PROD_HOST = "herinean.com";
const MTA_STS_HOST = "mta-sts.herinean.com";

export default {
  async fetch(request, env) {
    const url = new URL(request.url);

    if (url.host === MTA_STS_HOST) {
      if (url.pathname === "/.well-known/mta-sts.txt") return env.ASSETS.fetch(request);
      return new Response("Not found\n", { status: 404, headers: { "content-type": "text/plain; charset=utf-8" } });
    }

    if (url.host !== PROD_HOST) {
      if (url.pathname === "/robots.txt") {
        return new Response("User-agent: *\nDisallow: /\n", {
          status: 200,
          headers: { "content-type": "text/plain; charset=utf-8", "x-robots-tag": "noindex, nofollow" },
        });
      }
      const res = await env.ASSETS.fetch(request);
      const headers = new Headers(res.headers);
      headers.set("x-robots-tag", "noindex, nofollow");
      return new Response(res.body, { status: res.status, statusText: res.statusText, headers });
    }

    return env.ASSETS.fetch(request);
  },
};
