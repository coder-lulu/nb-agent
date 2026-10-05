package executor

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/coder-lulu/nb-agent/internal/agent/connection"
	"github.com/coder-lulu/nb-agent/internal/biz"
)

// TaskExecutorService 任务执行器服务
// 集成各种执行器并提供统一的任务执行接口
type TaskExecutorService struct {
	manager TaskExecutorManager
	logger  *slog.Logger
	
	// 回调接口
	taskCallback func(taskID string, result *ExecutionResult, err error)
}

// NewTaskExecutorService 创建任务执行器服务
func NewTaskExecutorService(logger *slog.Logger) *TaskExecutorService {
	if logger == nil {
		logger = slog.Default()
	}

	service := &TaskExecutorService{
		manager: NewTaskExecutorManager(logger),
		logger:  logger.With("component", "executor_service"),
	}

	// 注册默认执行器
	service.registerDefaultExecutors()

	return service
}

// Start 启动执行器服务
func (tes *TaskExecutorService) Start(ctx context.Context) error {
	tes.logger.Info("starting task executor service")
	return tes.manager.Start(ctx)
}

// Stop 停止执行器服务
func (tes *TaskExecutorService) Stop() error {
	tes.logger.Info("stopping task executor service")
	return tes.manager.Stop()
}

// SetTaskCallback 设置任务完成回调
func (tes *TaskExecutorService) SetTaskCallback(callback func(taskID string, result *ExecutionResult, err error)) {
	tes.taskCallback = callback
}

// ExecuteRemoteTask 执行远程任务
func (tes *TaskExecutorService) ExecuteRemoteTask(ctx context.Context, task *biz.RemoteTask) (*ExecutionResult, error) {
	tes.logger.Info("executing remote task",
		"task_id", task.TaskID,
		"type", task.Type,
		"command", task.Command)

	// 构建任务执行对象
	execution, err := tes.buildTaskExecution(task)
	if err != nil {
		return nil, fmt.Errorf("failed to build task execution: %w", err)
	}

	// 执行任务
	result, err := tes.manager.ExecuteTask(ctx, execution)

	// 调用回调
	if tes.taskCallback != nil {
		tes.taskCallback(task.TaskID, result, err)
	}

	return result, err
}

// ExecuteConnectionTask 执行连接任务（来自Proxy的任务）
func (tes *TaskExecutorService) ExecuteConnectionTask(ctx context.Context, req *connection.TaskRequest) (*connection.TaskResponse, error) {
	tes.logger.Info("executing connection task",
		"task_id", req.ID,
		"type", req.Type,
		"command", req.Command)

	startTime := time.Now()

	// 将连接任务转换为执行对象
	execution := tes.buildExecutionFromRequest(req)

	// 执行任务
	result, err := tes.manager.ExecuteTask(ctx, execution)

	duration := time.Since(startTime)

	// 构建响应
	response := &connection.TaskResponse{
		ID:       req.ID,
		Duration: duration.Milliseconds(),
	}

	if err != nil {
		response.Success = false
		response.Error = err.Error()
		tes.logger.Error("connection task execution failed",
			"task_id", req.ID,
			"error", err)
	} else if result != nil {
		response.Success = result.Success
		response.Result = result.Data
		if result.Error != "" {
			response.Error = result.Error
		}
		tes.logger.Info("connection task execution completed",
			"task_id", req.ID,
			"success", result.Success)
	}

	return response, nil
}

// GetExecutorStats 获取执行器统计信息
func (tes *TaskExecutorService) GetExecutorStats() *ExecutorManagerStats {
	return tes.manager.GetStats()
}

// registerDefaultExecutors 注册默认执行器
func (tes *TaskExecutorService) registerDefaultExecutors() {
	tes.logger.Info("registering default executors")

	// 注册Shell执行器
	shellConfig := &ExecutorConfig{
		Type:           ExecutorTypeShell,
		MaxConcurrency: 5,
		DefaultTimeout: 5 * time.Minute,
	}
	shellExecutor := NewShellExecutor(shellConfig, tes.logger)
	if err := tes.manager.RegisterExecutor(ExecutorTypeShell, shellExecutor); err != nil {
		tes.logger.Error("failed to register shell executor", "error", err)
	}

	// 注册脚本执行器
	scriptConfig := &ExecutorConfig{
		Type:           ExecutorTypeScript,
		MaxConcurrency: 3,
		DefaultTimeout: 10 * time.Minute,
	}
	scriptExecutor := NewScriptExecutor(scriptConfig, tes.logger)
	if err := tes.manager.RegisterExecutor(ExecutorTypeScript, scriptExecutor); err != nil {
		tes.logger.Error("failed to register script executor", "error", err)
	}

	// 注册文件执行器
	fileConfig := &ExecutorConfig{
		Type:           ExecutorTypeFile,
		MaxConcurrency: 5,
		DefaultTimeout: 5 * time.Minute,
		Settings: map[string]interface{}{
			"allowed_dirs": []string{"/tmp", "/var/tmp", "/opt/data"},
		},
	}
	fileExecutor := NewFileExecutor(fileConfig, tes.logger)
	if err := tes.manager.RegisterExecutor(ExecutorTypeFile, fileExecutor); err != nil {
		tes.logger.Error("failed to register file executor", "error", err)
	}

	tes.logger.Info("default executors registered successfully")
}

