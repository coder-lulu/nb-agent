package executor

import (
	"context"
	"time"
)

// ExecutorType 执行器类型
type ExecutorType string

const (
	ExecutorTypeShell   ExecutorType = "shell"   // Shell命令执行器
	ExecutorTypeScript  ExecutorType = "script"  // 脚本执行器
	ExecutorTypeFile    ExecutorType = "file"    // 文件操作执行器
	ExecutorTypeService ExecutorType = "service" // 服务管理执行器
	ExecutorTypeSystem  ExecutorType = "system"  // 系统操作执行器
	ExecutorTypeCustom  ExecutorType = "custom"  // 自定义执行器
)

// ExecutionStatus 执行状态
type ExecutionStatus string

const (
	StatusPending   ExecutionStatus = "pending"   // 等待执行
	StatusRunning   ExecutionStatus = "running"   // 正在执行
	StatusCompleted ExecutionStatus = "completed" // 执行完成
	StatusFailed    ExecutionStatus = "failed"    // 执行失败
	StatusCancelled ExecutionStatus = "cancelled" // 执行取消
	StatusTimeout   ExecutionStatus = "timeout"   // 执行超时
)

// TaskExecution 任务执行
type TaskExecution struct {
	ID          string                 `json:"id"`
	Type        ExecutorType           `json:"type"`
	Command     string                 `json:"command"`
	Parameters  map[string]interface{} `json:"parameters"`
	WorkingDir  string                 `json:"working_dir"`
	Environment map[string]string      `json:"environment"`
	Timeout     time.Duration          `json:"timeout"`
	Status      ExecutionStatus        `json:"status"`
	Result      *ExecutionResult       `json:"result"`
	StartedAt   *time.Time             `json:"started_at"`
	CompletedAt *time.Time             `json:"completed_at"`
	Error       string                 `json:"error"`
}

// ExecutionResult 执行结果
type ExecutionResult struct {
	Success    bool                   `json:"success"`
	ExitCode   int                    `json:"exit_code"`
	Output     string                 `json:"output"`
	Error      string                 `json:"error"`
	Data       map[string]interface{} `json:"data"`
	Duration   time.Duration          `json:"duration"`
	Metrics    *ExecutionMetrics      `json:"metrics"`
}

// ExecutionMetrics 执行指标
type ExecutionMetrics struct {
	CPUUsage     float64 `json:"cpu_usage"`
	MemoryUsage  int64   `json:"memory_usage"`
	DiskRead     int64   `json:"disk_read"`
	DiskWrite    int64   `json:"disk_write"`
	NetworkRead  int64   `json:"network_read"`
	NetworkWrite int64   `json:"network_write"`
}

// TaskExecutor 任务执行器接口
type TaskExecutor interface {
	// 生命周期管理
	Initialize() error
	Start(ctx context.Context) error
	Stop() error
	
	// 任务执行
	Execute(ctx context.Context, execution *TaskExecution) (*ExecutionResult, error)
	
	// 状态查询
	GetStatus() ExecutorStatus
	GetSupportedTypes() []ExecutorType
	
	// 配置管理
	UpdateConfig(config map[string]interface{}) error
}

// TaskExecutorManager 任务执行器管理器接口
type TaskExecutorManager interface {
	// 执行器管理
	RegisterExecutor(executorType ExecutorType, executor TaskExecutor) error
	UnregisterExecutor(executorType ExecutorType) error
	GetExecutor(executorType ExecutorType) (TaskExecutor, error)
	
	// 任务执行
	ExecuteTask(ctx context.Context, execution *TaskExecution) (*ExecutionResult, error)
	
	// 生命周期管理
	Start(ctx context.Context) error
	Stop() error
	
	// 状态查询
	GetStats() *ExecutorManagerStats
}

// ExecutorStatus 执行器状态
type ExecutorStatus string

const (
	ExecutorStatusStopped    ExecutorStatus = "stopped"
	ExecutorStatusStarting   ExecutorStatus = "starting"
	ExecutorStatusRunning    ExecutorStatus = "running"
	ExecutorStatusStopping   ExecutorStatus = "stopping"
	ExecutorStatusError      ExecutorStatus = "error"
)

// ExecutorManagerStats 执行器管理器统计
type ExecutorManagerStats struct {
	RegisteredExecutors int                        `json:"registered_executors"`
	ActiveExecutions    int                        `json:"active_executions"`
	TotalExecutions     int64                      `json:"total_executions"`
	SuccessfulExecutions int64                     `json:"successful_executions"`
	FailedExecutions    int64                      `json:"failed_executions"`
	ExecutorStats       map[ExecutorType]*ExecutorStats `json:"executor_stats"`
}

// ExecutorStats 执行器统计
type ExecutorStats struct {
	Type             ExecutorType      `json:"type"`
	Status           ExecutorStatus    `json:"status"`
	ActiveExecutions int               `json:"active_executions"`
	TotalExecutions  int64             `json:"total_executions"`
	SuccessExecutions int64            `json:"success_executions"`
	FailedExecutions int64             `json:"failed_executions"`
	LastExecutedAt   *time.Time        `json:"last_executed_at"`
	AverageExecutionTime time.Duration `json:"average_execution_time"`
}

// ExecutorConfig 执行器配置
type ExecutorConfig struct {
	Type            ExecutorType           `json:"type"`
	MaxConcurrency  int                    `json:"max_concurrency"`
	DefaultTimeout  time.Duration          `json:"default_timeout"`
	WorkingDir      string                 `json:"working_dir"`
	Environment     map[string]string      `json:"environment"`
	Settings        map[string]interface{} `json:"settings"`
}

// ExecutionOptions 执行选项
type ExecutionOptions struct {
	Timeout         time.Duration     `json:"timeout"`
	WorkingDir      string            `json:"working_dir"`
	Environment     map[string]string `json:"environment"`
	CaptureOutput   bool              `json:"capture_output"`
	CaptureMetrics  bool              `json:"capture_metrics"`
	FailFast        bool              `json:"fail_fast"`
}

// ExecutionCallback 执行回调
type ExecutionCallback interface {
	OnStarted(execution *TaskExecution)
	OnProgress(execution *TaskExecution, progress float64, message string)
	OnCompleted(execution *TaskExecution, result *ExecutionResult)
	OnFailed(execution *TaskExecution, err error)
}

// AsyncExecution 异步执行接口
type AsyncExecution interface {
	// 状态查询
	GetID() string
	GetStatus() ExecutionStatus
	GetProgress() float64
	
	// 结果获取
	Wait(ctx context.Context) (*ExecutionResult, error)
	GetResult() (*ExecutionResult, error)
	
	// 控制操作
	Cancel() error
	
	// 事件通知
	SetCallback(callback ExecutionCallback)
}