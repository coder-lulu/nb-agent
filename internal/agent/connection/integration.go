package connection

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"time"
)

// AgentConnection Agent连接集成器
type AgentConnection struct {
	connMgr        ConnectionManager
	messageHandler *AgentMessageHandler
	logger         *slog.Logger
	
	// 配置
	config    *ConnectionConfig
	regConfig *RegistrationConfig
	
	// 回调函数
	taskHandler func(*TaskRequest) (*TaskResponse, error)
}

// NewAgentConnection 创建Agent连接集成器
func NewAgentConnection(proxyEndpoints []string, agentID, agentName string, logger *slog.Logger) *AgentConnection {
	if logger == nil {
		logger = slog.Default()
	}

	// 创建默认配置
	config := DefaultConnectionConfig()
	config.ProxyEndpoints = proxyEndpoints

	// 创建注册配置
	regConfig := &RegistrationConfig{
		AgentID:      agentID,
		AgentName:    agentName,
		Tags:         make(map[string]string),
		Capabilities: []string{"command", "file", "monitor", "ssh"},
		Metadata:     createAgentMetadata(),
	}

	// 创建连接管理器
	connMgr := NewConnectionManager(config, regConfig, logger)

	// 创建消息处理器
	messageHandler := NewAgentMessageHandler(connMgr, logger)

	agent := &AgentConnection{
		connMgr:        connMgr,
		messageHandler: messageHandler,
		logger:         logger.With("component", "agent_connection"),
		config:         config,
		regConfig:      regConfig,
	}

	// 设置消息处理器
	connMgr.SetMessageHandler(messageHandler)
	
	// 设置任务回调
	messageHandler.SetTaskCallback(agent.handleTask)

	return agent
}

// Start 启动Agent连接
func (ac *AgentConnection) Start(ctx context.Context) error {
	ac.logger.Info("starting agent connection")
	
	if err := ac.connMgr.Start(ctx); err != nil {
		return fmt.Errorf("failed to start connection manager: %w", err)
	}

	ac.logger.Info("agent connection started successfully")
	return nil
}

// Stop 停止Agent连接
func (ac *AgentConnection) Stop() error {
	ac.logger.Info("stopping agent connection")
	
	if err := ac.connMgr.Stop(); err != nil {
		return fmt.Errorf("failed to stop connection manager: %w", err)
	}

	ac.logger.Info("agent connection stopped")
	return nil
}

// SetTaskHandler 设置任务处理器
func (ac *AgentConnection) SetTaskHandler(handler func(*TaskRequest) (*TaskResponse, error)) {
	ac.taskHandler = handler
}

// SendMessage 发送消息到Proxy
func (ac *AgentConnection) SendMessage(msgType MessageType, payload interface{}) error {
	msg := &Message{
		Type:      msgType,
		Timestamp: time.Now(),
		Payload:   payload,
	}
	
	return ac.connMgr.SendMessage(msg)
}

// SendTaskResult 发送任务结果
func (ac *AgentConnection) SendTaskResult(taskID string, result interface{}, err error) error {
	response := &TaskResponse{
		ID:      taskID,
		Success: err == nil,
		Result:  result,
	}
	
	if err != nil {
		response.Error = err.Error()
	}
	
	return ac.SendMessage(MsgTypeResult, response)
}

// SendStatus 发送状态信息
func (ac *AgentConnection) SendStatus(status interface{}) error {
	return ac.SendMessage(MsgTypeStatus, status)
}

// GetConnectionStatus 获取连接状态
func (ac *AgentConnection) GetConnectionStatus() ConnectionStatus {
	return ac.connMgr.GetStatus()
}

// GetConnectionStats 获取连接统计
func (ac *AgentConnection) GetConnectionStats() *ConnectionStats {
	return ac.connMgr.GetStats()
}

// IsConnected 检查是否已连接
func (ac *AgentConnection) IsConnected() bool {
	return ac.connMgr.GetStatus() == StatusConnected
}

// UpdateConfig 更新连接配置
func (ac *AgentConnection) UpdateConfig(config *ConnectionConfig) error {
	ac.config = config
	return ac.connMgr.UpdateConfig(config)
}

// AddTags 添加Agent标签
func (ac *AgentConnection) AddTags(tags map[string]string) {
	for k, v := range tags {
		ac.regConfig.Tags[k] = v
	}
}

// AddCapabilities 添加Agent能力
func (ac *AgentConnection) AddCapabilities(capabilities ...string) {
	ac.regConfig.Capabilities = append(ac.regConfig.Capabilities, capabilities...)
}

// handleTask 处理任务请求
func (ac *AgentConnection) handleTask(req *TaskRequest) (*TaskResponse, error) {
	start := time.Now()
	
	ac.logger.Info("handling task", 
		"task_id", req.ID, 
		"task_type", req.Type,
		"command", req.Command)

	// 如果设置了自定义任务处理器，使用自定义处理器
	if ac.taskHandler != nil {
		resp, err := ac.taskHandler(req)
		if resp != nil {
			resp.Duration = time.Since(start).Milliseconds()
		}
		return resp, err
	}

	// 默认任务处理逻辑
	result, err := ac.executeDefaultTask(req)
	
	response := &TaskResponse{
		ID:       req.ID,
		Success:  err == nil,
		Result:   result,
		Duration: time.Since(start).Milliseconds(),
	}
	
	if err != nil {
		response.Error = err.Error()
		ac.logger.Error("task execution failed",
			"task_id", req.ID,
			"error", err)
	} else {
		ac.logger.Info("task execution completed",
			"task_id", req.ID,
			"duration", response.Duration)
	}
	
	return response, err
}

// executeDefaultTask 执行默认任务
func (ac *AgentConnection) executeDefaultTask(req *TaskRequest) (interface{}, error) {
	switch req.Type {
	case "ping":
		return map[string]interface{}{
			"pong": time.Now().Unix(),
			"agent_id": ac.regConfig.AgentID,
		}, nil
		
	case "info":
		return map[string]interface{}{
			"agent_id":   ac.regConfig.AgentID,
			"agent_name": ac.regConfig.AgentName,
			"os":         runtime.GOOS,
			"arch":       runtime.GOARCH,
			"hostname":   ac.regConfig.Metadata.Hostname,
			"status":     ac.GetConnectionStatus().String(),
		}, nil
		
	default:
		return nil, fmt.Errorf("unsupported task type: %s", req.Type)
	}
}

// createAgentMetadata 创建Agent元数据
func createAgentMetadata() AgentMetadata {
	hostname, _ := os.Hostname()
	
	return AgentMetadata{
		OS:           runtime.GOOS,
		Arch:         runtime.GOARCH,
		Version:      "1.0.0", // TODO: 从版本配置中获取
		Hostname:     hostname,
		CPUCores:     runtime.NumCPU(),
		Tags:         make(map[string]string),
		Capabilities: []string{"command", "file", "monitor", "ssh"},
	}
}