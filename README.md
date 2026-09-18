# herinean.com

The personal site of Radu Herinean — writing for executives, boards and founders about which AI projects are worth funding. English and Romanian.

**Status:** M1a (generator core, on branch m1a/generator). Nothing is published yet.

This repository is the whole platform: a bespoke Go static-site generator, a ~80-line Cloudflare Worker, the zone configuration as code, and the CI that audits every commit against a public scorecard. The design is in [`docs/specs/2026-09-17-herinean-com-design.md`](docs/specs/2026-09-17-herinean-com-design.md); decisions are in `docs/adr/`; operations in [`RUNBOOK.md`](RUNBOOK.md).

Code: [MIT](LICENSE). Writing under `content/`: [CC BY-NC-ND 4.0](content/LICENSE).
