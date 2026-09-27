package studio

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"time"
)

const PolicyVersion = "bolty-brief-v1.0.0"
const PromptVersion = "studio-pipeline-v1.0.0"

type Source struct {
	ID          string         `json:"id"`
	Kind        string         `json:"kind"`
	Title       string         `json:"title"`
	URL         string         `json:"url"`
	VideoID     string         `json:"video_id"`
	Transcript  string         `json:"transcript"`
	VisualNotes string         `json:"visual_notes"`
	Rights      bool           `json:"rights"`
	Status      string         `json:"status"`
	Hash        string         `json:"hash"`
	Analysis    *StyleAnalysis `json:"analysis"`
	CreatedAt   string         `json:"created_at"`
}
type StyleAnalysis struct {
	Summary        string   `json:"summary"`
	Craft          []Craft  `json:"craft"`
	DoNotTransfer  []string `json:"do_not_transfer"`
	VisualCoverage string   `json:"visual_coverage"`
}
type Craft struct {
	Technique   string `json:"technique"`
	Evidence    string `json:"evidence"`
	Timestamp   string `json:"timestamp"`
	Application string `json:"application"`
}
type Role struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}
type Settings struct {
	Writer       Role `json:"writer"`
	Judge        Role `json:"judge"`
	Researcher   Role `json:"researcher"`
	PassScore    int  `json:"pass_score"`
	MinDimension int  `json:"min_dimension"`
	MaxCalls     int  `json:"max_calls"`
	DailyCalls   int  `json:"daily_calls"`
}
type Project struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Premise      string   `json:"premise"`
	Platform     string   `json:"platform"`
	Category     string   `json:"category"`
	Basis        string   `json:"basis"`
	CaseSourceID string   `json:"case_source_id"`
	SourceIDs    []string `json:"source_ids"`
	Status       string   `json:"status"`
	CreatedAt    string   `json:"created_at"`
}
type Idea struct {
	Title             string   `json:"title"`
	Hook              string   `json:"hook"`
	Injustice         string   `json:"injustice"`
	EvidenceLadder    []string `json:"evidence_ladder"`
	Payoff            string   `json:"payoff"`
	Category          string   `json:"category"`
	Platform          string   `json:"platform"`
	Thumbnail         string   `json:"thumbnail"`
	Originality       string   `json:"originality"`
	MechanicsToVerify []string `json:"mechanics_to_verify"`
}
type Ideas struct {
	Ideas []Idea `json:"ideas"`
}
type Outline struct {
	Logline        string  `json:"logline"`
	Beats          []Beat  `json:"beats"`
	MechanicClaims []Claim `json:"mechanic_claims"`
	Payoff         string  `json:"payoff"`
}
type Beat struct {
	Name        string `json:"name"`
	Purpose     string `json:"purpose"`
	TargetWords int    `json:"target_words"`
	Evidence    string `json:"evidence"`
}
type Claim struct {
	ID      string `json:"id"`
	Text    string `json:"text"`
	LineIDs []int  `json:"line_ids"`
}
type ClaimAudit struct {
	Claims      []Claim `json:"claims"`
	OmittedRisk string  `json:"omitted_risk"`
}
type Voice struct {
	Name     string `json:"name"`
	Role     string `json:"role"`
	Delivery string `json:"delivery"`
}
type Line struct {
	ID         int    `json:"id"`
	Speaker    string `json:"speaker"`
	Text       string `json:"text"`
	Delivery   string `json:"delivery"`
	SecondTake bool   `json:"second_take"`
	Beat       string `json:"beat"`
}
type Cue struct {
	LineID      int     `json:"line_id"`
	Gameplay    string  `json:"gameplay"`
	Edit        string  `json:"edit"`
	Sound       string  `json:"sound"`
	HoldSeconds float64 `json:"hold_seconds"`
}
type Thumbnail struct {
	Left     string `json:"left"`
	Right    string `json:"right"`
	Evidence string `json:"evidence"`
	Text     string `json:"text"`
}
type Asset struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Provenance  string `json:"provenance"`
}
type TaskSpec struct {
	Role       string `json:"role"`
	Task       string `json:"task"`
	Acceptance string `json:"acceptance"`
}
type Script struct {
	Title       string     `json:"title"`
	Basis       string     `json:"basis"`
	Disclosure  string     `json:"disclosure"`
	Voices      []Voice    `json:"voices"`
	Lines       []Line     `json:"lines"`
	Cues        []Cue      `json:"cues"`
	Thumbnail   Thumbnail  `json:"thumbnail"`
	Tone        string     `json:"tone"`
	Privacy     []string   `json:"privacy"`
	Assets      []Asset    `json:"assets"`
	RunningGags []string   `json:"running_gags"`
	Tasks       []TaskSpec `json:"tasks"`
}
type EvidencePage struct {
	URL         string `json:"url"`
	Text        string `json:"text"`
	Hash        string `json:"hash"`
	RetrievedAt string `json:"retrieved_at"`
	Error       string `json:"error"`
}
type Citation struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}
type SearchResult struct {
	Text      string     `json:"text"`
	Citations []Citation `json:"citations"`
}
type Verdict struct {
	ClaimID string `json:"claim_id"`
	Verdict string `json:"verdict"`
	URL     string `json:"url"`
	Quote   string `json:"quote"`
	Reason  string `json:"reason"`
}
type Verification struct {
	Verdicts []Verdict `json:"verdicts"`
}
type Research struct {
	Claims       []Claim        `json:"claims"`
	Search       SearchResult   `json:"search"`
	Pages        []EvidencePage `json:"pages"`
	Verification Verification   `json:"verification"`
}
type Dimension struct {
	Name    string `json:"name"`
	Score   int    `json:"score"`
	Reason  string `json:"reason"`
	LineIDs []int  `json:"line_ids"`
}
type Critique struct {
	Dimensions        []Dimension `json:"dimensions"`
	Blockers          []string    `json:"blockers"`
	Improvements      []string    `json:"improvements"`
	MechanicsComplete bool        `json:"mechanics_complete"`
	PrivacySafe       bool        `json:"privacy_safe"`
	ScopeSafe         bool        `json:"scope_safe"`
	NoVictimMockery   bool        `json:"no_victim_mockery"`
	HonestBasis       bool        `json:"honest_basis"`
}
type Check struct {
	Code     string `json:"code"`
	Passed   bool   `json:"passed"`
	Detail   string `json:"detail"`
	Blocking bool   `json:"blocking"`
}
type Checks struct {
	Items            []Check `json:"items"`
	SpokenWords      int     `json:"spoken_words"`
	VoiceoverSeconds float64 `json:"voiceover_seconds"`
	FinishedSeconds  float64 `json:"finished_seconds"`
	Score            int     `json:"score"`
	Ready            bool    `json:"ready"`
	Calibrated       bool    `json:"calibrated"`
}
type Comparison struct {
	Winner     string   `json:"winner"`
	Reason     string   `json:"reason"`
	Weaknesses []string `json:"weaknesses"`
}
type Benchmark struct {
	GoldID            string     `json:"gold_id"`
	CandidatePosition string     `json:"candidate_position"`
	Result            Comparison `json:"result"`
}
type Snapshot struct {
	PolicyVersion string   `json:"policy_version"`
	PromptVersion string   `json:"prompt_version"`
	Settings      Settings `json:"settings"`
	Project       Project  `json:"project"`
	Sources       []Source `json:"sources"`
	Golds         []Source `json:"golds"`
	Excerpts      []string `json:"excerpts"`
	Feedback      []string `json:"feedback"`
	BaselineIDs   []string `json:"baseline_ids"`
}
type Draft struct {
	ID         string      `json:"id"`
	ProjectID  string      `json:"project_id"`
	JobID      string      `json:"job_id"`
	Version    int         `json:"version"`
	Script     Script      `json:"script"`
	Research   Research    `json:"research"`
	Critique   *Critique   `json:"critique"`
	Benchmarks []Benchmark `json:"benchmarks"`
	Checks     Checks      `json:"checks"`
	Snapshot   Snapshot    `json:"snapshot"`
	Hash       string      `json:"hash"`
	Approved   bool        `json:"approved"`
	CreatedAt  string      `json:"created_at"`
}
type Job struct {
	ID           string          `json:"id"`
	Kind         string          `json:"kind"`
	ProjectID    string          `json:"project_id"`
	SourceID     string          `json:"source_id"`
	DraftID      string          `json:"draft_id"`
	Input        string          `json:"input"`
	Status       string          `json:"status"`
	Stage        string          `json:"stage"`
	Progress     int             `json:"progress"`
	Error        string          `json:"error"`
	Calls        int             `json:"calls"`
	InputTokens  int             `json:"input_tokens"`
	OutputTokens int             `json:"output_tokens"`
	Result       json.RawMessage `json:"result"`
	Snapshot     Snapshot        `json:"snapshot"`
	CreatedAt    string          `json:"created_at"`
	UpdatedAt    string          `json:"updated_at"`
}
type Feedback struct {
	ID        string `json:"id"`
	DraftID   string `json:"draft_id"`
	Decision  string `json:"decision"`
	Notes     string `json:"notes"`
	Reusable  bool   `json:"reusable"`
	CreatedAt string `json:"created_at"`
}
type Task struct {
	ID         string `json:"id"`
	ProjectID  string `json:"project_id"`
	DraftID    string `json:"draft_id"`
	Role       string `json:"role"`
	Text       string `json:"text"`
	Acceptance string `json:"acceptance"`
	Done       bool   `json:"done"`
}
type Usage struct {
	Input     int    `json:"input"`
	Output    int    `json:"output"`
	RequestID string `json:"request_id"`
}

