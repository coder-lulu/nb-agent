package service

import (
	"context"
	"log/slog"

	"github.com/coder-lulu/nb-agent/internal/agent/connection"
	"github.com/coder-lulu/nb-agent/internal/biz"
)

// TaskIntegrationService 任务集成服务
// 将远程任务系统与现有的NewBee Agent集成
type TaskIntegrationService struct {
	// 现有服务
	taskService       *TaskService
	remoteTaskService *RemoteTaskService
	
	// 新的远程任务组件
	dispatcher        *TaskDispatcher
	agentConnection   *connection.AgentConnection
	
	// 存储
	taskRepo          biz.TaskRepo
	remoteTaskRepo    biz.RemoteTaskRepo
	
	logger            *slog.Logger
}

// NewTaskIntegrationService 创建任务集成服务
func NewTaskIntegrationService(
	taskService *TaskService,
	taskRepo biz.TaskRepo,
	remoteTaskRepo biz.RemoteTaskRepo,
	logger *slog.Logger) *TaskIntegrationService {

	if logger == nil {
		logger = slog.Default()
	}

	// 创建任务分发器
	dispatcher := NewTaskDispatcher(logger)

	// 创建远程任务服务
	remoteTaskService := NewRemoteTaskService(remoteTaskRepo, dispatcher)

	// 设置分发器回调
	dispatcher.SetTaskService(remoteTaskService)

	return &TaskIntegrationService{
		taskService:       taskService,
		remoteTaskService: remoteTaskService,
		dispatcher:        dispatcher,
		taskRepo:          taskRepo,
		remoteTaskRepo:    remoteTaskRepo,
		logger:            logger.With("component", "task_integration"),
	}
}

// InitializeAgentConnection 初始化Agent连接
func (tis *TaskIntegrationService) InitializeAgentConnection(proxyEndpoints []string, agentID, agentName string) error {
	tis.logger.Info("initializing agent connection for remote tasks")

	// 创建Agent连接
	tis.agentConnection = connection.NewAgentConnection(proxyEndpoints, agentID, agentName, tis.logger)

	// 设置连接管理器到分发器
	tis.dispatcher.SetConnectionManager(tis.agentConnection)

	// 设置任务处理器
	tis.agentConnection.SetTaskHandler(tis.handleIncomingTask)

	tis.logger.Info("agent connection initialized")
	return nil
}

// Start 启动任务集成服务
func (tis *TaskIntegrationService) Start(ctx context.Context) error {
	tis.logger.Info("starting task integration service")

	// 启动Agent连接（如果已配置）
	if tis.agentConnection != nil {
		if err := tis.agentConnection.Start(ctx); err != nil {
			return err
		}
	}

	// 清理僵尸任务
	go tis.cleanupTasks()

	tis.logger.Info("task integration service started")
	return nil
}

// Stop 停止任务集成服务
func (tis *TaskIntegrationService) Stop() error {
	tis.logger.Info("stopping task integration service")

	// 停止Agent连接
	if tis.agentConnection != nil {
		if err := tis.agentConnection.Stop(); err != nil {
			tis.logger.Error("failed to stop agent connection", "error", err)
		}
	}

	tis.logger.Info("task integration service stopped")
	return nil
}

// handleIncomingTask 处理来自Proxy的任务
func (tis *TaskIntegrationService) handleIncomingTask(req *connection.TaskRequest) (*connection.TaskResponse, error) {
	tis.logger.Info("handling incoming task", 
		"task_id", req.ID,
		"type", req.Type,
		"command", req.Command)

	// 根据任务类型决定处理方式
	switch req.Type {
	case "local", "shell":
		return tis.handleLocalTask(req)
	case "remote", "proxy":
		return tis.handleRemoteTask(req)
	default:
		return tis.handleGenericTask(req)
	}
}

// handleLocalTask 处理本地任务
func (tis *TaskIntegrationService) handleLocalTask(req *connection.TaskRequest) (*connection.TaskResponse, error) {
	// 创建传统的本地任务
	localTask := &biz.Task{
		Name:   req.Command,
		Shell:  req.Command,
		Status: biz.TaskStatusWaiting,
	}

	// 提交到现有的任务队列
	if err := tis.taskRepo.Push(localTask); err != nil {
		return &connection.TaskResponse{
			ID:      req.ID,
			Success: false,
			Error:   err.Error(),
		}, err
	}

	return &connection.TaskResponse{
		ID:      req.ID,
		Success: true,
		Result:  map[string]interface{}{"message": "Local task queued", "task_id": localTask.ID},
	}, nil
}

