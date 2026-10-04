package starter

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"
)

const scheduleGrace = 2 * time.Minute

type Engine struct {
	provider   string
	child      *Engine
	mu         sync.Mutex
	host       Host
	now        func() time.Time
	cfg        Config
	state      diskState
	loadedPath string
	lastError  string
	running    bool
	batchDone  chan struct{}
	closed     bool
	started    bool
	changed    chan struct{}
	stop       chan struct{}
	workers    sync.WaitGroup
}

func NewEngine(host Host, now func() time.Time) *Engine {
	if now == nil {
		now = time.Now
	}
	e := &Engine{provider: "codex", host: host, now: now, cfg: DefaultConfig(), changed: make(chan struct{}, 1), stop: make(chan struct{})}
	e.child = &Engine{provider: "antigravity", host: host, now: now, cfg: DefaultConfig(), changed: make(chan struct{}, 1), stop: make(chan struct{})}
	return e
}

func (e *Engine) Apply(cfg Config) error {
	cfg, err := validatePlan(cfg)
	if e.child != nil {
		cfg, err = ValidateConfig(cfg)
	}
	if err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return errors.New("插件正在关闭")
	}
	if e.loadedPath != cfg.StateFile {
		if e.running {
			return errors.New("触发运行期间不能修改状态文件位置")
		}
		state, errRead := readState(cfg.StateFile)
		if errRead != nil {
			e.lastError = errRead.Error()
			return errRead
		}
		e.state = state
		e.loadedPath = cfg.StateFile
	}
	if e.child != nil {
		if err := e.child.Apply(cfg.antigravityConfig()); err != nil {
			e.lastError = err.Error()
			return err
		}
	}
	changed := !samePlan(e.cfg, cfg) || !e.started || e.lastError != ""
	e.cfg = cfg
	e.lastError = ""
	if !e.started {
		e.started = true
		e.workers.Add(1)
		go e.scheduleLoop()
	}
	if changed {
		select {
		case e.changed <- struct{}{}:
		default:
		}
	}
	return nil
}

func (e *Engine) Invalidate(message string) {
	if e.child != nil {
		e.child.Invalidate(message)
	}
	e.mu.Lock()
	e.lastError = message
	e.mu.Unlock()
	select {
	case e.changed <- struct{}{}:
	default:
	}
}

func (e *Engine) Close() {
	if e.child != nil {
		e.child.Close()
	}
	e.mu.Lock()
	if !e.closed {
		e.closed = true
		close(e.stop)
	}
	e.mu.Unlock()
	e.workers.Wait()
}

func (e *Engine) Status() Status {
	e.mu.Lock()
	defer e.mu.Unlock()
	now := e.now()
	cfg := e.cfg
	cfg.Times = append([]string{}, cfg.Times...)
	cfg.Accounts = append([]string{}, cfg.Accounts...)
	cfg.AGAccounts = append([]string{}, cfg.AGAccounts...)
	cfg.AGTimes = append([]string{}, cfg.AGTimes...)
	status := Status{Version: Version, Config: cfg, Running: e.running, LastError: e.lastError, Results: append([]Result{}, e.state.Results...), Now: now}
	if !e.closed && cfg.Enabled && cfg.ScheduleEnabled && len(cfg.Accounts) > 0 && e.lastError == "" {
		if at, err := NextOccurrence(now, cfg); err == nil {
			status.NextTrigger = &at
		}
	}
	if e.child != nil {
		ag := e.child.Status()
		status.Antigravity = &ag
	}
	return status
}

func (e *Engine) Accounts() ([]Account, error) {
	var response struct {
		Files []Account `json:"files"`
	}
	if err := e.host.Call("host.auth.list", struct{}{}, &response); err != nil {
		return nil, errors.New("无法从 CPA 获取账号列表")
	}
	result := []Account{}
	for _, account := range response.Files {
		provider := account.Provider
		if provider == "" {
			provider = account.Type
		}
		if strings.EqualFold(provider, e.provider) && account.ID != "" {
			account.Provider = e.provider
			result = append(result, account)
		}
	}
	return result, nil
}

func (e *Engine) Manual() error { return e.Start("manual", e.now()) }

// Start reserves a batch synchronously; no second batch can overlap it.
func (e *Engine) Start(mode string, at time.Time) error {
	return e.start(mode, at, nil)
}

