package executor

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// executorManager 任务执行器管理器实现
type executorManager struct {
	// 执行器注册表
	executors map[ExecutorType]TaskExecutor
	
	// 状态管理
	status ExecutorStatus
	statusMu sync.RWMutex
	
	// 执行状态跟踪
	activeExecutions map[string]*TaskExecution
	executionsMu     sync.RWMutex
	
	// 统计信息
	stats   *ExecutorManagerStats
	statsMu sync.RWMutex
	
	// 配置
	configs map[ExecutorType]*ExecutorConfig
	
	// 上下文管理
	ctx    context.Context
	cancel context.CancelFunc
	
	// 日志
	logger *slog.Logger
}

// NewTaskExecutorManager 创建任务执行器管理器
func NewTaskExecutorManager(logger *slog.Logger) TaskExecutorManager {
	if logger == nil {
		logger = slog.Default()
	}

	return &executorManager{
		executors:        make(map[ExecutorType]TaskExecutor),
		activeExecutions: make(map[string]*TaskExecution),
		configs:          make(map[ExecutorType]*ExecutorConfig),
		status:           ExecutorStatusStopped,
		stats: &ExecutorManagerStats{
			ExecutorStats: make(map[ExecutorType]*ExecutorStats),
		},
		logger: logger.With("component", "executor_manager"),
	}
}

// RegisterExecutor 注册执行器
func (em *executorManager) RegisterExecutor(executorType ExecutorType, executor TaskExecutor) error {
	em.statusMu.Lock()
	defer em.statusMu.Unlock()

	if _, exists := em.executors[executorType]; exists {
		return fmt.Errorf("executor type %s already registered", executorType)
	}

	em.executors[executorType] = executor
	
	// 初始化执行器统计
	em.statsMu.Lock()
	em.stats.ExecutorStats[executorType] = &ExecutorStats{
		Type:   executorType,
		Status: ExecutorStatusStopped,
	}
	em.stats.RegisteredExecutors++
	em.statsMu.Unlock()

	em.logger.Info("executor registered", "type", executorType)
	return nil
}

// UnregisterExecutor 注销执行器
func (em *executorManager) UnregisterExecutor(executorType ExecutorType) error {
	em.statusMu.Lock()
	defer em.statusMu.Unlock()

	executor, exists := em.executors[executorType]
	if !exists {
		return fmt.Errorf("executor type %s not found", executorType)
	}

	// 停止执行器
	if err := executor.Stop(); err != nil {
		em.logger.Warn("failed to stop executor during unregistration", "type", executorType, "error", err)
	}

	delete(em.executors, executorType)
	
	// 更新统计
	em.statsMu.Lock()
	delete(em.stats.ExecutorStats, executorType)
	em.stats.RegisteredExecutors--
	em.statsMu.Unlock()

	em.logger.Info("executor unregistered", "type", executorType)
	return nil
}

// GetExecutor 获取执行器
func (em *executorManager) GetExecutor(executorType ExecutorType) (TaskExecutor, error) {
	em.statusMu.RLock()
	defer em.statusMu.RUnlock()

	executor, exists := em.executors[executorType]
	if !exists {
		return nil, fmt.Errorf("executor type %s not found", executorType)
	}

	return executor, nil
}

// ExecuteTask 执行任务
func (em *executorManager) ExecuteTask(ctx context.Context, execution *TaskExecution) (*ExecutionResult, error) {
	em.logger.Info("executing task", 
		"execution_id", execution.ID,
		"type", execution.Type,
		"command", execution.Command)

	// 获取对应的执行器
	executor, err := em.GetExecutor(execution.Type)
	if err != nil {
		return nil, fmt.Errorf("failed to get executor: %w", err)
	}

	// 生成执行ID
	if execution.ID == "" {
		execution.ID = uuid.New().String()
	}

	// 设置默认超时
	if execution.Timeout == 0 {
		execution.Timeout = 5 * time.Minute
	}

	// 记录执行开始
	em.recordExecutionStart(execution)

	// 创建执行上下文
	execCtx, cancel := context.WithTimeout(ctx, execution.Timeout)
	defer cancel()

	// 执行任务
	startTime := time.Now()
	result, err := executor.Execute(execCtx, execution)
	duration := time.Since(startTime)

	// 记录执行完成
	em.recordExecutionComplete(execution, result, err, duration)

	if err != nil {
		em.logger.Error("task execution failed",
			"execution_id", execution.ID,
			"type", execution.Type,
			"duration", duration,
			"error", err)
		return result, err
	}

	em.logger.Info("task execution completed",
		"execution_id", execution.ID,
		"type", execution.Type,
		"duration", duration,
		"success", result != nil && result.Success)

	return result, nil
}

