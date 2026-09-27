package studio

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

// This intentionally performs real, billable calls only with two opt-in flags.
// It is a connectivity/wire smoke test, not a creative-quality acceptance test.
func TestLiveProvider(t *testing.T) {
	if os.Getenv("BOLTY_LIVE") != "1" || os.Getenv("I_ACCEPT_API_CHARGES") != "1" {
		t.Skip("real provider calls disabled; set BOLTY_LIVE=1 and I_ACCEPT_API_CHARGES=1 explicitly")
	}
	provider := os.Getenv("BOLTY_LIVE_PROVIDER")
	if provider != "openai" && provider != "anthropic" {
		t.Fatal("set BOLTY_LIVE_PROVIDER to openai or anthropic")
	}
	role := Role{Provider: provider, Model: os.Getenv("BOLTY_LIVE_MODEL")}
	p := NewProviders(os.Getenv("OPENAI_API_KEY"), os.Getenv("ANTHROPIC_API_KEY"))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	var result struct {
		Status string `json:"status"`
	}
	raw, usage, err := p.JSON(ctx, role, "Return the required JSON only.", "Return status equal to ok.", schemaOf(&result), 2000)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal([]byte(raw), &result); err != nil || result.Status != "ok" {
		t.Fatalf("unexpected structured output: %v", err)
	}
	t.Logf("Structured output passed: %s / %s; known input=%d output=%d", provider, role.Model, usage.Input, usage.Output)
	if os.Getenv("BOLTY_LIVE_SEARCH") == "1" {
		search, _, err := p.Search(ctx, role, "Use official documentation and native source citations.", "Look up the official Discord documentation explaining role permissions. Summarize one permission requirement and cite its official source.")
		if err != nil {
			t.Fatal(err)
		}
		if len(search.Citations) == 0 {
			t.Fatal("no native official citations returned")
		}
		t.Logf("Native search returned %d allowlisted citation(s). Fetched-page semantic verification is a separate full-workflow acceptance test.", len(search.Citations))
	}
}
