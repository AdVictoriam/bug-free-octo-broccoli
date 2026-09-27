package studio

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"
)

var wordRE = regexp.MustCompile(`[\p{L}\p{N}]+(?:['’][\p{L}\p{N}]+)*`)
var episodeRE = regexp.MustCompile(`(?i)\b(?:episode|ep\.?)\s*#?\s*\d+\b`)
var piiRE = regexp.MustCompile(`(?i)(https?://|discord\.gg/|@[a-z0-9_]{2,}|[a-z0-9._%+-]+@[a-z0-9.-]+\.[a-z]{2,}|\b(?:\d{1,3}\.){3}\d{1,3}\b)`)
var stageRE = regexp.MustCompile(`(?i)(\[(?:cut|zoom|sfx|show|on.?screen|camera|fade)|\b(?:b-roll|sfx:|cut to:|on-screen:))`)
var deliveryVisualRE = regexp.MustCompile(`(?i)\b(?:camera|zoom|sfx|cut to|music|fade|on.screen|insert|bleep)\b`)
var bannedThemeRE = regexp.MustCompile(`(?i)\b(?:predator|predatory|grooming|groomer|sexual|paedophile|pedophile|pred.catching)\b`)
var platformRE = regexp.MustCompile(`(?i)\b(?:roblox|minecraft|discord)\b`)
var titleRE = regexp.MustCompile(`^(?:.+ Thinks He's .+ But It's Me|So I Called The .+|I Caught A .+ Doing This|This .+ Did .+ To A Player)\.\.$`)

