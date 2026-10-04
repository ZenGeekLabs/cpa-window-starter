package starter

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"testing/fstest"
	"time"
)

type fakeHost struct {
	mu       sync.Mutex
	accounts []Account
	calls    []ModelRequest
	headers  http.Header
	body     []byte
	failures map[string]error
	before   func(ModelRequest)
}

func (h *fakeHost) Call(method string, request any, out any) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	var value any
	switch method {
	case "host.auth.list":
		value = map[string]any{"files": h.accounts}
	case "host.model.execute":
		req := request.(ModelRequest)
		h.calls = append(h.calls, req)
		if h.before != nil {
			h.before(req)
		}
		if err := h.failures[req.AuthID]; err != nil {
			return err
		}
		body := h.body
		if body == nil {
			body = []byte(`{"choices":[{"message":{"content":"OK"}}]}`)
		}
		value = ModelResponse{StatusCode: 200, Headers: h.headers, Body: body}
	default:
		return errors.New("unexpected callback")
	}
	raw, _ := json.Marshal(value)
	return json.Unmarshal(raw, out)
}
func (h *fakeHost) count() int { h.mu.Lock(); defer h.mu.Unlock(); return len(h.calls) }
func fixture(t *testing.T, host Host) (*Engine, Config, time.Time) {
	t.Helper()
	now := time.Date(2030, 10, 2, 7, 0, 0, 0, time.FixedZone("CST", 8*3600))
	cfg := DefaultConfig()
	cfg.StateFile = filepath.Join(t.TempDir(), "state.json")
	cfg.Accounts = []string{"a", "b"}
	e := NewEngine(host, func() time.Time { return now })
	if err := e.Apply(cfg); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e.Close)
	return e, cfg, now
}
func waitBatch(e *Engine) {
	e.mu.Lock()
	done := e.batchDone
	e.mu.Unlock()
	if done != nil {
		<-done
	}
}

func TestEachAccountPinnedAndSlotSurvivesRestart(t *testing.T) {
	h := &fakeHost{accounts: []Account{{ID: "a", Provider: "codex", Label: "A"}, {ID: "b", Provider: "codex", Label: "B"}, {ID: "g", Provider: "gemini"}}}
	e, cfg, now := fixture(t, h)
	h.before = func(req ModelRequest) {
		state, err := readState(cfg.StateFile)
		if err != nil || len(state.Claims) == 0 {
			t.Error("claim must reach disk before an outbound request")
		}
	}
	if err := e.Start("scheduled", now); err != nil {
		t.Fatal(err)
	}
	waitBatch(e)
	if h.count() != 2 {
		t.Fatalf("got %d executions, want two accounts", h.count())
	}
	for i, req := range h.calls {
		if req.AuthID != cfg.Accounts[i] || req.ForcedProvider != "codex" {
			t.Fatalf("request is not pinned: %#v", req)
		}
		var body map[string]any
		_ = json.Unmarshal(req.Body, &body)
		if body["reasoning_effort"] != "low" || len(body["messages"].([]any)) != 1 {
			t.Fatal("prewarm must be a small isolated request")
		}
	}
	if err := e.Start("scheduled", now); err != nil {
		t.Fatal(err)
	}
	waitBatch(e)
	if h.count() != 2 {
		t.Fatal("duplicate slot was sent again")
	}
	e.Close()
	next := NewEngine(h, func() time.Time { return now })
	defer next.Close()
	if err := next.Apply(cfg); err != nil {
		t.Fatal(err)
	}
	if err := next.Start("scheduled", now); err != nil {
		t.Fatal(err)
	}
	waitBatch(next)
	if h.count() != 2 {
		t.Fatal("restart replayed a previously claimed slot")
	}
}

func TestDisabledMissingAndOtherProvidersDoNotSend(t *testing.T) {
	h := &fakeHost{accounts: []Account{{ID: "a", Provider: "codex", Disabled: true}, {ID: "b", Provider: "gemini"}}}
	e, _, now := fixture(t, h)
	if err := e.Start("scheduled", now); err != nil {
		t.Fatal(err)
	}
	waitBatch(e)
	if h.count() != 0 {
		t.Fatal("disabled or non-Codex accounts were used")
	}
	for _, r := range e.Status().Results {
		if r.Status != "skipped" {
			t.Fatalf("unexpected status %s", r.Status)
		}
	}
}

