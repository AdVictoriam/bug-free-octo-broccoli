/* Bolty Studio: dependency-free interface. All AI results come from the server.
 * Imported/source/model text is escaped before it reaches the DOM.
 */
const $ = (q, el = document) => el.querySelector(q);
const esc = (v) =>
  String(v ?? "").replace(
    /[&<>"']/g,
    (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c],
  );
const arr = (v) => (Array.isArray(v) ? v : []);
const label = (v) => String(v || "").replaceAll("_", " ");
const minute = (n) =>
  `${Math.floor(Math.round(n || 0) / 60)}:${String(Math.round(n || 0) % 60).padStart(2, "0")}`;
const num = (n) => Number(n || 0).toLocaleString();
const iconPaths = {
  studio: "M3 3h7v7H3z M14 3h7v7h-7z M3 14h7v7H3z M14 14h7v7h-7z",
  library: "M4 4h4v16H4z M11 4h4v16h-4z M18 5l3 14",
  dna: "M5 3c0 8 14 10 14 18 M19 3c0 8-14 10-14 18 M7 6h10 M7 18h10 M9 10h6 M9 14h6",
  quality: "M12 3l8 3v6c0 5-8 9-8 9s-8-4-8-9V6z M8 12l3 3 5-6",
  production: "M4 5h5v14H4z M10 5h5v10h-5z M16 5h5v7h-5z",
  models: "M9 3v3 M15 3v3 M9 18v3 M15 18v3 M3 9h3 M3 15h3 M18 9h3 M18 15h3 M6 6h12v12H6z",
  plus: "M12 5v14 M5 12h14",
  arrow: "M5 12h14 M13 6l6 6-6 6",
  spark: "M12 3l2.5 6.5L21 12l-6.5 2.5L12 21l-2.5-6.5L3 12l6.5-2.5z",
  play: "M8 5l11 7-11 7z",
  check: "M5 12l4 4L19 6",
  clock: "M12 8v5l3 2 M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0",
  close: "M6 6l12 12 M6 18L18 6",
  file: "M5 3h9l5 5v13H5z M14 3v6h5 M8 13h8 M8 17h6",
  upload: "M12 16V3 M7 8l5-5 5 5 M4 15v6h16v-6",
  download: "M12 3v13 M7 11l5 5 5-5 M4 18v3h16v-3",
  edit: "M4 20l4-1L20 7l-3-3L5 16z M14 7l3 3",
  logout: "M9 4H4v16h5 M10 12h11 M17 8l4 4-4 4",
  menu: "M4 6h16 M4 12h16 M4 18h16",
  link: "M10 13a4 4 0 0 0 6 0l4-4a4 4 0 0 0-6-6l-2 2 M14 11a4 4 0 0 0-6 0l-4 4a4 4 0 0 0 6 6l2-2",
  alert: "M12 3L2 21h20z M12 9v5 M12 17v1",
};
const ico = (n, cls = "") =>
  `<svg class="icon ${cls}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.65" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="${iconPaths[n] || iconPaths.file}"/></svg>`;
const pill = (v, kind = "") => `<span class="pill ${kind}">${esc(label(v))}</span>`;
const statusPill = (v) =>
  pill(
    v,
    ["approved", "production", "complete"].includes(v)
      ? "good"
      : ["failed", "needs_changes", "interrupted"].includes(v)
        ? "bad"
        : ["running", "review"].includes(v)
          ? "purple"
          : "warn",
  );
const btn = (action, text, cls = "", extra = "") =>
  `<button class="btn ${cls}" data-action="${action}" ${extra}>${text}</button>`;
const safeLink = (url, text, cls = "") => {
  try {
    const u = new URL(url);
    if (u.protocol === "https:")
      return `<a class="${cls}" href="${esc(u.href)}" target="_blank" rel="noopener noreferrer">${text}</a>`;
  } catch {}
  return text;
};
let S = null,
  D = null,
  pinnedVersion = false,
  selectedDraft = "",
  storyTab = "voice",
  sourceTab = "all",
  modalOpen = false,
  busy = false,
  pollTimer = null,
  sourceEditing = null,
  lastFocus = null;
const activeJobs = () => arr(S?.jobs).filter((j) => ["queued", "running"].includes(j.status));
const route = () => location.hash.slice(1) || "studio";
const projectID = () => (route().startsWith("story/") ? route().split("/")[1] : "");
const currentProject = () => arr(S?.projects).find((p) => p.id === projectID());
const approvedRefs = () => arr(S?.sources).filter((s) => s.kind === "reference" && s.status === "approved");
const golds = () => arr(S?.sources).filter((s) => s.kind === "gold" && s.status === "approved");
const latestDraft = (projectId) =>
  arr(S?.drafts)
    .filter((d) => d.project_id === projectId)
    .sort((a, b) => b.version - a.version)[0];
const lineID = (n) => "L" + String(n).padStart(2, "0");
// The CSP forbids inline style attributes, so each reading voice maps to one of six CSS classes.
const voiceClass = (name) => "v" + (Math.max(0, arr(D?.script?.voices).findIndex((v) => v.name === name)) % 6);
const ago = (iso) => {
  const s = (Date.now() - Date.parse(iso)) / 1000;
  if (!isFinite(s)) return "";
  if (s < 60) return "just now";
  if (s < 3600) return `${Math.floor(s / 60)}m ago`;
  if (s < 86400) return `${Math.floor(s / 3600)}h ago`;
  if (s < 604800) return `${Math.floor(s / 86400)}d ago`;
  return new Date(iso).toLocaleDateString(undefined, { month: "short", day: "numeric" });
};
const greeting = () => {
  const h = new Date().getHours();
  return h < 5 ? "Working late" : h < 12 ? "Good morning" : h < 18 ? "Good afternoon" : "Good evening";
};
async function api(path, method = "GET", body) {
  const res = await fetch("/api" + path, {
    method,
    credentials: "same-origin",
    headers: method === "GET" ? {} : { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  let data;
  try {
    data = await res.json();
  } catch {
    throw new Error("The server returned an unreadable response.");
  }
  if (!res.ok) {
    if (res.status === 401 && !path.includes("login")) {
      S = null;
      closeModal();
      renderLogin();
    }
    throw new Error(data.error || `Request failed (${res.status})`);
  }
  return data;
}
function toast(msg, error = false) {
  const el = $("#toast");
  el.textContent = msg;
  el.className = "visible" + (error ? " error" : "");
  clearTimeout(toast.timer);
  toast.timer = setTimeout(() => (el.className = ""), 6000);
}
async function refresh(render = true) {
  S = await api("/state");
  for (const k of ["sources", "projects", "jobs", "drafts", "tasks", "feedback"]) S[k] = arr(S[k]);
  if (projectID()) {
    const ds = S.drafts.filter((x) => x.project_id === projectID()).sort((a, b) => b.version - a.version);
    const chosen = (pinnedVersion && ds.find((d) => d.id === selectedDraft)) || ds[0];
    if (chosen) {
      selectedDraft = chosen.id;
      D = await api("/drafts/" + chosen.id);
    } else {
      D = null;
      selectedDraft = "";
      pinnedVersion = false;
    }
  }
  if (render && !modalOpen) renderApp();
  schedulePoll();
}
function schedulePoll() {
  clearTimeout(pollTimer);
  pollTimer = setTimeout(
    async () => {
      if (document.hidden || modalOpen || busy) {
        schedulePoll();
        return;
      }
      try {
        await refresh(activeJobs().length > 0 || !!projectID());
      } catch {}
    },
    activeJobs().length ? 4000 : 60000,
  );
}
function navigate(path) {
  location.hash = path;
}
function empty(title, text, action = "new-project", cta = "Create a story", icon = "file") {
  return `<div class="empty"><div class="empty-icon">${ico(icon)}</div><h3>${esc(title)}</h3><p>${esc(text)}</p>${action ? btn(action, esc(cta), "small") : ""}</div>`;
}
function head(eyebrow, title, description, actions = "") {
  return `<div class="heading"><div><div class="eyebrow">${esc(eyebrow)}</div><h1>${esc(title)}</h1><p>${esc(description)}</p></div><div class="actions">${actions}</div></div>`;
}
function layout(body) {
  const nav = [
    ["studio", "Studio", "studio"],
    ["library", "Reference library", "library"],
    ["dna", "Channel DNA", "dna"],
    ["quality", "Quality lab", "quality"],
    ["production", "Production", "production"],
  ];
  let section = route().split("/")[0];
  if (section === "story") section = "studio";
  const titles = {
    studio: "Studio",
    library: "Reference library",
    dna: "Channel DNA",
    quality: "Quality lab",
    production: "Production",
    models: "Models & settings",
  };
  const jobs = activeJobs();
  const running = jobs.find((j) => j.status === "running") || jobs[0];
  const openTasks = S.tasks.filter((t) => !t.done).length;
  const story = route().startsWith("story/") ? currentProject() : null;
  const crumbs = story
    ? `<a class="crumb-parent" href="#studio">Studio</a><span class="sep">/</span><strong>${esc(story.title)}</strong>`
    : `<span class="crumb-parent">Workspace</span><span class="sep">/</span><strong>${esc(titles[section] || "Studio")}</strong>`;
  const status = running
    ? `<span class="job-chip" title="${esc(running.stage || label(running.kind))}"><span class="dot busy"></span><span>${esc(running.stage || label(running.kind))}</span></span>`
    : `<span class="dot"></span><span class="status-text">Writing room idle</span>`;
  return `<div class="shell"><aside class="sidebar"><a href="#studio" class="brand"><img src="/mark.svg" width="38" height="38" alt=""><span><span class="brand-word">bolty<span>studio</span></span><span class="brand-sub">THE STORY SYSTEM</span></span></a><div class="nav-label">WORKSPACE</div><nav class="nav">${nav.map(([id, name, icon]) => `<a class="${section === id ? "active" : ""}" href="#${id}">${ico(icon)}<span>${name}</span>${id === "library" ? `<span class="count">${S.sources.filter((s) => s.status !== "archived").length}</span>` : id === "production" && openTasks ? `<span class="count">${openTasks}</span>` : id === "studio" && S.projects.length ? `<span class="count">${S.projects.length}</span>` : ""}</a>`).join("")}</nav><div class="nav-bottom"><div class="sidebar-note"><span class="dot ${jobs.length ? "busy" : ""}"></span><div>${jobs.length ? `${jobs.length} run${jobs.length > 1 ? "s" : ""} in progress<p>${esc(running.stage || label(running.kind))}</p>` : `Writing room idle<p>${golds().length ? "Gold baseline set." : '<a href="#quality">Set a gold baseline</a> to calibrate reviews.'}</p>`}</div></div><nav class="nav"><a href="#models" class="${section === "models" ? "active" : ""}">${ico("models")}<span>Models & settings</span></a></nav><div class="sidebar-footer"><div class="avatar">B</div><div>Bolty workspace<span>Private · single team</span></div><button class="icon-btn" data-action="logout" aria-label="Sign out" title="Sign out">${ico("logout")}</button></div></div></aside><div class="nav-scrim" data-action="menu"></div><div class="main-shell"><header class="topbar"><div class="breadcrumb"><button class="icon-btn mobile-menu" data-action="menu" aria-label="Toggle navigation">${ico("menu")}</button>${crumbs}</div><div class="top-actions">${status}</div></header><main class="content">${body}</main></div></div>`;
}
function renderApp() {
  if (!S) return renderLogin();
  let page;
  switch (route().split("/")[0]) {
    case "library":
      page = library();
      break;
    case "dna":
      page = dna();
      break;
    case "quality":
      page = quality();
      break;
    case "production":
      page = production();
      break;
    case "models":
      page = models();
      break;
    case "story":
      page = story();
      break;
    default:
      page = studio();
  }
  $("#app").innerHTML = layout(page);
  // CSSOM updates are permitted by the CSP; inline style attributes are not.
  for (const el of document.querySelectorAll("[data-p]")) el.style.setProperty("--p", el.dataset.p);
  const crumb = $(".breadcrumb strong")?.textContent;
  document.title = crumb && crumb !== "Studio" ? `${crumb} · Bolty Studio` : "Bolty Studio";
}
function studio() {
  const ready = S.drafts.filter((d) => d.approved).length;
  const setup = [
    [
      "01",
      "Connect your models",
      "Add an OpenAI or Anthropic key on the server, then choose a writer, reviewer and researcher.",
      "go-models",
      S.capabilities.openai || S.capabilities.anthropic,
    ],
    [
      "02",
      "Teach it what works",
      "Import transcripts. Review the extracted craft. Approve only useful references.",
      "go-library",
      approvedRefs().length > 0,
    ],
    [
      "03",
      "Define “good enough”",
      "Upload a strong gold script, or commission and approve your first baseline.",
      "go-quality",
      golds().length > 0,
    ],
  ];
  const setupDone = setup.filter((x) => x[4]).length;
  const recent = S.projects[0];
  const recentDraft = recent && latestDraft(recent.id);
  const recentJob = recent && activeJobs().find((j) => j.project_id === recent.id);
  const hero = recent
    ? `<section class="hero resume"><div><div class="eyebrow">Pick up where you left off</div><h2>${esc(recent.title)}</h2><p>${esc(recent.platform)} · ${esc(label(recent.category))} · ${recentJob ? esc(recentJob.stage || "Working…") : recentDraft ? `Revision ${recentDraft.version}${recentDraft.approved ? ", approved for production" : recentDraft.checks?.ready ? ", ready for sign-off" : ", in review"}` : "Direction set, no script yet"}</p><div class="actions"><a class="btn primary" href="#story/${esc(recent.id)}">${recentDraft ? "Continue writing" : "Open story"} ${ico("arrow")}</a>${btn("new-project", ico("plus") + " New story", "ghost")}</div></div>${
        recentDraft
          ? `<div class="resume-stats"><div><strong>${num(recentDraft.checks?.spoken_words)}</strong><span>spoken words</span></div><div><strong>${recentDraft.checks?.score ?? "–"}</strong><span>quality / 100</span></div><div><strong>${arr(recentDraft.checks?.items).filter((x) => !x.passed && x.blocking).length}</strong><span>blocking checks</span></div></div>`
          : ""
      }</section>`
    : `<section class="hero"><div><div class="eyebrow">From inspiration to execution</div><h2>Keep the craft.<br><em>Make the story yours.</em></h2><p>A writing room that learns from approved references, checks the receipts, and hands your team two perfectly synced scripts.</p><div class="actions">${btn("new-project", "Start a new story " + ico("arrow"), "primary")}${btn("go-library", "Build your reference library", "ghost")}</div><div class="method"><span>${ico("check")} Hosted models</span><span>${ico("check")} No GPUs</span><span>${ico("check")} Human-approved quality</span></div></div><div class="story-stack" aria-label="The storytelling structure in the Bolty brief"><div class="beat-card"><div class="beat-top"><span>01 / THE SETUP</span>${ico("play")}</div><h3>Show what normal<br>looked like.</h3><div class="beat-bottom"><span>Let the audience settle in.</span><span>→</span></div></div><div class="beat-card"><div class="beat-top"><span>02 / THE RECEIPTS</span>${ico("file")}</div><h3>Let the messages<br>do the talking.</h3><div class="beat-bottom"><span>One reveal at a time.</span><span>→</span></div></div><div class="beat-card"><div class="beat-top"><span>03 / THE TURN</span>${ico("spark")}</div><h3>The bigger the moment,<br>the quieter the voice.</h3><div class="beat-bottom"><span>Then make the wrong right.</span><span>↗</span></div></div><div class="flow-caption">THE BOLTY STORYTELLING DNA</div></div></section>`;
  const setupCard =
    setupDone < setup.length
      ? `<section class="setup-card"><div class="panel-head"><h3>Finish setting up the studio</h3><span class="sub">${setupDone} of ${setup.length} done</span></div><progress class="setup-progress" value="${setupDone}" max="${setup.length}"></progress><div class="onboarding">${setup
          .map(
            ([n, t, p, a, done]) =>
              `<button class="onboarding-card ${done ? "done" : ""}" data-action="${a}"><span class="step-number">${done ? ico("check") : n}</span><div><h4>${t}</h4><p>${p}</p><span class="sub">${done ? "Done" : "Get started →"}</span></div></button>`,
          )
          .join("")}</div></section>`
      : "";
  return (
    head(
      new Date().toLocaleDateString(undefined, { weekday: "long", month: "long", day: "numeric" }),
      greeting() + ".",
      S.projects.length
        ? `${S.projects.length} ${S.projects.length === 1 ? "story" : "stories"} in the workspace${activeJobs().length ? ` · ${activeJobs().length} run${activeJobs().length > 1 ? "s" : ""} in progress` : ""}.`
        : "Turn the right references into scripts your team can actually make.",
      btn("new-project", ico("plus") + " New story", "primary"),
    ) +
    hero +
    setupCard +
    `<div class="stat-grid"><div class="stat"><div class="stat-label">Active stories ${ico("file")}</div><div class="stat-value">${S.projects.length}<span>in your workspace</span></div></div><a class="stat" href="#library"><div class="stat-label">Approved references ${ico("library")}</div><div class="stat-value">${approvedRefs().length}<span>${S.sources.filter((s) => s.status === "awaiting_transcript").length} links awaiting transcripts</span></div></a><a class="stat" href="#production"><div class="stat-label">Ready for production ${ico("production")}</div><div class="stat-value">${ready}<span>editorially signed off</span></div></a></div><div class="two-col"><section class="panel"><div class="panel-head"><h3>Your stories</h3>${S.projects.length ? `<button class="text-link" data-action="new-project">${ico("plus")} New</button>` : ""}</div>${S.projects.length ? S.projects.map(projectRow).join("") : empty("Your next story starts here.", "Choose the injustice, set the direction, and let the writing room build the evidence ladder.", "new-project", "Create your first story")}</section><div class="stack"><section class="panel"><div class="panel-head"><h3>Recent activity</h3><a href="#quality">All runs</a></div>${runs(S.jobs.slice(0, 4))}</section><section class="panel floor"><div class="panel-head"><h3>The quality floor</h3>${pill(golds().length ? "Baseline approved" : "Needs calibration", golds().length ? "good" : "warn")}</div><div class="score-row"><div class="score-orb" data-p="${Number(S.settings.pass_score) || 0}">${S.settings.pass_score}<span>/ 100</span></div><p>A proposed floor, not a quality guarantee. Anchor it to a script you genuinely approve.</p></div><div class="mini-rule"><span>Every craft dimension</span><strong>≥ ${S.settings.min_dimension} / 10</strong></div><div class="mini-rule"><span>Spoken-word target</span><strong>2,300–2,450</strong></div><div class="mini-rule"><span>Unverified mechanics</span><strong>Block delivery</strong></div>${btn("go-quality", "Open quality lab " + ico("arrow"), "ghost small")}</section></div></div>`
  );
}
function projectRow(p) {
  const d = latestDraft(p.id);
  const job = activeJobs().find((j) => j.project_id === p.id);
  const meta = job
    ? `<span class="meta"><b>Working…</b>${esc(job.stage || label(job.kind))}</span>`
    : d
      ? `<span class="meta"><b>Rev ${d.version}</b>${d.checks?.score != null ? `${d.checks.score}/100` : ""}</span>`
      : "";
  return `<a class="project-row" href="#story/${esc(p.id)}"><div class="project-icon">${ico(d ? "file" : "spark")}</div><div><strong>${esc(p.title)}</strong><div class="sub">${esc(p.platform)} · ${esc(label(p.category))} · ${p.basis === "fictional" ? "Disclosed scenario" : "Documented reconstruction"}</div></div><div class="row-end">${meta}${statusPill(p.status)}${ico("arrow")}</div></a>`;
}
function library() {
  const sources = S.sources.filter((s) =>
    sourceTab === "all"
      ? s.status !== "archived"
      : sourceTab === "archived"
        ? s.status === "archived"
        : s.kind === sourceTab && s.status !== "archived",
  );
  return (
    head(
      "REFERENCE LIBRARY",
      "Teach the craft. Not the copy.",
      "You approve what the agent learns. Links alone are never treated as studied content.",
      btn("new-source", ico("plus") + " Add reference", "primary"),
    ) +
    `<div class="notice">${ico("library")}<div><strong>The six references from the brief are already bookmarked.</strong> Add an authorized transcript and optional visual observations. The agent extracts techniques with source excerpts; you review them before they enter its memory.</div></div><div class="toolbar"><div class="segmented">${[
      ["all", "All sources"],
      ["reference", "References"],
      ["gold", "Gold scripts"],
      ["case", "Case records"],
      ["archived", "Archived"],
    ]
      .map(
        ([v, t]) =>
          `<button class="${sourceTab === v ? "active" : ""}" data-action="source-tab" data-value="${v}">${t}</button>`,
      )
      .join(
        "",
      )}</div><span class="spacer"></span><span class="sub">${sources.length} sources</span></div><div class="source-grid">${sources.map((s, i) => `<article class="source-card"><div class="source-cover"><span class="big-number">${String(i + 1).padStart(2, "0")}</span><span class="video-mark">${ico(s.kind === "reference" ? "play" : "file")}</span>${pill(s.kind)}</div><div class="source-body"><div class="source-meta">${statusPill(s.status)}<span>${num(s.word_count || 0)} words</span></div><h3>${esc(s.title)}</h3><p>${esc(s.analysis?.summary || s.visual_notes || "No transcript has been supplied. This source is not part of the agent’s approved memory.")}</p><div class="source-footer">${btn("edit-source", s.status === "awaiting_transcript" ? "Add transcript" : "Open source", "small", `data-id="${esc(s.id)}"`)}${s.url ? safeLink(s.url, ico("link") + " YouTube", "sub") : ""}</div></div></article>`).join("")}</div>${!sources.length ? empty("No sources in this view.", "Add a reference, a complete approved gold script, or a redacted case record.", "new-source", "Add source", "library") : ""}<p class="bottom-note">Rights and privacy stay with you. Upload only material you can use. Official caption download is available only through configured owner OAuth; arbitrary public-video extraction is not included.</p>`
  );
}
function dna() {
  return (
    head(
      "THE CHANNEL CONTRACT",
      "A point of view, not just a prompt.",
      "The uploaded Bolty brief is the fixed foundation. Reference material cannot overrule it.",
      pill(S.policy_version, "good"),
    ) +
    `<div class="dna-grid">${[
      [
        "The mission",
        "Someone did something unfair. By the end, it has been undone.",
        "Protect the player. Deliver a credible remedy.",
      ],
      [
        "The voice",
        "Hype, sarcastic, protective. When it gets serious, get quieter.",
        "The joke is on the antagonist. Never the victim.",
      ],
      [
        "The craft",
        "Normal → receipts → escalation → quiet reveal → remedy.",
        "Let the messages speak. Hold back the worst line.",
      ],
      [
        "The audience",
        "A ten-year-old should follow every beat without being talked down to.",
        "Faceless, narrated and written to be performed.",
      ],
    ]
      .map(
        ([t, h, p]) =>
          `<section class="dna-card"><div class="eyebrow">${t}</div><h3>${h}</h3><p>${p}</p></section>`,
      )
      .join(
        "",
      )}</div><section class="panel contract"><div class="panel-head"><h3>The non-negotiables</h3>${pill("Delivery checks", "good")}</div><div class="rule-matrix">${[
      ["2,300–2,450", "Spoken words only. Headers, notes and cues do not count."],
      ["Two documents", "Voice actor + editor. Each exported as Markdown and a colour PDF."],
      ["One line system", "Every editor cue references a numbered voice-actor line."],
      ["Real mechanics", "Official-source verification. Unknown or stale evidence blocks approval."],
      ["No pred catching", "Craft transfers. Predator-catching subject matter does not."],
      ["Privacy by default", "Changed identities. Original assets. Recreated UI and own worlds."],
    ]
      .map(([t, p]) => `<div class="rule-cell"><strong>${t}</strong><p>${p}</p></div>`)
      .join(
        "",
      )}</div></section><div class="notice warning">${ico("alert")}<div><strong>Two deliberate interpretations.</strong> The brief’s explicit “out of scope” direction overrides its legacy channel description. Because it says games must not be named, platform names stay in internal planning, not narration. Additionally, fictional scenarios require an audible disclosure; documented stories require an approved case record. That disclosure rule is a studio safeguard, not a claim about the brief.</div></div><details class="panel contract"><summary>Read the complete versioned channel contract</summary><pre>${esc(S.policy)}</pre></details>`
  );
}
function runs(jobs = S.jobs) {
  return `<div class="recent-runs">${
    jobs.length
      ? jobs
          .slice(0, 12)
          .map(
            (j) =>
              `<div class="run-row"><div class="row-copy"><strong>${esc(label(j.kind))} ${statusPill(j.status)}</strong><div class="sub">${esc(j.stage || "Queued")} · ${j.calls || 0} calls · ${num((j.input_tokens || 0) + (j.output_tokens || 0))} tokens${j.updated_at || j.created_at ? ` · ${ago(j.updated_at || j.created_at)}` : ""}</div>${j.error ? `<div class="run-error">${esc(j.error)}</div>` : ""}</div>${["failed", "cancelled", "interrupted"].includes(j.status) ? btn("retry-job", "Retry", "small", `data-id="${j.id}"`) : ["running", "queued"].includes(j.status) ? btn("cancel-job", "Cancel", "small ghost", `data-id="${j.id}"`) : ""}</div>`,
          )
          .join("")
      : empty(
          "No runs yet.",
          "Actual requests, failures and known token usage appear here. No simulated scores.",
          "",
          "",
          "clock",
        )
  }</div>`;
}
function quality() {
  return (
    head(
      "QUALITY LAB",
      "A score is not a standard. Your taste is.",
      "Set a frozen reference point, then evaluate every candidate against it.",
      btn("new-gold", ico("plus") + " Add gold script", "primary"),
    ) +
    `<div class="two-col"><section class="panel contract"><div class="panel-head"><h3>Frozen gold baseline</h3>${pill(golds().length ? "Approved anchor" : "Not established", golds().length ? "good" : "warn")}</div><p>The newest approved gold script is held out of the writer’s context. The reviewer compares it with each candidate twice, reversing A/B order. A candidate must win or tie both times.</p>${
      golds().length
        ? golds()
            .map(
              (s, i) =>
                `<div class="project-row"><div class="project-icon">${ico("quality")}</div><div><strong>${esc(s.title)}</strong><div class="sub">${i === 0 ? "Active for new runs" : "Earlier approved anchor"} · ${num(s.word_count)} spoken words</div></div>${btn("edit-source", "View", "small", `data-id="${s.id}"`)}</div>`,
            )
            .join("")
        : empty(
            "First, define a good script.",
            "Upload a complete script you approve, or commission a draft and explicitly accept it as your starting baseline.",
            "new-gold",
            "Add a gold script",
            "quality",
          )
    }</section><section class="panel floor"><div class="panel-head"><h3>Release gates</h3>${pill("Fail closed", "good")}</div><div class="mini-rule"><span>Overall model-rated craft</span><strong>≥ ${S.settings.pass_score}/100</strong></div><div class="mini-rule"><span>Each of eight dimensions</span><strong>≥ ${S.settings.min_dimension}/10</strong></div><div class="mini-rule"><span>Fact evidence</span><strong>≤ 7 days old</strong></div><div class="mini-rule"><span>Automatic repairs</span><strong>At most 2</strong></div><div class="mini-rule"><span>Final release</span><strong>Owner sign-off</strong></div><p>These numerical thresholds are proposed defaults. They become useful only after you compare the model’s judgements with your own.</p></section></div><div class="notice">${ico("quality")}<div><strong>Quality does not silently drift with a model switch.</strong> Each run freezes its model IDs, prompt version, approved sources, feedback and baseline. Changed settings require a fresh review. Model judgement remains imperfect; a green result is a review aid, not proof of creative quality.</div></div><section class="panel"><div class="panel-head"><h3>Run history</h3><span class="sub">Bounded, resumable, inspectable</span></div>${runs()}</section>`
  );
}
function production() {
  const tasks = S.tasks;
  const done = tasks.filter((t) => t.done).length;
  return (
    head(
      "EXECUTION BOARD",
      "From a good script to a finished video.",
      "Only signed-off drafts become production tasks. Original footage and privacy checks still need real execution.",
      tasks.length
        ? `<div class="task-progress"><progress value="${done}" max="${tasks.length}"></progress><span class="sub">${done} of ${tasks.length} done</span></div>`
        : "",
    ) +
    `${
      tasks.length
        ? `<div class="production-grid">${[...new Set(tasks.map((t) => t.role))]
            .map(
              (role) =>
                `<section class="task-list"><div class="panel-head"><h3>${esc(role)}</h3>${pill(`${tasks.filter((t) => t.role === role && t.done).length} / ${tasks.filter((t) => t.role === role).length}`)}</div>${tasks
                  .filter((t) => t.role === role)
                  .sort((a, b) => a.done - b.done)
                  .map(
                    (t) =>
                      `<label class="task-card ${t.done ? "done" : ""}"><input type="checkbox" data-action="task" data-id="${t.id}" ${t.done ? "checked" : ""}><div><strong>${esc(t.text)}</strong><p>${esc(t.acceptance)}</p><a class="sub" href="#story/${t.project_id}">Open script ↗</a></div></label>`,
                  )
                  .join("")}</section>`,
            )
            .join("")}</div>`
        : `<section class="panel">${empty("Nothing has been handed off yet.", "Finish the script, resolve its checks, and approve the exact revision. The app then creates role-assigned tasks with acceptance criteria.", "new-project", "Create a story", "production")}</section>`
    }`
  );
}
function models() {
  const s = S.settings;
  return (
    head(
      "MODELS & SETTINGS",
      "Your providers. One small studio.",
      "API keys live on the server. The browser never receives them.",
      pill("Single workspace"),
    ) +
    `<form id="models-form"><div class="settings-grid"><section><div class="notice">${ico("models")}<div><strong>Configure keys in your server environment.</strong><br><code>OPENAI_API_KEY</code> / <code>ANTHROPIC_API_KEY</code>. “Configured” means a key is present, not that a live request has succeeded. Enter exact model IDs available to your account.</div></div>${[
      ["writer", "Writer", "Reference analysis, ideas and scripts."],
      ["judge", "Reviewer", "Claim audit, quality critique and blind comparison."],
      ["researcher", "Researcher", "Official web search for game and platform mechanics."],
    ]
      .map(
        ([id, t, p]) =>
          `<section class="model-card"><div class="model-role">${ico(id === "writer" ? "edit" : id === "judge" ? "quality" : "library")}<div><h3>${t}</h3><p>${p}</p></div></div><div class="fields"><label class="field">Provider<select name="${id}_provider">${["openai", "anthropic"].map((v) => `<option value="${v}" ${s[id].provider === v ? "selected" : ""}>${v === "openai" ? "OpenAI" : "Anthropic"} · ${S.capabilities[v] ? "key configured" : "key missing"}</option>`).join("")}</select></label><label class="field">Exact model ID<input name="${id}_model" value="${esc(s[id].model)}" maxlength="120" placeholder="Enter a supported model ID"></label></div></section>`,
      )
      .join(
        "",
      )}<section class="model-card"><h3>Quality and request budgets</h3><div class="fields"><label class="field">Overall floor (85–100)<input type="number" name="pass_score" min="85" max="100" value="${s.pass_score}" required></label><label class="field">Dimension floor (7–10)<input type="number" name="min_dimension" min="7" max="10" value="${s.min_dimension}" required></label><label class="field">Calls per run (10–40)<input type="number" name="max_calls" min="10" max="40" value="${s.max_calls}" required></label><label class="field">Calls per UTC day (10–1,000)<input type="number" name="daily_calls" min="10" max="1000" value="${s.daily_calls}" required></label></div><p class="model-notes">One worker. At most eight queued/running jobs. Logical call caps are not dollar caps: provider tools and retries can add charges. Set hard spending limits in the provider console.</p></section><div class="actions"><button class="btn primary" type="submit">Save model settings</button></div></section><aside><section class="panel contract"><div class="eyebrow">SMALL BY DESIGN</div><div class="arch-stack"><div>${ico("studio")} Embedded web interface</div><span class="arrow">↓</span><div>${ico("models")} One Go service + worker</div><span class="arrow">↓</span><div>${ico("library")} SQLite + source memory</div><span class="arrow">↔</span><div>${ico("spark")} Hosted model APIs</div></div><p>No GPU, Redis, vector database or Kubernetes required. A small Python helper renders the PDFs.</p></section><section class="panel contract"><h3>Separate judgement</h3><p>Using a different provider for the reviewer can reduce shared tendencies. It does not make the review objective or guarantee success.</p><p>Research needs a model that supports its provider’s native web-search tool. Writing and review need structured JSON output.</p></section></aside></div></form>`
  );
}
function jobbar(j) {
  return `<div class="jobbar" role="status"><div>${ico("spark")}<strong>${esc(j.stage || label(j.kind))}</strong><span class="sub">${j.status === "queued" ? "Queued" : `${Number(j.progress) || 0}%`} · ${j.calls || 0} calls · ${num((j.input_tokens || 0) + (j.output_tokens || 0))} tokens</span></div><progress value="${Number(j.progress) || 0}" max="100"></progress>${btn("cancel-job", "Cancel", "small ghost", `data-id="${j.id}"`)}</div>`;
}
function story() {
  const p = currentProject();
  if (!p) return empty("Story not found.", "Choose a story from the studio.", "go-studio", "Back to studio");
  const job = activeJobs().find((j) => j.project_id === p.id);
  const versions = S.drafts.filter((d) => d.project_id === p.id).sort((a, b) => b.version - a.version);
  let out = head(
    p.platform + " · " + label(p.category),
    p.title,
    p.basis === "fictional"
      ? "Disclosed fictional scenario · never presented as a documented real event."
      : "Documented reconstruction · anchored to an approved, redacted case record.",
    btn("edit-project", ico("edit") + " Direction", "small") +
      (!D
        ? btn("write", ico("spark") + " Write script", "primary", job ? "disabled" : "")
        : btn("review", ico("quality") + " Run fresh review", "primary", job ? "disabled" : "")),
  );
  if (job) out += jobbar(job);
  if (!D) {
    out += `<section class="panel contract"><div class="eyebrow">THE DIRECTION</div><h3>${esc(p.premise)}</h3><div class="chip-row">${arr(
      p.source_ids,
    )
      .map((id) => pill(S.sources.find((s) => s.id === id)?.title || "Reference"))
      .join(
        "",
      )}</div><div class="pipeline">${["Outline", "Verify mechanics", "Write", "Audit & critique", "Compare to gold", "Approve & hand off"].map((t, i) => `<div class="pipeline-step"><span>${String(i + 1).padStart(2, "0")}</span>${t}</div>`).join("")}</div><div class="actions">${btn("ideas", ico("spark") + " Explore six ideas", "primary", job ? "disabled" : "")}<span class="sub">Ideas are concepts, not verified cases.</span></div></section>`;
    const ij = S.jobs.find((j) => j.project_id === p.id && j.kind === "ideas" && j.status === "complete");
    if (ij?.result?.ideas)
      out += `<div class="ideas-grid">${ij.result.ideas
        .map(
          (i, n) =>
            `<article class="idea"><div class="eyebrow">CONCEPT ${String(n + 1).padStart(2, "0")} / ${esc(i.category)}</div><h3>${esc(i.title)}</h3><p>${esc(i.hook)}</p><div class="callout-label">The injustice</div><p>${esc(i.injustice)}</p><div class="callout-label">The evidence ladder</div>${arr(
              i.evidence_ladder,
            )
              .map((r, k) => `<p>${k + 1}. ${esc(r)}</p>`)
              .join(
                "",
              )}<div class="callout-label">Make it right</div><p>${esc(i.payoff)}</p><p class="sub">${esc(i.originality)}</p>${btn("use-idea", "Use this direction " + ico("arrow"), "small", `data-job="${ij.id}" data-index="${n}"`)}</article>`,
        )
        .join("")}</div>`;
    out += `<section class="panel"><div class="panel-head"><h3>Story runs</h3></div>${runs(S.jobs.filter((j) => j.project_id === p.id))}</section>`;
    return out;
  }
  const blocking = arr(D.checks?.items).filter((x) => !x.passed && x.blocking).length;
  const unsupported = arr(D.research?.claims).filter(
    (c) => arr(D.research.verification?.verdicts).find((v) => v.claim_id === c.id)?.verdict !== "supported",
  ).length;
  out += `<div class="toolbar"><div class="tabs" role="tablist">${[
    ["voice", "Voice actor", arr(D.script?.lines).length],
    ["editor", "Editor", ""],
    ["checks", "Quality", blocking, blocking ? "bad" : ""],
    ["evidence", "Evidence", unsupported, unsupported ? "bad" : ""],
    ["tasks", "Handoff", ""],
  ]
    .map(
      ([v, t, n, cls = ""]) =>
        `<button role="tab" aria-selected="${storyTab === v}" class="${storyTab === v ? "active" : ""}" data-action="story-tab" data-value="${v}">${t}${n ? `<span class="tab-count ${cls}">${n}</span>` : ""}</button>`,
    )
    .join(
      "",
    )}</div><span class="spacer"></span><select id="version-select" aria-label="Draft version">${versions.map((v) => `<option value="${v.id}" ${D.id === v.id ? "selected" : ""}>Revision ${v.version}${v.approved ? " · approved" : ""}</option>`).join("")}</select></div><div class="story-layout"><section>${storyTab === "voice" ? voiceView() : storyTab === "editor" ? editorView() : storyTab === "checks" ? checksView() : storyTab === "evidence" ? evidenceView() : tasksView()}</section>${inspector(job)}</div>`;
  return out;
}
function voiceView() {
  const s = D.script;
  return `<article class="script-sheet"><div class="sheet-head"><div class="eyebrow">VOICE ACTOR SCRIPT / REVISION ${D.version}</div><h2>${esc(s.title)}</h2><p>Spoken lines only. Purple notes are delivery, not dialogue. ★ means record a second take.</p></div><div class="reading-voices">${arr(
    s.voices,
  )
    .map(
      (v) =>
        `<div class="reading-voice ${voiceClass(v.name)}"><span class="voice-dot"></span><div><strong>${esc(v.name)} <span>${esc(v.role)}</span></strong><p>${esc(v.delivery)}</p></div></div>`,
    )
    .join("")}</div>${arr(s.lines)
    .map(
      (l) =>
        `<div class="script-line ${voiceClass(l.speaker)}" id="line-${l.id}"><div class="line-id">${lineID(l.id)}${l.second_take ? '<span class="line-star" title="Record a second take">★</span>' : ""}</div><div><div class="speaker">${esc(l.speaker)}${l.delivery ? ` <span class="delivery">${esc(l.delivery)}</span>` : ""}</div><p class="spoken">${esc(l.text)}</p></div><button class="icon-btn" data-action="edit-line" data-id="${l.id}" aria-label="Edit line ${l.id}" title="Edit line">${ico("edit")}</button></div>`,
    )
    .join("")}</article>`;
}
function editorView() {
  const s = D.script,
    t = s.thumbnail;
  return `<article class="script-sheet"><div class="sheet-head"><div class="eyebrow">Editor script · Revision ${D.version}</div><h2>${esc(s.title)}</h2><p><span class="blue">Blue</span> is gameplay and general direction. <span class="red">Red</span> is edit and sound.</p><div class="sheet-meta"><div><div class="callout-label">Thumbnail text</div><h3>${esc(t.text)}</h3></div><div><div class="callout-label">Thumbnail layout</div><p><strong>Left</strong> ${esc(t.left)}<br><strong>Right</strong> ${esc(t.right)}<br><strong>Centre</strong> ${esc(t.evidence)}</p></div><div><div class="callout-label">Tone</div><p>${esc(s.tone)}</p></div></div><details><summary>Privacy, original assets and running gags</summary>${arr(
    s.privacy,
  )
    .map((x) => `<p class="red">${esc(x)}</p>`)
    .join("")}${arr(s.assets)
    .map(
      (a) =>
        `<p class="blue"><strong>${esc(a.name)} (${esc(a.provenance)}):</strong> ${esc(a.description)}</p>`,
    )
    .join("")}<p>${arr(s.running_gags).map(esc).join(" · ")}</p></details></div>${arr(s.cues)
    .map(
      (c) =>
        `<div class="cue"><div class="cue-title"><span>${lineID(c.line_id)}</span>${c.hold_seconds ? pill(`${c.hold_seconds}s non-spoken hold`, "purple") : ""}</div>${c.gameplay ? `<p class="cue-text blue"><span class="label">Gameplay</span><span>${esc(c.gameplay)}</span></p>` : ""}${c.edit ? `<p class="cue-text red"><span class="label">Edit</span><span>${esc(c.edit)}</span></p>` : ""}${c.sound ? `<p class="cue-text red"><span class="label">Sound</span><span>${esc(c.sound)}</span></p>` : ""}</div>`,
    )
    .join("")}</article>`;
}
function checksView() {
  const c = D.checks;
  return `<section class="panel contract"><div class="quality-head"><div class="big-score ${c.ready ? "" : "bad"}">${c.score}<span>/100</span></div><div><h3>${c.ready ? "All current gates passed" : "Not cleared for handoff"}</h3><p>Model-rated craft. The score never overrides a failed factual or structural check.</p>${statusPill(D.approved ? "approved" : c.ready ? "review" : "needs_changes")}</div></div><div class="dimensions">${arr(
    D.critique?.dimensions,
  )
    .map(
      (d) =>
        `<div class="dimension ${d.score < S.settings.min_dimension ? "low" : ""}"><div><strong>${esc(label(d.name))}</strong><span>${d.score}/10</span></div><progress max="10" value="${d.score}"></progress><p>${esc(d.reason)}</p></div>`,
    )
    .join(
      "",
    )}</div>${!D.critique ? '<p class="notice warning">This revision has not received an independent model review.</p>' : ""}</section><section class="panel contract"><div class="panel-head"><h3>Delivery checks</h3><span class="sub">${arr(c.items).filter((x) => x.passed).length} of ${arr(c.items).length} passing</span></div>${arr(
    c.items,
  )
    .slice()
    .sort((a, b) => a.passed - b.passed)
    .map(
      (x) =>
        `<div class="check-row ${x.passed ? "" : "fail"}">${ico(x.passed ? "check" : "alert")}<div><strong>${esc(label(x.code))}</strong><p>${esc(x.detail)}</p></div>${pill(x.passed ? "Pass" : "Blocked", x.passed ? "good" : "bad")}</div>`,
    )
    .join(
      "",
    )}</section>${arr(D.benchmarks).length ? `<section class="panel contract"><h3>Blind baseline comparisons</h3>${D.benchmarks.map((b) => `<p><strong>Candidate ${esc(b.candidate_position)} · winner ${esc(b.result.winner)}</strong><br>${esc(b.result.reason)}</p>`).join("")}</section>` : ""}`;
}
function evidenceView() {
  return `<div class="notice">${ico("quality")}<div>Each supported mechanic needs a native search citation, a fetched allowlisted official page, and a matching quote. Inaccessible, unmatched or older-than-seven-day evidence blocks handoff. This checks platform mechanics, not whether an alleged real event happened.</div></div>${arr(
    D.research.claims,
  )
    .map((c) => {
      const v = arr(D.research.verification?.verdicts).find((v) => v.claim_id === c.id);
      return `<article class="panel evidence"><div class="panel-head"><h3>${esc(c.id)}</h3>${pill(v?.verdict || "unknown", v?.verdict === "supported" ? "good" : "bad")}</div><p class="claim">${esc(c.text)}</p><div class="chip-row">${arr(
        c.line_ids,
      )
        .map((n) => pill(lineID(n)))
        .join(
          "",
        )}</div>${v ? `<blockquote>${esc(v.quote)}</blockquote><p>${esc(v.reason)}</p>${safeLink(v.url, esc(v.url), "sub")}` : ""}</article>`;
    })
    .join(
      "",
    )}<details class="panel contract"><summary>Inspect the frozen audit record</summary><pre>${esc(JSON.stringify({ policy: D.snapshot.policy_version, prompts: D.snapshot.prompt_version, models: D.snapshot.settings, source_ids: arr(D.snapshot.sources).map((s) => ({ id: s.id, hash: s.hash })), baseline_ids: D.snapshot.baseline_ids, script_hash: D.hash, pages: arr(D.research.pages).map((p) => ({ url: p.url, hash: p.hash, retrieved_at: p.retrieved_at, error: p.error })) }, null, 2))}</pre></details>`;
}
function tasksView() {
  return `<section class="panel contract"><h3>Production handoff</h3><p>${D.approved ? "Tasks have been created in the execution board." : "These are proposed tasks. Approval creates them in the execution board."}</p>${arr(
    D.script.tasks,
  )
    .map(
      (t) =>
        `<div class="cue"><div class="eyebrow">${esc(t.role)}</div><h4>${esc(t.task)}</h4><p>${esc(t.acceptance)}</p></div>`,
    )
    .join("")}</section>`;
}
function inspector(job) {
  const c = D.checks;
  return `<aside class="inspector"><section class="panel contract"><div class="panel-head"><h3>Revision ${D.version}</h3>${statusPill(D.approved ? "approved" : c.ready ? "review" : "needs_changes")}</div><div class="mini-rule"><span>Spoken words</span><strong>${num(c.spoken_words)}</strong></div><div class="mini-rule"><span>Voiceover @ 225 wpm</span><strong>${minute(c.voiceover_seconds)}</strong></div><div class="mini-rule"><span>Estimated finished cut</span><strong>${minute(c.finished_seconds)}</strong></div><div class="mini-rule"><span>Quality score</span><strong>${c.score}/100</strong></div><div class="mini-rule"><span>Blocking checks</span><strong>${c.items.filter((x) => !x.passed && x.blocking).length}</strong></div><div class="subtle-divider"></div>${btn("revise", ico("edit") + " Request a revision", "small", job ? "disabled" : "")}${btn("feedback", "Give editorial feedback", "small ghost")}${!c.calibrated ? btn("accept-gold", "Accept as first gold baseline", "small ghost") : ""}<div class="subtle-divider"></div>${D.approved ? `<a class="btn primary small" href="/api/drafts/${D.id}/export?format=zip">${ico("download")} Production ZIP</a><p class="sub">Voice actor + editor.<br>Markdown and colour PDFs.</p>` : btn("approve-draft", ico("check") + " Approve for production", "primary small", !c.ready || job ? "disabled" : "")}<p class="model-notes">Editing creates a new unreviewed revision. A high score alone never permits a download.</p></section><section class="panel contract"><div class="eyebrow">HUMAN IN THE LOOP</div><p>Read the story. Check that the disclosure fits, the payoff makes sense, and the evidence really supports the claim. Your sign-off is an editorial decision.</p></section></aside>`;
}
function renderLogin() {
  clearTimeout(pollTimer);
  $("#app").innerHTML =
    `<div class="login-page"><section class="login-brand"><a class="brand" href="#"><img src="/mark.svg" width="42" height="42" alt=""><span class="brand-word">bolty<span>studio</span></span></a><div class="login-copy"><div class="eyebrow">YOUR PRIVATE WRITING ROOM</div><h1>A better story.<br>Every time you<br><em>sit down to write.</em></h1><p>The references, the judgement, the scripts, the handoff.<br>One small studio built around the Bolty brief.</p></div><div class="login-footer">HOSTED MODELS. ORIGINAL STORIES. YOUR STANDARD.</div></section><section class="login-form-side"><form id="login-form" class="login-form"><div class="eyebrow">BOLTY WORKSPACE</div><h2>Back to the writing room.</h2><p>Sign in with your studio password.</p><label class="field">Studio password<input name="password" type="password" autocomplete="current-password" required autofocus minlength="1"></label><div class="inline-error" id="login-error" role="alert"></div><button class="btn primary" type="submit">Open studio ${ico("arrow")}</button><p class="model-notes">Single-workspace access. API keys stay on your server.</p></form></section></div>`;
}
function showModal(title, body, wide = false) {
  lastFocus = document.activeElement;
  modalOpen = true;
  $("#modal-root").innerHTML =
    `<div class="modal-backdrop"><section class="modal ${wide ? "wide" : ""}" role="dialog" aria-modal="true" aria-labelledby="modal-title"><div class="modal-header"><h2 id="modal-title">${esc(title)}</h2><button class="icon-btn" data-action="close-modal" aria-label="Close dialog">${ico("close")}</button></div><div class="modal-body">${body}</div></section></div>`;
  const focus = $("input:not([type=checkbox]),textarea,button", $("#modal-root"));
  focus?.focus();
}
function closeModal() {
  modalOpen = false;
  $("#modal-root").innerHTML = "";
  lastFocus?.focus?.();
}
function sourceModal(s = null, kind = "reference") {
  sourceEditing = s;
  const k = s?.kind || kind;
  showModal(
    s ? "Reference details" : k === "gold" ? "Add an approved gold script" : "Add a source",
    `<form id="source-form"><div class="fields"><label class="field">Title<input name="title" value="${esc(s?.title || "")}" required minlength="3" maxlength="180"></label><label class="field">Source type<select name="kind">${[
      ["reference", "Style reference"],
      ["gold", "Gold script"],
      ["case", "Redacted case record"],
    ]
      .map(([v, t]) => `<option value="${v}" ${k === v ? "selected" : ""}>${t}</option>`)
      .join(
        "",
      )}</select></label></div><label class="field">YouTube URL (optional)<input name="url" value="${esc(s?.url || "")}" placeholder="https://www.youtube.com/watch?v=…"></label><label class="field">Transcript / spoken gold script / redacted case record<textarea class="transcript" name="transcript" maxlength="180000" placeholder="Paste authorized text. Timestamps are accepted.">${esc(s?.transcript || "")}</textarea></label><label class="upload-control">${ico("upload")} Import .txt, .srt or .vtt<input id="transcript-file" type="file" accept=".txt,.srt,.vtt,text/plain"></label><p class="file-note">A link is only a bookmark. Gold scripts need at least 1,800 spoken words. Case records must remove identifying details.</p><label class="field">Your visual observations (optional)<textarea name="visual_notes" maxlength="12000" placeholder="Describe pacing, framing or editing you actually observed. The app does not watch the video.">${esc(s?.visual_notes || "")}</textarea></label><label class="check-label"><input name="rights" type="checkbox" ${s?.rights ? "checked" : ""}>I am authorized to use this content and have removed private or identifying information.</label>${
      s?.analysis
        ? `<div class="analysis-card"><div class="eyebrow">EXTRACTED CRAFT / ${esc(s.analysis.visual_coverage)}</div><h3>${esc(s.analysis.summary)}</h3>${arr(
            s.analysis.craft,
          )
            .map(
              (c) =>
                `<h4>${esc(c.technique)}</h4><blockquote>${esc(c.evidence)}</blockquote><p>${esc(c.application)}</p>`,
            )
            .join(
              "",
            )}<div class="callout-label">Do not transfer</div><p>${arr(s.analysis.do_not_transfer).map(esc).join(" · ")}</p></div>`
        : ""
    }<div class="form-footer">${s ? btn("archive-source", "Archive", "ghost small", `data-id="${s.id}"`) : ""}<span class="spacer"></span><button class="btn" type="submit" value="save">Save source</button><button class="btn primary" type="submit" value="analyse">Save & analyse</button>${s || k !== "reference" ? '<button class="btn" type="submit" value="approve">Save & approve</button>' : ""}</div>${s?.video_id && S.capabilities.youtube_oauth ? `<div class="form-footer">${btn("import-captions", "Import owner-authorized captions", "small ghost", `data-id="${s.id}"`)}<span class="sub">Requires permission to edit this video.</span></div>` : ""}</form>`,
    true,
  );
}
function projectModal(p = null) {
  const refs = approvedRefs(),
    cases = S.sources.filter((s) => s.kind === "case" && s.status === "approved");
  showModal(
    p ? "Story direction" : "Start a new story",
    `<form id="project-form" data-id="${p?.id || ""}"><label class="field">Working title<input name="title" value="${esc(p?.title || "")}" required minlength="3" maxlength="180" placeholder="A moderator charging players to stay safe"></label><label class="field">What is the injustice? What should change by the end?<textarea name="premise" required minlength="20" maxlength="5000" placeholder="Describe an original premise, the stakes, and the remedy. The agent will develop the story, not copy a reference.">${esc(p?.premise || "")}</textarea></label><div class="fields"><label class="field">Platform (internal planning)<select name="platform">${["Roblox", "Minecraft", "Discord"].map((v) => `<option ${p?.platform === v ? "selected" : ""}>${v}</option>`).join("")}</select></label><label class="field">Story category<select name="category">${S.categories.map((v) => `<option value="${esc(v)}" ${p?.category === v ? "selected" : ""}>${esc(label(v))}</option>`).join("")}</select></label></div><label class="field">Story basis<select name="basis"><option value="fictional" ${p?.basis !== "documented" ? "selected" : ""}>Fictional scenario — audible disclosure required</option><option value="documented" ${p?.basis === "documented" ? "selected" : ""}>Documented reconstruction — approved case required</option></select></label><label class="field">Approved case record (documented mode only)<select name="case_source_id"><option value="">No case record</option>${cases.map((s) => `<option value="${s.id}" ${p?.case_source_id === s.id ? "selected" : ""}>${esc(s.title)}</option>`).join("")}</select></label><div class="field"><span>Approved style references · choose up to six</span><div class="reference-picks">${refs.length ? refs.map((s) => `<label class="check-label"><input name="source_ids" type="checkbox" value="${s.id}" ${p ? (arr(p.source_ids).includes(s.id) ? "checked" : "") : "checked"}>${esc(s.title)}</label>`).join("") : '<p class="sub">No approved references yet. Save the direction now; approve a reference before generating.</p>'}</div></div><div class="form-footer"><button class="btn primary" type="submit">${p ? "Save direction" : "Create story"} ${ico("arrow")}</button></div></form>`,
    true,
  );
}
function confirmModal(title, text, action, buttonText, extra = "") {
  showModal(
    title,
    `<p>${esc(text)}</p><div class="form-footer">${btn("close-modal", "Cancel", "ghost")}${btn(action, esc(buttonText), "primary", extra)}</div>`,
  );
}
async function startJob(kind, input = "") {
  const p = currentProject();
  await api("/jobs", "POST", {
    kind,
    project_id: p?.id || "",
    source_id: "",
    draft_id: kind === "review" || kind === "revise" ? D.id : "",
    input,
  });
  closeModal();
  toast("Added to the studio queue.");
  await refresh();
}
function lineModal(id) {
  const l = D.script.lines.find((l) => l.id === Number(id)),
    c = D.script.cues.find((c) => c.line_id === l.id);
  showModal(
    `Edit L${String(l.id).padStart(2, "0")} · new revision`,
    `<form id="line-form" data-id="${l.id}"><div class="fields"><label class="field">Reading voice<select name="speaker">${D.script.voices.map((v) => `<option ${v.name === l.speaker ? "selected" : ""}>${esc(v.name)}</option>`).join("")}</select></label><label class="field">Delivery note<input name="delivery" value="${esc(l.delivery)}" maxlength="140" required></label></div><label class="field">Spoken text<textarea name="text" required>${esc(l.text)}</textarea></label><label class="check-label"><input name="second_take" type="checkbox" ${l.second_take ? "checked" : ""}>★ Record a second take</label><label class="field">Gameplay / general direction<textarea name="gameplay">${esc(c?.gameplay || "")}</textarea></label><label class="field">Editor direction<textarea name="edit">${esc(c?.edit || "")}</textarea></label><label class="field">Sound / silence<input name="sound" value="${esc(c?.sound || "")}"></label><label class="field">Exclusive non-spoken hold (seconds)<input name="hold_seconds" type="number" min="0" max="45" step="0.1" value="${c?.hold_seconds || 0}"></label><p class="notice warning">This saves a new revision and clears its verification and quality review. Previous signed-off files remain versioned.</p><div class="form-footer"><button class="btn primary" type="submit">Save new revision</button></div></form>`,
    true,
  );
}
async function act(action, el) {
  switch (action) {
    case "menu": {
      const open = $(".sidebar")?.classList.toggle("mobile-open");
      $(".nav-scrim")?.classList.toggle("visible", !!open);
      break;
    }
    case "logout":
      await api("/logout", "POST", {});
      S = null;
      renderLogin();
      break;
    case "close-modal":
      closeModal();
      break;
    case "new-project":
      projectModal();
      break;
    case "edit-project":
      projectModal(currentProject());
      break;
    case "new-source":
      sourceModal();
      break;
    case "new-gold":
      sourceModal(null, "gold");
      break;
    case "edit-source":
      sourceModal(await api("/sources/" + el.dataset.id));
      break;
    case "source-tab":
      sourceTab = el.dataset.value;
      renderApp();
      break;
    case "story-tab":
      storyTab = el.dataset.value;
      renderApp();
      break;
    case "go-studio":
    case "go-library":
    case "go-quality":
    case "go-models":
      navigate(action.slice(3));
      break;
    case "archive-source":
      confirmModal(
        "Archive this source?",
        "It will leave future agent memory. Frozen run snapshots retain the text for auditability; this is not a historical-data purge.",
        "confirm-archive",
        "Archive source",
        `data-id="${el.dataset.id}"`,
      );
      break;
    case "confirm-archive":
      await api("/sources/" + el.dataset.id + "/archive", "POST", {});
      closeModal();
      await refresh();
      toast("Source archived for future runs.");
      break;
    case "import-captions":
      await api("/jobs", "POST", {
        kind: "import",
        source_id: el.dataset.id,
        project_id: "",
        draft_id: "",
        input: "",
      });
      closeModal();
      await refresh();
      toast("Owner caption import queued.");
      break;
    case "ideas":
      await startJob("ideas");
      break;
    case "write":
      await startJob("write");
      break;
    case "review":
      selectedDraft = "";
      pinnedVersion = false;
      await startJob("review");
      break;
    case "cancel-job":
      await api("/jobs/" + el.dataset.id + "/cancel", "POST", {});
      await refresh();
      break;
    case "retry-job":
      confirmModal(
        "Retry this stopped run?",
        "Completed steps are reused. An interrupted network request may already have been billed, so repeating an uncertain step can incur another charge.",
        "confirm-retry",
        "Retry with possible charges",
        `data-id="${el.dataset.id}"`,
      );
      break;
    case "confirm-retry":
      await api("/jobs/" + el.dataset.id + "/retry", "POST", { accept_possible_charges: true });
      closeModal();
      await refresh();
      break;
    case "use-idea": {
      const j = S.jobs.find((j) => j.id === el.dataset.job),
        i = j.result.ideas[Number(el.dataset.index)],
        p = currentProject();
      await api("/projects/" + p.id, "PUT", {
        title: i.title,
        premise: `${i.hook}\nInjustice: ${i.injustice}\nEvidence ladder: ${i.evidence_ladder.join(" → ")}\nRemedy: ${i.payoff}`,
        platform: i.platform,
        category: i.category,
        basis: p.basis,
        case_source_id: p.case_source_id,
        source_ids: p.source_ids,
      });
      await refresh();
      toast("Story direction updated.");
      break;
    }
    case "edit-line":
      lineModal(el.dataset.id);
      break;
    case "revise":
      showModal(
        "Give the writer a direction",
        `<form id="revision-form"><label class="field">What should change?<textarea name="input" required minlength="10" maxlength="5000" placeholder="Be specific: tighten the opening, delay the worst receipt, or make the remedy more credible."></textarea></label><p class="sub">The writer returns a full new script, then repeats verification and quality checks.</p><div class="form-footer"><button class="btn primary" type="submit">Request revision</button></div></form>`,
      );
      break;
    case "feedback":
      showModal(
        "Editorial feedback",
        `<form id="feedback-form"><label class="field">Decision<select name="decision"><option value="keep">Keep — this is working</option><option value="change">Change — explain what to improve</option><option value="reject">Reject — below the standard</option></select></label><label class="field">Your judgement<textarea name="notes" minlength="5" maxlength="5000" required></textarea></label><label class="check-label"><input name="reusable" type="checkbox">Approve this feedback as reusable guidance for future stories.</label><div class="form-footer"><button class="btn primary" type="submit">Save feedback</button></div></form>`,
      );
      break;
    case "accept-gold":
      confirmModal(
        "Freeze this as your gold baseline?",
        "Only do this after reading the whole script and deciding it represents your standard. All non-baseline checks must already pass. This does not approve a production export; a fresh review must compare against the new baseline.",
        "confirm-gold",
        "Accept as gold baseline",
      );
      break;
    case "confirm-gold":
      await api("/drafts/" + D.id + "/gold", "POST", { accept_as_baseline: true });
      closeModal();
      await refresh();
      toast("Baseline frozen. Run a fresh review to use it.");
      break;
    case "approve-draft":
      confirmModal(
        "Approve this exact revision?",
        "I have reviewed the story, disclosure, evidence and creative quality. I authorize this exact script for production. Real footage, assets and privacy must still be checked during execution.",
        "confirm-approve",
        "Sign off for production",
      );
      break;
    case "confirm-approve":
      D = await api("/drafts/" + D.id + "/approve", "POST", {
        expected_hash: D.hash,
        editorial_signoff: true,
      });
      closeModal();
      await refresh();
      toast("Revision approved. Production tasks created.");
      break;
  }
}
document.addEventListener("click", async (e) => {
  const el = e.target.closest("[data-action]");
  if (!el || el.disabled || el.dataset.action === "task") return;
  e.preventDefault();
  if (busy) return;
  busy = true;
  try {
    await act(el.dataset.action, el);
  } catch (err) {
    toast(err.message, true);
  } finally {
    busy = false;
  }
});
document.addEventListener("submit", async (e) => {
  e.preventDefault();
  if (busy) return;
  busy = true;
  const form = e.target,
    data = new FormData(form),
    button = e.submitter;
  button?.setAttribute("disabled", "");
  try {
    switch (form.id) {
      case "login-form":
        await api("/login", "POST", { password: data.get("password") });
        await refresh();
        break;
      case "models-form": {
        const settings = {};
        for (const role of ["writer", "judge", "researcher"])
          settings[role] = {
            provider: data.get(role + "_provider"),
            model: data.get(role + "_model").trim(),
          };
        for (const n of ["pass_score", "min_dimension", "max_calls", "daily_calls"])
          settings[n] = Number(data.get(n));
        await api("/settings", "PUT", settings);
        await refresh();
        toast("Settings saved. Existing drafts require fresh review.");
        break;
      }
      case "source-form": {
        const payload = {
          title: data.get("title"),
          kind: data.get("kind"),
          url: data.get("url").trim(),
          transcript: data.get("transcript"),
          visual_notes: data.get("visual_notes"),
          rights: data.has("rights"),
        };
        const s = await api(
          "/sources" + (sourceEditing ? "/" + sourceEditing.id : ""),
          sourceEditing ? "PUT" : "POST",
          payload,
        );
        sourceEditing = s;
        if (button?.value === "analyse") {
          if (s.kind !== "reference")
            throw new Error(
              "Gold scripts and case records use explicit approval, not reference analysis. The source was saved.",
            );
          await api("/jobs", "POST", {
            kind: "analyse",
            source_id: s.id,
            project_id: "",
            draft_id: "",
            input: "",
          });
        } else if (button?.value === "approve") {
          await api("/sources/" + s.id + "/approve", "POST", {});
        }
        closeModal();
        await refresh();
        toast(
          button?.value === "analyse"
            ? "Reference analysis queued. Review it before approving."
            : "Source saved.",
        );
        break;
      }
      case "project-form": {
        const payload = {
          title: data.get("title"),
          premise: data.get("premise"),
          platform: data.get("platform"),
          category: data.get("category"),
          basis: data.get("basis"),
          case_source_id: data.get("case_source_id"),
          source_ids: data.getAll("source_ids"),
        };
        const p = await api(
          "/projects" + (form.dataset.id ? "/" + form.dataset.id : ""),
          form.dataset.id ? "PUT" : "POST",
          payload,
        );
        closeModal();
        await refresh(false);
        selectedDraft = "";
        pinnedVersion = false;
        D = null;
        navigate("story/" + p.id);
        if (projectID() === p.id) await refresh();
        break;
      }
      case "revision-form":
        selectedDraft = "";
        pinnedVersion = false;
        await startJob("revise", data.get("input"));
        break;
      case "feedback-form":
        await api("/feedback", "POST", {
          draft_id: D.id,
          decision: data.get("decision"),
          notes: data.get("notes"),
          reusable: data.has("reusable"),
        });
        closeModal();
        await refresh();
        toast("Editorial feedback saved.");
        break;
      case "line-form": {
        const script = structuredClone(D.script),
          id = Number(form.dataset.id),
          l = script.lines.find((x) => x.id === id);
        Object.assign(l, {
          text: data.get("text"),
          delivery: data.get("delivery"),
          speaker: data.get("speaker"),
          second_take: data.has("second_take"),
        });
        let cue = script.cues.find((x) => x.line_id === id);
        if (!cue) {
          cue = { line_id: id };
          script.cues.push(cue);
        }
        Object.assign(cue, {
          gameplay: data.get("gameplay"),
          edit: data.get("edit"),
          sound: data.get("sound"),
          hold_seconds: Number(data.get("hold_seconds")),
        });
        const d = await api("/drafts/" + D.id, "PUT", { expected_hash: D.hash, script });
        selectedDraft = d.id;
        closeModal();
        await refresh();
        toast("New revision saved. Run a fresh review before handoff.");
        break;
      }
    }
  } catch (err) {
    if (form.id === "login-form") $("#login-error").textContent = err.message;
    else toast(err.message, true);
  } finally {
    busy = false;
    button?.removeAttribute("disabled");
  }
});
document.addEventListener("change", async (e) => {
  try {
    if (e.target.id === "transcript-file") {
      const f = e.target.files?.[0];
      if (!f) return;
      if (f.size > 1000000) throw new Error("Transcript files must be under 1 MB.");
      if (!/\.(txt|srt|vtt)$/i.test(f.name)) throw new Error("Choose a .txt, .srt or .vtt file.");
      $("#source-form [name=transcript]").value = await f.text();
      toast("Transcript loaded. Save to store it.");
    } else if (e.target.id === "version-select") {
      selectedDraft = e.target.value;
      pinnedVersion = true;
      await refresh();
    } else if (e.target.dataset.action === "task") {
      await api("/tasks/" + e.target.dataset.id, "PUT", { done: e.target.checked });
      await refresh();
    }
  } catch (err) {
    toast(err.message, true);
  }
});
document.addEventListener("keydown", (e) => {
  if (e.key === "Escape" && modalOpen) closeModal();
  if (e.key === "Tab" && modalOpen) {
    const els = [...$("#modal-root").querySelectorAll("button,input,select,textarea,a[href]")].filter(
      (x) => !x.disabled,
    );
    const first = els[0],
      last = els.at(-1);
    if (e.shiftKey && document.activeElement === first) {
      e.preventDefault();
      last.focus();
    } else if (!e.shiftKey && document.activeElement === last) {
      e.preventDefault();
      first.focus();
    }
  }
});
window.addEventListener("hashchange", async () => {
  selectedDraft = "";
  pinnedVersion = false;
  D = null;
  storyTab = "voice";
  scrollTo(0, 0);
  if (S)
    try {
      await refresh();
    } catch (err) {
      toast(err.message, true);
    }
});
refresh().catch((err) => {
  if (!S) renderLogin();
  else toast(err.message, true);
});
