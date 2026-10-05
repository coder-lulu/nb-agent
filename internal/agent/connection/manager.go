package connection

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// connectionManager 连接管理器实现
type connectionManager struct {
	// 配置
	config *ConnectionConfig
	regConfig *RegistrationConfig
	
	// 连接
	connection Connection
	
	// 组件
	heartbeatMgr  *HeartbeatManager
	reconnectMgr  *ReconnectManager
	messageHandler MessageHandler
	
	// 状态管理
	status   ConnectionStatus
	statusMu sync.RWMutex
	
	// 上下文管理
	ctx    context.Context
	cancel context.CancelFunc
	
	// 日志
	logger *slog.Logger
	
	// 统计
	stats   *ConnectionStats
	statsMu sync.RWMutex
	
	// 消息通道
	incomingMsgs chan *Message
	outgoingMsgs chan *Message
	
	// 工作组
	wg sync.WaitGroup
}

// NewConnectionManager 创建连接管理器
func NewConnectionManager(config *ConnectionConfig, regConfig *RegistrationConfig, logger *slog.Logger) ConnectionManager {
	if config == nil {
		config = DefaultConnectionConfig()
	}
	
	if logger == nil {
		logger = slog.Default()
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	mgr := &connectionManager{
		config:    config,
		regConfig: regConfig,
		logger:    logger.With("component", "connection_manager"),
		ctx:       ctx,
		cancel:    cancel,
		status:    StatusDisconnected,
		stats:     &ConnectionStats{},
		incomingMsgs: make(chan *Message, config.MessageBufferSize),
		outgoingMsgs: make(chan *Message, config.MessageBufferSize),
	}
	
	// 创建连接
	mgr.connection = NewProxyConnection(config, logger)
	
	// 创建心跳管理器
	mgr.heartbeatMgr = NewHeartbeatManager(mgr.connection, config.HeartbeatInterval, config.HeartbeatTimeout)
	
	// 创建重连管理器
	mgr.reconnectMgr = NewReconnectManager(mgr.connection, config)
	
	return mgr
}

// Start 启动连接管理器
func (cm *connectionManager) Start(ctx context.Context) error {
	cm.logger.Info("starting connection manager")
	
	// 设置上下文
	cm.ctx, cm.cancel = context.WithCancel(ctx)
	
	// 启动消息处理协程
	cm.wg.Add(1)
	go cm.processIncomingMessages()
	
	cm.wg.Add(1)
	go cm.processOutgoingMessages()
	
	// 启动连接
	if err := cm.Connect(); err != nil {
		cm.logger.Error("failed to start connection", "error", err)
		return fmt.Errorf("failed to start connection: %w", err)
	}
	
	cm.logger.Info("connection manager started successfully")
	return nil
}

// Stop 停止连接管理器
func (cm *connectionManager) Stop() error {
	cm.logger.Info("stopping connection manager")
	
	// 停止心跳
	if cm.heartbeatMgr != nil {
		cm.heartbeatMgr.Stop()
	}
	
	// 停止重连
	if cm.reconnectMgr != nil {
		cm.reconnectMgr.Stop()
	}
	
	// 断开连接
	if cm.connection != nil {
		cm.connection.Disconnect()
	}
	
	// 取消上下文
	if cm.cancel != nil {
		cm.cancel()
	}
	
	// 等待所有协程结束
	cm.wg.Wait()
	
	// 关闭通道
	close(cm.incomingMsgs)
	close(cm.outgoingMsgs)
	
	cm.setStatus(StatusDisconnected)
	cm.logger.Info("connection manager stopped")
	return nil
}

// Connect 建立连接
func (cm *connectionManager) Connect() error {
	cm.logger.Info("connecting to proxy")
	cm.setStatus(StatusConnecting)
	
	// 连接到Proxy
	if err := cm.connection.Connect(cm.ctx); err != nil {
		cm.setStatus(StatusError)
		cm.updateStats(func(stats *ConnectionStats) {
			stats.ErrorCount++
			stats.LastError = err.Error()
		})
		return fmt.Errorf("failed to connect to proxy: %w", err)
	}
	
	// 注册Agent
	if err := cm.connection.Register(cm.regConfig); err != nil {
		cm.setStatus(StatusError)
		cm.connection.Disconnect()
		cm.updateStats(func(stats *ConnectionStats) {
			stats.ErrorCount++
			stats.LastError = err.Error()
		})
		return fmt.Errorf("failed to register agent: %w", err)
	}
	
	// 启动心跳
	if err := cm.heartbeatMgr.Start(); err != nil {
		cm.logger.Warn("failed to start heartbeat", "error", err)
	}
	
	// 启动消息接收
	cm.wg.Add(1)
	go cm.receiveMessages()
	
	cm.setStatus(StatusConnected)
	now := time.Now()
	cm.updateStats(func(stats *ConnectionStats) {
		stats.ConnectedAt = &now
	})
	
	cm.logger.Info("connected to proxy successfully")
	return nil
}

// Disconnect 断开连接
func (cm *connectionManager) Disconnect() error {
	cm.logger.Info("disconnecting from proxy")
	
	cm.setStatus(StatusDisconnected)
	
	// 停止心跳
	if cm.heartbeatMgr != nil {
		cm.heartbeatMgr.Stop()
	}
	
	// 断开连接
	if cm.connection != nil {
		cm.connection.Disconnect()
	}
	
	cm.logger.Info("disconnected from proxy")
	return nil
}

// Reconnect 重新连接
func (cm *connectionManager) Reconnect() error {
	cm.logger.Info("reconnecting to proxy")
	cm.setStatus(StatusReconnecting)
	
	// 先断开当前连接
	cm.Disconnect()
	
	// 等待一段时间再重连
	time.Sleep(cm.config.RetryInterval)
	
	// 重新连接
	if err := cm.Connect(); err != nil {
		cm.updateStats(func(stats *ConnectionStats) {
			stats.ReconnectCount++
			stats.ErrorCount++
			stats.LastError = err.Error()
		})
		return err
	}
	
	cm.updateStats(func(stats *ConnectionStats) {
		stats.ReconnectCount++
	})
	
	cm.logger.Info("reconnected to proxy successfully")
	return nil
}

// SendMessage 发送消息
func (cm *connectionManager) SendMessage(msg *Message) error {
	if cm.GetStatus() != StatusConnected {
		return fmt.Errorf("connection not established")
	}
	
	// 设置消息ID和时间戳
	if msg.ID == "" {
		msg.ID = uuid.New().String()
	}
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}
	
	select {
	case cm.outgoingMsgs <- msg:
		return nil
	case <-cm.ctx.Done():
		return cm.ctx.Err()
	default:
		return fmt.Errorf("outgoing message buffer full")
	}
}

