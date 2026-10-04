package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/zengeeklabs/cpa-window-starter/internal/starter"
)

type demoHost struct{}

func (demoHost) Call(method string, request any, out any) error {
	var value any
	switch method {
	case "host.auth.list":
		value = map[string]any{"files": []starter.Account{{ID: "demo-a", Provider: "codex", Name: "account-a.json", Label: "账号 A · 日常开发", Status: "active"}, {ID: "demo-b", Provider: "codex", Name: "account-b.json", Label: "账号 B · 项目备用", Status: "active"}, {ID: "demo-g", Provider: "antigravity", Name: "gemini-demo.json", Label: "Gemini demo", Status: "active"}, {ID: "demo-c", Provider: "codex", Name: "account-c.json", Label: "账号 C · 暂停使用", Disabled: true, Status: "disabled"}}}
	case "host.model.execute":
		at := time.Now().Add(5 * time.Hour)
		value = starter.ModelResponse{StatusCode: 200, Headers: http.Header{"X-Codex-Primary-Window-Minutes": {"300"}, "X-Codex-Primary-Reset-At": {fmt.Sprint(at.Unix())}, "X-Codex-Primary-Used-Percent": {"0.1"}}, Body: []byte(`{"choices":[{"message":{"content":"OK"}}]}`)}
	default:
		return fmt.Errorf("unsupported mock callback")
	}
	raw, _ := json.Marshal(value)
	return json.Unmarshal(raw, out)
}
func main() {
	address := flag.String("listen", "127.0.0.1:18418", "loopback address")
	stateDir := flag.String("state-dir", ".demo", "isolated demo state directory")
	assets := flag.String("assets", "web", "web assets directory")
	flag.Parse()
	cfg := starter.DefaultConfig()
	cfg.StateFile = filepath.Join(*stateDir, "state.json")
	cfg.Accounts = []string{"demo-a", "demo-b"}
	configPath := filepath.Join(*stateDir, "config.json")
	if raw, err := os.ReadFile(configPath); err == nil {
		_ = json.Unmarshal(raw, &cfg)
	}
	cfg.StateFile = filepath.Join(*stateDir, "state.json")
	engine := starter.NewEngine(demoHost{}, time.Now)
	defer engine.Close()
	if err := engine.Apply(cfg); err != nil {
		log.Print(err)
		return
	}
	app := starter.Application{Engine: engine, Assets: os.DirFS(*assets)}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if len(r.URL.Path) >= len("/v0/management/") && r.URL.Path[:len("/v0/management/")] == "/v0/management/" {
			if r.Header.Get("Authorization") != "Bearer demo-key" {
				http.Error(w, `{"error":"unauthorized"}`, 401)
				return
			}
		}
		if r.URL.Path == "/v0/management/auth-files" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"files":[{"id":"demo-a","plan_type":"plus"},{"id":"demo-b","id_token":{"plan_type":"team"}},{"id":"demo-c","plan_type":"pro"},{"id":"demo-g"}]}`))
			return
		}
		if r.URL.Path == "/v0/management/auth-files/models" {
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Query().Get("name") == "gemini-demo.json" {
				_, _ = w.Write([]byte(`{"models":[{"id":"gemini-demo-flash"},{"id":"gemini-demo-pro"}]}`))
			} else {
				_, _ = w.Write([]byte(`{"models":[{"id":"gpt-6-sol"},{"id":"gpt-demo-small"}]}`))
			}
			return
		}
		if r.URL.Path == "/v0/management/plugins/cpa-window-starter/config" && r.Method == "PATCH" {
			patch := map[string]json.RawMessage{}
			if json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536)).Decode(&patch) != nil {
				http.Error(w, `{"error":"invalid JSON"}`, 400)
				return
			}
			next := engine.Status().Config
			rawCurrent, _ := json.Marshal(next)
			merged := map[string]json.RawMessage{}
			_ = json.Unmarshal(rawCurrent, &merged)
			for key, value := range patch {
				merged[key] = value
			}
			rawNext, _ := json.Marshal(merged)
			_ = json.Unmarshal(rawNext, &next)
			next.StateFile = filepath.Join(*stateDir, "state.json")
			if err := engine.Apply(next); err != nil {
				http.Error(w, `{"error":"invalid config"}`, 400)
				return
			}
			_ = os.MkdirAll(*stateDir, 0700)
			raw, _ := json.Marshal(next)
			_ = os.WriteFile(configPath, raw, 0600)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"ok"}`))
			return
		}
		var body []byte
		if r.Body != nil {
			decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536))
			var value json.RawMessage
			if decoder.Decode(&value) == nil {
				body = value
			}
		}
		request := starter.ManagementRequest{Method: r.Method, Path: r.URL.Path, Headers: r.Header, Query: r.URL.Query(), Body: body}
		raw, _ := json.Marshal(request)
		var env struct {
			OK     bool                       `json:"ok"`
			Result starter.ManagementResponse `json:"result"`
		}
		_ = json.Unmarshal(app.Call("management.handle", raw), &env)
		for key, values := range env.Result.Headers {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.WriteHeader(env.Result.StatusCode)
		_, _ = w.Write(env.Result.Body)
	})
	log.Printf("Demo: http://%s/v0/resource/plugins/cpa-window-starter/status?demo=1", *address)
	if err := http.ListenAndServe(*address, nil); err != nil {
		log.Print(err)
	}
}