func (e *Engine) start(mode string, at time.Time, expected *Config) error {
	e.mu.Lock()
	if expected != nil && !samePlan(e.cfg, *expected) {
		e.mu.Unlock()
		return errors.New("计划已更改，本次未发送")
	}
	if e.closed || !e.cfg.Enabled {
		e.mu.Unlock()
		return errors.New("插件未启用")
	}
	if mode == "scheduled" && !e.cfg.ScheduleEnabled {
		e.mu.Unlock()
		return errors.New("定时触发已暂停")
	}
	if e.lastError != "" {
		e.mu.Unlock()
		return errors.New(e.lastError)
	}
	if len(e.cfg.Accounts) == 0 {
		e.mu.Unlock()
		return errors.New("请先选择账号并保存配置")
	}
	if e.running {
		e.mu.Unlock()
		return errors.New("已有触发任务正在运行")
	}
	cfg := e.cfg
	cfg.Accounts = append([]string{}, cfg.Accounts...)
	e.running = true
	done := make(chan struct{})
	e.batchDone = done
	e.workers.Add(1)
	e.mu.Unlock()
	go func() {
		defer e.workers.Done()
		defer func() { e.mu.Lock(); e.running = false; close(done); e.mu.Unlock() }()
		e.runBatch(mode, at, cfg)
	}()
	return nil
}

func (e *Engine) scheduleLoop() {
	defer e.workers.Done()
	for {
		e.mu.Lock()
		cfg := e.cfg
		active := !e.closed && cfg.Enabled && cfg.ScheduleEnabled && len(cfg.Accounts) > 0 && e.lastError == ""
		e.mu.Unlock()
		if !active {
			select {
			case <-e.stop:
				return
			case <-e.changed:
				continue
			}
		}
		next, err := NextOccurrence(e.now(), cfg)
		if err != nil {
			select {
			case <-e.stop:
				return
			case <-e.changed:
				continue
			}
		}
		timer := time.NewTimer(next.Sub(e.now()))
		select {
		case <-e.stop:
			timer.Stop()
			return
		case <-e.changed:
			timer.Stop()
			continue
		case <-timer.C:
			if errStart := e.start("scheduled", next, &cfg); errStart != nil {
				e.recordMissed(next, cfg, "上一次触发尚未结束或计划已更改，本次未发送")
			}
		}
	}
}

func (e *Engine) runBatch(mode string, at time.Time, cfg Config) {
	accounts, err := e.Accounts()
	byID := map[string]Account{}
	for _, account := range accounts {
		byID[account.ID] = account
	}
	manualID := randomID()
	for _, id := range cfg.Accounts {
		e.mu.Lock()
		current := e.cfg
		allowed := !e.closed && e.lastError == "" && current.Enabled && contains(current.Accounts, id) && (mode != "scheduled" || (current.ScheduleEnabled && samePlan(current, cfg)))
		e.mu.Unlock()
		if !allowed {
			continue
		}
		account, exists := byID[id]
		label := id
		if exists {
			label = account.DisplayName()
		}
		key := fmt.Sprintf("scheduled:%d:%s", at.Unix(), id)
		if mode == "manual" {
			key = "manual:" + manualID + ":" + id
		}
		result, ok := e.claim(key, id, label, mode, at)
		if !ok {
			continue
		}
		result.Model = cfg.Model
		switch {
		case err != nil:
			result.Status = "failed"
			result.Message = "无法获取账号列表，本次未发送"
		case mode == "scheduled" && e.now().Sub(at) > scheduleGrace:
			result.Status = "missed"
			result.Message = "已错过触发时间，本次不补发"
		case !exists:
			result.Status = "skipped"
			result.Message = "账号已不在 CPA 中，本次未发送"
		case account.Disabled || account.Status == "disabled":
			result.Status = "skipped"
			result.Message = "账号已禁用，本次未发送"
		default:
			result = e.execute(result, cfg.Model)
		}
		result.CompletedAt = e.now()
		if !e.finish(result) {
			break
		}
	}
}