// SetMessageHandler 设置消息处理器
func (cm *connectionManager) SetMessageHandler(handler MessageHandler) {
	cm.messageHandler = handler
}

// GetStatus 获取连接状态
func (cm *connectionManager) GetStatus() ConnectionStatus {
	cm.statusMu.RLock()
	defer cm.statusMu.RUnlock()
	return cm.status
}

// GetStats 获取连接统计
func (cm *connectionManager) GetStats() *ConnectionStats {
	cm.statsMu.RLock()
	defer cm.statsMu.RUnlock()
	
	// 深拷贝统计信息
	stats := *cm.stats
	return &stats
}

// UpdateConfig 更新配置
func (cm *connectionManager) UpdateConfig(config *ConnectionConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}
	
	cm.config = config
	
	// 如果已连接，需要重新连接以应用新配置
	if cm.GetStatus() == StatusConnected {
		return cm.Reconnect()
	}
	
	return nil
}

// setStatus 设置连接状态
func (cm *connectionManager) setStatus(status ConnectionStatus) {
	cm.statusMu.Lock()
	defer cm.statusMu.Unlock()
	
	if cm.status != status {
		oldStatus := cm.status
		cm.status = status
		cm.logger.Debug("connection status changed", 
			"old_status", oldStatus.String(), 
			"new_status", status.String())
	}
}

// updateStats 更新统计信息
func (cm *connectionManager) updateStats(fn func(*ConnectionStats)) {
	cm.statsMu.Lock()
	defer cm.statsMu.Unlock()
	fn(cm.stats)
}

// processIncomingMessages 处理传入消息
func (cm *connectionManager) processIncomingMessages() {
	defer cm.wg.Done()
	
	cm.logger.Debug("started processing incoming messages")
	
	for {
		select {
		case msg := <-cm.incomingMsgs:
			if msg == nil {
				continue
			}
			
			cm.updateStats(func(stats *ConnectionStats) {
				stats.MessagesReceived++
				stats.MessagesRecvTotal++
			})
			
			if cm.messageHandler != nil {
				if err := cm.handleMessage(msg); err != nil {
					cm.logger.Error("failed to handle message", 
						"message_id", msg.ID,
						"message_type", msg.Type,
						"error", err)
				}
			}
			
		case <-cm.ctx.Done():
			cm.logger.Debug("stopped processing incoming messages")
			return
		}
	}
}

// processOutgoingMessages 处理传出消息
func (cm *connectionManager) processOutgoingMessages() {
	defer cm.wg.Done()
	
	cm.logger.Debug("started processing outgoing messages")
	
	for {
		select {
		case msg := <-cm.outgoingMsgs:
			if msg == nil {
				continue
			}
			
			if err := cm.connection.SendMessage(msg); err != nil {
				cm.logger.Error("failed to send message",
					"message_id", msg.ID,
					"message_type", msg.Type,
					"error", err)
				
				cm.updateStats(func(stats *ConnectionStats) {
					stats.ErrorCount++
					stats.LastError = err.Error()
				})
				continue
			}
			
			cm.updateStats(func(stats *ConnectionStats) {
				stats.MessagesSent++
				stats.MessagesSentTotal++
			})
			
		case <-cm.ctx.Done():
			cm.logger.Debug("stopped processing outgoing messages")
			return
		}
	}
}

// receiveMessages 接收消息
func (cm *connectionManager) receiveMessages() {
	defer cm.wg.Done()
	
	cm.logger.Debug("started receiving messages")
	
	msgChan := cm.connection.ReceiveMessage()
	
	for {
		select {
		case msg := <-msgChan:
			if msg == nil {
				continue
			}
			
			select {
			case cm.incomingMsgs <- msg:
			case <-cm.ctx.Done():
				return
			default:
				cm.logger.Warn("incoming message buffer full, dropping message",
					"message_id", msg.ID,
					"message_type", msg.Type)
			}
			
		case <-cm.ctx.Done():
			cm.logger.Debug("stopped receiving messages")
			return
		}
	}
}

// handleMessage 处理消息
func (cm *connectionManager) handleMessage(msg *Message) error {
	switch msg.Type {
	case MsgTypeTask:
		return cm.messageHandler.HandleTaskMessage(msg)
	case MsgTypeConfig:
		return cm.messageHandler.HandleConfigMessage(msg)
	case MsgTypeControl:
		return cm.messageHandler.HandleControlMessage(msg)
	case MsgTypeHeartbeat:
		return cm.messageHandler.HandleHeartbeatMessage(msg)
	default:
		cm.logger.Warn("unknown message type", "type", msg.Type, "id", msg.ID)
		return fmt.Errorf("unknown message type: %s", msg.Type)
	}
}