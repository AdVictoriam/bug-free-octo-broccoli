# Architecture and decisions

## Single-team product boundary

Bolty Studio is a private writing workspace, not a many-tenant platform. A synchronous API handles short mutations; a durable serialized worker owns expensive model work. Browser requests never wait on an entire generation. The Go service embeds its frontend assets. PDFs are rendered in a bounded Python subprocess. No model runs locally.

Go was selected for a small deployable service, explicit timeouts and bounded concurrency. SQLite removes database operations overhead for one workspace. FTS5 provides lexical reference retrieval without another provider/service; source craft notes are included even when a lexical query misses an excerpt. This is not semantic-vector retrieval. A real local disk, one replica, measured workload and consistent backups are requirements, not optional optimizations.

## Storage and checkpoints

SQLite uses WAL, synchronous FULL, prepared statements and short BEGIN IMMEDIATE transactions. Transactions never contain network work. A process-level volume lock prevents two workers from performing restart recovery against one database. Tables store JSON objects, jobs, provider-call checkpoints, FTS chunks and session hashes. Sources are chunked into 160-word windows with 40-word overlap. At most six selected references and six retrieved excerpts enter a writing run.

Each job records policy/prompt versions, exact configured model IDs, selected approved source content, feedback and the active gold baseline. Writer context excludes gold text. Approved gold is used only by the comparison reviewer. Completed step responses are reused on explicit retry. A request whose outcome is uncertain is not automatically replayed after a restart; the owner must accept possible duplicate billing. This is not exactly-once delivery to an external model provider.

Draft revision numbers and stale-edit checks are allocated transactionally. Canonical Script JSON is the common source for both documents. Saving spoken text or cues creates a new unreviewed revision. Approval binds the exact script hash and checks current settings/latest revision inside a transaction. Export reevaluates evidence freshness and settings; old PDF files already downloaded cannot be remotely revoked.

## Pipeline

Reference analysis extracts craft with exact excerpts from supplied transcripts/observations. A source cannot enter writing memory until the owner approves it. The writer first proposes an outline; preliminary research informs the script. A separate claim auditor then extracts factual mechanics from the complete canonical script, including editor directions and resolution. It does not trust the writer's earlier list.

The research adapter requests native search on official domains. Only actual provider citation objects are accepted as search citations. The backend independently fetches allowlisted pages, hashes the normalized text and records UTC retrieval time. A separate structured pass maps every audited claim to supported/unsupported/unknown and an exact quote. The deterministic gate verifies coverage of the *extracted* claim list, exact quote presence, page provenance and freshness. Semantic support and completeness remain model-mediated.

A creative reviewer evaluates hook, clarity, escalation, voice, payoff, originality, editability and realism. Every dimension must meet the floor. Only candidates without non-baseline blockers proceed to two blind comparisons against gold with candidate order reversed. The candidate must win or tie both. Failed drafts get up to two repair rounds; they are never silently approved after exhausting the budget.

The final owner approves production. Tasks refer to the exact approved draft. Four exports are assembled only after both PDFs are successfully rendered. A PDF renderer failure cannot return a superficially complete ZIP missing a file.

## Cost, limits and failure behavior

One worker; eight queued/running jobs; 30-minute job timeout; four-minute model HTTP timeout; 25-second evidence-fetch timeout; 45-second PDF-render timeout; one PDF export subprocess at a time. Per-run/day logical call reservations are atomic. Default call budgets are 30/run and 90/day. Native web search can incur its own fees. Automatic 429/503 retries are bounded; generic transport failures are not retried silently. Known usage is retained when returned by the provider; unknown costs are not fabricated.

The API body cap is 2 MiB, source transcript cap 180,000 characters, full prompt cap 260,000 characters, research at most 18 claims and six fetched pages capped at 24,000 normalized characters each. Official documentation may be inaccessible, dynamic or too broad; unknown blocks rather than becoming invented proof. Simplify an over-complex story rather than indiscriminately increasing every limit.

## Deliberate omissions / growth path

No fine-tuning, arbitrary YouTube scraping, speech-to-text, video/frame analysis, automatic assets, video rendering, publishing, user roles, billing, invitations or real-time multi-editor merge. The owner can paste visual observations but that is not computer vision. There is no general-purpose PDF-brief importer: this version implements the supplied Bolty contract.

Object listings are bounded to 1,000 per bucket and job history to 100 entries. Historical records are retained, but the UI is not a large archive browser. Before substantial growth, add pagination and archival/data-retention tooling. Before multiple replicas/tenants, replace the process-local ownership boundary with explicit tenant authorization, a scalable database/job-lease design and load-tested infrastructure. Do not deploy this SQLite volume over an arbitrary network filesystem or run it with a serverless ephemeral disk.
