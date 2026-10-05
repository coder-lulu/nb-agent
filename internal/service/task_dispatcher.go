package service

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/coder-lulu/nb-agent/internal/agent/connection"
	"github.com/coder-lulu/nb-agent/internal/biz"
)

// TaskDispatcher 任务分发器实现
type TaskDispatcher struct {
	connMgr   connection.ConnectionManager
	logger    *slog.Logger
	
	// 统计信息
	stats     *biz.TaskDispatchStats
	statsMu   sync.RWMutex
	
	// 任务回调
	taskService *RemoteTaskService
}

// NewTaskDispatcher 创建任务分发器
func NewTaskDispatcher(logger *slog.Logger) *TaskDispatcher {
	if logger == nil {
		logger = slog.Default()
	}

	return &TaskDispatcher{
		logger: logger.With("component", "task_dispatcher"),
		stats: &biz.TaskDispatchStats{},
	}
}

// SetConnectionManager 设置连接管理器
func (td *TaskDispatcher) SetConnectionManager(connMgr interface{}) {
	if cm, ok := connMgr.(connection.ConnectionManager); ok {
		td.connMgr = cm
	}
}

// SetTaskService 设置任务服务（用于回调）
func (td *TaskDispatcher) SetTaskService(taskService *RemoteTaskService) {
	td.taskService = taskService
}

// DispatchTask 分发任务
func (td *TaskDispatcher) DispatchTask(task *biz.RemoteTask) error {
	if td.connMgr == nil {
		return fmt.Errorf("connection manager not configured")
	}

	// 检查连接状态
	if td.connMgr.GetStatus() != connection.StatusConnected {
		return fmt.Errorf("not connected to proxy")
	}

	td.logger.Info("dispatching remote task", 
		"task_id", task.TaskID,
		"task_type", task.Type,
		"command", task.Command)

	// 构建任务请求
	taskReq, err := td.buildTaskRequest(task)
	if err != nil {
		return fmt.Errorf("failed to build task request: %w", err)
	}

	// 创建任务消息
	msg := &connection.Message{
		Type:      connection.MsgTypeTask,
		Timestamp: time.Now(),
		Payload:   taskReq,
	}

	// 发送任务
	if err := td.connMgr.SendMessage(msg); err != nil {
		td.updateStats(func(stats *biz.TaskDispatchStats) {
			stats.FailedTasks++
		})
		return fmt.Errorf("failed to send task message: %w", err)
	}

	// 更新统计信息
	td.updateStats(func(stats *biz.TaskDispatchStats) {
		stats.DispatchedTasks++
		stats.ActiveTasks++
	})

	td.logger.Info("task dispatched successfully", "task_id", task.TaskID)
	return nil
}

// OnTaskCompleted 任务完成回调
func (td *TaskDispatcher) OnTaskCompleted(taskID string, result interface{}, err error) error {
	td.logger.Info("task completed", 
		"task_id", taskID,
		"success", err == nil)

	// 更新统计信息
	td.updateStats(func(stats *biz.TaskDispatchStats) {
		stats.ActiveTasks--
		if err != nil {
			stats.FailedTasks++
		} else {
			stats.CompletedTasks++
		}
	})

	// 调用任务服务完成回调
	if td.taskService != nil {
		return td.taskService.CompleteTask(taskID, result, err)
	}

	return nil
}

// GetDispatchStats 获取分发统计
func (td *TaskDispatcher) GetDispatchStats() *biz.TaskDispatchStats {
	td.statsMu.RLock()
	defer td.statsMu.RUnlock()
	
	// 返回统计信息副本
	statsCopy := *td.stats
	return &statsCopy
}

// buildTaskRequest 构建任务请求
func (td *TaskDispatcher) buildTaskRequest(task *biz.RemoteTask) (*connection.TaskRequest, error) {
	// 获取任务参数
	params, err := task.GetParameters()
	if err != nil {
		return nil, fmt.Errorf("failed to get task parameters: %w", err)
	}

	// 构建任务请求
	taskReq := &connection.TaskRequest{
		ID:         task.TaskID,
		Type:       string(task.Type),
		Command:    task.Command,
		Parameters: params,
		Timeout:    task.Timeout,
		WorkingDir: task.WorkingDir,
	}

	return taskReq, nil
}

