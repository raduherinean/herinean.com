---
title: "How governance decisions show up in the build pipeline"
date: 2026-09-15
key: media
pillar: governance
summary: "Every AI-assisted commit on this site carries a visible trail: what was drafted, what was reviewed, and what the build refuses to ship without sign-off."
---
Using AI assistance to draft parts of a site raises a governance question before it raises a technical one: how do you make the review trail visible instead of trusting that someone remembered to check? The answer here is to push as many checks as possible into the build itself, so a missing review shows up as a failed build rather than a quiet gap.

![A screenshot of the build log output](fig.png "The four checks every commit passes through")

The screenshot above is from an actual run: front matter validation, diacritics checking, alt-text enforcement, and the i18n completeness check, each reported with the file and line that triggered it. None of these checks are specific to AI-assisted drafts — they apply to everything, including this paragraph — which is the point: the governance model does not ask "was this AI-written?" so much as "did it pass the same gate as everything else?" That framing traces back to [the piece that started this build log](/writing/paired/), where the decision to write a bespoke generator was made for exactly this kind of control.

A small icon marks pages that carry this disclosure: ![Diagram](diagram.svg), rendered next to the colophon link in the footer of every page. Reusing one visual element across the whole site keeps the disclosure legible rather than turning it into another paragraph of legal text nobody reads.

The check that enforces this is small enough to show in full[^enforcement]:

```go
func requireDisclosure(pages []*Page) error {
	for _, p := range pages {
		if p.Kind == "colophon" && !strings.Contains(string(p.Body), "AI") {
			return fmt.Errorf("%s: colophon must disclose AI assistance", p.File)
		}
	}
	return nil
}
```

The table below lists where each governance check lives in the pipeline, with the stage that runs it first.

| Check | Where it runs |
|---:|:---|
| Alt text present | `ParseBody` |
| Front matter valid | `ParseFrontMatter` |
| i18n keys complete | `loadStrings` |
| AI disclosure present | `check --dist` |

None of this replaces editorial judgment. It just makes sure the judgment gets applied every time, not only when someone remembers to look.

[^enforcement]: The real check also verifies the disclosure text matches `site.yaml`'s `ai_disclosure` field for the page's language; this listing keeps the illustration short.