func (e *Engine) claim(key, id, label, mode string, at time.Time) (Result, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.state.Claims[key]; exists {
		return Result{}, false
	}
	now := e.now()
	result := Result{Provider: e.provider, ID: key, AuthID: id, Label: label, Mode: mode, ScheduledFor: at, StartedAt: now, Status: "running", Message: "正在触发"}
	for claim, timestamp := range e.state.Claims {
		if now.Sub(timestamp) > 14*24*time.Hour {
			delete(e.state.Claims, claim)
		}
	}
	e.state.Claims[key] = now
	e.state.Results = append(e.state.Results, result)
	if len(e.state.Results) > 1000 {
		e.state.Results = append([]Result{}, e.state.Results[len(e.state.Results)-1000:]...)
	}
	if err := writeState(e.loadedPath, e.state); err != nil {
		e.lastError = err.Error()
		return Result{}, false
	}
	return result, true
}

func (e *Engine) finish(result Result) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i := len(e.state.Results) - 1; i >= 0; i-- {
		if e.state.Results[i].ID == result.ID {
			e.state.Results[i] = result
			break
		}
	}
	if err := writeState(e.loadedPath, e.state); err != nil {
		e.lastError = err.Error()
		return false
	}
	return true
}

func (e *Engine) execute(result Result, model string) Result {
	payload := map[string]any{"model": model, "stream": false, "messages": []map[string]string{{"role": "user", "content": "Reply with exactly: OK"}}}
	if e.provider == "codex" {
		payload["reasoning_effort"] = "low"
	}
	body, _ := json.Marshal(payload)
	request := ModelRequest{EntryProtocol: "openai", ExitProtocol: "openai", Model: model, ForcedProvider: e.provider, AuthID: result.AuthID, Body: body}
	var response ModelResponse
	err := e.host.Call("host.model.execute", request, &response)
	if err != nil {
		var hostErr *HostError
		if errors.As(err, &hostErr) {
			result.HTTPStatus = hostErr.StatusCode
		}
		result.Status = "failed"
		result.Message = failureMessage(result.HTTPStatus)
		return result
	}
	result.HTTPStatus = response.StatusCode
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		result.Status = "failed"
		result.Message = failureMessage(response.StatusCode)
		return result
	}
	var completion struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error json.RawMessage `json:"error"`
	}
	if json.Unmarshal(response.Body, &completion) != nil || (len(completion.Error) > 0 && string(completion.Error) != "null") || len(completion.Choices) == 0 || strings.TrimSpace(completion.Choices[0].Message.Content) == "" {
		result.Status = "uncertain"
		result.Message = "CPA 返回成功状态，但没有有效模型回复；不会自动重复发送"
		return result
	}
	if e.provider == "codex" {
		result.ResetAt, result.UsedPercent = FiveHourQuota(response.Headers, e.now())
	}
	result.Status = "success"
	if result.ResetAt != nil {
		result.ResetSource = "upstream_header"
		result.Message = "请求成功，已读取五小时窗口的重置时间"
	} else {
		result.Message = "请求成功，未获取到五小时重置时间"
		if e.provider == "antigravity" {
			result.Message = "请求成功；额度恢复机制及时间未确认"
		}
	}
	return result
}

func failureMessage(status int) string {
	switch status {
	case 401, 403:
		return "账号认证失败或无模型权限，本次触发失败"
	case 429:
		return "账号额度不足或上游限流，本次触发失败"
	case 400, 404:
		return "模型或请求不被上游支持，本次触发失败"
	default:
		return "CPA 或上游请求失败，请检查账号状态；不会自动重复发送"
	}
}

// Only this provider's fields can invalidate its running batch or timer.
func samePlan(a, b Config) bool {
	return a.Enabled == b.Enabled && a.ScheduleEnabled == b.ScheduleEnabled && a.Model == b.Model && a.Timezone == b.Timezone && a.StateFile == b.StateFile && reflect.DeepEqual(a.Times, b.Times) && reflect.DeepEqual(a.Accounts, b.Accounts)
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
func randomID() string { var raw [12]byte; _, _ = rand.Read(raw[:]); return hex.EncodeToString(raw[:]) }

func (e *Engine) recordMissed(at time.Time, cfg Config, message string) {
	for _, id := range cfg.Accounts {
		result, ok := e.claim(fmt.Sprintf("scheduled:%d:%s", at.Unix(), id), id, id, "scheduled", at)
		if !ok {
			continue
		}
		result.Status = "missed"
		result.Message = message
		result.CompletedAt = e.now()
		if !e.finish(result) {
			return
		}
	}
}
