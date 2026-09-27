# Bolty Studio

**A private, hosted-model writing room for the Bolty brief.** Turn approved YouTube references into original ideas, develop complete scripts, verify platform mechanics, compare against an owner-approved standard, and deliver synchronized voice-actor/editor packages.

This repository contains the web interface, Go API and agent worker, SQLite storage, real OpenAI/Anthropic adapters, PDF renderer, tests and deployment configuration. It is not a deployed service. Live model quality, OAuth and hosted infrastructure still require acceptance testing with your credentials and references. See `docs/TEST_REPORT.md` for the exact tested boundary.

## Start locally with Docker

```sh
python3 tools/setup.py
# Open .env locally. Add at least one provider key and the corresponding exact model ID.
# The generated APP_PASSWORD is your studio login password. Keep this file private.
docker compose up --build -d
```

Open `http://127.0.0.1:8080` and sign in with `APP_PASSWORD`. The database persists in the `studio_data` Docker volume. Do not run `docker compose down -v` unless deliberately deleting that workspace.

The supplied Docker configuration has been reviewed but was not built in the authoring environment. The native application was built and exercised. Deployment uses a current Go builder; do not publish binaries built with an unsupported local toolchain.

**Model setup:** the first start seeds settings from `DEFAULT_PROVIDER` and `OPENAI_MODEL` or `ANTHROPIC_MODEL`. Later changes belong in **Models & settings**; editing model environment variables does not overwrite saved settings. Keys stay in the server environment. Select exact IDs available to your account, not display names. The writer and reviewer need structured JSON output; the researcher needs native web search. One provider can fill all three roles. A second provider is optional, not an infrastructure requirement. Configured-key indicators do not prove valid credentials or compatible model access.

## First working story

1. **Reference library:** the six videos in the brief are bookmarks, not pre-studied content. Open a reference, add an authorized transcript (`.txt`, `.vtt`, `.srt`, or pasted text), optionally add your observations of editing/visuals, and confirm rights/redaction. Choose **Save & analyse**, inspect the extracted techniques and supporting excerpts, then approve. Only approved sources enter future writing memory.
2. **Quality lab:** add a complete *spoken-text* script you genuinely approve as a gold baseline. It is held out of the writer and used for comparison. Alternatively, write a first draft and explicitly accept it as gold once every non-baseline gate passes, then run a fresh review. The PDF itself supplies rules, not a complete gold script.
3. **New story:** describe the injustice and intended remedy, select Roblox/Minecraft/Discord for internal research, choose a category and approved references. Fictional stories require an audible disclosure; documented reconstructions require an approved, redacted case record. Generate six ideas or proceed to a complete script.
4. **Story workspace:** inspect the voice-actor, editor, quality and evidence tabs. Request a targeted revision or edit individual lines and their cues. A line edit creates an unreviewed new revision. Automatic repair is bounded to two rounds; unresolved blockers remain visible.
5. **Handoff:** once structural, evidence, creative and baseline checks pass, sign off the exact revision. Download a ZIP containing exactly `voice-actor.md`, `voice-actor.pdf`, `editor.md`, `editor.pdf`. Approved scripts create production tasks with acceptance criteria. The app does not record gameplay, render the finished video or publish it.

## What “training” means here

No model weights are fine-tuned. Approved transcripts become source-grounded craft notes plus searchable excerpts; approved feedback becomes reusable editorial guidance. Each run freezes the source versions, retrieved excerpts, feedback, model settings, prompt and gold baseline. Changing source content revokes its approval. Archiving removes it from future memory but does not erase historical snapshots.

The agent is a bounded workflow, not a permanently running autonomous loop:

```text
approved references + brief + owner-approved feedback
  -> ideas / outline
  -> preliminary official-source mechanics research
  -> canonical full script
  -> independent final-claim audit
  -> official search + fetched pages + quoted support
  -> creative critique + deterministic checks
  -> blind A/B comparison with gold (both orders)
  -> at most two repairs
  -> exact-version owner approval
  -> four-file package + execution tasks
```

## Keeping the standard

The brief's mandatory rules are delivery gates: 2,300–2,450 spoken words, synchronized numbered lines, separate VA/editor instructions, color rules, title syntax, privacy/recreation, excluded subject matter, believable platform mechanics and the intended duration.

**Proposed creative thresholds are 85/100 overall and at least 7/10 in each of eight dimensions.** These are implementation defaults, not numbers in the brief or empirically calibrated promises. A high model score cannot establish that a script is genuinely good. The owner-approved gold comparison and editorial sign-off are therefore separate gates. Establish a strong first baseline and measure reviewer agreement with your own taste before relying on scores. Instructions are in `docs/QUALITY.md`.

