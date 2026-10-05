package interpreter

import (
	"context"
	"time"
)

// InterpreterType 解释器类型
type InterpreterType string

const (
	InterpreterBash InterpreterType = "bash"
	InterpreterZsh  InterpreterType = "zsh"
	InterpreterFish InterpreterType = "fish"
	InterpreterSh   InterpreterType = "sh"
	InterpreterDash InterpreterType = "dash"
	InterpreterAsh  InterpreterType = "ash"
	InterpreterCsh  InterpreterType = "csh"
	InterpreterTcsh InterpreterType = "tcsh"
	InterpreterKsh  InterpreterType = "ksh"
)

// InterpreterInfo 解释器信息
type InterpreterInfo struct {
	Type        InterpreterType `json:"type"`
	Path        string          `json:"path"`
	Version     string          `json:"version"`
	Available   bool            `json:"available"`
	Features    []string        `json:"features"`
	Priority    int             `json:"priority"`
	Description string          `json:"description"`
}

// SystemDetector 系统检测器接口
type SystemDetector interface {
	// 检测可用解释器
	DetectAvailableInterpreters() ([]*InterpreterInfo, error)
	
	// 获取默认解释器
	GetDefaultInterpreter() (*InterpreterInfo, error)
	
	// 检查解释器是否可用
	IsInterpreterAvailable(interpreterType InterpreterType) (bool, error)
	
	// 获取解释器路径
	GetInterpreterPath(interpreterType InterpreterType) (string, error)
	
	// 获取解释器版本
	GetInterpreterVersion(interpreterType InterpreterType) (string, error)
}

// InterpreterManager 解释器管理器接口
type InterpreterManager interface {
	// 初始化管理器
	Initialize() error
	
	// 设置用户首选解释器
	SetUserPreference(userID string, interpreterType InterpreterType) error
	
	// 获取用户首选解释器
	GetUserPreference(userID string) (InterpreterType, error)
	
	// 选择最佳解释器
	SelectBestInterpreter(userID string, preferredType InterpreterType, command string) (*InterpreterInfo, error)
	
	// 获取可用解释器列表
	GetAvailableInterpreters() ([]*InterpreterInfo, error)
	
	// 刷新解释器信息
	RefreshInterpreters() error
	
	// 获取解释器统计信息
	GetUsageStatistics() (*InterpreterStatistics, error)
}

// CommandExecutor 命令执行器接口
type CommandExecutor interface {
	// 执行命令
	ExecuteCommand(ctx context.Context, command string, interpreter *InterpreterInfo, options *ExecutionOptions) (*ExecutionResult, error)
	
	// 准备执行环境
	PrepareEnvironment(interpreter *InterpreterInfo, options *ExecutionOptions) error
	
	// 清理执行环境
	CleanupEnvironment(interpreter *InterpreterInfo) error
}

// ExecutionOptions 执行选项
type ExecutionOptions struct {
	WorkingDir     string            `json:"working_dir"`
	Environment    map[string]string `json:"environment"`
	Timeout        time.Duration     `json:"timeout"`
	InteractiveMode bool             `json:"interactive_mode"`
	CaptureOutput  bool              `json:"capture_output"`
	StdinInput     string            `json:"stdin_input"`
	UserID         string            `json:"user_id"`
	SessionID      string            `json:"session_id"`
}

// ExecutionResult 执行结果
type ExecutionResult struct {
	ExitCode      int           `json:"exit_code"`
	Success       bool          `json:"success"`
	Stdout        string        `json:"stdout"`
	Stderr        string        `json:"stderr"`
	Duration      time.Duration `json:"duration"`
	InterpreterUsed *InterpreterInfo `json:"interpreter_used"`
	MemoryUsage   int64         `json:"memory_usage"`
	CPUUsage      float64       `json:"cpu_usage"`
	Error         string        `json:"error,omitempty"`
}

// InterpreterStatistics 解释器统计信息
type InterpreterStatistics struct {
	TotalExecutions     int64                        `json:"total_executions"`
	UsageByInterpreter  map[InterpreterType]int64    `json:"usage_by_interpreter"`
	UsageByUser         map[string]int64             `json:"usage_by_user"`
	SuccessRate         float64                      `json:"success_rate"`
	AverageExecutionTime time.Duration               `json:"average_execution_time"`
	LastUpdate          time.Time                    `json:"last_update"`
	ErrorsByInterpreter map[InterpreterType]int64    `json:"errors_by_interpreter"`
}

// InterpreterPreference 用户解释器首选项
type InterpreterPreference struct {
	UserID           string          `json:"user_id"`
	PreferredType    InterpreterType `json:"preferred_type"`
	FallbackTypes    []InterpreterType `json:"fallback_types"`
	AutoDetect       bool            `json:"auto_detect"`
	LastUpdated      time.Time       `json:"last_updated"`
}

// CommandCompatibility 命令兼容性信息
type CommandCompatibility struct {
	Command            string                     `json:"command"`
	SupportedInterpreters []InterpreterType        `json:"supported_interpreters"`
	RecommendedInterpreter InterpreterType         `json:"recommended_interpreter"`
	CompatibilityNotes map[InterpreterType]string `json:"compatibility_notes"`
}

// InterpreterFeature 解释器特性
type InterpreterFeature string

const (
	FeaturePOSIXCompliant     InterpreterFeature = "posix_compliant"
	FeatureJobControl         InterpreterFeature = "job_control"
	FeatureCommandHistory     InterpreterFeature = "command_history"
	FeatureTabCompletion      InterpreterFeature = "tab_completion"
	FeatureArraySupport       InterpreterFeature = "array_support"
	FeatureAdvancedGlobbing   InterpreterFeature = "advanced_globbing"
	FeatureBuiltinCommands    InterpreterFeature = "builtin_commands"
	FeatureExtendedTest       InterpreterFeature = "extended_test"
	FeatureArithmeticExpansion InterpreterFeature = "arithmetic_expansion"
	FeatureProcessSubstitution InterpreterFeature = "process_substitution"
)

// InterpreterConfig 解释器配置
type InterpreterConfig struct {
	DefaultInterpreter     InterpreterType              `yaml:"default_interpreter"`
	EnableAutoDetection    bool                         `yaml:"enable_auto_detection"`
	RefreshInterval        time.Duration                `yaml:"refresh_interval"`
	CacheExpiry           time.Duration                `yaml:"cache_expiry"`
	FallbackOrder         []InterpreterType            `yaml:"fallback_order"`
	InterpreterPaths      map[InterpreterType]string   `yaml:"interpreter_paths"`
	FeatureRequirements   map[string][]InterpreterFeature `yaml:"feature_requirements"`
	ExecutionTimeout      time.Duration                `yaml:"execution_timeout"`
	MaxConcurrentExecutions int                        `yaml:"max_concurrent_executions"`
}

// ValidationResult 验证结果
type ValidationResult struct {
	Valid        bool     `json:"valid"`
	Interpreter  *InterpreterInfo `json:"interpreter"`
	Errors       []string `json:"errors"`
	Warnings     []string `json:"warnings"`
	Suggestions  []string `json:"suggestions"`
}

// InterpreterValidator 解释器验证器接口
type InterpreterValidator interface {
	// 验证命令与解释器兼容性
	ValidateCommand(command string, interpreter *InterpreterInfo) (*ValidationResult, error)
	
	// 检查解释器健康状态
	CheckInterpreterHealth(interpreter *InterpreterInfo) (*ValidationResult, error)
	
	// 验证执行环境
	ValidateEnvironment(interpreter *InterpreterInfo, options *ExecutionOptions) (*ValidationResult, error)
}