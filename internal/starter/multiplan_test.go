package starter

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func TestIndependentProvidersAndUnknownGeminiReset(t *testing.T) {
	h := &fakeHost{accounts: []Account{{ID: "a", Provider: "codex"}, {ID: "g", Provider: "antigravity"}}}
	e := NewEngine(h, time.Now)
	defer e.Close()
	cfg := DefaultConfig()
	cfg.StateFile = filepath.Join(t.TempDir(), "state.json")
	cfg.ScheduleEnabled = false
	cfg.Accounts = []string{"a"}
	cfg.AGAccounts = []string{"g"}
	cfg.AGModel = "gemini-test"
	cfg.AGTimezone = "America/New_York"
	cfg.AGTimes = []string{"08:15"}
	if err := e.Apply(cfg); err != nil {
		t.Fatal(err)
	}
	if err := e.Manual(); err != nil {
		t.Fatal(err)
	}
	waitBatch(e)
	if err := e.child.Manual(); err != nil {
		t.Fatal(err)
	}
	waitBatch(e.child)
	if len(h.calls) != 2 || h.calls[0].ForcedProvider != "codex" || h.calls[1].ForcedProvider != "antigravity" || h.calls[1].AuthID != "g" {
		t.Fatalf("wrong routing: %+v", h.calls)
	}
	var body map[string]any
	_ = json.Unmarshal(h.calls[1].Body, &body)
	if _, exists := body["reasoning_effort"]; exists {
		t.Fatal("Codex-only field sent to Gemini")
	}
	s := e.Status()
	if len(s.Results) != 1 || len(s.Antigravity.Results) != 1 || s.Antigravity.Results[0].ResetAt != nil {
		t.Fatal("results or reset crossed providers")
	}
	if s.Antigravity.Config.Timezone != "America/New_York" || s.Antigravity.Config.ScheduleEnabled {
		t.Fatal("independent config lost")
	}
	if _, err := readState(cfg.StateFile + ".antigravity"); err != nil {
		t.Fatal(err)
	}
}

func TestLegacyConfigDoesNotEnableAntigravity(t *testing.T) {
	cfg, err := ParseConfig([]byte("schedule_enabled: false\naccounts: [a]\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AGScheduleEnabled || len(cfg.AGAccounts) != 0 || cfg.AGModel != "" {
		t.Fatal("legacy config unexpectedly enables Gemini")
	}
	cfg.AGAccounts = []string{"g"}
	if _, err := ValidateConfig(cfg); err == nil {
		t.Fatal("selected Gemini accounts require a model")
	}
	cfg.AGModel = "gemini-test"
	cfg.AGTimezone = "invalid"
	if _, err := ValidateConfig(cfg); err == nil {
		t.Fatal("invalid AG timezone accepted")
	}
}

func TestAntigravityScheduledClaimsSurviveRestart(t *testing.T) {
	h := &fakeHost{accounts: []Account{{ID: "g", Provider: "antigravity"}}}
	now := time.Now()
	cfg := DefaultConfig()
	cfg.StateFile = filepath.Join(t.TempDir(), "state.json")
	cfg.ScheduleEnabled = false
	cfg.AGScheduleEnabled = true
	cfg.AGAccounts = []string{"g"}
	cfg.AGModel = "gemini-test"
	e := NewEngine(h, func() time.Time { return now })
	if err := e.Apply(cfg); err != nil {
		t.Fatal(err)
	}
	if err := e.child.Start("scheduled", now); err != nil {
		t.Fatal(err)
	}
	waitBatch(e.child)
	e.Close()
	next := NewEngine(h, func() time.Time { return now })
	defer next.Close()
	if err := next.Apply(cfg); err != nil {
		t.Fatal(err)
	}
	if err := next.child.Start("scheduled", now); err != nil {
		t.Fatal(err)
	}
	waitBatch(next.child)
	if h.count() != 1 {
		t.Fatal("replayed persisted Antigravity slot")
	}
	// A Codex account ID in the Antigravity plan must never fall back to another provider.
	cfg.AGAccounts = []string{"codex-only"}
	if err := next.Apply(cfg); err != nil {
		t.Fatal(err)
	}
	if err := next.child.Manual(); err != nil {
		t.Fatal(err)
	}
	waitBatch(next.child)
	if h.count() != 1 {
		t.Fatal("provider fallback sent a request")
	}
}

func TestIndependentTimezoneDST(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Timezone = "America/New_York"
	cfg.Times = []string{"02:30"}
	now := time.Date(2030, 3, 10, 0, 0, 0, 0, time.UTC)
	next, err := NextOccurrence(now, cfg)
	if err != nil {
		t.Fatal(err)
	}
	loc, _ := time.LoadLocation(cfg.Timezone)
	if next.In(loc).Day() != 11 || next.In(loc).Hour() != 2 {
		t.Fatalf("nonexistent DST time was not skipped: %v", next)
	}
}

func TestEditingGeminiDoesNotInterruptCodexBatch(t *testing.T) {
	h := &fakeHost{accounts: []Account{{ID: "a", Provider: "codex"}, {ID: "b", Provider: "codex"}}}
	e, cfg, now := fixture(t, h)
	first := true
	h.before = func(ModelRequest) {
		if first {
			first = false
			cfg.AGTimezone = "Europe/London"
			cfg.AGTimes = []string{"09:15"}
			if err := e.Apply(cfg); err != nil {
				t.Error(err)
			}
		}
	}
	if err := e.Start("scheduled", now); err != nil {
		t.Fatal(err)
	}
	waitBatch(e)
	if h.count() != 2 {
		t.Fatal("editing Gemini interrupted Codex batch")
	}
}
