package models

import "time"

type AppConfig struct {
	InstanceName  string         `json:"instance_name"`
	Listen        string         `json:"listen"`
	AdminToken    string         `json:"admin_token"`
	DatabasePath  string         `json:"database_path"`
	Projects      []Project      `json:"projects"`
	APIKeys       []APIKey       `json:"api_keys"`
	Providers     []Provider     `json:"providers"`
	LogicalModels []LogicalModel `json:"logical_models"`
	RouteRules    []RouteRule    `json:"route_rules"`
}

type Project struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	DailyBudget float64 `json:"daily_budget"`
}

type APIKey struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Key       string `json:"key"`
	ProjectID string `json:"project_id"`
	Enabled   bool   `json:"enabled"`
}

type Provider struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Type             string   `json:"type"`
	BaseURL          string   `json:"base_url"`
	APIKey           string   `json:"api_key"`
	Enabled          bool     `json:"enabled"`
	Priority         int      `json:"priority"`
	Tags             []string `json:"tags"`
	LatencyMS        int      `json:"latency_ms"`
	JitterMS         int      `json:"jitter_ms"`
	CostPer1KInput   float64  `json:"cost_per_1k_input"`
	CostPer1KOutput  float64  `json:"cost_per_1k_output"`
	ResponseTemplate string   `json:"response_template"`
	SystemPrompt     string   `json:"system_prompt"`
	Capabilities     []string `json:"capabilities"`
	HealthScore      int      `json:"health_score"`
}

type LogicalModel struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	TaskType    string   `json:"task_type"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Targets     []Target `json:"targets"`
}

type Target struct {
	ProviderID string   `json:"provider_id"`
	Model      string   `json:"model"`
	Priority   int      `json:"priority"`
	Weight     int      `json:"weight"`
	Tags       []string `json:"tags"`
}

type RouteRule struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Enabled  bool        `json:"enabled"`
	Priority int         `json:"priority"`
	Match    RouteMatch  `json:"match"`
	Policy   RoutePolicy `json:"policy"`
}

type RouteMatch struct {
	LogicalModel string   `json:"logical_model"`
	ProjectID    string   `json:"project_id"`
	ProviderTags []string `json:"provider_tags"`
	MinChars     int      `json:"min_chars"`
	RequireTools bool     `json:"require_tools"`
}

type RoutePolicy struct {
	Strategy           string   `json:"strategy"`
	Winner             string   `json:"winner"`
	MaxCandidates      int      `json:"max_candidates"`
	HedgeDelayMS       int      `json:"hedge_delay_ms"`
	ProviderIDs        []string `json:"provider_ids"`
	PreferSessionBound bool     `json:"prefer_session_bound"`
}

type NormalizedRequest struct {
	RequestID     string         `json:"request_id"`
	EntryProtocol string         `json:"entry_protocol"`
	TaskType      string         `json:"task_type"`
	ProjectID     string         `json:"project_id"`
	APIKeyID      string         `json:"api_key_id"`
	SessionKey    string         `json:"session_key"`
	LogicalModel  string         `json:"logical_model"`
	Stream        bool           `json:"stream"`
	MaxTokens     int            `json:"max_tokens"`
	Temperature   float64        `json:"temperature"`
	Messages      []Message      `json:"messages"`
	Metadata      map[string]any `json:"metadata"`
	RawBody       map[string]any `json:"raw_body"`
	CreatedAt     time.Time      `json:"created_at"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type NormalizedResponse struct {
	ID           string            `json:"id"`
	ProviderID   string            `json:"provider_id"`
	ProviderType string            `json:"provider_type"`
	Model        string            `json:"model"`
	OutputText   string            `json:"output_text"`
	FinishReason string            `json:"finish_reason"`
	Usage        Usage             `json:"usage"`
	Raw          map[string]any    `json:"raw"`
	Headers      map[string]string `json:"headers"`
	LatencyMS    int64             `json:"latency_ms"`
}

type Usage struct {
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	TotalTokens  int     `json:"total_tokens"`
	Cost         float64 `json:"cost"`
}

type Candidate struct {
	Provider Provider `json:"provider"`
	Target   Target   `json:"target"`
	Score    int      `json:"score"`
}

type RouteDecision struct {
	MatchedRuleIDs []string    `json:"matched_rule_ids"`
	Reason         string      `json:"reason"`
	Policy         RoutePolicy `json:"policy"`
	Candidates     []Candidate `json:"candidates"`
}

type SessionBinding struct {
	SessionKey    string    `json:"session_key"`
	ProjectID     string    `json:"project_id"`
	ProviderID    string    `json:"provider_id"`
	ProviderModel string    `json:"provider_model"`
	BoundAt       time.Time `json:"bound_at"`
	LastSeenAt    time.Time `json:"last_seen_at"`
}

type RequestRecord struct {
	RequestID       string         `json:"request_id"`
	ProjectID       string         `json:"project_id"`
	APIKeyID        string         `json:"api_key_id"`
	SessionKey      string         `json:"session_key"`
	LogicalModel    string         `json:"logical_model"`
	EntryProtocol   string         `json:"entry_protocol"`
	RouteStrategy   string         `json:"route_strategy"`
	MatchedRuleIDs  []string       `json:"matched_rule_ids"`
	FinalProviderID string         `json:"final_provider_id"`
	FinalModel      string         `json:"final_model"`
	Status          string         `json:"status"`
	Error           string         `json:"error"`
	StartedAt       time.Time      `json:"started_at"`
	CompletedAt     time.Time      `json:"completed_at"`
	LogicalUsage    Usage          `json:"logical_usage"`
	PhysicalCost    float64        `json:"physical_cost"`
	NormalizedReq   map[string]any `json:"normalized_request"`
	RouteDecision   map[string]any `json:"route_decision"`
	ResponsePreview string         `json:"response_preview"`
}

type AttemptRecord struct {
	RequestID     string    `json:"request_id"`
	AttemptIndex  int       `json:"attempt_index"`
	ProviderID    string    `json:"provider_id"`
	ProviderModel string    `json:"provider_model"`
	Status        string    `json:"status"`
	IsWinner      bool      `json:"is_winner"`
	LaunchMode    string    `json:"launch_mode"`
	StartedAt     time.Time `json:"started_at"`
	CompletedAt   time.Time `json:"completed_at"`
	CancelledAt   time.Time `json:"cancelled_at"`
	LatencyMS     int64     `json:"latency_ms"`
	Usage         Usage     `json:"usage"`
	Error         string    `json:"error"`
}

type UsageSummary struct {
	Requests       int     `json:"requests"`
	Successes      int     `json:"successes"`
	Failures       int     `json:"failures"`
	InputTokens    int     `json:"input_tokens"`
	OutputTokens   int     `json:"output_tokens"`
	LogicalCost    float64 `json:"logical_cost"`
	PhysicalCost   float64 `json:"physical_cost"`
	RaceExtraCost  float64 `json:"race_extra_cost"`
	ActiveSessions int     `json:"active_sessions"`
}
