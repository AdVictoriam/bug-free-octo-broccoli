package studio

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"bolty.studio/internal/sqlite"
)

type Server struct {
	Store     *Store
	Engine    *Engine
	Assets    fs.FS
	Password  string
	Origin    string
	Secure    bool
	Python    string
	Renderer  string
	Log       *slog.Logger
	mu        sync.Mutex
	limits    map[string]limit
	exporting chan struct{}
}
type limit struct {
	N     int
	Start time.Time
}

func (s *Server) Handler() http.Handler {
	s.limits = map[string]limit{}
	s.exporting = make(chan struct{}, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("POST /api/login", s.login)
	mux.HandleFunc("POST /api/logout", s.authorize(s.logout))
	mux.HandleFunc("GET /api/state", s.authorize(s.state))
	mux.HandleFunc("PUT /api/settings", s.authorize(s.settings))
	mux.HandleFunc("POST /api/sources", s.authorize(s.saveSource))
	mux.HandleFunc("GET /api/sources/{id}", s.authorize(s.getSource))
	mux.HandleFunc("PUT /api/sources/{id}", s.authorize(s.saveSource))
	mux.HandleFunc("POST /api/sources/{id}/approve", s.authorize(s.approveSource))
	mux.HandleFunc("POST /api/sources/{id}/archive", s.authorize(s.archiveSource))
	mux.HandleFunc("POST /api/projects", s.authorize(s.saveProject))
	mux.HandleFunc("PUT /api/projects/{id}", s.authorize(s.saveProject))
	mux.HandleFunc("POST /api/jobs", s.authorize(s.createJob))
	mux.HandleFunc("POST /api/jobs/{id}/cancel", s.authorize(s.cancelJob))
	mux.HandleFunc("POST /api/jobs/{id}/retry", s.authorize(s.retryJob))
	mux.HandleFunc("GET /api/drafts/{id}", s.authorize(s.getDraft))
	mux.HandleFunc("PUT /api/drafts/{id}", s.authorize(s.editDraft))
	mux.HandleFunc("POST /api/drafts/{id}/approve", s.authorize(s.approveDraft))
	mux.HandleFunc("POST /api/drafts/{id}/gold", s.authorize(s.draftGold))
	mux.HandleFunc("GET /api/drafts/{id}/export", s.authorize(s.exportDraft))
	mux.HandleFunc("POST /api/feedback", s.authorize(s.feedback))
	mux.HandleFunc("PUT /api/tasks/{id}", s.authorize(s.task))
	mux.HandleFunc("GET /", s.static)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; font-src 'self'; frame-src 'none'; object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'")
		if s.Secure {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000")
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		defer func() {
			if p := recover(); p != nil {
				s.Log.Error("http panic", "path", r.URL.Path, "panic", p)
				respondError(w, 500, "Internal request failure; no approval granted")
			}
		}()
		if r.Method != "GET" && r.Method != "HEAD" {
			if r.Header.Get("Origin") != s.Origin {
				respondError(w, 403, "Origin check failed")
				return
			}
			if r.Header.Get("Content-Type") != "application/json" {
				respondError(w, 415, "Use application/json")
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
		}
		mux.ServeHTTP(w, r)
	})
}
func reply(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func respondError(w http.ResponseWriter, status int, msg string) {
	reply(w, status, map[string]string{"error": msg})
}
func decode(r *http.Request, v any) error {
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return errors.New("invalid request JSON or unsupported field")
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return errors.New("request must contain exactly one JSON object")
	}
	return nil
}
func (s *Server) authorize(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, e := r.Cookie("bolty_session")
		if e != nil || len(cookie.Value) != 64 {
			respondError(w, 401, "Sign in to your studio")
			return
		}
		rows, e := s.Store.DB.Query(`SELECT expires_at FROM sessions WHERE token_hash=?`, tokenHash(cookie.Value))
		if e != nil {
			respondError(w, 503, "Session store unavailable")
			return
		}
		if len(rows) != 1 || rows[0].Int("expires_at") <= time.Now().Unix() {
			respondError(w, 401, "Session expired; sign in again")
			return
		}
		next(w, r)
	}
}
func tokenHash(v string) string { a := sha256.Sum256([]byte(v)); return hex.EncodeToString(a[:]) }
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	s.mu.Lock()
	lim := s.limits[host]
	if time.Since(lim.Start) > 10*time.Minute {
		lim = limit{Start: time.Now()}
	}
	lim.N++
	if len(s.limits) > 2048 {
		for k, v := range s.limits {
			if time.Since(v.Start) > 10*time.Minute {
				delete(s.limits, k)
			}
		}
	}
	s.limits[host] = lim
	s.mu.Unlock()
	if lim.N > 10 {
		w.Header().Set("Retry-After", "600")
		respondError(w, 429, "Too many sign-in attempts; try again in ten minutes")
		return
	}
	var in struct {
		Password string `json:"password"`
	}
	if e := decode(r, &in); e != nil {
		respondError(w, 400, e.Error())
		return
	}
	a := sha256.Sum256([]byte(in.Password))
	b := sha256.Sum256([]byte(s.Password))
	if subtle.ConstantTimeCompare(a[:], b[:]) != 1 {
		respondError(w, 401, "Incorrect studio password")
		return
	}
	token := newID() + newID()
	if _, e := s.Store.DB.Exec(`INSERT INTO sessions VALUES(?,?)`, tokenHash(token), time.Now().Add(24*time.Hour).Unix()); e != nil {
		respondError(w, 503, "Session store unavailable")
		return
	}
	s.Store.DB.Exec(`DELETE FROM sessions WHERE expires_at<?`, time.Now().Unix())
	s.mu.Lock()
	delete(s.limits, host)
	s.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "bolty_session", Value: token, Path: "/", HttpOnly: true, Secure: s.Secure, SameSite: http.SameSiteStrictMode, MaxAge: 86400})
	reply(w, 200, map[string]bool{"ok": true})
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if c, e := r.Cookie("bolty_session"); e == nil {
		s.Store.DB.Exec(`DELETE FROM sessions WHERE token_hash=?`, tokenHash(c.Value))
	}
	http.SetCookie(w, &http.Cookie{Name: "bolty_session", Path: "/", Value: "", HttpOnly: true, Secure: s.Secure, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	reply(w, 200, map[string]bool{"ok": true})
}
func (s *Server) state(w http.ResponseWriter, r *http.Request) {
	settings, e := s.Store.Settings()
	if e != nil {
		respondError(w, 503, "Could not load settings")
		return
	}
	sources, e := s.Store.Sources()
	if e != nil {
		respondError(w, 503, "Could not load sources")
		return
	}
	projects, e := s.Store.Projects()
	if e != nil {
		respondError(w, 503, "Could not load projects")
		return
	}
	jobs, e := s.Store.Jobs()
	if e != nil {
		respondError(w, 503, "Could not load jobs")
		return
	}
	drafts, e := s.Store.Drafts()
	if e != nil {
		respondError(w, 503, "Could not load drafts")
		return
	}
	tasks, e := s.Store.Tasks()
	if e != nil {
		respondError(w, 503, "Could not load production tasks")
		return
	}
	feedback, e := s.Store.Feedback()
	if e != nil {
		respondError(w, 503, "Could not load feedback")
		return
	}
	type summary struct {
		ID        string `json:"id"`
		ProjectID string `json:"project_id"`
		Version   int    `json:"version"`
		Title     string `json:"title"`
		Checks    Checks `json:"checks"`
		Approved  bool   `json:"approved"`
		CreatedAt string `json:"created_at"`
	}
	summaries := []summary{}
	for _, d := range drafts {
		checks := d.Checks // Full/fresh checks run on draft read, approval and export.
		summaries = append(summaries, summary{d.ID, d.ProjectID, d.Version, d.Script.Title, checks, d.Approved, d.CreatedAt})
	}
	// Snapshot source bodies and full model traces are not repeatedly sent in the
	// polling response. Detailed data is available on the specific draft screen.
	for i := range jobs {
		jobs[i].Snapshot = Snapshot{}
	}
	reply(w, 200, map[string]any{"settings": settings, "sources": sourceSummaries(sources), "projects": projects, "jobs": jobs, "drafts": summaries, "tasks": tasks, "feedback": feedback, "policy": ChannelPolicy, "policy_version": PolicyVersion, "prompt_version": PromptVersion, "categories": Categories, "capabilities": map[string]any{"openai": s.Engine.Providers.OpenAIKey != "", "anthropic": s.Engine.Providers.AnthropicKey != "", "youtube_oauth": s.Engine.YouTube.Ready(), "single_workspace": true}})
}
func (s *Server) settings(w http.ResponseWriter, r *http.Request) {
	var v Settings
	if e := decode(r, &v); e != nil {
		respondError(w, 400, e.Error())
		return
	}
	for _, role := range []Role{v.Writer, v.Judge, v.Researcher} {
		if !contains([]string{"openai", "anthropic"}, role.Provider) || len(role.Model) > 120 {
			respondError(w, 400, "Use OpenAI or Anthropic and an exact model ID under 120 characters")
			return
		}
	}
	if v.PassScore < 85 || v.PassScore > 100 || v.MinDimension < 7 || v.MinDimension > 10 || v.MaxCalls < 10 || v.MaxCalls > 40 || v.DailyCalls < 10 || v.DailyCalls > 1000 {
		respondError(w, 400, "Scores must be 85–100, dimensions 7–10, run calls 10–40, daily calls 10–1000")
		return
	}
	if e := s.Store.Put("settings", "active", v); e != nil {
		respondError(w, 500, "Could not save model settings")
		return
	}
	reply(w, 200, v)
}
func (s *Server) saveSource(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Kind        string `json:"kind"`
		Title       string `json:"title"`
		URL         string `json:"url"`
		Transcript  string `json:"transcript"`
		VisualNotes string `json:"visual_notes"`
		Rights      bool   `json:"rights"`
	}
	if e := decode(r, &in); e != nil {
		respondError(w, 400, e.Error())
		return
	}
	if !contains([]string{"reference", "gold", "case"}, in.Kind) || len(in.Title) < 3 || len(in.Title) > 180 || len(in.Transcript) > 180000 || len(in.VisualNotes) > 12000 {
		respondError(w, 400, "Choose a source type and a title (3–180 characters). Transcript limit: 180,000 characters")
		return
	}
	var src Source
	id := r.PathValue("id")
	if id != "" {
		var e error
		src, e = s.Store.Source(id)
		if e != nil {
			respondError(w, 404, "Source not found")
			return
		}
	} else {
		src.ID = newID()
		src.CreatedAt = now()
	}
	oldKind, oldURL := src.Kind, src.URL
	src.Kind = in.Kind
	src.Title = in.Title
	src.URL = ""
	src.VideoID = ""
	if strings.TrimSpace(in.URL) != "" {
		video, e := YouTubeID(in.URL)
		if e != nil {
			respondError(w, 400, e.Error())
			return
		}
		src.VideoID = video
		src.URL = "https://www.youtube.com/watch?v=" + video
	}
	text := NormalizeTranscript(in.Transcript)
	newHash := hash([]string{text, in.VisualNotes})
	changed := src.Hash != newHash || src.Rights != in.Rights || oldKind != src.Kind || oldURL != src.URL
	src.Transcript = text
	src.VisualNotes = in.VisualNotes
	src.Hash = newHash
	src.Rights = in.Rights
	if changed || id == "" {
		src.Analysis = nil
		src.Status = "awaiting_transcript"
		if len(text) >= 200 {
			src.Status = "needs_analysis"
			if src.Kind != "reference" {
				src.Status = "review"
			}
		}
	}
	if !src.Rights && len(src.Transcript) > 0 {
		respondError(w, 400, "Confirm rights and redact private information before saving source content")
		return
	}
	if e := s.Store.SaveSource(src); e != nil {
		respondError(w, 500, "Could not save reference")
		return
	}
	reply(w, 200, src)
}
func (s *Server) approveSource(w http.ResponseWriter, r *http.Request) {
	src, e := s.Store.Source(r.PathValue("id"))
	if e != nil {
		respondError(w, 404, "Source not found")
		return
	}
	if !src.Rights || len(src.Transcript) < 200 {
		respondError(w, 409, "Source needs an authorised transcript first")
		return
	}
	if src.Kind == "reference" && src.Analysis == nil {
		respondError(w, 409, "Analyse this reference and review the evidence before approving it")
		return
	}
	if src.Kind == "gold" && len(wordRE.FindAllString(src.Transcript, -1)) < 1800 {
		respondError(w, 409, "A gold baseline needs a complete long-form script, not a short excerpt (minimum 1,800 words)")
		return
	}
	src.Status = "approved"
	if e = s.Store.SaveSource(src); e != nil {
		respondError(w, 500, "Could not approve source")
		return
	}
	reply(w, 200, src)
}
func (s *Server) archiveSource(w http.ResponseWriter, r *http.Request) {
	src, e := s.Store.Source(r.PathValue("id"))
	if e != nil {
		respondError(w, 404, "Source not found")
		return
	}
	src.Status = "archived"
	if e = s.Store.SaveSource(src); e != nil {
		respondError(w, 500, "Could not archive source")
		return
	}
	reply(w, 200, map[string]string{"status": "archived", "retention": "Removed from future learning. Existing immutable run snapshots retain their original source data."})
}
func (s *Server) saveProject(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title        string   `json:"title"`
		Premise      string   `json:"premise"`
		Platform     string   `json:"platform"`
		Category     string   `json:"category"`
		Basis        string   `json:"basis"`
		CaseSourceID string   `json:"case_source_id"`
		SourceIDs    []string `json:"source_ids"`
	}
	if e := decode(r, &in); e != nil {
		respondError(w, 400, e.Error())
		return
	}
	if len(in.Title) < 3 || len(in.Title) > 180 || len(in.Premise) < 20 || len(in.Premise) > 8000 || !contains([]string{"Roblox", "Minecraft", "Discord"}, in.Platform) || !contains(Categories, in.Category) || !contains([]string{"fictional", "documented"}, in.Basis) || len(in.SourceIDs) > 6 {
		respondError(w, 400, "Provide a title, a 20–8,000-character premise, a supported platform/category, a story basis and at most six references")
		return
	}
	for _, id := range in.SourceIDs {
		src, e := s.Store.Source(id)
		if e != nil || src.Status != "approved" || src.Kind != "reference" {
			respondError(w, 409, "Select only approved reference sources; gold scripts are held out of writer context")
			return
		}
	}
	if in.Basis == "documented" {
		src, e := s.Store.Source(in.CaseSourceID)
		if e != nil || src.Kind != "case" || src.Status != "approved" {
			respondError(w, 409, "Documented mode requires an approved redacted case record")
			return
		}
	}
	p := Project{ID: r.PathValue("id"), Title: in.Title, Premise: in.Premise, Platform: in.Platform, Category: in.Category, Basis: in.Basis, CaseSourceID: in.CaseSourceID, SourceIDs: in.SourceIDs, Status: "concept", CreatedAt: now()}
	if p.ID == "" {
		p.ID = newID()
	} else {
		old, e := s.Store.Project(p.ID)
		if e != nil {
			respondError(w, 404, "Project not found")
			return
		}
		p.CreatedAt = old.CreatedAt
	}
	if e := s.Store.Put("projects", p.ID, p); e != nil {
		respondError(w, 500, "Could not save story")
		return
	}
	reply(w, 200, p)
}
func (s *Server) createJob(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Kind      string `json:"kind"`
		ProjectID string `json:"project_id"`
		SourceID  string `json:"source_id"`
		DraftID   string `json:"draft_id"`
		Input     string `json:"input"`
	}
	if e := decode(r, &in); e != nil {
		respondError(w, 400, e.Error())
		return
	}
	if !contains([]string{"import", "analyse", "ideas", "write", "revise", "review"}, in.Kind) || len(in.Input) > 8000 {
		respondError(w, 400, "Invalid job type or input too long")
		return
	}
	settings, e := s.Store.Settings()
	if e != nil {
		respondError(w, 503, "Settings unavailable")
		return
	}
	j := Job{ID: newID(), Kind: in.Kind, ProjectID: in.ProjectID, SourceID: in.SourceID, DraftID: in.DraftID, Input: in.Input, Status: "queued", Stage: "Queued", CreatedAt: now(), UpdatedAt: now(), Result: json.RawMessage(`{}`), Snapshot: Snapshot{PolicyVersion: PolicyVersion, PromptVersion: PromptVersion, Settings: settings}}
	if in.Kind == "import" || in.Kind == "analyse" {
		src, err := s.Store.Source(in.SourceID)
		if err != nil {
			respondError(w, 404, "Source not found")
			return
		}
		if !src.Rights {
			respondError(w, 409, "Confirm source rights first")
			return
		}
		if in.Kind == "analyse" && len(src.Transcript) < 200 {
			respondError(w, 409, "Add a transcript before analysis")
			return
		}
		if in.Kind == "import" && !s.Engine.YouTube.Ready() {
			respondError(w, 409, "Configure owner OAuth or upload an authorised transcript")
			return
		}
		j.Snapshot.Sources = []Source{src}
	} else {
		p, err := s.Store.Project(in.ProjectID)
		if err != nil {
			respondError(w, 404, "Project not found")
			return
		}
		snap, err := s.Store.Snapshot(p)
		if err != nil {
			respondError(w, 409, err.Error())
			return
		}
		j.Snapshot = snap
		if in.Kind == "revise" || in.Kind == "review" {
			d, err := s.Store.Draft(in.DraftID)
			if err != nil || d.ProjectID != p.ID {
				respondError(w, 409, "Select a draft belonging to this story")
				return
			}
		}
	}
	if in.Kind != "import" {
		roles := []Role{settings.Writer}
		if in.Kind == "write" || in.Kind == "revise" || in.Kind == "review" {
			roles = append(roles, settings.Judge, settings.Researcher)
		}
		for _, role := range roles {
			if e = s.Engine.Providers.Ready(role); e != nil {
				respondError(w, 409, e.Error())
				return
			}
		}
	}
	// Enqueue admission and duplicate detection share a transaction. This guards
	// double clicks and concurrent tabs without starting duplicate paid work.
	e = s.Store.DB.Transaction(func(tx *sqlite.Tx) error {
		rows, err := tx.Query(`SELECT data FROM jobs WHERE status IN ('queued','running')`)
		if err != nil {
			return err
		}
		if len(rows) >= 8 {
			return errors.New("queue is full (eight jobs); wait for a run to finish")
		}
		for _, row := range rows {
			var old Job
			if json.Unmarshal([]byte(row.String("data")), &old) != nil {
				continue
			}
			if (j.ProjectID != "" && old.ProjectID == j.ProjectID) || (j.SourceID != "" && old.SourceID == j.SourceID) {
				return errors.New("this story or source already has an active job")
			}
		}
		_, err = tx.Exec(`INSERT INTO jobs VALUES(?,?,?,?)`, j.ID, j.Status, j.CreatedAt, jsonString(j))
		return err
	})
	if e != nil {
		respondError(w, 409, e.Error())
		return
	}
	reply(w, 202, j)
}
func (s *Server) cancelJob(w http.ResponseWriter, r *http.Request) {
	if e := s.Engine.Cancel(r.PathValue("id")); e != nil {
		respondError(w, 409, e.Error())
		return
	}
	reply(w, 200, map[string]bool{"ok": true})
}
func (s *Server) retryJob(w http.ResponseWriter, r *http.Request) {
	var in struct {
		AcceptPossibleCharges bool `json:"accept_possible_charges"`
	}
	if decode(r, &in) != nil || !in.AcceptPossibleCharges {
		respondError(w, 400, "Explicitly acknowledge possible charges for retrying an interrupted provider call")
		return
	}
	j, e := s.Store.Job(r.PathValue("id"))
	if e != nil {
		respondError(w, 404, "Job not found")
		return
	}
	if !contains([]string{"failed", "interrupted", "cancelled"}, j.Status) {
		respondError(w, 409, "Only stopped jobs can be retried")
		return
	}
	e = s.Store.DB.Transaction(func(tx *sqlite.Tx) error {
		rows, err := tx.Query(`SELECT data FROM jobs WHERE status IN ('queued','running')`)
		if err != nil {
			return err
		}
		if len(rows) >= 8 {
			return errors.New("queue full")
		}
		for _, row := range rows {
			var other Job
			json.Unmarshal([]byte(row.String("data")), &other)
			if (j.ProjectID != "" && j.ProjectID == other.ProjectID) || (j.SourceID != "" && j.SourceID == other.SourceID) {
				return errors.New("source or story already has active work")
			}
		}
		if _, err = tx.Exec(`UPDATE calls SET step=step||'-retired-'||id WHERE job_id=? AND status!='complete'`, j.ID); err != nil {
			return err
		}
		j.Status = "queued"
		j.Error = ""
		j.Stage = "Retry queued; completed steps will be reused"
		j.UpdatedAt = now()
		_, err = tx.Exec(`UPDATE jobs SET status=?,data=? WHERE id=?`, j.Status, jsonString(j), j.ID)
		return err
	})
	if e != nil {
		respondError(w, 409, e.Error())
		return
	}
	reply(w, 202, j)
}
func (s *Server) getDraft(w http.ResponseWriter, r *http.Request) {
	d, e := s.Store.Draft(r.PathValue("id"))
	if e != nil {
		respondError(w, 404, "Draft not found")
		return
	}
	d.Checks = ValidateDraft(d)
	reply(w, 200, d)
}
func (s *Server) editDraft(w http.ResponseWriter, r *http.Request) {
	old, e := s.Store.Draft(r.PathValue("id"))
	if e != nil {
		respondError(w, 404, "Draft not found")
		return
	}
	var in struct {
		ExpectedHash string `json:"expected_hash"`
		Script       Script `json:"script"`
	}
	if e = decode(r, &in); e != nil {
		respondError(w, 400, e.Error())
		return
	}
	if in.ExpectedHash != old.Hash {
		respondError(w, 409, "Draft hash changed; reload before editing")
		return
	}
	if len(in.Script.Lines) > 200 || len(in.Script.Cues) > 200 {
		respondError(w, 400, "At most 200 lines and cue bundles")
		return
	}
	all, e := s.Store.Drafts()
	if e != nil {
		respondError(w, 500, "Could not inspect versions")
		return
	}
	for _, d := range all {
		if d.ProjectID == old.ProjectID && d.Version > old.Version {
			respondError(w, 409, "A newer revision exists; edit that version instead")
			return
		}
	}
	j := Job{ID: "manual-" + newID(), ProjectID: old.ProjectID, Snapshot: old.Snapshot}
	d, e := s.Store.NewDraft(j, in.Script, Research{}, nil, old.Version)
	if e != nil {
		respondError(w, 500, "Could not save new draft revision")
		return
	}
	reply(w, 201, d)
}
func (s *Server) approveDraft(w http.ResponseWriter, r *http.Request) {
	d, e := s.Store.Draft(r.PathValue("id"))
	if e != nil {
		respondError(w, 404, "Draft not found")
		return
	}
	var in struct {
		ExpectedHash     string `json:"expected_hash"`
		EditorialSignoff bool   `json:"editorial_signoff"`
	}
	if decode(r, &in) != nil || !in.EditorialSignoff || in.ExpectedHash != d.Hash {
		respondError(w, 400, "Confirm editorial sign-off for this exact draft hash")
		return
	}
	d.Checks = ValidateDraft(d)
	settings, e := s.Store.Settings()
	if e != nil {
		respondError(w, 503, "Settings unavailable")
		return
	}
	if !d.Checks.Ready || hash(settings) != hash(d.Snapshot.Settings) || d.Snapshot.PolicyVersion != PolicyVersion {
		respondError(w, 409, "Draft has blocking checks or model/policy settings changed; rerun review before approval")
		return
	}
	drafts, e := s.Store.Drafts()
	if e != nil {
		respondError(w, 503, "Version store unavailable")
		return
	}
	for _, x := range drafts {
		if x.ProjectID == d.ProjectID && x.Version > d.Version {
			respondError(w, 409, "A newer draft exists. Approve the latest reviewed revision.")
			return
		}
	}
	d.Approved = true
	e = s.Store.DB.Transaction(func(tx *sqlite.Tx) error {
		rows, err := tx.Query(`SELECT data FROM objects WHERE bucket='drafts' AND json_extract(data,'$.project_id')=?`, d.ProjectID)
		if err != nil {
			return err
		}
		for _, row := range rows {
			var current Draft
			if err = json.Unmarshal([]byte(row.String("data")), &current); err != nil {
				return err
			}
			if current.Version > d.Version {
				return errors.New("newer draft exists; reload before approval")
			}
		}
		rows, err = tx.Query(`SELECT data FROM objects WHERE bucket='settings' AND id='active'`)
		if err != nil || len(rows) != 1 {
			return errors.New("settings unavailable")
		}
		var currentSettings Settings
		if json.Unmarshal([]byte(rows[0].String("data")), &currentSettings) != nil || hash(currentSettings) != hash(d.Snapshot.Settings) {
			return errors.New("settings changed during approval")
		}

		if _, err := tx.Exec(`UPDATE objects SET data=?,updated_at=? WHERE bucket='drafts' AND id=?`, jsonString(d), now(), d.ID); err != nil {
			return err
		}
		for i, t := range d.Script.Tasks {
			task := Task{ID: d.ID + "-" + strconv.Itoa(i+1), ProjectID: d.ProjectID, DraftID: d.ID, Role: t.Role, Text: t.Task, Acceptance: t.Acceptance}
			if _, err := tx.Exec(`INSERT OR IGNORE INTO objects VALUES('tasks',?,?,?)`, task.ID, jsonString(task), now()); err != nil {
				return err
			}
		}
		return nil
	})
	if e != nil {
		respondError(w, 500, "Could not approve production package")
		return
	}
	p, _ := s.Store.Project(d.ProjectID)
	p.Status = "production"
	s.Store.Put("projects", p.ID, p)
	reply(w, 200, d)
}
func (s *Server) draftGold(w http.ResponseWriter, r *http.Request) {
	d, e := s.Store.Draft(r.PathValue("id"))
	if e != nil {
		respondError(w, 404, "Draft not found")
		return
	}
	var in struct {
		AcceptAsBaseline bool `json:"accept_as_baseline"`
	}
	if decode(r, &in) != nil || !in.AcceptAsBaseline {
		respondError(w, 400, "An owner must explicitly accept this script as the baseline")
		return
	}
	checks := ValidateDraft(d)
	for _, c := range checks.Items {
		if c.Code != "frozen_baseline" && !c.Passed {
			respondError(w, 409, "Fix all structural, factual and quality checks before commissioning this draft as a gold baseline")
			return
		}
	}
	text := spokenText(d.Script)
	src := Source{ID: newID(), Kind: "gold", Title: "Gold · " + d.Script.Title, Transcript: text, Rights: true, Status: "approved", Hash: hash([]string{text, ""}), CreatedAt: now()}
	if e = s.Store.SaveSource(src); e != nil {
		respondError(w, 500, "Could not freeze baseline")
		return
	}
	reply(w, 201, src)
}
func (s *Server) feedback(w http.ResponseWriter, r *http.Request) {
	var in struct {
		DraftID  string `json:"draft_id"`
		Decision string `json:"decision"`
		Notes    string `json:"notes"`
		Reusable bool   `json:"reusable"`
	}
	if e := decode(r, &in); e != nil {
		respondError(w, 400, e.Error())
		return
	}
	if _, e := s.Store.Draft(in.DraftID); e != nil {
		respondError(w, 404, "Draft not found")
		return
	}
	if !contains([]string{"keep", "change", "reject"}, in.Decision) || len(in.Notes) < 5 || len(in.Notes) > 5000 {
		respondError(w, 400, "Provide an editorial decision and 5–5,000 characters of feedback")
		return
	}
	f := Feedback{newID(), in.DraftID, in.Decision, in.Notes, in.Reusable, now()}
	if e := s.Store.Put("feedback", f.ID, f); e != nil {
		respondError(w, 500, "Could not save feedback")
		return
	}
	reply(w, 201, f)
}
func (s *Server) task(w http.ResponseWriter, r *http.Request) {
	var t Task
	if s.Store.Get("tasks", r.PathValue("id"), &t) != nil {
		respondError(w, 404, "Task not found")
		return
	}
	var in struct {
		Done bool `json:"done"`
	}
	if decode(r, &in) != nil {
		respondError(w, 400, "Set done to true or false")
		return
	}
	t.Done = in.Done
	if e := s.Store.Put("tasks", t.ID, t); e != nil {
		respondError(w, 500, "Could not update task")
		return
	}
	reply(w, 200, t)
}
func (s *Server) exportDraft(w http.ResponseWriter, r *http.Request) {
	d, e := s.Store.Draft(r.PathValue("id"))
	if e != nil {
		respondError(w, 404, "Draft not found")
		return
	}
	d.Checks = ValidateDraft(d)
	if !d.Approved || !d.Checks.Ready {
		respondError(w, 409, "Production downloads require editorial sign-off and passing, fresh checks")
		return
	}
	settings, e := s.Store.Settings()
	if e != nil || hash(settings) != hash(d.Snapshot.Settings) {
		respondError(w, 409, "Settings changed; run a fresh review")
		return
	}
	format := r.URL.Query().Get("format")
	if format == "audit" {
		w.Header().Set("Content-Disposition", `attachment; filename="bolty-audit.json"`)
		reply(w, 200, d)
		return
	}
	if format == "va.md" || format == "editor.md" {
		va, ed := Markdown(d.Script)
		content := va
		if format == "editor.md" {
			content = ed
		}
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="`+format+`"`)
		w.Write([]byte(content))
		return
	}
	if format != "" && format != "zip" {
		respondError(w, 400, "Choose zip, va.md, editor.md or audit")
		return
	}
	select {
	case s.exporting <- struct{}{}:
		defer func() { <-s.exporting }()
	default:
		respondError(w, 429, "An export is already rendering; try again shortly")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, s.Python, s.Renderer)
	cmd.Stdin = strings.NewReader(jsonString(d.Script))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if e = cmd.Run(); e != nil {
		s.Log.Error("PDF renderer failed", "error", e, "detail", stderr.String())
		respondError(w, 500, "PDF rendering failed; no partial production ZIP was returned")
		return
	}
	// Renderer returns a ZIP containing both PDFs. Assemble the four-file package
	// only after validating both PDFs, keeping incomplete exports off the wire.
	zr, e := zip.NewReader(bytes.NewReader(stdout.Bytes()), int64(stdout.Len()))
	if e != nil {
		respondError(w, 500, "Invalid renderer output")
		return
	}
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	va, ed := Markdown(d.Script)
	files := map[string][]byte{"voice-actor.md": []byte(va), "editor.md": []byte(ed)}
	for _, f := range zr.File {
		if f.Name != "voice-actor.pdf" && f.Name != "editor.pdf" {
			continue
		}
		reader, err := f.Open()
		if err != nil {
			respondError(w, 500, "PDF archive could not be read")
			return
		}
		data, err := io.ReadAll(io.LimitReader(reader, 10<<20))
		reader.Close()
		if err != nil || !bytes.HasPrefix(data, []byte("%PDF-")) {
			respondError(w, 500, "PDF validation failed")
			return
		}
		files[f.Name] = data
	}
	if len(files) != 4 {
		respondError(w, 500, "Both PDF exports are required")
		return
	}
	for _, name := range []string{"voice-actor.md", "editor.md", "voice-actor.pdf", "editor.pdf"} {
		f, err := zw.Create(name)
		if err != nil {
			respondError(w, 500, "Package creation failed")
			return
		}
		if _, err = f.Write(files[name]); err != nil {
			respondError(w, 500, "Package creation failed")
			return
		}
	}
	if e = zw.Close(); e != nil {
		respondError(w, 500, "Package creation failed")
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="bolty-v%d-production.zip"`, d.Version))
	w.Write(out.Bytes())
}
func (s *Server) static(w http.ResponseWriter, r *http.Request) {
	file := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if file == "." || file == "" {
		file = "index.html"
	}
	if file != "index.html" && file != "app.js" && file != "styles.css" && file != "mark.svg" {
		if strings.HasPrefix(file, "api/") {
			respondError(w, 404, "API route not found")
			return
		}
		file = "index.html"
	}
	data, e := fs.ReadFile(s.Assets, file)
	if e != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", mime.TypeByExtension(path.Ext(file)))
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(data)
}
func ValidateOrigin(raw string) (bool, error) {
	u, e := url.Parse(raw)
	if e != nil || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return false, errors.New("PUBLIC_URL must be a bare origin, e.g. http://127.0.0.1:8080")
	}
	if u.Scheme == "https" {
		return true, nil
	}
	if u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1") {
		return false, nil
	}
	return false, errors.New("public deployment requires an HTTPS PUBLIC_URL")
}

// Keep polling payloads bounded: full source bodies are requested only in the editor.
func sourceSummaries(sources []Source) []map[string]any {
	out := []map[string]any{}
	for _, s := range sources {
		out = append(out, map[string]any{"id": s.ID, "kind": s.Kind, "title": s.Title, "url": s.URL, "video_id": s.VideoID, "visual_notes": s.VisualNotes, "rights": s.Rights, "status": s.Status, "hash": s.Hash, "analysis": s.Analysis, "created_at": s.CreatedAt, "word_count": len(wordRE.FindAllString(s.Transcript, -1))})
	}
	return out
}
func (s *Server) getSource(w http.ResponseWriter, r *http.Request) {
	src, e := s.Store.Source(r.PathValue("id"))
	if e != nil {
		respondError(w, 404, "Source not found")
		return
	}
	reply(w, 200, src)
}