// Start 启动执行器管理器
func (em *executorManager) Start(ctx context.Context) error {
	em.statusMu.Lock()
	defer em.statusMu.Unlock()

	if em.status == ExecutorStatusRunning {
		return nil
	}

	em.logger.Info("starting executor manager")
	em.status = ExecutorStatusStarting

	// 设置上下文
	em.ctx, em.cancel = context.WithCancel(ctx)

	// 启动所有注册的执行器
	for executorType, executor := range em.executors {
		if err := executor.Start(em.ctx); err != nil {
			em.logger.Error("failed to start executor", "type", executorType, "error", err)
			continue
		}

		// 更新执行器状态
		em.statsMu.Lock()
		if stats, exists := em.stats.ExecutorStats[executorType]; exists {
			stats.Status = ExecutorStatusRunning
		}
		em.statsMu.Unlock()

		em.logger.Info("executor started", "type", executorType)
	}

	em.status = ExecutorStatusRunning
	em.logger.Info("executor manager started")
	return nil
}

// Stop 停止执行器管理器
func (em *executorManager) Stop() error {
	em.statusMu.Lock()
	defer em.statusMu.Unlock()

	if em.status == ExecutorStatusStopped {
		return nil
	}

	em.logger.Info("stopping executor manager")
	em.status = ExecutorStatusStopping

	// 取消上下文
	if em.cancel != nil {
		em.cancel()
	}

	// 停止所有执行器
	for executorType, executor := range em.executors {
		if err := executor.Stop(); err != nil {
			em.logger.Error("failed to stop executor", "type", executorType, "error", err)
		} else {
			em.logger.Info("executor stopped", "type", executorType)
		}

		// 更新执行器状态
		em.statsMu.Lock()
		if stats, exists := em.stats.ExecutorStats[executorType]; exists {
			stats.Status = ExecutorStatusStopped
		}
		em.statsMu.Unlock()
	}

	em.status = ExecutorStatusStopped
	em.logger.Info("executor manager stopped")
	return nil
}

// GetStats 获取统计信息
func (em *executorManager) GetStats() *ExecutorManagerStats {
	em.statsMu.RLock()
	defer em.statsMu.RUnlock()

	// 深拷贝统计信息
	stats := &ExecutorManagerStats{
		RegisteredExecutors:  em.stats.RegisteredExecutors,
		ActiveExecutions:     len(em.activeExecutions),
		TotalExecutions:      em.stats.TotalExecutions,
		SuccessfulExecutions: em.stats.SuccessfulExecutions,
		FailedExecutions:     em.stats.FailedExecutions,
		ExecutorStats:        make(map[ExecutorType]*ExecutorStats),
	}

	// 复制执行器统计
	for k, v := range em.stats.ExecutorStats {
		statsCopy := *v
		stats.ExecutorStats[k] = &statsCopy
	}

	return stats
}

// recordExecutionStart 记录执行开始
func (em *executorManager) recordExecutionStart(execution *TaskExecution) {
	now := time.Now()
	execution.StartedAt = &now
	execution.Status = StatusRunning

	// 记录活跃执行
	em.executionsMu.Lock()
	em.activeExecutions[execution.ID] = execution
	em.executionsMu.Unlock()

	// 更新统计
	em.statsMu.Lock()
	em.stats.TotalExecutions++
	if stats, exists := em.stats.ExecutorStats[execution.Type]; exists {
		stats.ActiveExecutions++
		stats.TotalExecutions++
	}
	em.statsMu.Unlock()
}

// recordExecutionComplete 记录执行完成
func (em *executorManager) recordExecutionComplete(execution *TaskExecution, result *ExecutionResult, err error, duration time.Duration) {
	now := time.Now()
	execution.CompletedAt = &now
	execution.Result = result

	if err != nil {
		execution.Status = StatusFailed
		execution.Error = err.Error()
	} else {
		execution.Status = StatusCompleted
	}

	// 移除活跃执行
	em.executionsMu.Lock()
	delete(em.activeExecutions, execution.ID)
	em.executionsMu.Unlock()

	// 更新统计
	em.statsMu.Lock()
	if err != nil {
		em.stats.FailedExecutions++
	} else {
		em.stats.SuccessfulExecutions++
	}

	if stats, exists := em.stats.ExecutorStats[execution.Type]; exists {
		stats.ActiveExecutions--
		stats.LastExecutedAt = &now
		
		if err != nil {
			stats.FailedExecutions++
		} else {
			stats.SuccessExecutions++
		}

		// 更新平均执行时间
		if stats.TotalExecutions > 0 {
			totalTime := stats.AverageExecutionTime * time.Duration(stats.TotalExecutions-1)
			stats.AverageExecutionTime = (totalTime + duration) / time.Duration(stats.TotalExecutions)
		}
	}
	em.statsMu.Unlock()
}