package studio

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"
)

func defaults() Settings {
	return Settings{Role{"openai", "test-model"}, Role{"anthropic", "test-model"}, Role{"openai", "test-model"}, 85, 7, 30, 90}
}
func testStore(t *testing.T) *Store {
	t.Helper()
	s, e := OpenStore(filepath.Join(t.TempDir(), "studio.db"), defaults())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.DB.Close() })
	return s
}
func cloneDraft(d Draft) Draft { var x Draft; json.Unmarshal([]byte(jsonString(d)), &x); return x }
func check(t *testing.T, d Draft, code string, want bool) {
	t.Helper()
	for _, c := range ValidateDraft(d).Items {
		if c.Code == code {
			if c.Passed != want {
				t.Fatalf("%s: got %v want %v (%s)", code, c.Passed, want, c.Detail)
			}
			return
		}
	}
	t.Fatalf("check %s missing", code)
}

// A synthetic structural fixture, NOT an editorial example or model output.
// It deliberately repeats filler. It tests the gate implementation, not craft.
func fixture() Draft {
	s := Script{Title: "So I Called The Server Owner..", Basis: "fictional", Disclosure: "This is a fictional scenario.", Tone: "Protective, with a quiet reveal.", Thumbnail: Thumbnail{"Player", "Moderator", "Permission receipt", "HE CHANGED THE RULES"}, Privacy: []string{"Changed names", "Original avatars", "No server identifiers", "Own world", "Recreated UI", "No private footage"}, Assets: []Asset{{"Chat recreation", "Original UI recreation", "recreated"}}, RunningGags: []string{"Talking is free"}, Tasks: []TaskSpec{{"Producer", "Confirm disclosure", "Audible disclosure"}, {"VA", "Record lines", "All line IDs"}, {"Editor", "Recreate UI", "Own assets"}, {"QA", "Check privacy", "No identifiers"}}}
	for _, role := range []string{"narrator", "victim", "antagonist", "undercover"} {
		s.Voices = append(s.Voices, Voice{role, role, "Calm and clear"})
	}
	for i := 1; i <= 80; i++ {
		text := strings.Repeat("This evidence helps every player understand the unfair rule. ", 3)
		s.Lines = append(s.Lines, Line{i, "narrator", text, "Calm", i == 40, fmt.Sprintf("beat-%d", (i-1)/10)})
		s.Cues = append(s.Cues, Cue{i, "Own world recreation", "Show the original receipt recreation", "Silence", 2})
	}
	s.Lines[0].Text = "YO GUYS! Welcome back to the channel!"
	s.Lines[1].Text = "This is a fictional scenario, recreated to explain an unfair rule."
	s.Lines[79].Text = "I've been Bolty… subscribe, and I'll see you soon with the next one!"
	if deficit := 2375 - SpokenWords(s); deficit > 0 {
		s.Lines[2].Text += strings.Repeat(" Evidence.", deficit)
	}
	for SpokenWords(s) > 2375 {
		w := strings.Fields(s.Lines[2].Text)
		s.Lines[2].Text = strings.Join(w[:len(w)-1], " ")
	}
	page := EvidencePage{URL: "https://support.discord.com/hc/en-us/articles/test-fixture", Text: "Synthetic test evidence only: moderators require the appropriate permission to perform this particular local action.", RetrievedAt: now()}
	page.Hash = hash(page.Text)
	d := Draft{ID: "draft-fixture", ProjectID: "project-fixture", JobID: "job-fixture", Version: 1, Script: s, Snapshot: Snapshot{PolicyVersion: PolicyVersion, PromptVersion: PromptVersion, Settings: defaults(), Project: Project{ID: "project-fixture", Platform: "Discord", Basis: "fictional", Category: "staff abuse"}, Golds: []Source{{ID: "gold-fixture", Kind: "gold", Status: "approved", Rights: true, Transcript: "Synthetic baseline for gate tests, not a real editorial baseline."}}}, Research: Research{Claims: []Claim{{"C1", "Synthetic permission claim", []int{3}}}, Pages: []EvidencePage{page}, Verification: Verification{[]Verdict{{"C1", "supported", page.URL, page.Text, "Synthetic local test support only."}}}}}
	d.Critique = &Critique{MechanicsComplete: true, PrivacySafe: true, ScopeSafe: true, NoVictimMockery: true, HonestBasis: true}
	for _, name := range Dimensions {
		d.Critique.Dimensions = append(d.Critique.Dimensions, Dimension{name, 9, "Synthetic reason for gate testing only.", []int{3}})
	}
	d.Benchmarks = []Benchmark{{"gold-fixture", "A", Comparison{"A", "Synthetic candidate passes this test.", nil}}, {"gold-fixture", "B", Comparison{"B", "Synthetic candidate passes this test.", nil}}}
	d.Hash = hash(s)
	d.Checks = ValidateDraft(d)
	return d
}
func TestSyntheticFixturePassesGates(t *testing.T) {
	d := fixture()
	if !d.Checks.Ready {
		for _, c := range d.Checks.Items {
			if !c.Passed {
				t.Error(c.Code, c.Detail)
			}
		}
	}
	if d.Checks.SpokenWords != 2375 {
		t.Fatal(d.Checks.SpokenWords)
	}
}
func TestWordCountIsSpokenOnly(t *testing.T) {
	s := Script{Title: "ignored ignored", Lines: []Line{{Text: "Don't change twenty-one words. Café 42.", Delivery: strings.Repeat("ignored ", 100)}}}
	if n := SpokenWords(s); n != 7 {
		t.Fatalf("word count %d", n)
	}
}
func TestDeliveryGatesFailClosed(t *testing.T) {
	base := fixture()
	cases := []struct {
		name, code string
		mutate     func(*Draft)
	}{
		{"short", "spoken_words", func(d *Draft) { d.Script.Lines[2].Text = "Short." }},
		{"bad-title", "title", func(d *Draft) { d.Script.Title += "." }},
		{"episode", "no_episode_numbers", func(d *Draft) { d.Script.Tone = "Episode 7" }},
		{"unsafe-theme", "scope_keywords", func(d *Draft) { d.Script.Lines[3].Text = "This is a predator catch." }},
		{"pii", "identity_screen", func(d *Draft) { d.Script.Lines[3].Text = "Contact @real_player" }},
		{"platform-name", "platform_privacy", func(d *Draft) { d.Script.Lines[3].Text = "This happened on Roblox." }},
		{"visual-in-VA", "voice_actor_isolation", func(d *Draft) { d.Script.Lines[3].Text = "[CUT TO: the account]" }},
		{"origin", "origin", func(d *Draft) { d.Script.Lines[3].Text = "We got this from the comments." }},
		{"duplicate-ID", "line_integrity", func(d *Draft) { d.Script.Lines[3].ID = 1 }},
		{"undefined-voice", "line_integrity", func(d *Draft) { d.Script.Lines[3].Speaker = "nobody" }},
		{"visual-delivery", "delivery_notes", func(d *Draft) { d.Script.Lines[3].Delivery = "Zoom camera" }},
		{"missing-star", "second_takes", func(d *Draft) {
			for i := range d.Script.Lines {
				d.Script.Lines[i].SecondTake = false
			}
		}},
		{"missing-cue", "editor_sync", func(d *Draft) { d.Script.Cues = d.Script.Cues[1:] }},
		{"duplicate-cue", "editor_sync", func(d *Draft) { d.Script.Cues[3].LineID = 1 }},
		{"negative-hold", "editor_sync", func(d *Draft) { d.Script.Cues[3].HoldSeconds = -3 }},
		{"runtime", "finished_runtime", func(d *Draft) {
			for i := range d.Script.Cues {
				d.Script.Cues[i].HoldSeconds = 0
			}
		}},
		{"missing-silence", "quiet_reveal", func(d *Draft) {
			for i := range d.Script.Cues {
				d.Script.Cues[i].Sound = "Background bed"
			}
		}},
		{"thumbnail", "thumbnail", func(d *Draft) { d.Script.Thumbnail.Text = "Two words" }},
		{"third-party-asset", "editor_package", func(d *Draft) { d.Script.Assets[0].Provenance = "stolen" }},
		{"fiction-without-disclosure", "story_basis", func(d *Draft) { d.Script.Lines[1].Text = "This really happened." }},
		{"documented-no-case", "story_basis", func(d *Draft) { d.Script.Basis = "documented"; d.Snapshot.Project.Basis = "documented" }},
		{"unverified", "mechanic_evidence", func(d *Draft) { d.Research.Verification.Verdicts[0].Verdict = "unknown" }},
		{"quote-invented", "mechanic_evidence", func(d *Draft) {
			d.Research.Verification.Verdicts[0].Quote = "This totally invented quote is not present in the page at all."
		}},
		{"stale-evidence", "mechanic_evidence", func(d *Draft) {
			d.Research.Pages[0].RetrievedAt = time.Now().Add(-8 * 24 * time.Hour).Format(time.RFC3339)
		}},
		{"tampered-page", "mechanic_evidence", func(d *Draft) { d.Research.Pages[0].Text += "changed" }},
		{"missing-verdict", "mechanic_evidence", func(d *Draft) { d.Research.Verification.Verdicts = nil }},
		{"extra-verdict", "mechanic_evidence", func(d *Draft) {
			v := d.Research.Verification.Verdicts[0]
			v.ClaimID = "C2"
			d.Research.Verification.Verdicts = append(d.Research.Verification.Verdicts, v)
		}},
		{"copied-reference", "originality_overlap", func(d *Draft) { d.Snapshot.Sources = []Source{{Kind: "reference", Transcript: d.Script.Lines[3].Text}} }},
		{"no-review", "independent_review", func(d *Draft) { d.Critique = nil }},
		{"judge-blocker", "independent_review", func(d *Draft) { d.Critique.Blockers = []string{"Unsafe payoff"} }},
		{"missing-dimension", "independent_review", func(d *Draft) { d.Critique.Dimensions = d.Critique.Dimensions[1:] }},
		{"low-dimension", "quality_floor", func(d *Draft) { d.Critique.Dimensions[0].Score = 6 }},
		{"no-gold", "frozen_baseline", func(d *Draft) { d.Snapshot.Golds = nil }},
		{"one-comparison", "frozen_baseline", func(d *Draft) { d.Benchmarks = d.Benchmarks[:1] }},
		{"lost-comparison", "frozen_baseline", func(d *Draft) { d.Benchmarks[1].Result.Winner = "A" }},
		{"duplicate-position", "frozen_baseline", func(d *Draft) { d.Benchmarks[1].CandidatePosition = "A" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := cloneDraft(base)
			tc.mutate(&d)
			check(t, d, tc.code, false)
			if ValidateDraft(d).Ready {
				t.Error("failed gate released draft")
			}
		})
	}
}
func TestCanonicalSignoffNotCopying(t *testing.T) {
	s := "YO GUYS! Welcome back to the channel! I've been Bolty… subscribe, and I'll see you soon with the next one!"
	if hasCopy(s, s, 12) {
		t.Fatal("canonical phrases treated as copying")
	}
}
func TestYouTubeInputsAndAllowlist(t *testing.T) {
	for _, raw := range []string{"96UuXwlYy8Q", "https://youtu.be/96UuXwlYy8Q?si=1", "https://www.youtube.com/watch?v=96UuXwlYy8Q", "https://youtube.com/shorts/96UuXwlYy8Q"} {
		if id, e := YouTubeID(raw); e != nil || id != "96UuXwlYy8Q" {
			t.Fatal(raw, id, e)
		}
	}
	for _, raw := range []string{"http://169.254.169.254/", "https://youtube.com.evil.test/watch?v=96UuXwlYy8Q", "https://user:pass@youtube.com/watch?v=96UuXwlYy8Q", "https://youtu.be:444/96UuXwlYy8Q", "bad"} {
		if _, e := YouTubeID(raw); e == nil {
			t.Fatal("accepted", raw)
		}
	}
	for _, raw := range []string{"https://support.discord.com/article", "https://create.roblox.com/docs", "https://discord.com/developers/docs"} {
		if !allowedOfficial(raw) {
			t.Fatal("rejected", raw)
		}
	}
	for _, raw := range []string{"https://support.discord.com.evil.test/article", "http://support.discord.com/", "https://support.discord.com:444/", "https://discord.com/api/v10", "https://127.0.0.1/", "https://u@support.discord.com/"} {
		if allowedOfficial(raw) {
			t.Fatal("allowed", raw)
		}
	}
	for _, raw := range []string{"127.0.0.1", "10.0.0.1", "169.254.169.254", "::1", "fc00::1", "::ffff:127.0.0.1"} {
		if publicIP(netip.MustParseAddr(raw)) {
			t.Fatal("private accepted", raw)
		}
	}
	if !publicIP(netip.MustParseAddr("8.8.8.8")) {
		t.Fatal("public rejected")
	}
}
func TestCaptionAndHTMLNormalization(t *testing.T) {
	raw := "WEBVTT\n\n1\n00:00:01.000 --> 00:00:03.000\n<b>Hello</b> &amp; welcome.\n\n2\n00:00:03.000 --> 00:00:04.000\nHello &amp; welcome.\nNext line."
	out := NormalizeTranscript(raw)
	if strings.Contains(out, "WEBVTT") || strings.Contains(out, "<b>") || !strings.Contains(out, "Hello & welcome.") {
		t.Fatal(out)
	}
	if got := htmlText(`<header>menu</header><script>bad()</script><p>Official &amp; real</p>`); got != "Official & real" {
		t.Fatal(got)
	}
}
func TestSourceRetrievalAndFrozenSnapshot(t *testing.T) {
	s := testStore(t)
	p := Project{ID: "p", Premise: "unfair moderator permission", SourceIDs: []string{"ref"}}
	if _, e := s.Snapshot(p); e == nil {
		t.Fatal("unapproved references admitted")
	}
	src := Source{ID: "ref", Kind: "reference", Status: "approved", Rights: true, Transcript: strings.Repeat("unfair moderator permission evidence ", 100), Analysis: &StyleAnalysis{Summary: "Original craft"}}
	src.Hash = hash(src.Transcript)
	s.SaveSource(src)
	gold := Source{ID: "gold", Kind: "gold", Status: "approved", Rights: true, Transcript: "Held out baseline"}
	s.SaveSource(gold)
	snap, e := s.Snapshot(p)
	if e != nil {
		t.Fatal(e)
	}
	if len(snap.Sources) != 1 || len(snap.Golds) != 1 || len(snap.Excerpts) == 0 {
		t.Fatalf("snapshot %#v", snap)
	}
	src.Status = "archived"
	s.SaveSource(src)
	if len(s.Retrieve(p.Premise, p.SourceIDs)) != 0 {
		t.Fatal("archived reference searchable")
	}
	if !strings.Contains(snap.Sources[0].Transcript, "permission") {
		t.Fatal("snapshot mutated")
	}
}
func TestAtomicVersionsAndStaleEdits(t *testing.T) {
	s := testStore(t)
	base := fixture()
	var wg sync.WaitGroup
	var failed atomic.Int32
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			j := Job{ID: fmt.Sprintf("job-%d", i), ProjectID: "p", Snapshot: base.Snapshot}
			if _, e := s.NewDraft(j, base.Script, base.Research, base.Critique); e != nil {
				failed.Add(1)
				t.Error(e)
			}
		}(i)
	}
	wg.Wait()
	ds, _ := s.Drafts()
	seen := map[int]bool{}
	for _, d := range ds {
		if seen[d.Version] {
			t.Fatal("duplicate version")
		}
		seen[d.Version] = true
	}
	if len(seen) != 12 || failed.Load() != 0 {
		t.Fatal(seen)
	}
	j := Job{ID: "stale", ProjectID: "p", Snapshot: base.Snapshot}
	if _, e := s.NewDraft(j, base.Script, base.Research, nil, 1); e == nil {
		t.Fatal("stale edit accepted")
	}
	j.ID = "last"
	d, e := s.NewDraft(j, base.Script, base.Research, nil, 12)
	if e != nil || d.Version != 13 {
		t.Fatal(d.Version, e)
	}
	again, e := s.NewDraft(j, base.Script, base.Research, nil)
	if e != nil || again.ID != d.ID {
		t.Fatal("duplicate replay revision", e)
	}
}
func TestCallBudgetAndCheckpoint(t *testing.T) {
	s := testStore(t)
	j := Job{ID: "j", Snapshot: Snapshot{Settings: defaults()}}
	j.Snapshot.Settings.MaxCalls = 2
	for _, step := range []string{"one", "two"} {
		if e := s.ReserveCall(j, step, j.Snapshot.Settings.Writer, "hash"); e != nil {
			t.Fatal(e)
		}
	}
	if e := s.ReserveCall(j, "three", j.Snapshot.Settings.Writer, "hash"); e == nil {
		t.Fatal("run exceeded budget")
	}
	if _, _, e := s.CachedCall("j", "one"); e == nil {
		t.Fatal("uncertain paid call replayed")
	}
	s.FinishCall("j", "one", `{"ok":true}`, Usage{10, 20, "req"}, "complete")
	raw, hit, e := s.CachedCall("j", "one")
	if e != nil || !hit || raw != `{"ok":true}` {
		t.Fatal(raw, hit, e)
	}
	n, i, o := s.Counts("j")
	if n != 2 || i != 10 || o != 20 {
		t.Fatal(n, i, o)
	}
}
func TestClaimAndRestartRecovery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db")
	s, e := OpenStore(path, defaults())
	if e != nil {
		t.Fatal(e)
	}
	s.SaveJob(Job{ID: "j", Status: "queued", CreatedAt: now()})
	var wg sync.WaitGroup
	var claims atomic.Int32
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, ok, e := s.Claim()
			if e != nil {
				t.Error(e)
			}
			if ok {
				claims.Add(1)
			}
		}()
	}
	wg.Wait()
	if claims.Load() != 1 {
		t.Fatal(claims.Load())
	}
	s.DB.Close()
	s, e = OpenStore(path, defaults())
	if e != nil {
		t.Fatal(e)
	}
	defer s.DB.Close()
	j, _ := s.Job("j")
	if j.Status != "interrupted" {
		t.Fatal(j.Status)
	}
}

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func protocolProvider(t *testing.T, provider, response string, inspect func(map[string]any)) *Providers {
	t.Helper()
	p := NewProviders("test-not-real-key", "test-not-real-key")
	p.Client = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		var req map[string]any
		if e := json.NewDecoder(r.Body).Decode(&req); e != nil {
			t.Fatal(e)
		}
		if inspect != nil {
			inspect(req)
		}
		if provider == "openai" && r.Header.Get("Authorization") == "" {
			t.Error("auth missing")
		}
		if provider == "anthropic" && r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Error("version missing")
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"X-Request-Id": []string{"fixture-request"}}, Body: io.NopCloser(strings.NewReader(response))}, nil
	})}
	return p
}
func TestProviderWireContracts(t *testing.T) {
	for _, provider := range []string{"openai", "anthropic"} {
		t.Run(provider, func(t *testing.T) {
			response := `{"status":"completed","usage":{"input_tokens":12,"output_tokens":8},"output":[{"type":"message","content":[{"type":"output_text","text":"{\"winner\":\"A\",\"reason\":\"test reason\",\"weaknesses\":[]}"}]}]}`
			if provider == "anthropic" {
				response = `{"stop_reason":"end_turn","usage":{"input_tokens":12,"output_tokens":8},"content":[{"type":"text","text":"{\"winner\":\"A\",\"reason\":\"test reason\",\"weaknesses\":[]}"}]}`
			}
			p := protocolProvider(t, provider, response, func(m map[string]any) {
				if provider == "openai" {
					if m["store"] != false || m["text"] == nil {
						t.Error("wrong Responses schema")
					}
				} else if m["output_config"] == nil || m["output_format"] != nil {
					t.Error("wrong Claude output schema")
				}
			})
			raw, u, e := p.JSON(context.Background(), Role{provider, "test-model"}, "system", "user", schemaOf(Comparison{}), 500)
			if e != nil || !json.Valid([]byte(raw)) || u.Input != 12 || u.Output != 8 {
				t.Fatal(raw, u, e)
			}
		})
	}
}
func TestProvidersRejectRefusalAndTruncation(t *testing.T) {
	for _, tc := range []struct{ provider, raw string }{{"openai", `{"status":"incomplete"}`}, {"openai", `{"status":"completed","output":[{"type":"message","content":[{"type":"refusal"}]}]}`}, {"anthropic", `{"stop_reason":"max_tokens","content":[{"type":"text","text":"{}"}]}`}, {"anthropic", `{"stop_reason":"refusal"}`}, {"openai", `{"status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"{broken"}]}]}`}} {
		p := protocolProvider(t, tc.provider, tc.raw, nil)
		if _, _, e := p.JSON(context.Background(), Role{tc.provider, "test"}, "s", "u", schemaOf(Comparison{}), 100); e == nil {
			t.Fatal("accepted", tc.raw)
		}
	}
}
func TestSearchRequiresNativeOfficialCitation(t *testing.T) {
	for _, provider := range []string{"openai", "anthropic"} {
		for _, valid := range []bool{false, true} {
			name := fmt.Sprintf("%s-native=%v", provider, valid)
			t.Run(name, func(t *testing.T) {
				c := `[]`
				if valid {
					c = `[{"type":"url_citation","url":"https://support.discord.com/article","title":"Official"}]`
				}
				r := `{"status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"https://support.discord.com/article is just text","annotations":` + c + `}]}]}`
				if provider == "anthropic" {
					r = `{"stop_reason":"end_turn","content":[{"type":"text","text":"https://support.discord.com/article is just text","citations":` + c + `}]}`
				}
				p := protocolProvider(t, provider, r, func(m map[string]any) {
					if m["tools"] == nil {
						t.Error("native search tool missing")
					}
				})
				got, _, e := p.Search(context.Background(), Role{provider, "test"}, "s", "u")
				if valid && (e != nil || len(got.Citations) != 1) {
					t.Fatal(got, e)
				}
				if !valid && e == nil {
					t.Fatal("bare URL counted as evidence")
				}
			})
		}
	}
}
func TestNoAutomaticReplayOnTransportFailure(t *testing.T) {
	p := NewProviders("key", "")
	calls := 0
	p.Client = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) { calls++; return nil, fmt.Errorf("uncertain network") })}
	_, _, e := p.JSON(context.Background(), Role{"openai", "model"}, "s", "u", nil, 100)
	if e == nil || calls != 1 {
		t.Fatal(calls, e)
	}
}
func testHTTP(t *testing.T) (*Server, http.Handler, *http.Cookie) {
	t.Helper()
	s := testStore(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	engine := &Engine{Store: s, Providers: NewProviders("", ""), Log: log}
	app := &Server{Store: s, Engine: engine, Password: "a-strong-test-password-only", Origin: "http://localhost", Assets: fstest.MapFS{"index.html": {Data: []byte("studio")}}, Log: log}
	h := app.Handler()
	w := request(h, nil, "POST", "/api/login", `{"password":"a-strong-test-password-only"}`, "http://localhost")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	return app, h, w.Result().Cookies()[0]
}
func request(h http.Handler, c *http.Cookie, method, path, body, origin string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.RemoteAddr = "127.0.0.1:1234"
	if method != "GET" {
		r.Header.Set("Content-Type", "application/json")
	}
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	if c != nil {
		r.AddCookie(c)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func TestHTTPAuthenticationOriginAndLogout(t *testing.T) {
	_, h, c := testHTTP(t)
	if !c.HttpOnly || c.SameSite != http.SameSiteStrictMode {
		t.Fatal("weak cookie")
	}
	for _, tc := range []struct {
		method, path, body, origin string
		cookie                     *http.Cookie
		want                       int
	}{{"GET", "/api/state", "", "", nil, 401}, {"PUT", "/api/settings", jsonString(defaults()), "https://evil.test", c, 403}, {"PUT", "/api/settings", jsonString(defaults()), "", c, 403}, {"GET", "/api/state", "", "", c, 200}, {"POST", "/api/logout", "{}", "http://localhost", c, 200}, {"GET", "/api/state", "", "", c, 401}} {
		w := request(h, tc.cookie, tc.method, tc.path, tc.body, tc.origin)
		if w.Code != tc.want {
			t.Fatal(tc.path, w.Code, w.Body.String())
		}
	}
}
func TestReferenceApprovalAndMissingCredentials(t *testing.T) {
	app, h, c := testHTTP(t)
	src := Source{ID: "ref", Kind: "reference", Title: "Reference", Transcript: strings.Repeat("Test transcript ", 30), Rights: true, Status: "needs_analysis"}
	app.Store.SaveSource(src)
	w := request(h, c, "POST", "/api/sources/ref/approve", "{}", "http://localhost")
	if w.Code != 409 {
		t.Fatal(w.Code)
	}
	w = request(h, c, "POST", "/api/jobs", `{"kind":"analyse","source_id":"ref","project_id":"","draft_id":"","input":""}`, "http://localhost")
	if w.Code != 409 || !strings.Contains(w.Body.String(), "API_KEY") {
		t.Fatal(w.Code, w.Body.String())
	}
	w = request(h, c, "GET", "/api/sources/ref", "", " ")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Test transcript") {
		t.Fatal(w.Code, w.Body.String())
	}
	w = request(h, c, "GET", "/api/state", "", "")
	if bytes.Contains(w.Body.Bytes(), []byte(src.Transcript)) {
		t.Fatal("poll response contains full transcript")
	}
}
func TestSourceTypeChangeRevokesApproval(t *testing.T) {
	app, h, c := testHTTP(t)
	src := Source{ID: "ref", Kind: "reference", Title: "Reference", Transcript: strings.Repeat("Test words ", 100), Rights: true, Status: "approved", Analysis: &StyleAnalysis{Summary: "Prior analysis"}}
	src.Hash = hash([]string{src.Transcript, ""})
	app.Store.SaveSource(src)
	payload := map[string]any{"kind": "gold", "title": src.Title, "url": "", "transcript": src.Transcript, "visual_notes": "", "rights": true}
	w := request(h, c, "PUT", "/api/sources/ref", jsonString(payload), "http://localhost")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	got, _ := app.Store.Source("ref")
	if got.Status == "approved" || got.Analysis != nil {
		t.Fatal("changed source inherited approval")
	}
}
func TestNoExportWithoutSignoffAndFreshEvidence(t *testing.T) {
	app, h, c := testHTTP(t)
	d := fixture()
	app.Store.Put("drafts", d.ID, d)
	path := "/api/drafts/" + d.ID + "/export?format=va.md"
	w := request(h, c, "GET", path, "", "")
	if w.Code != 409 {
		t.Fatal(w.Code)
	}
	d.Approved = true
	app.Store.Put("drafts", d.ID, d)
	w = request(h, c, "GET", path, "", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "L01") {
		t.Fatal(w.Code, w.Body.String())
	}
	d.Research.Pages[0].RetrievedAt = time.Now().Add(-8 * 24 * time.Hour).Format(time.RFC3339)
	app.Store.Put("drafts", d.ID, d)
	w = request(h, c, "GET", path, "", "")
	if w.Code != 409 {
		t.Fatal("stale evidence exported", w.Code)
	}
}
func TestEditCreatesUnreviewedRevision(t *testing.T) {
	app, h, c := testHTTP(t)
	d := fixture()
	d.Approved = true
	app.Store.Put("drafts", d.ID, d)
	d.Script.Lines[3].Text = "This line was changed."
	body := jsonString(map[string]any{"expected_hash": d.Hash, "script": d.Script})
	w := request(h, c, "PUT", "/api/drafts/"+d.ID, body, "http://localhost")
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var next Draft
	json.Unmarshal(w.Body.Bytes(), &next)
	if next.Approved || next.Critique != nil || next.Checks.Ready || next.Version != 2 {
		t.Fatal("edit inherited approval")
	}
	w = request(h, c, "PUT", "/api/drafts/"+d.ID, body, "http://localhost")
	if w.Code != 409 {
		t.Fatal("stale overwrite accepted", w.Code)
	}
}
func TestApprovalCreatesTasksAndIsIdempotent(t *testing.T) {
	app, h, c := testHTTP(t)
	d := fixture()
	app.Store.Put("drafts", d.ID, d)
	app.Store.Put("projects", d.ProjectID, d.Snapshot.Project)
	body := jsonString(map[string]any{"expected_hash": d.Hash, "editorial_signoff": true})
	for i := 0; i < 2; i++ {
		w := request(h, c, "POST", "/api/drafts/"+d.ID+"/approve", body, "http://localhost")
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	tasks, _ := app.Store.Tasks()
	if len(tasks) != 4 {
		t.Fatal("task duplication", len(tasks))
	}
	d2 := d
	d2.ID = "new"
	d2.Version = 2
	app.Store.Put("drafts", d2.ID, d2)
	w := request(h, c, "POST", "/api/drafts/"+d.ID+"/approve", body, "http://localhost")
	if w.Code != 409 {
		t.Fatal("older draft approved")
	}
}
func TestMarkdownSeparation(t *testing.T) {
	d := fixture()
	d.Script.Cues[0].Edit = "EDITOR_ONLY_SENTINEL"
	va, ed := Markdown(d.Script)
	if strings.Contains(va, "EDITOR_ONLY_SENTINEL") || !strings.Contains(ed, "EDITOR_ONLY_SENTINEL") || !strings.Contains(va, "#8242BB") || !strings.Contains(ed, "L01") {
		t.Fatal("documents mixed or unformatted")
	}
}
func TestOriginValidation(t *testing.T) {
	for _, raw := range []string{"http://localhost:8080", "http://127.0.0.1:8080", "https://studio.example.com"} {
		if _, e := ValidateOrigin(raw); e != nil {
			t.Fatal(raw, e)
		}
	}
	for _, raw := range []string{"http://example.com", "https://example.com/path", "https://user@example.com", "javascript:alert(1)"} {
		if _, e := ValidateOrigin(raw); e == nil {
			t.Fatal("accepted", raw)
		}
	}
}
func TestEmitFormatFixture(t *testing.T) {
	if path := os.Getenv("BOLTY_FORMAT_FIXTURE"); path != "" {
		b, e := json.MarshalIndent(fixture().Script, "", "  ")
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(path, b, 0600); e != nil {
			t.Fatal(e)
		}
		// In the opt-in PDF fixture run, also exercise the authenticated API
		// assembly path with the real renderer and this synthetic fixture.
		app, handler, cookie := testHTTP(t)
		app.Python = os.Getenv("PYTHON_BIN")
		if app.Python == "" {
			app.Python = "python3"
		}
		app.Renderer, e = filepath.Abs("../../tools/render.py")
		if e != nil {
			t.Fatal(e)
		}
		d := fixture()
		d.Approved = true
		if e = app.Store.Put("drafts", d.ID, d); e != nil {
			t.Fatal(e)
		}
		response := request(handler, cookie, "GET", "/api/drafts/"+d.ID+"/export?format=zip", "", "")
		if response.Code != 200 {
			t.Fatal(response.Code, response.Body.String())
		}
		if e = os.WriteFile(path+".production.zip", response.Body.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
	}
}
