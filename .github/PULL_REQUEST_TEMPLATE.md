## What changes

<!-- One sentence. Link to the issue or spec: "Closes #12" / "specs/004-.../spec.md" -->

## How it was tested

- [ ] `make test` green locally
- [ ] `make security` green locally
- [ ] A new convention has a scanner test, a route in `examples/blog` and an integration test
- [ ] Goldens rewritten if the generator changed (`make golden`)
- [ ] Documentation updated in both languages (`site/internal/docs/content/en` and `pt`) when visible behavior changed
- [ ] `CHANGELOG.md` updated under "Unreleased"

## Constitution

<!-- If any principle of .specify/memory/constitution.md is affected, say which one and why. -->

## Security and privacy impact

- [ ] Assets and trust boundaries affected are described, or marked not applicable with a reason.
- [ ] Applicable OWASP ASVS 5.0 Level 2 controls and OWASP Top 10:2025 risks are named.
- [ ] Authentication, authorization, inputs, outputs, secrets, personal data, logs and limits were reviewed.
- [ ] Any exception has an owner, expiry date, compensating control and tracking issue.