func TestOneFailureDoesNotStopOtherAccountAndDoesNotRetry(t *testing.T) {
	h := &fakeHost{accounts: []Account{{ID: "a", Provider: "codex"}, {ID: "b", Provider: "codex"}}, failures: map[string]error{"a": &HostError{StatusCode: 429}}}
	e, _, now := fixture(t, h)
	_ = e.Start("scheduled", now)
	waitBatch(e)
	results := e.Status().Results
	if h.count() != 2 || results[0].Status != "failed" || results[0].HTTPStatus != 429 || results[1].Status != "success" {
		t.Fatalf("wrong account failure handling: %#v", results)
	}
	if results[1].ResetAt != nil {
		t.Fatal("no upstream reset may be fabricated")
	}
}

func TestCorruptStateFailsClosed(t *testing.T) {
	h := &fakeHost{}
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	e := NewEngine(h, nil)
	defer e.Close()
	cfg := DefaultConfig()
	cfg.StateFile = path
	cfg.Accounts = []string{"a"}
	if e.Apply(cfg) == nil {
		t.Fatal("corrupt state must prevent loading")
	}
	if e.Manual() == nil || h.count() != 0 {
		t.Fatal("corrupt state must prevent model requests")
	}
}

func TestInterruptedRunIsUncertainAndNotRepeated(t *testing.T) {
	h := &fakeHost{accounts: []Account{{ID: "a", Provider: "codex"}}}
	e, cfg, now := fixture(t, h)
	e.Close()
	key := "scheduled:" + strconv.FormatInt(now.Unix(), 10) + ":a"
	state := diskState{Version: 1, Claims: map[string]time.Time{key: now}, Results: []Result{{ID: key, AuthID: "a", Status: "running"}}}
	if err := writeState(cfg.StateFile, state); err != nil {
		t.Fatal(err)
	}
	cfg.Accounts = []string{"a"}
	next := NewEngine(h, func() time.Time { return now })
	defer next.Close()
	if err := next.Apply(cfg); err != nil {
		t.Fatal(err)
	}
	if next.Status().Results[0].Status != "uncertain" {
		t.Fatal("interrupted request not marked uncertain")
	}
	_ = next.Start("scheduled", now)
	waitBatch(next)
	if h.count() != 0 {
		t.Fatal("interrupted request must not be replayed")
	}
}

func TestMissedTimeDoesNotBackfill(t *testing.T) {
	h := &fakeHost{accounts: []Account{{ID: "a", Provider: "codex"}, {ID: "b", Provider: "codex"}}}
	e, _, now := fixture(t, h)
	_ = e.Start("scheduled", now.Add(-3*time.Minute))
	waitBatch(e)
	if h.count() != 0 {
		t.Fatal("missed schedule must not send")
	}
	for _, r := range e.Status().Results {
		if r.Status != "missed" {
			t.Fatal("missed slot not recorded")
		}
	}
}

func TestSuccessStatusWithoutModelReplyIsNotSuccess(t *testing.T) {
	h := &fakeHost{accounts: []Account{{ID: "a", Provider: "codex"}}, body: []byte(`{"object":"list"}`)}
	e, _, now := fixture(t, h)
	_ = e.Start("scheduled", now)
	waitBatch(e)
	if e.Status().Results[0].Status != "uncertain" {
		t.Fatal("HTTP 200 alone must not prove model completion")
	}
}

func TestPauseBlocksScheduledCalls(t *testing.T) {
	h := &fakeHost{}
	e, cfg, now := fixture(t, h)
	cfg.ScheduleEnabled = false
	if err := e.Apply(cfg); err != nil {
		t.Fatal(err)
	}
	if e.Status().NextTrigger != nil {
		t.Fatal("paused scheduler has a next trigger")
	}
	if e.Start("scheduled", now) == nil {
		t.Fatal("paused scheduler accepted an automatic call")
	}
}

