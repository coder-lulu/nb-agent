package connection

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// AgentMessageHandler Agent消息处理器实现
type AgentMessageHandler struct {
	logger       *slog.Logger
	connMgr      ConnectionManager
	taskCallback func(task *TaskRequest) (*TaskResponse, error)
}

// TaskRequest 任务请求
type TaskRequest struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Command     string                 `json:"command"`
	Parameters  map[string]interface{} `json:"parameters"`
	Timeout     int                    `json:"timeout"`
	WorkingDir  string                 `json:"working_dir"`
}

// TaskResponse 任务响应
type TaskResponse struct {
	ID       string      `json:"id"`
	Success  bool        `json:"success"`
	Result   interface{} `json:"result"`
	Error    string      `json:"error,omitempty"`
	Duration int64       `json:"duration"`
}

// ConfigUpdate 配置更新
type ConfigUpdate struct {
	Type   string      `json:"type"`
	Config interface{} `json:"config"`
}

// ControlCommand 控制命令
type ControlCommand struct {
	Action     string                 `json:"action"`
	Parameters map[string]interface{} `json:"parameters"`
}

// NewAgentMessageHandler 创建Agent消息处理器
func NewAgentMessageHandler(connMgr ConnectionManager, logger *slog.Logger) *AgentMessageHandler {
	if logger == nil {
		logger = slog.Default()
	}

	return &AgentMessageHandler{
		logger:  logger.With("component", "message_handler"),
		connMgr: connMgr,
	}
}

// SetTaskCallback 设置任务回调函数
func (h *AgentMessageHandler) SetTaskCallback(callback func(task *TaskRequest) (*TaskResponse, error)) {
	h.taskCallback = callback
}

// HandleTaskMessage 处理任务消息
func (h *AgentMessageHandler) HandleTaskMessage(msg *Message) error {
	h.logger.Info("handling task message", "message_id", msg.ID)

	// 解析任务请求
	var taskReq TaskRequest
	if err := h.parsePayload(msg.Payload, &taskReq); err != nil {
		return fmt.Errorf("failed to parse task request: %w", err)
	}

	// 验证任务请求
	if taskReq.ID == "" {
		taskReq.ID = msg.ID
	}

	// 执行任务
	var taskResp *TaskResponse
	var err error

	if h.taskCallback != nil {
		taskResp, err = h.taskCallback(&taskReq)
	} else {
		err = fmt.Errorf("no task callback registered")
	}

	// 构建响应
	if taskResp == nil {
		taskResp = &TaskResponse{
			ID:      taskReq.ID,
			Success: false,
		}
	}

	if err != nil {
		taskResp.Success = false
		taskResp.Error = err.Error()
	}

	// 发送响应
	return h.sendResponse(msg.ReplyTo, MsgTypeResult, taskResp)
}

// HandleConfigMessage 处理配置消息
func (h *AgentMessageHandler) HandleConfigMessage(msg *Message) error {
	h.logger.Info("handling config message", "message_id", msg.ID)

	// 解析配置更新
	var configUpdate ConfigUpdate
	if err := h.parsePayload(msg.Payload, &configUpdate); err != nil {
		return fmt.Errorf("failed to parse config update: %w", err)
	}

	// 处理不同类型的配置更新
	switch configUpdate.Type {
	case "connection":
		return h.handleConnectionConfigUpdate(configUpdate.Config)
	case "logging":
		return h.handleLoggingConfigUpdate(configUpdate.Config)
	default:
		h.logger.Warn("unknown config type", "type", configUpdate.Type)
		return fmt.Errorf("unknown config type: %s", configUpdate.Type)
	}
}

