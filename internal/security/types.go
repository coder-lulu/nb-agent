package security

import (
	"context"
	"time"
)

// SecurityLevel 安全级别
type SecurityLevel int

const (
	SecurityLevelLow    SecurityLevel = 1
	SecurityLevelMedium SecurityLevel = 2
	SecurityLevelHigh   SecurityLevel = 3
	SecurityLevelCritical SecurityLevel = 4
)

// AuditLevel 审计级别
type AuditLevel int

const (
	AuditLevelNone    AuditLevel = 0
	AuditLevelBasic   AuditLevel = 1
	AuditLevelDetailed AuditLevel = 2
	AuditLevelVerbose AuditLevel = 3
)

// CommandRisk 命令风险级别
type CommandRisk int

const (
	RiskLow      CommandRisk = 1
	RiskMedium   CommandRisk = 2
	RiskHigh     CommandRisk = 3
	RiskCritical CommandRisk = 4
)

// SecurityPolicy 安全策略配置
type SecurityPolicy struct {
	// 命令控制策略
	AllowedCommands     []string    `yaml:"allowed_commands"`
	BlockedCommands     []string    `yaml:"blocked_commands"`
	DangerousPatterns   []string    `yaml:"dangerous_patterns"`
	RequireConfirmation []string    `yaml:"require_confirmation"`
	
	// 执行限制
	MaxConcurrentTasks   int           `yaml:"max_concurrent_tasks"`
	MaxExecutionTime     time.Duration `yaml:"max_execution_time"`
	MaxOutputSize        int64         `yaml:"max_output_size"`
	MaxMemoryUsage       int64         `yaml:"max_memory_mb"`
	MaxCPUPercent        float64       `yaml:"max_cpu_percent"`
	
	// 权限控制
	RequireRootConfirm   bool          `yaml:"require_root_confirm"`
	AllowedUsers         []string      `yaml:"allowed_users"`
	BlockedUsers         []string      `yaml:"blocked_users"`
	
	// 网络安全
	AllowedProxies       []string      `yaml:"allowed_proxies"`
	TLSRequired          bool          `yaml:"tls_required"`
	CertificateVerify    bool          `yaml:"certificate_verify"`
	
	// 审计策略
	AuditLevel           AuditLevel    `yaml:"audit_level"`
	LogRetentionDays     int           `yaml:"log_retention_days"`
	AlertThreshold       int           `yaml:"alert_threshold"`
	
	// 沙箱模式
	EnableSandbox        bool          `yaml:"enable_sandbox"`
	SandboxPath          string        `yaml:"sandbox_path"`
	RestrictNetworkAccess bool         `yaml:"restrict_network_access"`
}

// CommandContext 命令执行上下文
type CommandContext struct {
	Command        string            `json:"command"`
	User           string            `json:"user"`
	WorkingDir     string            `json:"working_dir"`
	Environment    map[string]string `json:"environment"`
	Interpreter    string            `json:"interpreter"`
	Source         string            `json:"source"` // local, remote, web
	SessionID      string            `json:"session_id"`
	RequestID      string            `json:"request_id"`
	Timestamp      time.Time         `json:"timestamp"`
}

// SecurityCheck 安全检查结果
type SecurityCheck struct {
	Allowed      bool           `json:"allowed"`
	RiskLevel    CommandRisk    `json:"risk_level"`
	Reason       string         `json:"reason"`
	Suggestions  []string       `json:"suggestions"`
	RequireAuth  bool           `json:"require_auth"`
	RequireConfirm bool         `json:"require_confirm"`
	Warnings     []string       `json:"warnings"`
}

// AuditEvent 审计事件
type AuditEvent struct {
	ID            string                 `json:"id"`
	Timestamp     time.Time              `json:"timestamp"`
	EventType     AuditEventType         `json:"event_type"`
	SecurityLevel SecurityLevel          `json:"security_level"`
	User          string                 `json:"user"`
	Command       string                 `json:"command"`
	Result        CommandExecutionResult `json:"result"`
	RiskLevel     CommandRisk            `json:"risk_level"`
	Duration      int64                  `json:"duration_ms"`
	SystemInfo    SystemContext          `json:"system_info"`
	Metadata      map[string]interface{} `json:"metadata"`
}

// AuditEventType 审计事件类型
type AuditEventType string