func SpokenWords(s Script) int {
	n := 0
	for _, l := range s.Lines {
		n += len(wordRE.FindAllString(l.Text, -1))
	}
	return n
}
func spokenText(s Script) string {
	b := strings.Builder{}
	for _, l := range s.Lines {
		b.WriteString(l.Text)
		b.WriteByte('\n')
	}
	return b.String()
}
func normQuote(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }
func validEvidence(v Verdict, pages []EvidencePage) bool {
	if v.Verdict != "supported" || len(strings.TrimSpace(v.Quote)) < 32 || !allowedOfficial(v.URL) {
		return false
	}
	for _, p := range pages {
		if p.URL == v.URL && p.Error == "" && p.Hash == hash(p.Text) {
			t, e := time.Parse(time.RFC3339, p.RetrievedAt)
			if e != nil || time.Since(t) > 7*24*time.Hour || time.Until(t) > 5*time.Minute {
				return false
			}
			return strings.Contains(normQuote(p.Text), normQuote(v.Quote))
		}
	}
	return false
}
func ValidateDraft(d Draft) Checks {
	s := d.Script
	c := Checks{Items: []Check{}}
	add := func(code string, ok bool, detail string) { c.Items = append(c.Items, Check{code, ok, detail, true}) }
	c.SpokenWords = SpokenWords(s)
	c.VoiceoverSeconds = float64(c.SpokenWords) / 225 * 60
	c.FinishedSeconds = c.VoiceoverSeconds
	add("spoken_words", c.SpokenWords >= 2300 && c.SpokenWords <= 2450, fmt.Sprintf("%d spoken words; required 2,300–2,450. Delivery/editor notes excluded.", c.SpokenWords))
	add("title", titleRE.MatchString(s.Title) && !strings.HasSuffix(s.Title, "..."), "Use a brief title pattern and exactly two trailing dots.")
	all := spokenText(s)
	episodeFree := !episodeRE.MatchString(jsonString(s))
	add("no_episode_numbers", episodeFree, "No episode numbers in title, narration or editor copy.")
	add("scope_keywords", !bannedThemeRE.MatchString(all+" "+s.Title), "No predator-catching subject matter. This lexical screen is supplemented by editorial review.")
	add("identity_screen", !piiRE.MatchString(all), "No obvious URLs, handles, email addresses or IPs in spoken copy; model review checks other identifying details.")
	add("platform_privacy", !platformRE.MatchString(all), "The conservative privacy default keeps game/platform names out of spoken output; internal research can name them.")
	add("voice_actor_isolation", !stageRE.MatchString(all), "VA spoken text must not contain stage/editing directions.")
	add("origin", !strings.Contains(strings.ToLower(all), "from the comments") && !strings.Contains(strings.ToLower(all), "in the comments section"), "Do not attribute the story to the comments.")
	opener := len(s.Lines) > 0 && strings.TrimSpace(s.Lines[0].Text) == "YO GUYS! Welcome back to the channel!"
	add("opener", opener, "The first spoken line is the brief's exact channel opener.")
	outro := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(all, "’", "'"), "…", "..."))
	add("outro", strings.Contains(outro, "i've been bolty") && strings.Contains(outro, "subscribe") && strings.Contains(outro, "i'll see you soon with the next one"), "Include the channel's spoken sign-off.")
	ids := map[int]bool{}
	voices := map[string]bool{}
	roles := map[string]bool{}
	for _, v := range s.Voices {
		voices[v.Name] = true
		roles[strings.ToLower(v.Role)] = strings.TrimSpace(v.Delivery) != ""
	}
	lineOK := len(s.Lines) >= 30 && len(s.Lines) <= 200
	deliveryOK := true
	stars := 0
	beats := map[string]bool{}
	for i, l := range s.Lines {
		if l.ID != i+1 || ids[l.ID] || strings.TrimSpace(l.Text) == "" || !voices[l.Speaker] {
			lineOK = false
		}
		ids[l.ID] = true
		if strings.TrimSpace(l.Delivery) == "" || len(l.Delivery) > 140 || deliveryVisualRE.MatchString(l.Delivery) {
			deliveryOK = false
		}
		if l.SecondTake {
			stars++
		}
		beats[l.Beat] = true
	}
	add("line_integrity", lineOK, "30–200 sequential, unique L01… lines, nonempty speech and a defined reading voice.")
	add("reading_voices", roles["narrator"] && roles["victim"] && roles["antagonist"] && roles["undercover"], "Define narrator, victim, antagonist and undercover reading voices.")
	add("delivery_notes", deliveryOK, "Each line needs a short performance-only delivery note; visual/sound notes belong in editor cues.")
	add("second_takes", stars >= 1, "Star at least one major moment for a second take.")
	add("story_beats", len(beats) >= 6 && !beats[""], "At least six named story beats with a normal-before-the-turn setup.")
	cueOK := len(s.Cues) > 0
	covered := map[int]bool{}
	silence := false
	for _, q := range s.Cues {
		if !ids[q.LineID] || covered[q.LineID] || math.IsNaN(q.HoldSeconds) || math.IsInf(q.HoldSeconds, 0) || q.HoldSeconds < 0 || q.HoldSeconds > 45 || (q.Edit == "" && q.Gameplay == "" && q.Sound == "") {
			cueOK = false
		}
		covered[q.LineID] = true
		c.FinishedSeconds += q.HoldSeconds
		if strings.Contains(strings.ToLower(q.Sound), "silence") || strings.Contains(strings.ToLower(q.Sound), "no music") {
			silence = true
		}
	}
	for id := range ids {
		if !covered[id] {
			cueOK = false
		}
	}
	add("editor_sync", cueOK, "Exactly one cue bundle per spoken line; all IDs resolve. Holds are exclusive non-spoken seconds, 0–45 each.")
	add("finished_runtime", c.FinishedSeconds >= 780 && c.FinishedSeconds <= 840, fmt.Sprintf("Planned finished runtime %.1f minutes; target 13–14. This is an estimate, not measured footage.", c.FinishedSeconds/60))
	add("quiet_reveal", silence, "At least one explicit no-music/silence cue for the serious reveal; the critic checks its placement.")
	tn := len(wordRE.FindAllString(s.Thumbnail.Text, -1))
	add("thumbnail", tn >= 4 && tn <= 5 && s.Thumbnail.Text == strings.ToUpper(s.Thumbnail.Text) && s.Thumbnail.Left != "" && s.Thumbnail.Right != "" && s.Thumbnail.Evidence != "", "Thumbnail requires left/right subjects, central evidence and four to five ALL-CAPS words.")
	assetOK := len(s.Assets) > 0
	for _, a := range s.Assets {
		if a.Name == "" || a.Description == "" || (a.Provenance != "original" && a.Provenance != "recreated") {
			assetOK = false
		}
	}
	add("editor_package", len(s.Privacy) >= 6 && len(s.RunningGags) > 0 && s.Tone != "" && assetOK && len(s.Tasks) >= 4, "Provide tone, six or more privacy instructions, running gags, original/recreated assets and execution tasks.")
	basisOK := s.Basis == d.Snapshot.Project.Basis
	if s.Basis == "fictional" {
		first := ""
		for i, l := range s.Lines {
			if i >= 5 {
				break
			}
			first += strings.ToLower(l.Text) + " "
		}
		basisOK = basisOK && strings.Contains(first, "fictional") && strings.Contains(strings.ToLower(s.Disclosure), "fictional")
	} else if s.Basis == "documented" {
		found := false
		for _, src := range d.Snapshot.Sources {
			if src.ID == d.Snapshot.Project.CaseSourceID && src.Kind == "case" && src.Status == "approved" && src.Rights && len(src.Transcript) > 100 {
				found = true
			}
		}
		basisOK = basisOK && found
	} else {
		basisOK = false
	}
	add("story_basis", basisOK, "Documented stories need an approved redacted case record. Fictional scenarios need an early audible disclosure (build addition).")
	claimsOK := len(d.Research.Claims) > 0
	claimIDs := map[string]bool{}
	for _, claim := range d.Research.Claims {
		if claim.ID == "" || claimIDs[claim.ID] || len(claim.LineIDs) == 0 {
			claimsOK = false
		}
		claimIDs[claim.ID] = true
		for _, id := range claim.LineIDs {
			if !ids[id] {
				claimsOK = false
			}
		}
		matches := 0
		for _, v := range d.Research.Verification.Verdicts {
			if v.ClaimID == claim.ID {
				matches++
				if !validEvidence(v, d.Research.Pages) {
					claimsOK = false
				}
			}
		}
		if matches != 1 {
			claimsOK = false
		}
	}
	for _, v := range d.Research.Verification.Verdicts {
		if !claimIDs[v.ClaimID] {
			claimsOK = false
		}
	}
	add("mechanic_evidence", claimsOK, "Every extracted mechanic needs a supported verdict and an exact quote on an independently fetched official page, no older than seven days.")
	copying := false
	for _, src := range d.Snapshot.Sources {
		if src.Kind == "reference" && hasCopy(all, src.Transcript, 12) {
			copying = true
		}
	}
	add("originality_overlap", !copying, "No 12-word verbatim overlap with the selected reference transcripts (required sign-off is excluded).")
	criticOK := d.Critique != nil
	dimensionsOK := criticOK
	minOK := criticOK
	if d.Critique != nil {
		j := d.Critique
		seen := map[string]bool{}
		sum := 0
		for _, v := range j.Dimensions {
			if !contains(Dimensions, v.Name) || seen[v.Name] || v.Score < 0 || v.Score > 10 || len(strings.TrimSpace(v.Reason)) < 12 {
				dimensionsOK = false
			}
			for _, id := range v.LineIDs {
				if !ids[id] {
					dimensionsOK = false
				}
			}
			seen[v.Name] = true
			sum += v.Score
			if v.Score < d.Snapshot.Settings.MinDimension {
				minOK = false
			}
		}
		if len(seen) != 8 || len(j.Dimensions) != 8 {
			dimensionsOK = false
		} else {
			c.Score = int(math.Round(float64(sum) * 1.25))
		}
		criticOK = j.MechanicsComplete && j.PrivacySafe && j.ScopeSafe && j.NoVictimMockery && j.HonestBasis && len(j.Blockers) == 0
	}
	add("independent_review", criticOK && dimensionsOK, "A separate judge must complete all eight grounded dimensions and pass mechanics completeness, privacy, scope, victim treatment and truthfulness.")
	add("quality_floor", dimensionsOK && minOK && c.Score >= d.Snapshot.Settings.PassScore, fmt.Sprintf("Score %d/100; floor %d, every dimension at least %d/10. These are configurable editorial targets, not PDF-specified or scientifically calibrated scores.", c.Score, d.Snapshot.Settings.PassScore, d.Snapshot.Settings.MinDimension))
	c.Calibrated = len(d.Snapshot.Golds) > 0
	comparisonsOK := c.Calibrated && len(d.Benchmarks) == 2*len(d.Snapshot.Golds)
	for _, gold := range d.Snapshot.Golds {
		positions := map[string]bool{}
		for _, b := range d.Benchmarks {
			if b.GoldID != gold.ID {
				continue
			}
			if b.CandidatePosition != "A" && b.CandidatePosition != "B" {
				comparisonsOK = false
			}
			if positions[b.CandidatePosition] {
				comparisonsOK = false
			}
			positions[b.CandidatePosition] = true
			if (b.Result.Winner != b.CandidatePosition && b.Result.Winner != "tie") || len(b.Result.Reason) < 12 {
				comparisonsOK = false
			}
		}
		if !positions["A"] || !positions["B"] {
			comparisonsOK = false
		}
	}
	add("frozen_baseline", comparisonsOK, "Production approval requires an owner-approved gold script and two blind A/B judgments with order reversed. Candidate must tie or beat it in both. This reduces, but cannot eliminate, judge error.")
	c.Ready = true
	for _, item := range c.Items {
		if item.Blocking && !item.Passed {
			c.Ready = false
		}
	}
	return c
}
func hasCopy(candidate, source string, n int) bool {
	clean := func(s string) []string {
		s = strings.ReplaceAll(strings.ToLower(s), "’", "'")
		for _, phrase := range []string{"yo guys! welcome back to the channel!", "i've been bolty… subscribe, and i'll see you soon with the next one!", "i've been bolty... subscribe, and i'll see you soon with the next one!"} {
			s = strings.ReplaceAll(s, phrase, "")
		}
		return wordRE.FindAllString(s, -1)
	}
	a, b := clean(candidate), clean(source)
	if n < 1 || len(a) < n || len(b) < n {
		return false
	}
	index := map[string]bool{}
	for i := 0; i <= len(b)-n; i++ {
		index[strings.Join(b[i:i+n], " ")] = true
	}
	for i := 0; i <= len(a)-n; i++ {
		if index[strings.Join(a[i:i+n], " ")] {
			return true
		}
	}
	return false
}
