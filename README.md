# herinean.com

[![ci](https://github.com/raduherinean/herinean.com/actions/workflows/ci.yml/badge.svg)](https://github.com/raduherinean/herinean.com/actions/workflows/ci.yml)

The personal site of Radu Herinean — writing for executives, boards and founders about which AI projects are worth funding. English and Romanian.

**Status:** M2a (CI, bench, scorecard pipeline) built; production stays on the placeholder until launch (spec §11).

This repository is the whole platform: a bespoke Go static-site generator, a Cloudflare Worker of about a hundred lines, and the zone configuration as code. Every pull request is audited against the public scorecard by `.github/workflows/ci.yml` (spec §7); merges to `main` deploy once the site is launched (`launched:` in `site.yaml`). The design is in [`docs/specs/2026-09-17-herinean-com-design.md`](docs/specs/2026-09-17-herinean-com-design.md); decisions are in `docs/adr/`; operations in [`RUNBOOK.md`](RUNBOOK.md).

Code: [MIT](LICENSE). Writing under `content/`: [CC BY-NC-ND 4.0](content/LICENSE).
