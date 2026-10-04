package starter

import (
	"net/http"
	"net/url"
	"time"
)

type Host interface{ Call(string, any, any) error }

type HostError struct {
	StatusCode int
	Code       string
}

func (e *HostError) Error() string { return "CPA 宿主调用失败" }

type Account struct {
	ID          string `json:"id"`
	AuthIndex   string `json:"auth_index,omitempty"`
	Name        string `json:"name"`
	Provider    string `json:"provider"`
	Type        string `json:"type,omitempty"`
	Label       string `json:"label,omitempty"`
	Email       string `json:"email,omitempty"`
	Status      string `json:"status,omitempty"`
	Disabled    bool   `json:"disabled"`
	Unavailable bool   `json:"unavailable"`
}

func (a Account) DisplayName() string {
	if a.Label != "" {
		return a.Label
	}
	if a.Email != "" {
		return a.Email
	}
	return a.Name
}

type ModelRequest struct {
	EntryProtocol  string      `json:"entry_protocol"`
	ExitProtocol   string      `json:"exit_protocol"`
	Model          string      `json:"model"`
	Stream         bool        `json:"stream"`
	Body           []byte      `json:"body"`
	Headers        http.Header `json:"headers"`
	Query          url.Values  `json:"query"`
	Alt            string      `json:"alt"`
	ForcedProvider string      `json:"forced_provider"`
	AuthID         string      `json:"auth_id"`
}

type ModelResponse struct {
	StatusCode int         `json:"status_code"`
	Headers    http.Header `json:"headers"`
	Body       []byte      `json:"body"`
}

type Result struct {
	Model        string     `json:"model,omitempty"`
	Provider     string     `json:"provider,omitempty"`
	ID           string     `json:"id"`
	AuthID       string     `json:"auth_id"`
	Label        string     `json:"label"`
	Mode         string     `json:"mode"`
	ScheduledFor time.Time  `json:"scheduled_for"`
	StartedAt    time.Time  `json:"started_at"`
	CompletedAt  time.Time  `json:"completed_at"`
	Status       string     `json:"status"`
	HTTPStatus   int        `json:"http_status,omitempty"`
	Message      string     `json:"message"`
	ResetAt      *time.Time `json:"reset_at,omitempty"`
	ResetSource  string     `json:"reset_source,omitempty"`
	UsedPercent  *float64   `json:"used_percent,omitempty"`
}

type Status struct {
	Antigravity *Status    `json:"antigravity,omitempty"`
	Version     string     `json:"version"`
	Config      Config     `json:"config"`
	Running     bool       `json:"running"`
	NextTrigger *time.Time `json:"next_trigger,omitempty"`
	LastError   string     `json:"last_error,omitempty"`
	Results     []Result   `json:"results"`
	Now         time.Time  `json:"now"`
}
