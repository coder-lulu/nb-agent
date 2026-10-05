package connection

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ConnectionStatus 连接状态
type ConnectionStatus int

const (
	StatusDisconnected ConnectionStatus = iota
	StatusConnecting
	StatusConnected
	StatusReconnecting
	StatusError
)

func (s ConnectionStatus) String() string {
	switch s {
	case StatusDisconnected:
		return "disconnected"
	case StatusConnecting:
		return "connecting"
	case StatusConnected:
		return "connected"
	case StatusReconnecting:
		return "reconnecting"
	case StatusError:
		return "error"
	default:
		return "unknown"
	}
}

// MessageType 消息类型
type MessageType string

const (
	MsgTypeTask      MessageType = "task"      // 任务消息
	MsgTypeConfig    MessageType = "config"    // 配置消息
	MsgTypeControl   MessageType = "control"   // 控制消息
	MsgTypeResult    MessageType = "result"    // 结果消息
	MsgTypeStatus    MessageType = "status"    // 状态消息
	MsgTypeHeartbeat MessageType = "heartbeat" // 心跳消息
	MsgTypeRegister  MessageType = "register"  // 注册消息
)

// Message 消息结构
type Message struct {
	ID        string                 `json:"id"`
	Type      MessageType            `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Payload   interface{}            `json:"payload"`
	Headers   map[string]string      `json:"headers"`
	ReplyTo   string                 `json:"reply_to,omitempty"`
}

// AgentMetadata Agent元数据
type AgentMetadata struct {
	OS            string            `json:"os"`
	Arch          string            `json:"arch"`
	Version       string            `json:"version"`
	IP            string            `json:"ip"`
	Hostname      string            `json:"hostname"`
	KernelVersion string            `json:"kernel_version"`
	CPUCores      int               `json:"cpu_cores"`
	Memory        int64             `json:"memory"`
	Disk          int64             `json:"disk"`
	Tags          map[string]string `json:"tags"`
	Capabilities  []string          `json:"capabilities"`
}

// RegistrationConfig 注册配置
type RegistrationConfig struct {
	AgentID      string            `json:"agent_id"`
	AgentName    string            `json:"agent_name"`
	Tags         map[string]string `json:"tags"`
	Capabilities []string          `json:"capabilities"`
	Metadata     AgentMetadata     `json:"metadata"`
}

// ConnectionConfig 连接配置
type ConnectionConfig struct {
	// Proxy连接配置
	ProxyEndpoints []string `json:"proxy_endpoints"`
	
	// 超时配置
	ConnectTimeout    time.Duration `json:"connect_timeout"`
	ReadTimeout       time.Duration `json:"read_timeout"`
	WriteTimeout      time.Duration `json:"write_timeout"`
	
	// 重连配置
	RetryCount        int           `json:"retry_count"`
	RetryInterval     time.Duration `json:"retry_interval"`
	MaxRetryInterval  time.Duration `json:"max_retry_interval"`
	
	// 心跳配置
	HeartbeatInterval time.Duration `json:"heartbeat_interval"`
	HeartbeatTimeout  time.Duration `json:"heartbeat_timeout"`
	
	// 缓冲区配置
	MessageBufferSize int `json:"message_buffer_size"`
	ResultBufferSize  int `json:"result_buffer_size"`
	
	// TLS配置
	TLSEnabled    bool   `json:"tls_enabled"`
	TLSCertFile   string `json:"tls_cert_file"`
	TLSKeyFile    string `json:"tls_key_file"`
	TLSCACertFile string `json:"tls_ca_cert_file"`
	TLSSkipVerify bool   `json:"tls_skip_verify"`
}

// DefaultConnectionConfig 默认连接配置
func DefaultConnectionConfig() *ConnectionConfig {
	return &ConnectionConfig{
		ConnectTimeout:    30 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      30 * time.Second,
		RetryCount:        5,
		RetryInterval:     5 * time.Second,
		MaxRetryInterval:  5 * time.Minute,
		HeartbeatInterval: 30 * time.Second,
		HeartbeatTimeout:  10 * time.Second,
		MessageBufferSize: 1000,
		ResultBufferSize:  1000,
		TLSEnabled:        false,
		TLSSkipVerify:     false,
	}
}

// Connection 连接接口
type Connection interface {
	// 连接管理
	Connect(ctx context.Context) error
	Disconnect() error
	IsConnected() bool
	GetStatus() ConnectionStatus
	
	// 消息处理
	SendMessage(msg *Message) error
	ReceiveMessage() <-chan *Message
	
	// 注册和认证
	Register(config *RegistrationConfig) error
	
	// 心跳管理
	StartHeartbeat() error
	StopHeartbeat() error
}

// MessageHandler 消息处理器接口
type MessageHandler interface {
	// 处理任务消息
	HandleTaskMessage(msg *Message) error
	
	// 处理配置消息
	HandleConfigMessage(msg *Message) error
	
	// 处理控制消息
	HandleControlMessage(msg *Message) error
	
	// 处理心跳消息
	HandleHeartbeatMessage(msg *Message) error
}

// ConnectionManager 连接管理器接口
type ConnectionManager interface {
	// 生命周期管理
	Start(ctx context.Context) error
	Stop() error
	
	// 连接管理
	Connect() error
	Disconnect() error
	Reconnect() error
	
	// 消息处理
	SendMessage(msg *Message) error
	SetMessageHandler(handler MessageHandler)
	
	// 状态查询
	GetStatus() ConnectionStatus
	GetStats() *ConnectionStats
	
	// 配置管理
	UpdateConfig(config *ConnectionConfig) error
}

// ConnectionStats 连接统计
type ConnectionStats struct {
	Status            ConnectionStatus `json:"status"`
	ConnectedAt       *time.Time       `json:"connected_at"`
	LastHeartbeat     *time.Time       `json:"last_heartbeat"`
	ReconnectCount    int64            `json:"reconnect_count"`
	MessagesSent      int64            `json:"messages_sent"`
	MessagesReceived  int64            `json:"messages_received"`
	MessagesSentTotal int64            `json:"messages_sent_total"`
	MessagesRecvTotal int64            `json:"messages_recv_total"`
	ErrorCount        int64            `json:"error_count"`
	LastError         string           `json:"last_error"`
}

// ProxyConnection Proxy连接封装
type ProxyConnection struct {
	// 连接信息
	endpoint   string
	wsConn     *websocket.Conn
	httpClient *http.Client
	
	// 状态管理
	status     ConnectionStatus
	statusMu   sync.RWMutex
	
	// 消息通道
	sendChan   chan *Message
	recvChan   chan *Message
	
	// 上下文管理
	ctx        context.Context
	cancel     context.CancelFunc
	
	// 配置
	config     *ConnectionConfig
	
	// 统计信息
	stats      *ConnectionStats
	statsMu    sync.RWMutex
}

// HeartbeatManager 心跳管理器
type HeartbeatManager struct {
	conn       Connection
	interval   time.Duration
	timeout    time.Duration
	ticker     *time.Ticker
	ctx        context.Context
	cancel     context.CancelFunc
	mu         sync.Mutex
	running    bool
}

// ReconnectManager 重连管理器
type ReconnectManager struct {
	conn           Connection
	config         *ConnectionConfig
	retryCount     int
	maxRetries     int
	retryInterval  time.Duration
	maxInterval    time.Duration
	ctx            context.Context
	cancel         context.CancelFunc
	mu             sync.Mutex
	reconnecting   bool
	reconnectChan  chan struct{}
}