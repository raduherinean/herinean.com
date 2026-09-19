import { chromium } from "playwright";
// One browser per audit run, launched on first use; every module that needs a page gets its own context.
export function makeBrowser() {
  let b;
  const get = async () => (b ??= await chromium.launch());
  get.close = async () => { if (b) await b.close(); };
  return get;
}