// buildTaskExecution 从远程任务构建执行对象
func (tes *TaskExecutorService) buildTaskExecution(task *biz.RemoteTask) (*TaskExecution, error) {
	// 获取任务参数
	params, err := task.GetParameters()
	if err != nil {
		return nil, fmt.Errorf("failed to get task parameters: %w", err)
	}

	// 确定执行器类型
	executorType := tes.determineExecutorType(task, params)

	// 设置超时
	timeout := time.Duration(task.Timeout) * time.Second
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	execution := &TaskExecution{
		ID:          task.TaskID,
		Type:        executorType,
		Command:     task.Command,
		Parameters:  params,
		WorkingDir:  task.WorkingDir,
		Environment: make(map[string]string),
		Timeout:     timeout,
		Status:      StatusPending,
	}

	return execution, nil
}

// buildExecutionFromRequest 从连接请求构建执行对象
func (tes *TaskExecutorService) buildExecutionFromRequest(req *connection.TaskRequest) *TaskExecution {
	// 确定执行器类型
	executorType := ExecutorTypeShell // 默认使用Shell执行器
	
	if taskType, ok := req.Parameters["executor_type"].(string); ok {
		executorType = ExecutorType(taskType)
	} else {
		// 根据请求类型推断执行器类型
		switch req.Type {
		case "shell", "command":
			executorType = ExecutorTypeShell
		case "script":
			executorType = ExecutorTypeScript
		case "file":
			executorType = ExecutorTypeFile
		}
	}

	// 设置超时
	timeout := time.Duration(req.Timeout) * time.Second
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	execution := &TaskExecution{
		ID:          req.ID,
		Type:        executorType,
		Command:     req.Command,
		Parameters:  req.Parameters,
		WorkingDir:  req.WorkingDir,
		Environment: make(map[string]string),
		Timeout:     timeout,
		Status:      StatusPending,
	}

	return execution
}

// determineExecutorType 确定执行器类型
func (tes *TaskExecutorService) determineExecutorType(task *biz.RemoteTask, params map[string]interface{}) ExecutorType {
	// 检查参数中是否指定了执行器类型
	if executorType, ok := params["executor_type"].(string); ok {
		return ExecutorType(executorType)
	}

	// 检查是否是脚本任务
	if _, ok := params["script_type"]; ok {
		return ExecutorTypeScript
	}

	// 检查是否是文件操作任务
	if operation, ok := params["operation"].(string); ok {
		fileOps := []string{"copy", "move", "delete", "create", "read", "write", "list", "stat", "mkdir"}
		for _, op := range fileOps {
			if operation == op {
				return ExecutorTypeFile
			}
		}
	}

	// 默认使用Shell执行器
	return ExecutorTypeShell
}

// RegisterCustomExecutor 注册自定义执行器
func (tes *TaskExecutorService) RegisterCustomExecutor(executorType ExecutorType, executor TaskExecutor) error {
	return tes.manager.RegisterExecutor(executorType, executor)
}

// UnregisterExecutor 注销执行器
func (tes *TaskExecutorService) UnregisterExecutor(executorType ExecutorType) error {
	return tes.manager.UnregisterExecutor(executorType)
}

// GetAvailableExecutors 获取可用执行器列表
func (tes *TaskExecutorService) GetAvailableExecutors() []ExecutorType {
	stats := tes.manager.GetStats()
	executors := make([]ExecutorType, 0, len(stats.ExecutorStats))
	
	for executorType := range stats.ExecutorStats {
		executors = append(executors, executorType)
	}
	
	return executors
}

// IsExecutorAvailable 检查执行器是否可用
func (tes *TaskExecutorService) IsExecutorAvailable(executorType ExecutorType) bool {
	_, err := tes.manager.GetExecutor(executorType)
	return err == nil
}

// TaskExecutorIntegration 任务执行器集成接口
// 提供与其他服务集成的标准接口
type TaskExecutorIntegration interface {
	// 生命周期管理
	Start(ctx context.Context) error
	Stop() error
	
	// 任务执行
	ExecuteRemoteTask(ctx context.Context, task *biz.RemoteTask) (*ExecutionResult, error)
	ExecuteConnectionTask(ctx context.Context, req *connection.TaskRequest) (*connection.TaskResponse, error)
	
	// 回调设置
	SetTaskCallback(callback func(taskID string, result *ExecutionResult, err error))
	
	// 执行器管理
	RegisterCustomExecutor(executorType ExecutorType, executor TaskExecutor) error
	UnregisterExecutor(executorType ExecutorType) error
	GetAvailableExecutors() []ExecutorType
	IsExecutorAvailable(executorType ExecutorType) bool
	
	// 统计信息
	GetExecutorStats() *ExecutorManagerStats
}

// 确保TaskExecutorService实现了TaskExecutorIntegration接口
var _ TaskExecutorIntegration = (*TaskExecutorService)(nil)