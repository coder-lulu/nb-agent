package connection

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// proxyConnection Proxy连接实现
type proxyConnection struct {
	// 配置
	config *ConnectionConfig
	logger *slog.Logger
	
	// 连接信息
	currentEndpoint string
	wsConn         *websocket.Conn
	httpClient     *http.Client
	
	// 状态管理
	status   ConnectionStatus
	statusMu sync.RWMutex
	
	// 消息通道
	sendChan chan *Message
	recvChan chan *Message
	
	// 上下文管理
	ctx    context.Context
	cancel context.CancelFunc
	
	// 同步原语
	connMu sync.Mutex
	wg     sync.WaitGroup
	
	// 统计信息
	stats   *ConnectionStats
	statsMu sync.Mutex
}

// NewProxyConnection 创建Proxy连接
func NewProxyConnection(config *ConnectionConfig, logger *slog.Logger) Connection {
	if logger == nil {
		logger = slog.Default()
	}
	
	// 创建HTTP客户端
	httpClient := &http.Client{
		Timeout: config.ConnectTimeout,
	}
	
	// 配置TLS
	if config.TLSEnabled {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: config.TLSSkipVerify,
		}
		
		if config.TLSCertFile != "" && config.TLSKeyFile != "" {
			cert, err := tls.LoadX509KeyPair(config.TLSCertFile, config.TLSKeyFile)
			if err != nil {
				logger.Error("failed to load TLS certificates", "error", err)
			} else {
				tlsConfig.Certificates = []tls.Certificate{cert}
			}
		}
		
		transport := &http.Transport{
			TLSClientConfig: tlsConfig,
		}
		httpClient.Transport = transport
	}
	
	conn := &proxyConnection{
		config:     config,
		logger:     logger.With("component", "proxy_connection"),
		httpClient: httpClient,
		status:     StatusDisconnected,
		sendChan:   make(chan *Message, config.MessageBufferSize),
		recvChan:   make(chan *Message, config.MessageBufferSize),
		stats:      &ConnectionStats{},
	}
	
	return conn
}

// Connect 建立连接
func (pc *proxyConnection) Connect(ctx context.Context) error {
	pc.connMu.Lock()
	defer pc.connMu.Unlock()
	
	if pc.status == StatusConnected {
		return nil
	}
	
	pc.setStatus(StatusConnecting)
	pc.ctx, pc.cancel = context.WithCancel(ctx)
	
	// 尝试连接到所有Proxy端点
	var lastErr error
	for _, endpoint := range pc.config.ProxyEndpoints {
		if err := pc.connectToEndpoint(endpoint); err != nil {
			pc.logger.Warn("failed to connect to endpoint", "endpoint", endpoint, "error", err)
			lastErr = err
			continue
		}
		
		pc.currentEndpoint = endpoint
		pc.setStatus(StatusConnected)
		
		// 启动消息处理协程
		pc.wg.Add(2)
		go pc.sendLoop()
		go pc.receiveLoop()
		
		pc.logger.Info("connected to proxy", "endpoint", endpoint)
		return nil
	}
	
	pc.setStatus(StatusError)
	return fmt.Errorf("failed to connect to any proxy endpoint, last error: %w", lastErr)
}

// Disconnect 断开连接
func (pc *proxyConnection) Disconnect() error {
	pc.connMu.Lock()
	defer pc.connMu.Unlock()
	
	if pc.status == StatusDisconnected {
		return nil
	}
	
	pc.logger.Info("disconnecting from proxy")
	pc.setStatus(StatusDisconnected)
	
	// 取消上下文
	if pc.cancel != nil {
		pc.cancel()
	}
	
	// 关闭WebSocket连接
	if pc.wsConn != nil {
		pc.wsConn.Close()
		pc.wsConn = nil
	}
	
	// 等待协程结束
	pc.wg.Wait()
	
	pc.logger.Info("disconnected from proxy")
	return nil
}

// IsConnected 检查是否已连接
func (pc *proxyConnection) IsConnected() bool {
	return pc.GetStatus() == StatusConnected
}

// GetStatus 获取连接状态
func (pc *proxyConnection) GetStatus() ConnectionStatus {
	pc.statusMu.RLock()
	defer pc.statusMu.RUnlock()
	return pc.status
}

// SendMessage 发送消息
func (pc *proxyConnection) SendMessage(msg *Message) error {
	if !pc.IsConnected() {
		return fmt.Errorf("not connected to proxy")
	}
	
	select {
	case pc.sendChan <- msg:
		return nil
	case <-pc.ctx.Done():
		return pc.ctx.Err()
	default:
		return fmt.Errorf("send buffer full")
	}
}

// ReceiveMessage 接收消息
func (pc *proxyConnection) ReceiveMessage() <-chan *Message {
	return pc.recvChan
}

// Register 注册Agent
func (pc *proxyConnection) Register(config *RegistrationConfig) error {
	if !pc.IsConnected() {
		return fmt.Errorf("not connected to proxy")
	}
	
	// 创建注册消息
	regMsg := &Message{
		Type:      MsgTypeRegister,
		Timestamp: time.Now(),
		Payload:   config,
	}
	
	// 发送注册消息
	if err := pc.SendMessage(regMsg); err != nil {
		return fmt.Errorf("failed to send registration message: %w", err)
	}
	
	// TODO: 等待注册响应
	// 这里应该等待Proxy的注册确认消息
	
	pc.logger.Info("agent registered successfully", "agent_id", config.AgentID)
	return nil
}

