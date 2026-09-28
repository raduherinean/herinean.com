// row.mjs — the one shape every module returns; identical to internal/site/scorecard.go's ciRow.
export function row(check, pass, value, ctx, detail) {
  const r = { check, pass: !!pass, value, when: ctx.when, link: ctx.link || "", link_text: ctx.link ? ctx.linkText || "CI run" : "" };
  if (detail !== undefined) r.detail = detail;
  return r;
}
export const today = () => new Date().toISOString().slice(0, 10);
// summarise a list of problems into a value string: "ok" or "N problems: first; second; …"
export function summary(okText, problems, max = 3) {
  if (problems.length === 0) return okText;
  const shown = problems.slice(0, max).join("; ");
  return `${problems.length} problem${problems.length === 1 ? "" : "s"}: ${shown}${problems.length > max ? "; …" : ""}`;
}
