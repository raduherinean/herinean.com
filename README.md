# herinean.com

The personal site of Radu Herinean — writing for executives, boards and founders about which AI projects are worth funding. English and Romanian.

**Status:** M0 (edge), M1a (generator core) and M1b (edge function, fonts, preview) built; production stays on the placeholder until launch (spec §11).

This repository is the whole platform: a bespoke Go static-site generator, a Cloudflare Worker of about a hundred lines, and the zone configuration as code. The CI that will audit every commit against a public scorecard is designed in the spec (§7) and is the next milestone, M2. The design is in [`docs/specs/2026-09-17-herinean-com-design.md`](docs/specs/2026-09-17-herinean-com-design.md); decisions are in `docs/adr/`; operations in [`RUNBOOK.md`](RUNBOOK.md).

Code: [MIT](LICENSE). Writing under `content/`: [CC BY-NC-ND 4.0](content/LICENSE).
