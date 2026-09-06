# Governance

> 🇺🇸 English · [🇧🇷 Português](docs/pt-BR/GOVERNANCE.md)

## Roles

- **Maintainer**: reviews and merges PRs, publishes releases, decides on proposals. Today:
  Emerson Oliveira dos Santos ([@emersonjoe](https://github.com/emersonjoe)).
- **Contributor**: anyone with an accepted PR. Recurring contributors with a history of
  quality reviews may be invited to become maintainers.

## How decisions are made

1. Convention or API proposals start in a proposal issue and, if accepted, become a spec in
   `specs/` (spec → plan → tasks → implement).
2. The [constitution](.specify/memory/constitution.md) prevails. Changing a principle
   requires an explicit amendment (new version of the file, with a rationale), never a
   one-off exception.
3. Disagreements are resolved by consensus in the issue; without consensus, the maintainer
   decides and records why.

## Releases

Semantic versioning. While the project is at 0.x, breaking changes may happen in minor
versions (0.2, 0.3), always listed in `CHANGELOG.md` with the migration path. 1.0 will be
published when the file conventions and the `Ctx`/`h` API stay stable for at least three
minor versions without a breaking change.

What "breaking" means, which packages the promise covers and how a symbol is retired are in
[`API.md`](API.md); the exported surface is versioned in
[`api/current.txt`](api/current.txt) and a test fails when it changes without anyone saying so.

Every release is an annotated `vX.Y.Z` tag with a GitHub release. **Every closed spec
(merged into `main`) produces a release**, so that `go get ...@latest` and the site's
documentation always describe the same API (issue #5).

## Protection of `main`

`main` has a GitHub *ruleset*, in the pattern of community projects:

- deleting the branch and force pushing are forbidden; linear history (fast-forward or
  rebase);
- every change lands through a pull request with **1 approval**, review by the owners in
  `CODEOWNERS`, every conversation resolved and green CI checks (`test (1.22)`,
  `test (1.25)`, `vuln`);
- earlier approvals are dismissed when new commits arrive on the PR.

The maintainer has *bypass* to merge closed specs by fast-forward (the local spec-kit flow)
and for urgent fixes; any use of the bypass is visible in the ruleset history. The files in
this repository are not the source of the rule: it lives in *Settings → Rules*, and changes
to it are announced in a Discussion.

## Usage metrics

We collect the minimum, without cookies and without personal data, and none of it lives in
the code:

- **Site** (emersonjoe.github.io/trilha): page counts with
  [GoatCounter](https://www.goatcounter.com) (free software, no cookies), enabled only when
  the repository variable `SITE_ANALYTICS` exists (`Settings → Secrets and variables →
  Actions → Variables`, value `goatcounter:<code>`). The footer then shows the privacy note
  and the public link to the numbers. To turn it off, delete the variable and republish.
- **Repository**: visits, clones, paths and referrers of the last 14 days
  (`scripts/traffic.sh`, with the maintainer's `gh`). The `traffic` workflow writes a daily
  snapshot to `traffic.jsonl` on the `stats` branch when the `TRAFFIC_TOKEN` secret exists
  (fine-grained token with `Administration: read` on this repository only). Without the
  secret it does nothing.

Stars, forks and watchers are public on the repository page.

## Communication

Issues and Discussions are the official channels. Decisions taken elsewhere are recorded
there before they count.
