# Security policy

> 🇺🇸 English · [🇧🇷 Português](docs/pt-BR/SECURITY.md)

## Supported versions

| Version | Support |
|---|---|
| latest 0.x | security fixes |
| earlier | no |

## How to report

**Do not open a public issue.** Use GitHub's
[private vulnerability report](https://github.com/emersonjoe/trilha/security/advisories/new).
You will get an answer within 72 hours with the initial assessment and, if confirmed, an
estimate for the fix. Credit goes to the reporter, in the advisory and in `CHANGELOG.md`,
unless asked otherwise.

## What is in scope

The runtime (`trilha`, `h`, `tmpl`), the CLI and the generated file. Examples and the
documentation site are welcome, but with lower priority.

## Guarantees the framework offers by default

- HTML escaping in text and attributes (`h`) and contextual escaping (`tmpl`).
- `Content-Security-Policy` with a per-request nonce; HSTS over HTTPS; `X-Frame-Options`,
  `X-Content-Type-Options`, `Referrer-Policy`, `Permissions-Policy`,
  `Cross-Origin-Opener-Policy`.
- CSRF protection by *double-submit cookie* on write methods of pages.
- Signed cookies (HMAC-SHA256, with expiry and key rotation).
- Request body limit (1 MiB by default), timeouts and header limit.
- Per-client rate limiting (optional) and client IP only through trusted proxies.
- Static files restricted to `public/` (no *path traversal*).
- Errors in production without stack or file paths; logs without body or cookies; security
  events (CSRF, 401/403, 413, 429, panic) logged and exposed through a hook.

The mapping of these controls to NIST CSF 2.0 and OWASP ASVS 4.0 is in the documentation:
<https://emersonjoe.github.io/trilha/learn/security>.

A report showing any of these guarantees failing is treated as a vulnerability.