// handleRemoteTask 处理远程任务
func (tis *TaskIntegrationService) handleRemoteTask(req *connection.TaskRequest) (*connection.TaskResponse, error) {
	// 创建远程任务记录
	remoteTask := &biz.RemoteTask{
		TaskID:     req.ID,
		Name:       req.Command,
		Type:       biz.TaskTypeRemote,
		Status:     biz.TaskStatusWaiting,
		Source:     biz.RemoteTaskSourceProxy,
		Command:    req.Command,
		WorkingDir: req.WorkingDir,
		Timeout:    req.Timeout,
	}

	// 设置参数
	if err := remoteTask.SetParameters(req.Parameters); err != nil {
		return &connection.TaskResponse{
			ID:      req.ID,
			Success: false,
			Error:   err.Error(),
		}, err
	}

	// 保存远程任务
	if err := tis.remoteTaskRepo.Create(remoteTask); err != nil {
		return &connection.TaskResponse{
			ID:      req.ID,
			Success: false,
			Error:   err.Error(),
		}, err
	}

	// 分发任务（这里可能是向其他Agent转发）
	if err := tis.dispatcher.DispatchTask(remoteTask); err != nil {
		return &connection.TaskResponse{
			ID:      req.ID,
			Success: false,
			Error:   err.Error(),
		}, err
	}

	return &connection.TaskResponse{
		ID:      req.ID,
		Success: true,
		Result:  map[string]interface{}{"message": "Remote task dispatched", "task_id": remoteTask.TaskID},
	}, nil
}

// handleGenericTask 处理通用任务
func (tis *TaskIntegrationService) handleGenericTask(req *connection.TaskRequest) (*connection.TaskResponse, error) {
	// 默认作为本地任务处理
	return tis.handleLocalTask(req)
}

// cleanupTasks 清理任务
func (tis *TaskIntegrationService) cleanupTasks() {
	tis.logger.Info("starting task cleanup routine")

	// 清理本地僵尸任务
	if err := tis.taskRepo.ClearZombieTasks(); err != nil {
		tis.logger.Error("failed to clear zombie tasks", "error", err)
	}

	// 清理远程僵尸任务
	if err := tis.remoteTaskRepo.ClearZombieTasks(); err != nil {
		tis.logger.Error("failed to clear zombie remote tasks", "error", err)
	}

	tis.logger.Info("task cleanup completed")
}

// GetTaskService 获取原始任务服务
func (tis *TaskIntegrationService) GetTaskService() *TaskService {
	return tis.taskService
}

// GetRemoteTaskService 获取远程任务服务
func (tis *TaskIntegrationService) GetRemoteTaskService() *RemoteTaskService {
	return tis.remoteTaskService
}

// GetDispatcher 获取任务分发器
func (tis *TaskIntegrationService) GetDispatcher() *TaskDispatcher {
	return tis.dispatcher
}

// GetAgentConnection 获取Agent连接
func (tis *TaskIntegrationService) GetAgentConnection() *connection.AgentConnection {
	return tis.agentConnection
}

// IsConnected 检查是否已连接到Proxy
func (tis *TaskIntegrationService) IsConnected() bool {
	if tis.agentConnection == nil {
		return false
	}
	return tis.agentConnection.IsConnected()
}

// GetConnectionStatus 获取连接状态
func (tis *TaskIntegrationService) GetConnectionStatus() connection.ConnectionStatus {
	if tis.agentConnection == nil {
		return connection.StatusDisconnected
	}
	return tis.agentConnection.GetConnectionStatus()
}

// SendTaskUpdate 发送任务更新到Proxy
func (tis *TaskIntegrationService) SendTaskUpdate(taskID string, status biz.TaskStatus, result interface{}, err error) error {
	if tis.agentConnection == nil {
		return nil // 没有连接，跳过
	}

	return tis.agentConnection.SendTaskResult(taskID, result, err)
}