## YouTube and verification boundaries

The official caption-download route requires OAuth authorization and permission to edit the video. Configure `YOUTUBE_CLIENT_ID`, `YOUTUBE_CLIENT_SECRET` and an appropriately scoped `YOUTUBE_REFRESH_TOKEN` only for videos you can edit. Otherwise upload an authorized transcript. OAuth token acquisition is external setup, not an in-app Google login flow. No arbitrary public-video scraper, audio transcription, frame analysis or claim to have watched a link is included. User-supplied visual observations are explicitly distinguished from transcript-only analysis.

Research uses native provider web-search citations restricted to configured official documentation domains. The backend separately fetches cited pages, stores their content hash and retrieval time, and requires matching verbatim support for each audited mechanic. Missing, unsupported, inaccessible or older-than-seven-day evidence blocks handoff. Independent claim extraction and evidence interpretation are still model judgements; no code proves that a model found every possible factual claim. Re-run review when official evidence becomes stale.

## Lightweight deployment

One Go process serves static assets and the JSON API and runs one durable worker. One SQLite database stores sources, full-text search, jobs, call checkpoints, revisions, feedback and tasks. Python/ReportLab runs only when creating the PDFs. No GPU, model hosting, Redis, vector service, Kubernetes, npm build or separate queue service.

Designed for **one private workspace on one persistent machine**, not a public multi-tenant SaaS or horizontally scaled serverless deployment. Start with a small instance, then measure actual memory and API traffic; no load/capacity claim is made. Bind the app privately behind an HTTPS reverse proxy for remote access; set `PUBLIC_URL` to the exact browser origin, without a trailing slash. Use exactly one replica and a real persistent disk. Keep remote access protected and credentials server-side. See `docs/SECURITY.md`.

Generation is serial; the queue admits at most eight active/queued jobs. Defaults are 30 logical provider calls per run and 90 per UTC day, with bounded prompts and output tokens. The UI reports known token usage, not invented dollar estimates. Search fees, reasoning tokens, response size and ambiguous retries can affect billing. These guards are **not a guaranteed dollar cap**; configure the provider account's available spending controls too. A restarted in-flight job requires explicit retry because a timed-out request may already have been billed.

## Native development

Linux or macOS with Go (minimum module compatibility 1.23; use a currently supported release), a C compiler, SQLite headers/library with FTS5 and JSON1, Python 3 and ReportLab. Windows users should use Docker or a compatible Linux environment. Node is used only to syntax-check the browser code. Native Windows builds are not supported by the filesystem-lock implementation.

```sh
# Debian/Ubuntu example prerequisite packages:
sudo apt-get install build-essential libsqlite3-dev python3 python3-reportlab
python3 tools/setup.py
# Edit .env, then:
make run
```

For all QA tools, install the development packages listed in `requirements-dev.txt` into a virtual environment, install Playwright Chromium, and run:

```sh
make check test build pdf-check
python3 -m playwright install chromium
make browser-check
```

Browser checks use a temporary database and deliberately clear provider keys. Unit/contract tests use synthetic fixtures, never pass them off as real AI output. A paid provider connectivity test is opt-in; see `docs/TEST_REPORT.md`.

## Backups and recovery

For a native install:

```sh
python3 tools/backup.py data/studio.db backups/studio-2026-09-26.db
```

For Docker, use the same SQLite backup API against the mounted database (the container includes Python):

```sh
docker compose exec studio python3 -c "import sqlite3; s=sqlite3.connect('/data/studio.db'); d=sqlite3.connect('/data/studio.backup.db'); s.backup(d); d.close(); s.close()"
docker compose cp studio:/data/studio.backup.db ./studio.backup.db
chmod 600 studio.backup.db
docker compose exec studio python3 -c "import os; os.remove('/data/studio.backup.db')"
```

A backup includes private source text, case records, model outputs and session hashes. Protect it like the live database. To restore, stop the application, move the existing database and its WAL/SHM sidecars aside, copy the verified backup into place with the application's ownership, delete saved sessions, then restart. Test restoration on an isolated copy before depending on it. Never copy only the main SQLite file while the live database has uncheckpointed WAL writes.

## Repository map

`cmd/bolty` is the entrypoint. `internal/studio` implements the contracts, orchestration, verification, API and export pipeline. `internal/sqlite` is the small system-SQLite binding. `web` contains the embedded dependency-free interface. `tools` contains setup, run, backup and PDF utilities. `tests` holds browser/PDF checks. `docs` contains brief traceability, architecture, security, quality calibration and the test record.
