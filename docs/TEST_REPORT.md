# Build validation record

**Date:** 26 September 2026. **Scope:** the code delivered in this archive, in a local Linux authoring runtime. A passing test is evidence for its exercised behavior, not a blanket reliability or editorial-quality guarantee.

## Executed successfully

| Check | Result |
|---|---|
| `go build ./cmd/bolty` / `make build` | Native binary compiled and served the actual API |
| `go test ./...` | **27 top-level tests passed; 69 passed test/subtest events; zero failures** |
| `go test -race ./...` | Passed; no race detector findings in the exercised paths |
| Paid provider test | One top-level live-provider test deliberately skipped: no credentials or authorization to incur charges |
| `go vet ./...` | Passed |
| `node --check web/app.js` | Passed |
| Python syntax compilation | Setup/run/backup/PDF/browser helpers passed |
| Browser UI + real API | Six routes at 1512px desktop and 390px mobile; no horizontal overflow; zero JavaScript page errors |
| Browser source/story workflow | Saved an actual source, verified literal markup escaping, created a real project, confirmed missing configuration blocked AI, confirmed a failed analysis did not cause duplicate source creation |
| Authenticated API ZIP | Real PDF subprocess plus exact four-file Markdown/PDF assembly passed |
| PDF structural export | Two real PDFs rendered: 13-page VA and 15-page editor synthetic layout fixture; 80 sequential IDs in each; correct color presence; text within page bounds; VA excludes editor directions |
| PDF visual review | First, middle and final pages inspected after rasterization; purple second-take vector star visible; no clipping or missing-glyph blocks observed |

The 69 event count includes nested subtests, not 69 independently authored top-level tests. The synthetic fixture intentionally repeats filler and contains synthetic evidence and scores **inside tests only**. It is not preloaded in the product, not a model-generated sample and not evidence of creative quality. Tests use local transport doubles to exercise provider request/response contracts; these are not live provider integrations.

## What the Go suite exercises

Thirty-six adverse mutations test the delivery rules: wrong length/title/origin, forbidden scope, identifying information, named platforms in narration, editor directions in VA text, duplicate/missing IDs, incorrect cues/holds/runtime, missing silence, bad thumbnail, third-party asset declaration, missing disclosure/case, unverified mechanics, invented quotes, stale/tampered evidence, missing/extra verdicts, reference overlap, absent/invalid critique, insufficient scores, missing gold, incomplete comparisons and lost comparisons.

Other tests cover prepared SQL with hostile/NUL text, rollback/panic recovery, concurrent writes, approved-only retrieval, immutable snapshots, held-out gold, concurrent revision allocation, stale edits, job/call idempotency, budget reservations, single job claiming, restart interruption, native citation requirements, refusal/truncation rejection, non-replay of transport failure, authentication/Origin/logout, missing keys, source approval invalidation, exact-version sign-off, blocked exports and production-task idempotency.

## Browser restriction and exact workaround

The available Chromium is managed with a policy that blocks all URL navigation; a direct localhost navigation returned `ERR_BLOCKED_BY_ADMINISTRATOR`. The policy was not altered. For the executed browser checks, Playwright rendered the actual local HTML structure/CSS/JavaScript into an offline page and bridged only `/api/*` requests to the actual local Go server. The bridge used a real HTTP cookie jar and actual Origin checks; it did not invent application data or model responses.

This verifies layout, DOM behavior and those real API workflows. It **does not verify native browser navigation, cookie handling, CSP enforcement or browser download behavior**. Server authentication/Origin controls were exercised in Go HTTP tests. The included browser test defaults to native navigation on an unrestricted machine; the restricted rendering mode requires the explicit `BOLTY_BROWSER_BRIDGE=1` flag.

## Environment and unexecuted deployment tests

Executed with Go 1.23.2, Node 22.16, Python 3.13.5, the installed system SQLite library, ReportLab, PyMuPDF and Chromium. Go 1.23.2 was the provided local compiler, not a production recommendation. Docker and CI target Go 1.27.1. The Docker image, Compose launch and GitHub Actions workflow were **not executed** in this environment. Pinned development-tool versions are specified for reproducibility, but their installation path was not separately exercised here.

No OpenAI/Anthropic credentialed generation or native search was executed. No owner OAuth/caption import was executed. No complete real source-to-gold-to-final-script creative acceptance run was executed. No reference video was watched and no transcript from the six examples was supplied. No public deployment, hosted TLS, capacity/soak test, external penetration test, video-production QA or operational backup-restore drill is established by this report.

## Opt-in paid connectivity smoke test

Run from the repository root on an internet-connected development machine. Export the relevant API key without committing it. Pick an exact supported model ID. This test bypasses application call-budget accounting and can incur real charges; run it deliberately, not in default CI.

```sh
export BOLTY_LIVE=1 I_ACCEPT_API_CHARGES=1
export BOLTY_LIVE_PROVIDER=openai  # or anthropic
export BOLTY_LIVE_MODEL='REPLACE_WITH_YOUR_EXACT_MODEL_ID'
# Optional extra billable native-search request:
export BOLTY_LIVE_SEARCH=1
go test ./internal/studio -run '^TestLiveProvider$' -v -count=1 -timeout=6m
```

Structured output plus native search connectivity is only the first acceptance step. Follow the full real-reference/approved-gold/editorial comparison protocol in QUALITY.md, then validate provider billing, official page fetches, revision/approval/download and actual production with your deployed environment. Do not describe the workflow as creatively calibrated before completing that work.
