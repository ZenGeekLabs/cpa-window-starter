package starter

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
)

type Application struct {
	Engine *Engine
	Assets fs.FS
}

type envelope struct {
	OK     bool      `json:"ok"`
	Result any       `json:"result,omitempty"`
	Error  *rpcError `json:"error,omitempty"`
}
type rpcError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ManagementRequest struct {
	Method         string
	Path           string
	Headers        http.Header
	Query          url.Values
	Body           []byte
	HostCallbackID string `json:"host_callback_id,omitempty"`
}
type ManagementResponse struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
}

func (a *Application) Call(method string, raw []byte) []byte {
	var result any
	var err error
	switch method {
	case "plugin.register", "plugin.reconfigure":
		var request struct {
			ConfigYAML    []byte `json:"config_yaml"`
			SchemaVersion uint32 `json:"schema_version"`
		}
		if json.Unmarshal(raw, &request) != nil {
			err = errors.New("注册请求无效")
			break
		}
		var cfg Config
		cfg, err = ParseConfig(request.ConfigYAML)
		if err != nil {
			a.Engine.Invalidate(err.Error())
			break
		}
		err = a.Engine.Apply(cfg)
		if err != nil {
			break
		}
		schema := uint32(1)
		if request.SchemaVersion >= 6 {
			schema = 6
		}
		result = a.registration(schema)
	case "plugin.quiesce", "plugin.shutdown":
		a.Engine.Close()
		result = struct{}{}
	case "management.register":
		prefix := "/plugins/" + PluginID
		result = map[string]any{
			"resources": []map[string]string{{"Path": "/status", "Menu": "账号定时预热", "Description": "设置触发时间、选择账号并查看预热结果"}, {"Path": "/app.js"}, {"Path": "/style.css"}, {"Path": "/cpa-session.js"}, {"Path": "/ui.js"}, {"Path": "/i18n.js"}, {"Path": "/gemini.svg"}, {"Path": "/openai.png"}, {"Path": "/timezone-picker.js"}, {"Path": "/timezone-picker.css"}},
			"routes":    []map[string]string{{"Method": "GET", "Path": prefix + "/status"}, {"Method": "GET", "Path": prefix + "/accounts"}, {"Method": "POST", "Path": prefix + "/validate"}, {"Method": "POST", "Path": prefix + "/run"}},
		}
	case "management.handle":
		var request ManagementRequest
		if len(raw) > 1024*1024 || json.Unmarshal(raw, &request) != nil {
			err = errors.New("管理请求无效")
			break
		}
		result = a.handleManagement(request)
	default:
		err = errors.New("插件不支持这个方法")
	}
	if err != nil {
		out, _ := json.Marshal(envelope{OK: false, Error: &rpcError{Code: "plugin_error", Message: err.Error()}})
		return out
	}
	out, _ := json.Marshal(envelope{OK: true, Result: result})
	return out
}

func (a *Application) registration(schema uint32) any {
	fields := []map[string]string{
		{"Name": "schedule_enabled", "Type": "boolean", "Description": "是否开启定时触发"},
		{"Name": "times", "Type": "array", "Description": "每天触发时间，HH:MM 格式"},
		{"Name": "timezone", "Type": "string", "Description": "时区，默认 Asia/Shanghai"},
		{"Name": "accounts", "Type": "array", "Description": "选中的 Codex 账号 ID"},
		{"Name": "model", "Type": "string", "Description": "最小预热请求所使用的模型"},
	}
	fields = append(fields,
		map[string]string{"Name": "antigravity_schedule_enabled", "Type": "boolean", "Description": "Antigravity: automatic requests"},
		map[string]string{"Name": "antigravity_times", "Type": "array", "Description": "Antigravity: daily HH:MM times"},
		map[string]string{"Name": "antigravity_timezone", "Type": "string", "Description": "Antigravity: IANA timezone"},
		map[string]string{"Name": "antigravity_accounts", "Type": "array", "Description": "Antigravity: selected account IDs"},
		map[string]string{"Name": "antigravity_model", "Type": "string", "Description": "Antigravity: Gemini model ID"})
	return map[string]any{"schema_version": schema, "metadata": map[string]any{"Name": PluginID, "Version": Version, "Author": "禅极科技", "GitHubRepository": "https://github.com/ZenGeekLabs/cpa-window-starter", "Logo": "", "ConfigFields": fields}, "capabilities": map[string]bool{"management_api": true}}
}

