import { row, summary } from "./row.mjs";
export const name = "Privacy";
export const modes = ["ci", "post"];
// Row 15: no Set-Cookie, no third-party request, no /cdn-cgi/, no executable script (every <script> is ld+json) — measured in a real browser.
export async function run(ctx) {
  const problems = [];
  const origin = new URL(ctx.base).origin;
  const browser = await ctx.browser();
  const bctx = await browser.newContext();
  try {
    for (const p of ctx.pages.filter((p) => p.path !== "/404.html")) {
      const page = await bctx.newPage();
      try {
        const third = [], cookies = [];
        page.on("request", (r) => { if (new URL(r.url()).origin !== origin) third.push(r.url()); });
        page.on("response", (r) => { if (r.headers()["set-cookie"]) cookies.push(r.url()); });
        await page.goto(ctx.base + p.path, { waitUntil: "networkidle" });
        if (third.length) problems.push(`${p.path}: third-party requests: ${third.slice(0, 2).join(", ")}`);
        if (cookies.length) problems.push(`${p.path}: Set-Cookie from ${cookies[0]}`);
        if ((await bctx.cookies()).length) problems.push(`${p.path}: a cookie was stored`);
        if (p.html.includes("/cdn-cgi/")) problems.push(`${p.path}: /cdn-cgi/ in HTML`);
        const scripts = p.$("script").toArray().filter((s) => p.$(s).attr("type") !== "application/ld+json");
        if (scripts.length) problems.push(`${p.path}: ${scripts.length} executable <script>`);
      } finally {
        await page.close();
      }
    }
  } finally {
    await bctx.close();
  }
  const check = ctx.mode === "post" ? `${name} (production)` : name;
  return [row(check, problems.length === 0, summary(`${ctx.pages.length - 1} pages: 0 cookies, 0 third-party requests, 0 executable scripts, no /cdn-cgi/`, problems), ctx, problems)];
}
