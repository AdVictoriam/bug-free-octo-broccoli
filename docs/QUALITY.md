# Maintaining quality without pretending to prove taste

## Three different questions

**Is the package internally correct?** Count spoken text, enforce line IDs and cue references, separate VA/editor material, validate title/thumbnail, measure estimated runtime, check asset declarations and exports. Code can directly test these rules.

**Is the story plausible and appropriate?** Verify platform claims, case basis/disclosure, privacy, scope and remedy. Native citations, fetched official evidence and exact quotes improve traceability, but a reviewer can still misunderstand a permission rule or miss a claim. Unknown support is a blocker. A genuinely fact-free claim audit is also blocked for this narrowly scoped gaming format.

**Is it actually good?** Judge hook, clarity, escalation, voice, payoff, originality, editability and realism. A model scores each and provides reasons. A score is an opinion, not an objective measurement. A high score does not compensate for any factual or structural blocker.

## A usable baseline

Use a complete approved spoken script, not a channel description or a 200-word hook. The uploaded gold source minimum is 1,800 words to reject short excerpts; the production target remains 2,300–2,450. Paste spoken text rather than editor notes. This workflow does not validate an uploaded gold against all production rules: its editorial approval belongs to the owner. A generated draft can be accepted as the initial gold only after its non-baseline gates pass.

The newest approved gold becomes the active baseline for new runs. Earlier snapshots keep their previous baseline. Changing approved gold alone does not retroactively rewrite an existing run's evaluation. For a new baseline comparison, run a fresh review; record which baseline a production decision used.

Two blind comparisons place the candidate in A and then B. A candidate must win or tie both. The reviewer has no labels identifying which script is gold, although the content may make it inferable. Reversing order reduces a particular source of bias; it does not remove all bias. Using a different reviewer model/provider can reduce shared habits but does not make the judgement independent in a statistical sense.

## Initial editorial acceptance protocol — not yet executed

Create a held-out evaluation set covering staff abuse, extortion, fake giveaway, impersonation, bullying and stolen work, with plausible remedies and different evidence progressions. Keep the set outside the writer's learning sources. Have the owner compare results blind against approved scripts and record concrete reasons for keep/change/reject.

Include deliberately bad variants: an impossible cross-server ban, a fake refund mechanism, an implausible UI, a loud rage monologue, jokes at the victim, identifying handles, comments-origin framing, a copied sequence, a missing disclosure and a weak remedy. The factual/safety failures must not receive production approval. Human rejection that nevertheless passes all model gates is a false accept to investigate, not a cue to lower standards.

Repeat candidate generation on the same held-out prompts. Track human preference, false accepts, repair rounds, unresolved mechanics, word/runtime failures, cost and time-to-approved-script. The useful acceptance target is human preference relative to the baseline, not an arbitrary average model score. This repository has a per-script comparison workflow; it does not include a fully automated creative regression dashboard or a measured evaluation dataset.

Before changing models/prompts, use that same held-out set and record whether the owner still prefers the resulting scripts. Pin exact model IDs where the provider supports stable snapshots. A mutable provider alias may change underlying behavior without its string changing; the app cannot detect that remotely. The run records what model identifier was requested, not an attestation of immutable provider weights.

## Repair and feedback policy

The agent gets at most two automatic repairs and retains the actual blockers. No unlimited “try until the judge says yes” loop. User edits reset checks. Settings changes require fresh review. Owner feedback becomes reusable guidance only when explicitly marked reusable. Gold is held out from writer retrieval, and copied reference spans are checked. A 12-word overlap detector is a useful narrow safeguard, not a plagiarism or originality guarantee.

Final sign-off is a separate exact-version human decision. Afterward, the VA and editor still need to make the work and check real footage/assets. Model approval cannot attest that a later screenshot is anonymized or that a later music choice preserves the intended silence.