func (a *Application) handleManagement(request ManagementRequest) ManagementResponse {
	resource := "/v0/resource/plugins/" + PluginID + "/"
	if strings.HasPrefix(request.Path, resource) {
		if request.Method != http.MethodGet {
			return JSONResponse(405, map[string]string{"error": "不支持这个请求方法"})
		}
		name := strings.TrimPrefix(request.Path, resource)
		files := map[string]string{"status": "status.html", "app.js": "app.js", "style.css": "style.css", "cpa-session.js": "cpa-session.js", "ui.js": "ui.js", "i18n.js": "i18n.js", "gemini.svg": "gemini.svg", "openai.png": "openai.png", "timezone-picker.js": "timezone-picker.js", "timezone-picker.css": "timezone-picker.css"}
		file, exists := files[name]
		if !exists {
			return JSONResponse(404, map[string]string{"error": "页面不存在"})
		}
		body, err := fs.ReadFile(a.Assets, file)
		if err != nil {
			return JSONResponse(500, map[string]string{"error": "页面资源不可用"})
		}
		policy := "default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'self' data:; base-uri 'none'; object-src 'none'; frame-ancestors 'self'"
		if name == "status" {
			appearance, err := fs.ReadFile(a.Assets, "ui.js")
			if err != nil {
				return JSONResponse(500, map[string]string{"error": "页面资源不可用"})
			}
			// Deliver the theme with the initial HTML: no extra request or deferred app startup.
			script := string(appearance) + "\nCPAWindowUI.applyAppearance(window);"
			critical := ":root{color-scheme:light;background:#f7f8fa;color:#192234}:root[data-theme=dark]{color-scheme:dark;background:#13161c;color:#e9ecf3}body{margin:0;background:inherit}"
			scriptHash := sha256.Sum256([]byte(script))
			styleHash := sha256.Sum256([]byte(critical))
			policy = strings.Replace(policy, "script-src 'self'", "script-src 'self' 'sha256-"+base64.StdEncoding.EncodeToString(scriptHash[:])+"'", 1)
			policy = strings.Replace(policy, "style-src 'self'", "style-src 'self' 'sha256-"+base64.StdEncoding.EncodeToString(styleHash[:])+"'", 1)
			body = []byte(strings.Replace(string(body), "<!-- CPA_APPEARANCE -->", "<style>"+critical+"</style><script>"+script+"</script>", 1))
		}
		types := map[string]string{"status": "text/html; charset=utf-8", "app.js": "text/javascript; charset=utf-8", "cpa-session.js": "text/javascript; charset=utf-8", "ui.js": "text/javascript; charset=utf-8", "i18n.js": "text/javascript; charset=utf-8", "style.css": "text/css; charset=utf-8", "gemini.svg": "image/svg+xml", "openai.png": "image/png", "timezone-picker.js": "text/javascript; charset=utf-8", "timezone-picker.css": "text/css; charset=utf-8"}
		return ManagementResponse{StatusCode: 200, Body: body, Headers: http.Header{"Content-Type": {types[name]}, "Cache-Control": {"no-store"}, "X-Content-Type-Options": {"nosniff"}, "Content-Security-Policy": {policy}}}
	}
	// Resource handlers never expose account data or execute privileged callbacks.
	prefix := "/v0/management/plugins/" + PluginID
	switch {
	case request.Method == "GET" && request.Path == prefix+"/status":
		return JSONResponse(200, a.Engine.Status())
	case request.Method == "GET" && request.Path == prefix+"/accounts":
		accounts, err := a.Engine.Accounts()
		if err != nil {
			return JSONResponse(502, map[string]string{"error": err.Error()})
		}
		if a.Engine.child != nil {
			ag, err := a.Engine.child.Accounts()
			if err != nil {
				return JSONResponse(502, map[string]string{"error": err.Error()})
			}
			accounts = append(accounts, ag...)
		}
		return JSONResponse(200, map[string]any{"accounts": accounts})
	case request.Method == "POST" && request.Path == prefix+"/validate":
		cfg := DefaultConfig()
		if len(request.Body) > 65536 || json.Unmarshal(request.Body, &cfg) != nil {
			return JSONResponse(400, map[string]string{"error": "配置格式无效"})
		}
		cfg, err := ValidateConfig(cfg)
		if err != nil {
			return JSONResponse(400, map[string]string{"error": err.Error()})
		}
		return JSONResponse(200, cfg)
	case request.Method == "POST" && request.Path == prefix+"/run":
		var selection struct {
			Provider string `json:"provider"`
		}
		if len(request.Body) > 0 && json.Unmarshal(request.Body, &selection) != nil {
			return JSONResponse(400, map[string]string{"error": "Invalid provider"})
		}
		engine := a.Engine
		if selection.Provider == "antigravity" {
			engine = a.Engine.child
		} else if selection.Provider != "" && selection.Provider != "codex" {
			return JSONResponse(400, map[string]string{"error": "Invalid provider"})
		}
		if engine == nil {
			return JSONResponse(400, map[string]string{"error": "Provider unavailable"})
		}
		if err := engine.Manual(); err != nil {
			return JSONResponse(409, map[string]string{"error": err.Error()})
		}
		return JSONResponse(202, map[string]string{"message": "已开始逐账号触发"})
	default:
		return JSONResponse(404, map[string]string{"error": "接口不存在"})
	}
}

func JSONResponse(status int, value any) ManagementResponse {
	body, _ := json.Marshal(value)
	return ManagementResponse{StatusCode: status, Headers: http.Header{"Content-Type": {"application/json; charset=utf-8"}, "Cache-Control": {"no-store"}, "X-Content-Type-Options": {"nosniff"}}, Body: body}
}