// StartHeartbeat 启动心跳
func (pc *proxyConnection) StartHeartbeat() error {
	// 心跳由HeartbeatManager管理
	return nil
}

// StopHeartbeat 停止心跳
func (pc *proxyConnection) StopHeartbeat() error {
	// 心跳由HeartbeatManager管理
	return nil
}

// connectToEndpoint 连接到指定端点
func (pc *proxyConnection) connectToEndpoint(endpoint string) error {
	// 解析端点URL
	u, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("invalid endpoint URL: %w", err)
	}
	
	// 根据协议选择连接方式
	switch u.Scheme {
	case "ws", "wss":
		return pc.connectWebSocket(endpoint)
	case "http", "https":
		return pc.connectHTTP(endpoint)
	default:
		return fmt.Errorf("unsupported protocol: %s", u.Scheme)
	}
}

// connectWebSocket 建立WebSocket连接
func (pc *proxyConnection) connectWebSocket(endpoint string) error {
	// 创建WebSocket拨号器
	dialer := &websocket.Dialer{
		HandshakeTimeout: pc.config.ConnectTimeout,
		ReadBufferSize:   4096,
		WriteBufferSize:  4096,
	}
	
	// 配置TLS
	if pc.config.TLSEnabled {
		dialer.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: pc.config.TLSSkipVerify,
		}
	}
	
	// 建立连接
	headers := http.Header{}
	headers.Set("User-Agent", "UltraThink-Agent/1.0")
	
	conn, resp, err := dialer.Dial(endpoint, headers)
	if err != nil {
		if resp != nil {
			return fmt.Errorf("websocket dial failed with status %d: %w", resp.StatusCode, err)
		}
		return fmt.Errorf("websocket dial failed: %w", err)
	}
	
	// 设置连接选项
	conn.SetReadDeadline(time.Now().Add(pc.config.ReadTimeout))
	conn.SetWriteDeadline(time.Now().Add(pc.config.WriteTimeout))
	
	pc.wsConn = conn
	return nil
}

// connectHTTP 建立HTTP连接（长轮询）
func (pc *proxyConnection) connectHTTP(endpoint string) error {
	// HTTP连接主要用于长轮询
	// 这里只是验证端点可达性
	resp, err := pc.httpClient.Get(endpoint + "/health")
	if err != nil {
		return fmt.Errorf("http connection failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("proxy health check failed with status: %d", resp.StatusCode)
	}
	
	return nil
}

// sendLoop 发送循环
func (pc *proxyConnection) sendLoop() {
	defer pc.wg.Done()
	
	for {
		select {
		case msg := <-pc.sendChan:
			if err := pc.writeMessage(msg); err != nil {
				pc.logger.Error("failed to write message", "error", err)
				pc.setStatus(StatusError)
				return
			}
			
		case <-pc.ctx.Done():
			return
		}
	}
}

// receiveLoop 接收循环
func (pc *proxyConnection) receiveLoop() {
	defer pc.wg.Done()
	
	for {
		select {
		case <-pc.ctx.Done():
			return
		default:
			msg, err := pc.readMessage()
			if err != nil {
				pc.logger.Error("failed to read message", "error", err)
				pc.setStatus(StatusError)
				return
			}
			
			if msg != nil {
				select {
				case pc.recvChan <- msg:
				case <-pc.ctx.Done():
					return
				default:
					pc.logger.Warn("receive buffer full, dropping message")
				}
			}
		}
	}
}

// writeMessage 写入消息
func (pc *proxyConnection) writeMessage(msg *Message) error {
	if pc.wsConn == nil {
		return fmt.Errorf("websocket connection not established")
	}
	
	// 序列化消息
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}
	
	// 设置写入超时
	pc.wsConn.SetWriteDeadline(time.Now().Add(pc.config.WriteTimeout))
	
	// 发送消息
	if err := pc.wsConn.WriteMessage(websocket.TextMessage, data); err != nil {
		return fmt.Errorf("failed to write websocket message: %w", err)
	}
	
	return nil
}

// readMessage 读取消息
func (pc *proxyConnection) readMessage() (*Message, error) {
	if pc.wsConn == nil {
		return nil, fmt.Errorf("websocket connection not established")
	}
	
	// 设置读取超时
	pc.wsConn.SetReadDeadline(time.Now().Add(pc.config.ReadTimeout))
	
	// 读取消息
	_, data, err := pc.wsConn.ReadMessage()
	if err != nil {
		return nil, fmt.Errorf("failed to read websocket message: %w", err)
	}
	
	// 反序列化消息
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}
	
	return &msg, nil
}

// setStatus 设置连接状态
func (pc *proxyConnection) setStatus(status ConnectionStatus) {
	pc.statusMu.Lock()
	defer pc.statusMu.Unlock()
	
	if pc.status != status {
		pc.logger.Debug("connection status changed",
			"old_status", pc.status.String(),
			"new_status", status.String())
		pc.status = status
		
		// 更新统计信息
		pc.updateStats(status)
	}
}

// updateStats 更新统计信息
func (pc *proxyConnection) updateStats(status ConnectionStatus) {
	pc.statsMu.Lock()
	defer pc.statsMu.Unlock()
	
	pc.stats.Status = status
	
	if status == StatusConnected {
		now := time.Now()
		pc.stats.ConnectedAt = &now
	}
}