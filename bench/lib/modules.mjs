// The row modules audit.mjs runs, by audit scope (--only checks | lighthouse). Kept apart from audit.mjs so a
// test can import it: every name here must have a row in merge.mjs's ORDER for each mode it declares, or a
// row that stops arriving would be neither sorted nor expected — silently absent instead of red.
export const MODULES = {
  checks: ["i18n", "social", "wellknown", "feeds", "jsonld", "headers", "privacy", "weight", "links", "html", "a11y", "fonts", "transport", "observatory", "dns", "caching"],
  lighthouse: ["lighthouse"],
};
