# Security policy

## Supported versions

Only the latest release (Docker tag `latest`) receives security fixes. Installations using the bundled Watchtower service are updated automatically.

## Reporting a vulnerability

Please **do not open a public issue**. Use GitHub's private vulnerability reporting:
<https://github.com/flocom/SEO-GEO-Report/security/advisories/new>

Include a description, the affected version (`seogeo version` or the home page), reproduction steps and the impact. You will get an answer within a few days; fixes are released as soon as possible and credited if you wish.

## Scope and design notes

- `/mcp` requires an OAuth access token issued by the server or the access password (bearer).
- Report URLs `/r/<token>` are public by design (unguessable 192-bit tokens).
- Favicon fetching refuses private / loopback / link-local addresses unless `FAVICON_ALLOW_PRIVATE=true`.
- `AUTH_DISABLED=true` disables authentication and is meant for local use only.
