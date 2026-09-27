package studio

import (
	"bolty.studio/internal/sqlite"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Store struct{ DB *sqlite.DB }

func OpenStore(path string, settings Settings) (*Store, error) {
	d, e := sqlite.Open(path)
	if e != nil {
		return nil, e
	}
	s := &Store{d}
	statements := []string{
		`CREATE TABLE IF NOT EXISTS objects(bucket TEXT NOT NULL,id TEXT NOT NULL,data TEXT NOT NULL CHECK(json_valid(data)),updated_at TEXT NOT NULL,PRIMARY KEY(bucket,id))`,
		`CREATE TABLE IF NOT EXISTS jobs(id TEXT PRIMARY KEY,status TEXT NOT NULL,created_at TEXT NOT NULL,data TEXT NOT NULL CHECK(json_valid(data)))`,
		`CREATE INDEX IF NOT EXISTS jobs_status_created ON jobs(status,created_at)`,
		`CREATE TABLE IF NOT EXISTS calls(id TEXT PRIMARY KEY,job_id TEXT NOT NULL,step TEXT NOT NULL,provider TEXT NOT NULL,model TEXT NOT NULL,status TEXT NOT NULL,prompt_hash TEXT NOT NULL,request_id TEXT NOT NULL DEFAULT '',input_tokens INTEGER NOT NULL DEFAULT 0,output_tokens INTEGER NOT NULL DEFAULT 0,data TEXT NOT NULL DEFAULT '',created_at TEXT NOT NULL,UNIQUE(job_id,step))`,
		`CREATE INDEX IF NOT EXISTS calls_created ON calls(created_at)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS chunks USING fts5(source_id UNINDEXED,ordinal UNINDEXED,body,tokenize='unicode61')`,
		`CREATE TABLE IF NOT EXISTS sessions(token_hash TEXT PRIMARY KEY,expires_at INTEGER NOT NULL)`,
	}
	for _, q := range statements {
		if _, e = d.Exec(q); e != nil {
			d.Close()
			return nil, e
		}
	}
	if _, e = d.Exec(`INSERT OR IGNORE INTO objects VALUES('settings','active',?,?)`, jsonString(settings), now()); e != nil {
		return nil, e
	}
	for _, src := range BriefReferences {
		src.CreatedAt = now()
		src.Hash = hash(src.Transcript)
		if _, e = d.Exec(`INSERT OR IGNORE INTO objects VALUES('sources',?,?,?)`, src.ID, jsonString(src), now()); e != nil {
			return nil, e
		}
	}
	// A process can die after a provider has charged for a call, before saving it.
	// Never replay that paid call silently on restart. The owner explicitly retries.
	rows, e := s.Jobs()
	if e != nil {
		return nil, e
	}
	for _, j := range rows {
		if j.Status == "running" {
			j.Status = "interrupted"
			j.Error = "Server restarted during a run. Saved steps are retained; retry requires confirmation because the in-flight call may have been billed."
			if e = s.SaveJob(j); e != nil {
				return nil, e
			}
		}
	}
	return s, nil
}
func (s *Store) Put(bucket, id string, v any) error {
	_, e := s.DB.Exec(`INSERT INTO objects VALUES(?,?,?,?) ON CONFLICT(bucket,id) DO UPDATE SET data=excluded.data,updated_at=excluded.updated_at`, bucket, id, jsonString(v), now())
	return e
}
func (s *Store) Get(bucket, id string, v any) error {
	r, e := s.DB.Query(`SELECT data FROM objects WHERE bucket=? AND id=?`, bucket, id)
	if e != nil {
		return e
	}
	if len(r) == 0 {
		return errors.New("not found")
	}
	return json.Unmarshal([]byte(r[0].String("data")), v)
}
func listObjects[T any](s *Store, bucket string) ([]T, error) {
	rows, e := s.DB.Query(`SELECT data FROM objects WHERE bucket=? ORDER BY updated_at DESC LIMIT 1000`, bucket)
	if e != nil {
		return nil, e
	}
	out := []T{}
	for _, r := range rows {
		var v T
		if e = json.Unmarshal([]byte(r.String("data")), &v); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, nil
}
func (s *Store) Settings() (Settings, error) {
	var v Settings
	e := s.Get("settings", "active", &v)
	return v, e
}
func (s *Store) Sources() ([]Source, error)    { return listObjects[Source](s, "sources") }
func (s *Store) Projects() ([]Project, error)  { return listObjects[Project](s, "projects") }
func (s *Store) Drafts() ([]Draft, error)      { return listObjects[Draft](s, "drafts") }
func (s *Store) Feedback() ([]Feedback, error) { return listObjects[Feedback](s, "feedback") }
func (s *Store) Tasks() ([]Task, error)        { return listObjects[Task](s, "tasks") }
func (s *Store) Source(id string) (Source, error) {
	var v Source
	e := s.Get("sources", id, &v)
	return v, e
}
func (s *Store) Project(id string) (Project, error) {
	var v Project
	e := s.Get("projects", id, &v)
	return v, e
}
func (s *Store) Draft(id string) (Draft, error) {
	var v Draft
	e := s.Get("drafts", id, &v)
	return v, e
}
func (s *Store) SaveSource(v Source) error {
	return s.DB.Transaction(func(tx *sqlite.Tx) error {
		if _, e := tx.Exec(`INSERT INTO objects VALUES('sources',?,?,?) ON CONFLICT(bucket,id) DO UPDATE SET data=excluded.data,updated_at=excluded.updated_at`, v.ID, jsonString(v), now()); e != nil {
			return e
		}
		if _, e := tx.Exec(`DELETE FROM chunks WHERE source_id=?`, v.ID); e != nil {
			return e
		}
		if v.Status == "approved" {
			for i, body := range chunkText(v.Transcript, 160, 40) {
				if _, e := tx.Exec(`INSERT INTO chunks(source_id,ordinal,body) VALUES(?,?,?)`, v.ID, i, body); e != nil {
					return e
				}
			}
		}
		return nil
	})
}
func chunkText(text string, size, overlap int) []string {
	if size <= overlap || size < 1 {
		return nil
	}
	words := strings.Fields(text)
	out := []string{}
	for i := 0; i < len(words); i += size - overlap {
		end := i + size
		if end > len(words) {
			end = len(words)
		}
		out = append(out, strings.Join(words[i:end], " "))
		if end == len(words) {
			break
		}
	}
	return out
}
func (s *Store) Retrieve(query string, allowed []string) []string {
	out := []string{}
	terms := wordRE.FindAllString(strings.ToLower(query), 12)
	quoted := []string{}
	for _, t := range terms {
		if len(t) > 2 {
			quoted = append(quoted, `"`+strings.ReplaceAll(t, `"`, ``)+`"`)
		}
	}
	if len(quoted) == 0 {
		return out
	}
	rows, e := s.DB.Query(`SELECT source_id,body FROM chunks WHERE chunks MATCH ? ORDER BY bm25(chunks) LIMIT 40`, strings.Join(quoted, " OR "))
	if e != nil {
		return out
	}
	for _, r := range rows {
		if contains(allowed, r.String("source_id")) {
			out = append(out, "SOURCE "+r.String("source_id")+": "+r.String("body"))
			if len(out) == 6 {
				break
			}
		}
	}
	return out
}
func (s *Store) SaveJob(j Job) error {
	j.UpdatedAt = now()
	_, e := s.DB.Exec(`INSERT INTO jobs VALUES(?,?,?,?) ON CONFLICT(id) DO UPDATE SET status=excluded.status,data=excluded.data`, j.ID, j.Status, j.CreatedAt, jsonString(j))
	return e
}
func (s *Store) Job(id string) (Job, error) {
	var j Job
	rows, e := s.DB.Query(`SELECT data FROM jobs WHERE id=?`, id)
	if e != nil {
		return j, e
	}
	if len(rows) == 0 {
		return j, errors.New("not found")
	}
	e = json.Unmarshal([]byte(rows[0].String("data")), &j)
	return j, e
}
func (s *Store) Jobs() ([]Job, error) {
	rows, e := s.DB.Query(`SELECT data FROM jobs ORDER BY created_at DESC LIMIT 100`)
	if e != nil {
		return nil, e
	}
	out := []Job{}
	for _, r := range rows {
		var j Job
		if e = json.Unmarshal([]byte(r.String("data")), &j); e != nil {
			return nil, e
		}
		out = append(out, j)
	}
	return out, nil
}
func (s *Store) Claim() (Job, bool, error) {
	var j Job
	found := false
	e := s.DB.Transaction(func(tx *sqlite.Tx) error {
		r, e := tx.Query(`SELECT data FROM jobs WHERE status='queued' ORDER BY created_at LIMIT 1`)
		if e != nil {
			return e
		}
		if len(r) == 0 {
			return nil
		}
		if e = json.Unmarshal([]byte(r[0].String("data")), &j); e != nil {
			return e
		}
		j.Status = "running"
		j.UpdatedAt = now()
		_, e = tx.Exec(`UPDATE jobs SET status='running',data=? WHERE id=? AND status='queued'`, jsonString(j), j.ID)
		found = e == nil
		return e
	})
	return j, found, e
}

// NewDraft allocates the version and inserts it in one SQLite transaction.
// expectedVersion is supplied by manual edits to reject two editors racing.
func (s *Store) NewDraft(j Job, script Script, research Research, critique *Critique, expectedVersion ...int) (Draft, error) {
	var d Draft
	err := s.DB.Transaction(func(tx *sqlite.Tx) error {
		rows, e := tx.Query(`SELECT data FROM objects WHERE bucket='drafts' AND json_extract(data,'$.project_id')=?`, j.ProjectID)
		if e != nil {
			return e
		}
		version := 1
		scriptHash := hash(script)
		for _, row := range rows {
			var old Draft
			if e = json.Unmarshal([]byte(row.String("data")), &old); e != nil {
				return e
			}
			if old.JobID == j.ID && old.Hash == scriptHash {
				d = old
				return nil
			}
			if old.Version >= version {
				version = old.Version + 1
			}
		}
		if len(expectedVersion) > 0 && version != expectedVersion[0]+1 {
			return errors.New("newer revision exists; reload before editing")
		}
		d = Draft{ID: newID(), ProjectID: j.ProjectID, JobID: j.ID, Version: version, Script: script, Research: research, Critique: critique, Snapshot: j.Snapshot, CreatedAt: now(), Hash: scriptHash}
		d.Checks = ValidateDraft(d)
		_, e = tx.Exec(`INSERT INTO objects(bucket,id,data,updated_at) VALUES('drafts',?,?,?)`, d.ID, jsonString(d), now())
		return e
	})
	return d, err
}

func (s *Store) Snapshot(p Project) (Snapshot, error) {
	st, e := s.Settings()
	if e != nil {
		return Snapshot{}, e
	}
	all, e := s.Sources()
	if e != nil {
		return Snapshot{}, e
	}
	refs := []Source{}
	baseline := []string{}
	golds := []Source{}
	for _, v := range all {
		if v.Status == "approved" && v.Rights && (contains(p.SourceIDs, v.ID) || v.ID == p.CaseSourceID) {
			refs = append(refs, v)
		}
		if v.Kind == "gold" && v.Status == "approved" && v.Rights && len(golds) < 1 {
			baseline = append(baseline, v.ID)
			golds = append(golds, v)
		}
	}
	hasReference := false
	for _, ref := range refs {
		if ref.Kind == "reference" {
			hasReference = true
		}
	}
	if !hasReference {
		return Snapshot{}, errors.New("approve at least one reference with its transcript before generating")
	}
	feedback, e := s.Feedback()
	if e != nil {
		return Snapshot{}, e
	}
	notes := []string{}
	for _, f := range feedback {
		if f.Reusable {
			notes = append(notes, f.Decision+": "+f.Notes)
			if len(notes) == 12 {
				break
			}
		}
	}
	return Snapshot{PolicyVersion: PolicyVersion, PromptVersion: PromptVersion, Settings: st, Project: p, Sources: refs, Golds: golds, Excerpts: s.Retrieve(p.Premise, p.SourceIDs), Feedback: notes, BaselineIDs: baseline}, nil
}
func (s *Store) ReserveCall(j Job, step string, role Role, promptHash string) error {
	return s.DB.Transaction(func(tx *sqlite.Tx) error {
		rows, e := tx.Query(`SELECT COUNT(*) AS n FROM calls WHERE job_id=?`, j.ID)
		if e != nil {
			return e
		}
		if int(rows[0].Int("n")) >= j.Snapshot.Settings.MaxCalls {
			return errors.New("per-run model-call budget reached")
		}
		midnight := time.Now().UTC().Truncate(24 * time.Hour).Format(time.RFC3339)
		rows, e = tx.Query(`SELECT COUNT(*) AS n FROM calls WHERE created_at>=?`, midnight)
		if e != nil {
			return e
		}
		if int(rows[0].Int("n")) >= j.Snapshot.Settings.DailyCalls {
			return errors.New("daily model-call budget reached")
		}
		_, e = tx.Exec(`INSERT INTO calls(id,job_id,step,provider,model,status,prompt_hash,created_at) VALUES(?,?,?,?,?,'started',?,?)`, newID(), j.ID, step, role.Provider, role.Model, promptHash, now())
		return e
	})
}
func (s *Store) CachedCall(jobID, step string) (string, bool, error) {
	r, e := s.DB.Query(`SELECT status,data FROM calls WHERE job_id=? AND step=?`, jobID, step)
	if e != nil {
		return "", false, e
	}
	if len(r) == 0 {
		return "", false, nil
	}
	if r[0].String("status") == "complete" {
		return r[0].String("data"), true, nil
	}
	return "", false, fmt.Errorf("step %s has an incomplete paid call; explicitly retry the job", step)
}
func (s *Store) FinishCall(jobID, step, data string, u Usage, status string) error {
	_, e := s.DB.Exec(`UPDATE calls SET status=?,data=?,request_id=?,input_tokens=?,output_tokens=? WHERE job_id=? AND step=?`, status, data, u.RequestID, u.Input, u.Output, jobID, step)
	return e
}
func (s *Store) Counts(jobID string) (int, int, int) {
	r, e := s.DB.Query(`SELECT COUNT(*) n,COALESCE(SUM(input_tokens),0) i,COALESCE(SUM(output_tokens),0) o FROM calls WHERE job_id=?`, jobID)
	if e != nil || len(r) == 0 {
		return 0, 0, 0
	}
	return int(r[0].Int("n")), int(r[0].Int("i")), int(r[0].Int("o"))
}
func contains(a []string, v string) bool {
	for _, x := range a {
		if x == v {
			return true
		}
	}
	return false
}