// updateStats 更新统计信息
func (td *TaskDispatcher) updateStats(fn func(*biz.TaskDispatchStats)) {
	td.statsMu.Lock()
	defer td.statsMu.Unlock()
	fn(td.stats)
}

// RemoteTaskMessageHandler 远程任务消息处理器
type RemoteTaskMessageHandler struct {
	*connection.AgentMessageHandler
	dispatcher  *TaskDispatcher
	taskRepo    biz.RemoteTaskRepo
	logger      *slog.Logger
}

// NewRemoteTaskMessageHandler 创建远程任务消息处理器
func NewRemoteTaskMessageHandler(
	baseHandler *connection.AgentMessageHandler,
	dispatcher *TaskDispatcher,
	taskRepo biz.RemoteTaskRepo,
	logger *slog.Logger) *RemoteTaskMessageHandler {
	
	return &RemoteTaskMessageHandler{
		AgentMessageHandler: baseHandler,
		dispatcher:         dispatcher,
		taskRepo:           taskRepo,
		logger:             logger.With("component", "remote_task_handler"),
	}
}

// HandleTaskMessage 处理远程任务消息
func (h *RemoteTaskMessageHandler) HandleTaskMessage(msg *connection.Message) error {
	h.logger.Info("handling remote task message", "message_id", msg.ID)

	// 解析任务请求
	var taskReq connection.TaskRequest
	if err := h.parsePayload(msg.Payload, &taskReq); err != nil {
		return fmt.Errorf("failed to parse task request: %w", err)
	}

	// 创建远程任务记录
	remoteTask := &biz.RemoteTask{
		TaskID:     taskReq.ID,
		Name:       fmt.Sprintf("Remote Task: %s", taskReq.Command),
		Type:       biz.TaskTypeRemote,
		Status:     biz.TaskStatusWaiting,
		Source:     biz.RemoteTaskSourceProxy,
		Command:    taskReq.Command,
		WorkingDir: taskReq.WorkingDir,
		Timeout:    taskReq.Timeout,
	}

	// 设置任务参数
	if err := remoteTask.SetParameters(taskReq.Parameters); err != nil {
		h.logger.Error("failed to set task parameters", "error", err)
		return err
	}

	// 保存任务记录
	if err := h.taskRepo.Create(remoteTask); err != nil {
		h.logger.Error("failed to create remote task", "error", err)
		return err
	}

	// 执行任务（这里可以集成到任务执行器）
	go h.executeTask(remoteTask, msg.ReplyTo)

	return nil
}

// executeTask 执行任务
func (h *RemoteTaskMessageHandler) executeTask(task *biz.RemoteTask, replyTo string) {
	startTime := time.Now()
	
	// 更新任务状态为运行中
	h.taskRepo.UpdateStatus(task.TaskID, biz.TaskStatusRunning)

	h.logger.Info("executing remote task", 
		"task_id", task.TaskID,
		"command", task.Command)

	// 模拟任务执行（这里应该集成实际的任务执行逻辑）
	var result interface{}
	var err error
	
	// TODO: 集成实际的任务执行器
	result = map[string]interface{}{
		"message": "Task executed successfully",
		"command": task.Command,
		"duration": time.Since(startTime).Milliseconds(),
	}

	// 更新任务完成状态
	if err != nil {
		h.taskRepo.UpdateStatus(task.TaskID, biz.TaskStatusFailed)
		h.taskRepo.UpdateProgress(task.TaskID, "", err.Error())
	} else {
		h.taskRepo.UpdateStatus(task.TaskID, biz.TaskStatusSuccess)
		if result != nil {
			resultStr := fmt.Sprintf("%v", result)
			h.taskRepo.UpdateProgress(task.TaskID, resultStr, "")
		}
	}

	// 通知分发器任务完成
	if h.dispatcher != nil {
		h.dispatcher.OnTaskCompleted(task.TaskID, result, err)
	}

	h.logger.Info("remote task completed", 
		"task_id", task.TaskID,
		"success", err == nil,
		"duration", time.Since(startTime))
}

// parsePayload 解析消息载荷（从基类复制）
func (h *RemoteTaskMessageHandler) parsePayload(payload interface{}, target interface{}) error {
	return h.AgentMessageHandler.(*connection.AgentMessageHandler).HandleTaskMessage(&connection.Message{Payload: payload})
}