func TestPublicResourceCannotRunModelsOrListAccounts(t *testing.T) {
	h := &fakeHost{}
	e, _, _ := fixture(t, h)
	app := Application{Engine: e, Assets: fstest.MapFS{"status.html": {Data: []byte("<html>static</html>")}, "ui.js": {Data: []byte("/* theme fixture */")}}}
	for _, suffix := range []string{"status?run=1", "accounts", "run"} {
		response := app.handleManagement(ManagementRequest{Method: "GET", Path: "/v0/resource/plugins/" + PluginID + "/" + suffix})
		if response.StatusCode != 404 {
			t.Fatal("public resource routed to a privileged operation")
		}
	}
	if h.count() != 0 {
		t.Fatal("public resource sent a model request")
	}
	response := app.handleManagement(ManagementRequest{Method: "GET", Path: "/v0/resource/plugins/" + PluginID + "/status"})
	if response.StatusCode != 200 || string(response.Body) != "<html>static</html>" {
		t.Fatal("static page failed")
	}
}

func TestRegistrationHasAllHostRequiredMetadata(t *testing.T) {
	h := &fakeHost{}
	e, cfg, _ := fixture(t, h)
	app := Application{Engine: e}
	rawConfig, _ := json.Marshal(cfg)
	payload, _ := json.Marshal(map[string]any{"schema_version": 6, "config_yaml": rawConfig})
	// ConfigFields are structured values, so decode metadata separately.
	var env map[string]json.RawMessage
	if err := json.Unmarshal(app.Call("plugin.register", payload), &env); err != nil {
		t.Fatal(err)
	}
	if string(env["ok"]) != "true" {
		t.Fatal("registration failed")
	}
	var result struct {
		Schema       uint32                     `json:"schema_version"`
		Metadata     map[string]json.RawMessage `json:"metadata"`
		Capabilities map[string]bool            `json:"capabilities"`
	}
	if err := json.Unmarshal(env["result"], &result); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"Name", "Version", "Author", "GitHubRepository"} {
		var value string
		if json.Unmarshal(result.Metadata[key], &value) != nil || value == "" {
			t.Fatalf("host-required metadata %s is empty", key)
		}
	}
	if result.Schema != 6 || !result.Capabilities["management_api"] || len(result.Capabilities) != 1 {
		t.Fatal("incorrect registration contract")
	}
}

func TestInvalidReconfigurationStopsFutureRequests(t *testing.T) {
	h := &fakeHost{accounts: []Account{{ID: "a", Provider: "codex"}}}
	e, _, now := fixture(t, h)
	app := Application{Engine: e}
	payload, _ := json.Marshal(map[string]any{"schema_version": 6, "config_yaml": []byte("timezone: invalid-zone")})
	var reply struct {
		OK bool `json:"ok"`
	}
	_ = json.Unmarshal(app.Call("plugin.reconfigure", payload), &reply)
	if reply.OK || e.Status().LastError == "" || e.Status().NextTrigger != nil {
		t.Fatal("invalid config kept scheduling")
	}
	if e.Start("scheduled", now) == nil || h.count() != 0 {
		t.Fatal("invalid config allowed a model request")
	}
}

func TestChangedScheduleCannotStartOldTimer(t *testing.T) {
	h := &fakeHost{accounts: []Account{{ID: "a", Provider: "codex"}}}
	e, old, now := fixture(t, h)
	next := old
	next.Times = []string{"08:30"}
	if err := e.Apply(next); err != nil {
		t.Fatal(err)
	}
	if e.start("scheduled", now, &old) == nil || h.count() != 0 {
		t.Fatal("a stale timer sent a model request after the schedule changed")
	}
}

func TestChangingScheduleStopsUnsentAccounts(t *testing.T) {
	h := &fakeHost{accounts: []Account{{ID: "a", Provider: "codex"}, {ID: "b", Provider: "codex"}}}
	e, cfg, now := fixture(t, h)
	h.before = func(req ModelRequest) {
		if req.AuthID == "a" {
			next := cfg
			next.Times = []string{"08:30"}
			if err := e.Apply(next); err != nil {
				t.Error(err)
			}
		}
	}
	if err := e.Start("scheduled", now); err != nil {
		t.Fatal(err)
	}
	waitBatch(e)
	if h.count() != 1 || len(e.Status().Results) != 1 {
		t.Fatal("schedule change did not stop remaining unsent accounts")
	}
}