func newID() string {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b[:])
}
func now() string { return time.Now().UTC().Format(time.RFC3339) }
func hash(v any) string {
	b, _ := json.Marshal(v)
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}
func jsonString(v any) string { b, _ := json.Marshal(v); return string(b) }
func lineLabel(i int) string  { return fmt.Sprintf("L%02d", i) }

// Both providers receive the same portable subset of JSON Schema. Semantic
// constraints are additionally enforced locally; JSON validity is not quality.
func schemaOf(v any) map[string]any { return typeSchema(reflect.TypeOf(v)) }
func typeSchema(t reflect.Type) map[string]any {
	if t.Kind() == reflect.Pointer {
		return typeSchema(t.Elem())
	}
	switch t.Kind() {
	case reflect.Struct:
		p := map[string]any{}
		r := []string{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			n := f.Tag.Get("json")
			if n == "" {
				n = f.Name
			}
			p[n] = typeSchema(f.Type)
			r = append(r, n)
		}
		return map[string]any{"type": "object", "properties": p, "required": r, "additionalProperties": false}
	case reflect.Slice:
		return map[string]any{"type": "array", "items": typeSchema(t.Elem())}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Int, reflect.Int64:
		return map[string]any{"type": "integer"}
	case reflect.Float64:
		return map[string]any{"type": "number"}
	default:
		return map[string]any{"type": "string"}
	}
}