// HandleControlMessage 处理控制消息
func (h *AgentMessageHandler) HandleControlMessage(msg *Message) error {
	h.logger.Info("handling control message", "message_id", msg.ID)

	// 解析控制命令
	var controlCmd ControlCommand
	if err := h.parsePayload(msg.Payload, &controlCmd); err != nil {
		return fmt.Errorf("failed to parse control command: %w", err)
	}

	// 处理控制命令
	switch controlCmd.Action {
	case "ping":
		return h.handlePingCommand(msg.ReplyTo)
	case "status":
		return h.handleStatusCommand(msg.ReplyTo)
	case "reconnect":
		return h.handleReconnectCommand()
	case "shutdown":
		return h.handleShutdownCommand()
	default:
		h.logger.Warn("unknown control action", "action", controlCmd.Action)
		return fmt.Errorf("unknown control action: %s", controlCmd.Action)
	}
}

// HandleHeartbeatMessage 处理心跳消息
func (h *AgentMessageHandler) HandleHeartbeatMessage(msg *Message) error {
	h.logger.Debug("handling heartbeat message", "message_id", msg.ID)

	// 心跳消息通常不需要特殊处理，只需要确认收到
	if msg.ReplyTo != "" {
		pongData := map[string]interface{}{
			"pong": time.Now().Unix(),
		}
		return h.sendResponse(msg.ReplyTo, MsgTypeHeartbeat, pongData)
	}

	return nil
}

// parsePayload 解析消息载荷
func (h *AgentMessageHandler) parsePayload(payload interface{}, target interface{}) error {
	// 将payload转换为JSON然后解析到目标结构
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	return nil
}

// sendResponse 发送响应消息
func (h *AgentMessageHandler) sendResponse(replyTo string, msgType MessageType, payload interface{}) error {
	if replyTo == "" {
		return nil // 无需响应
	}

	responseMsg := &Message{
		ID:        uuid.New().String(),
		Type:      msgType,
		Timestamp: time.Now(),
		Payload:   payload,
		ReplyTo:   replyTo,
	}

	return h.connMgr.SendMessage(responseMsg)
}

// handleConnectionConfigUpdate 处理连接配置更新
func (h *AgentMessageHandler) handleConnectionConfigUpdate(config interface{}) error {
	// 解析连接配置
	var connConfig ConnectionConfig
	if err := h.parsePayload(config, &connConfig); err != nil {
		return fmt.Errorf("failed to parse connection config: %w", err)
	}

	// 更新连接配置
	return h.connMgr.UpdateConfig(&connConfig)
}

// handleLoggingConfigUpdate 处理日志配置更新
func (h *AgentMessageHandler) handleLoggingConfigUpdate(config interface{}) error {
	h.logger.Info("logging config update received", "config", config)
	// TODO: 实现日志配置更新逻辑
	return nil
}

// handlePingCommand 处理ping命令
func (h *AgentMessageHandler) handlePingCommand(replyTo string) error {
	pongData := map[string]interface{}{
		"pong": time.Now().Unix(),
		"status": "ok",
	}
	return h.sendResponse(replyTo, MsgTypeControl, pongData)
}

// handleStatusCommand 处理状态查询命令
func (h *AgentMessageHandler) handleStatusCommand(replyTo string) error {
	stats := h.connMgr.GetStats()
	statusData := map[string]interface{}{
		"status":         h.connMgr.GetStatus().String(),
		"connected_at":   stats.ConnectedAt,
		"last_heartbeat": stats.LastHeartbeat,
		"messages_sent":  stats.MessagesSent,
		"messages_recv":  stats.MessagesReceived,
		"error_count":    stats.ErrorCount,
		"last_error":     stats.LastError,
	}
	return h.sendResponse(replyTo, MsgTypeStatus, statusData)
}

// handleReconnectCommand 处理重连命令
func (h *AgentMessageHandler) handleReconnectCommand() error {
	h.logger.Info("reconnect command received")
	return h.connMgr.Reconnect()
}

// handleShutdownCommand 处理关闭命令
func (h *AgentMessageHandler) handleShutdownCommand() error {
	h.logger.Info("shutdown command received")
	// TODO: 实现优雅关闭逻辑
	return h.connMgr.Stop()
}