const (
	EventTypeCommandExecution AuditEventType = "command_execution"
	EventTypeSecurityViolation AuditEventType = "security_violation"
	EventTypeLoginAttempt     AuditEventType = "login_attempt"
	EventTypeConfigChange     AuditEventType = "config_change"
	EventTypeSystemAccess     AuditEventType = "system_access"
	EventTypeFileOperation    AuditEventType = "file_operation"
	EventTypeNetworkActivity  AuditEventType = "network_activity"
)

// CommandExecutionResult 命令执行结果
type CommandExecutionResult struct {
	ExitCode     int           `json:"exit_code"`
	Success      bool          `json:"success"`
	Output       string        `json:"output"`
	Error        string        `json:"error"`
	Duration     time.Duration `json:"duration"`
	MemoryUsage  int64         `json:"memory_usage"`
	CPUUsage     float64       `json:"cpu_usage"`
}

// SystemContext 系统上下文
type SystemContext struct {
	Hostname     string  `json:"hostname"`
	OS           string  `json:"os"`
	Kernel       string  `json:"kernel"`
	Architecture string  `json:"architecture"`
	CPUCores     int     `json:"cpu_cores"`
	Memory       int64   `json:"memory"`
	LoadAverage  float64 `json:"load_average"`
}

// SecurityManager 安全管理器接口
type SecurityManager interface {
	// 策略管理
	LoadPolicy(policy *SecurityPolicy) error
	GetPolicy() *SecurityPolicy
	UpdatePolicy(policy *SecurityPolicy) error
	
	// 命令安全检查
	CheckCommand(ctx context.Context, cmdCtx *CommandContext) (*SecurityCheck, error)
	ValidateCommand(command string) (*SecurityCheck, error)
	
	// 权限检查
	CheckPermission(user, operation string) (bool, error)
	RequireConfirmation(command string) bool
	
	// 审计日志
	LogAuditEvent(event *AuditEvent) error
	GetAuditEvents(filter *AuditFilter) ([]*AuditEvent, error)
	
	// 威胁检测
	DetectThreats(ctx context.Context, cmdCtx *CommandContext) ([]ThreatInfo, error)
	
	// 沙箱管理
	CreateSandbox(ctx context.Context) (SandboxEnvironment, error)
	DestroySandbox(sandbox SandboxEnvironment) error
}

// CommandSecurityFilter 命令安全过滤器接口
type CommandSecurityFilter interface {
	// 语法分析
	AnalyzeCommand(command string) (*CommandAnalysis, error)
	
	// 危险模式检测
	DetectDangerousPatterns(command string) ([]DangerousPattern, error)
	
	// 风险评估
	AssessRisk(ctx context.Context, cmdCtx *CommandContext) (CommandRisk, error)
	
	// 命令清理
	SanitizeCommand(command string) (string, error)
}

// CommandAnalysis 命令分析结果
type CommandAnalysis struct {
	ParsedCommand  *ParsedCommand `json:"parsed_command"`
	CommandType    CommandType    `json:"command_type"`
	RequiredPerms  []string       `json:"required_permissions"`
	AccessedPaths  []string       `json:"accessed_paths"`
	NetworkAccess  bool           `json:"network_access"`
	SystemModify   bool           `json:"system_modify"`
	PrivEscalation bool           `json:"privilege_escalation"`
}

// ParsedCommand 解析后的命令
type ParsedCommand struct {
	Binary      string            `json:"binary"`
	Arguments   []string          `json:"arguments"`
	Flags       map[string]string `json:"flags"`
	Pipes       []*ParsedCommand  `json:"pipes"`
	Redirects   []Redirect        `json:"redirects"`
	Environment map[string]string `json:"environment"`
}

// CommandType 命令类型
type CommandType string

const (
	CommandTypeSystem     CommandType = "system"
	CommandTypeNetwork    CommandType = "network"
	CommandTypeFile       CommandType = "file"
	CommandTypeProcess    CommandType = "process"
	CommandTypeUser       CommandType = "user"
	CommandTypePackage    CommandType = "package"
	CommandTypeService    CommandType = "service"
	CommandTypeCustom     CommandType = "custom"
)

// Redirect 重定向
type Redirect struct {
	Type   RedirectType `json:"type"`
	Source string       `json:"source"`
	Target string       `json:"target"`
}

