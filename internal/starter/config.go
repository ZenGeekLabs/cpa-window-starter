package starter

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	_ "time/tzdata"

	"gopkg.in/yaml.v3"
)

const PluginID = "cpa-window-starter"
const Version = "0.2.6"

type Config struct {
	AGScheduleEnabled bool     `json:"antigravity_schedule_enabled" yaml:"antigravity_schedule_enabled"`
	AGTimes           []string `json:"antigravity_times" yaml:"antigravity_times"`
	AGTimezone        string   `json:"antigravity_timezone" yaml:"antigravity_timezone"`
	AGAccounts        []string `json:"antigravity_accounts" yaml:"antigravity_accounts"`
	AGModel           string   `json:"antigravity_model" yaml:"antigravity_model"`
	Enabled           bool     `json:"enabled" yaml:"enabled"`
	ScheduleEnabled   bool     `json:"schedule_enabled" yaml:"schedule_enabled"`
	Times             []string `json:"times" yaml:"times"`
	Timezone          string   `json:"timezone" yaml:"timezone"`
	Accounts          []string `json:"accounts" yaml:"accounts"`
	Model             string   `json:"model" yaml:"model"`
	StateFile         string   `json:"-" yaml:"state_file"`
}

func DefaultConfig() Config {
	return Config{AGTimes: []string{"07:00", "13:00", "19:00"}, AGTimezone: "Asia/Shanghai", AGAccounts: []string{}, Enabled: true, ScheduleEnabled: true, Times: []string{"07:00", "13:00", "19:00"}, Timezone: "Asia/Shanghai", Accounts: []string{}, Model: "gpt-6-sol", StateFile: "plugins/state/cpa-window-starter.json"}
}

func ParseConfig(raw []byte) (Config, error) {
	cfg := DefaultConfig()
	if len(raw) > 65536 {
		return cfg, errors.New("插件配置过大")
	}
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return cfg, errors.New("插件配置不是有效 YAML")
	}
	return ValidateConfig(cfg)
}

func ValidateConfig(cfg Config) (Config, error) {
	var err error
	cfg, err = validatePlan(cfg)
	if err != nil {
		return cfg, err
	}
	if cfg.AGTimezone == "" {
		cfg.AGTimezone = cfg.Timezone
	}
	if cfg.AGTimes == nil {
		cfg.AGTimes = []string{"07:00", "13:00", "19:00"}
	}
	ag, err := validatePlan(cfg.antigravityConfig())
	if err != nil {
		return cfg, fmt.Errorf("Antigravity: %w", err)
	}
	cfg.AGTimes, cfg.AGTimezone, cfg.AGAccounts = ag.Times, ag.Timezone, ag.Accounts
	cfg.AGModel = strings.TrimSpace(cfg.AGModel)
	return cfg, nil
}

func (cfg Config) antigravityConfig() Config {
	model := cfg.AGModel
	if strings.TrimSpace(model) == "" && len(cfg.AGAccounts) == 0 {
		model = "unselected"
	}
	return Config{Enabled: cfg.Enabled, ScheduleEnabled: cfg.AGScheduleEnabled, Times: cfg.AGTimes, Timezone: cfg.AGTimezone, Accounts: cfg.AGAccounts, Model: model, StateFile: cfg.StateFile + ".antigravity"}
}

func validatePlan(cfg Config) (Config, error) {
	if _, err := time.LoadLocation(cfg.Timezone); err != nil {
		return cfg, errors.New("请选择有效的 IANA 时区，例如 Asia/Shanghai")
	}
	if len(cfg.Times) > 24 {
		return cfg, errors.New("每天最多设置 24 个触发时间")
	}
	if cfg.ScheduleEnabled && len(cfg.Times) == 0 {
		return cfg, errors.New("开启定时触发时至少需要一个时间")
	}
	seen := map[string]bool{}
	for _, v := range cfg.Times {
		t, err := time.Parse("15:04", v)
		if err != nil || t.Format("15:04") != v {
			return cfg, errors.New("触发时间必须使用 HH:MM 格式")
		}
		if seen[v] {
			return cfg, errors.New("触发时间不能重复")
		}
		seen[v] = true
	}
	cfg.Times = append([]string{}, cfg.Times...)
	sort.Strings(cfg.Times)
	ids := map[string]bool{}
	normalized := []string{}
	for _, id := range cfg.Accounts {
		id = strings.TrimSpace(id)
		if id == "" || len(id) > 1024 {
			return cfg, errors.New("账号 ID 无效")
		}
		if !ids[id] {
			normalized = append(normalized, id)
			ids[id] = true
		}
	}
	if len(normalized) > 256 {
		return cfg, errors.New("最多选择 256 个账号")
	}
	cfg.Accounts = normalized
	cfg.Model = strings.TrimSpace(cfg.Model)
	if cfg.Model == "" || len(cfg.Model) > 160 || strings.ContainsAny(cfg.Model, "\r\n\t ") {
		return cfg, errors.New("请输入有效的模型名称")
	}
	if cfg.StateFile == "" {
		cfg.StateFile = DefaultConfig().StateFile
	}
	return cfg, nil
}

func NextOccurrence(now time.Time, cfg Config) (time.Time, error) {
	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return time.Time{}, err
	}
	local := now.In(loc)
	for day := 0; day < 3; day++ {
		date := time.Date(local.Year(), local.Month(), local.Day()+day, 12, 0, 0, 0, loc)
		var nearest time.Time
		for _, value := range cfg.Times {
			clock, err := time.Parse("15:04", value)
			if err != nil {
				return time.Time{}, fmt.Errorf("invalid schedule")
			}
			at := time.Date(date.Year(), date.Month(), date.Day(), clock.Hour(), clock.Minute(), 0, 0, loc)
			if at.Hour() != clock.Hour() || at.Minute() != clock.Minute() || at.Day() != date.Day() {
				continue
			}
			if at.After(now) && (nearest.IsZero() || at.Before(nearest)) {
				nearest = at
			}
		}
		if !nearest.IsZero() {
			return nearest, nil
		}
	}
	return time.Time{}, errors.New("没有可用的触发时间")
}
