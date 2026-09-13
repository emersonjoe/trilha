# Secure development baseline

> English · [Português](docs/pt-BR/SECURITY-BASELINE.md)

This baseline applies to every Trilha change. It uses NIST SSDF 1.1 for the development
process, OWASP ASVS 5.0 Level 2 for applicable web controls, and OWASP Top 10:2025 for threat
discovery. It does not claim that Trilha or an application built with it is certified.

## Required evidence

| Change area | Required control and evidence | Reference |
|---|---|---|
| Design | Identify assets, actors, trust boundaries, abuse cases and data classification in the spec | SSDF PO.1, PW.1; ASVS V15 |
| Authentication and sessions | Generic failures, rate limits, secure cookie/token lifecycle and tests | ASVS V6, V7, V9, V10 |
| Authorization and tenancy | Deny by default, enforce server-side on every object/action, test cross-tenant denial | ASVS V8; Top 10 A01 |
| Inputs, outputs and files | Positive validation, contextual encoding, bounded bodies and safe paths | ASVS V1, V2, V4, V5; Top 10 A05 |
| Secrets and cryptography | No repository/build secrets, least privilege, rotation and approved primitives | ASVS V11, V13; Top 10 A04 |
| Logs and errors | Security events without credentials, tokens, request bodies or personal-data values | ASVS V14, V16; Top 10 A09 |
| Dependencies and build | Review dependencies, pin CI actions and run `govulncheck` with the patched security toolchain | SSDF PS.1, PW.4; Top 10 A03 |
| Verification and response | Run `make test` and `make security`; triage privately and track remediation | SSDF PW.7, PW.8, RV.1-RV.3 |

Every pull request names the applicable rows. “Not applicable” needs a reason. An exception needs
an owner, expiry date, compensating control and issue; expired exceptions block release.

## Self-hosted CI boundary

- `push` to `main` and `workflow_dispatch` may use `RUNNER_CI=eoslab`; `pull_request` always uses
  `ubuntu-latest`.
- Never use `pull_request_target` to execute contributor code on a persistent runner.
- The service account is repository-specific and has neither `sudo` nor `docker` membership.
- Checkouts are clean, credentials are not persisted, job tokens use minimum permissions and Go
  caches live under `RUNNER_TEMP` and are removed after each self-hosted job.
- The Pages build may run on `eoslab`; the privileged Pages deployment remains GitHub-hosted with
  job-scoped `pages: write` and `id-token: write` permissions.
- Production secrets do not live on the runner. On compromise: stop the service, remove the
  runner in GitHub, delete its directory, rotate exposed credentials and reprovision.

## References

- [NIST SP 800-218, Secure Software Development Framework (SSDF) Version 1.1](https://csrc.nist.gov/pubs/sp/800/218/final).
- [OWASP Application Security Verification Standard 5.0.0](https://github.com/OWASP/ASVS/tree/v5.0.0_release/5.0).
- [OWASP Top 10:2025](https://owasp.org/Top10/2025/).