// RedirectType 重定向类型
type RedirectType string

const (
	RedirectStdout RedirectType = "stdout"
	RedirectStderr RedirectType = "stderr"
	RedirectStdin  RedirectType = "stdin"
	RedirectAppend RedirectType = "append"
)

// DangerousPattern 危险模式
type DangerousPattern struct {
	Pattern     string      `json:"pattern"`
	Risk        CommandRisk `json:"risk"`
	Description string      `json:"description"`
	Suggestion  string      `json:"suggestion"`
	Category    string      `json:"category"`
}

// ThreatInfo 威胁信息
type ThreatInfo struct {
	ThreatType  ThreatType  `json:"threat_type"`
	Severity    CommandRisk `json:"severity"`
	Description string      `json:"description"`
	Indicators  []string    `json:"indicators"`
	Mitigation  string      `json:"mitigation"`
}

// ThreatType 威胁类型
type ThreatType string

const (
	ThreatTypeCodeInjection    ThreatType = "code_injection"
	ThreatTypePrivEscalation   ThreatType = "privilege_escalation"
	ThreatTypeDataExfiltration ThreatType = "data_exfiltration"
	ThreatTypeSystemDamage     ThreatType = "system_damage"
	ThreatTypeMalware          ThreatType = "malware"
	ThreatTypeRecon            ThreatType = "reconnaissance"
)

// SandboxEnvironment 沙箱环境接口
type SandboxEnvironment interface {
	// 环境管理
	Initialize() error
	Cleanup() error
	GetPath() string
	
	// 资源限制
	SetMemoryLimit(limit int64) error
	SetCPULimit(percent float64) error
	SetNetworkAccess(allowed bool) error
	
	// 文件系统限制
	MountReadOnly(path string) error
	CreateTempDir() (string, error)
	
	// 执行控制
	ExecuteCommand(ctx context.Context, command string) (*CommandExecutionResult, error)
}

// AuditFilter 审计过滤器
type AuditFilter struct {
	StartTime     *time.Time      `json:"start_time"`
	EndTime       *time.Time      `json:"end_time"`
	EventTypes    []AuditEventType `json:"event_types"`
	SecurityLevel *SecurityLevel  `json:"security_level"`
	User          string          `json:"user"`
	Command       string          `json:"command"`
	RiskLevel     *CommandRisk    `json:"risk_level"`
	Limit         int             `json:"limit"`
	Offset        int             `json:"offset"`
}

// SecurityMetrics 安全指标
type SecurityMetrics struct {
	TotalCommands        int64                      `json:"total_commands"`
	BlockedCommands      int64                      `json:"blocked_commands"`
	HighRiskCommands     int64                      `json:"high_risk_commands"`
	SecurityViolations   int64                      `json:"security_violations"`
	AverageRiskLevel     float64                    `json:"average_risk_level"`
	ThreatsByType        map[ThreatType]int64       `json:"threats_by_type"`
	CommandsByType       map[CommandType]int64      `json:"commands_by_type"`
	UserActivity         map[string]int64           `json:"user_activity"`
	LastSecurityIncident *time.Time                 `json:"last_security_incident"`
}

// SecurityAlert 安全告警
type SecurityAlert struct {
	ID            string        `json:"id"`
	Timestamp     time.Time     `json:"timestamp"`
	AlertType     AlertType     `json:"alert_type"`
	Severity      SecurityLevel `json:"severity"`
	Title         string        `json:"title"`
	Description   string        `json:"description"`
	AffectedUser  string        `json:"affected_user"`
	Command       string        `json:"command"`
	Evidence      []string      `json:"evidence"`
	Recommendation string       `json:"recommendation"`
	Resolved      bool          `json:"resolved"`
	ResolvedAt    *time.Time    `json:"resolved_at"`
	ResolvedBy    string        `json:"resolved_by"`
}

// AlertType 告警类型
type AlertType string

const (
	AlertTypeSecurityViolation AlertType = "security_violation"
	AlertTypeSuspiciousActivity AlertType = "suspicious_activity"
	AlertTypeUnauthorizedAccess AlertType = "unauthorized_access"
	AlertTypeResourceAbuse     AlertType = "resource_abuse"
	AlertTypeConfigChange      AlertType = "config_change"
	AlertTypeThreatDetected    AlertType = "threat_detected"
)