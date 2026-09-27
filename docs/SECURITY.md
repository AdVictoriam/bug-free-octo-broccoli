# Security and operating boundary

This is a private, single-workspace application for one trusted team. It is not an audited multi-tenant product. There are no per-user roles, account recovery, SSO, invitations or access separation between case records and scripts.

## Implemented controls

A randomly generated studio password is recommended; startup requires at least 20 characters. The server compares credentials without exposing them to client scripts, rate-limits login attempts by the direct remote address, and issues random session cookies. Only token hashes are persisted. Cookies are HttpOnly, SameSite=Strict, expire after 24 hours and are Secure on HTTPS. Every mutation requires the configured exact Origin and JSON content type. The UI escapes source/model content before inserting it in HTML. Static resources are local, with a restrictive Content Security Policy.

Keys and YouTube OAuth secrets are environment variables and never returned to the browser. Do not place them in screenshots, transcripts, git or source-form fields. Source content, prompts, drafts and evidence are private data: approved selected content is sent to the configured model providers. OpenAI requests set store=false; this alone is not a claim of zero retention, a special contractual data setting or identical policies across providers. Review your own provider account settings and agreements.

The evidence fetcher allows only HTTPS on exact configured official domains, rejects userinfo/alternate ports/private IPs, resolves and pins public destination addresses, bounds redirects, and limits response size/time. Discord's general host is further path-restricted. Native provider citations are required before fetching; strings resembling URLs in a model answer are not accepted as citations. Provider/source text is labelled untrusted and never grants tool authority. These controls reduce risk; they are not a proof against all prompt injection.

The runtime uses prepared SQL, a single-owner disk lock, durable queued work and bounded network calls. PDF generation has a fixed script path, no shell interpolation, escaped text, a fixed input schema and a timeout. Containers drop capabilities and run as a non-root user. Dependency and image updates remain the deployer's responsibility.

## Deployment rules

Keep one replica on a persistent local disk. Bind to localhost or a private network and terminate remote HTTPS at a trusted reverse proxy. PUBLIC_URL must exactly match the browser origin and may be HTTP only for loopback. Do not trust arbitrary forwarded IP headers. The built-in direct-IP login limiter may group users behind a proxy; add appropriately configured edge controls when exposing the service.

Filesystem/database backups are not encrypted by the application. Use encrypted disks/backups and restrict host/container access. The database includes plaintext source content, model outputs and case text. The source “rights” checkbox is an attestation, not an automated license check. Redaction must happen before upload.

Archiving a source excludes it from future retrieval but intentionally retains frozen historical job/draft snapshots. There is no selective historical-data erasure UI. Do not represent archive as permanent deletion or a privacy-law compliance mechanism. Plan retention before importing sensitive data.

Changing APP_PASSWORD does not invalidate existing stored sessions automatically. To revoke all sessions, stop the service, back up the database, run `DELETE FROM sessions` against the database, rotate the password, then restart. Apply the same session-revocation step after restoring a backup.

Never copy only studio.db during live WAL activity. Use the SQLite backup API, protect the resulting file, and test recovery. Stopped or interrupted model requests may already have incurred provider charges. Retrying does not guarantee exactly-once billing.

## Not independently verified here

Public TLS/reverse-proxy configuration, Docker image execution, external-provider authentication, official-doc fetch behavior through the deployment network, real OAuth, native browser navigation/cookie/CSP enforcement in the restricted authoring browser, internet-facing penetration testing, load/soak tests and an operational backup-restore drill. These are deployment acceptance tasks, not properties established by passing the local unit suite.
