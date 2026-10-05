# UltraThink 代理层(Proxy) 功能设计文档

## 1. 概述

代理层(Proxy)是UltraThink分布式运维平台的区域控制节点，负责管理区域内的Agent连接、任务分发、数据收集和结果上报。每个Proxy可管理500台Agent，为Center和Agent之间提供高效的中间层服务。

## 2. 系统架构

### 2.1 Proxy架构图

```
┌─────────────────────────────────────────────────────────────┐
│                    UltraThink Proxy                        │
│ ┌─────────────────────────────────────────────────────────┐ │
│ │                Center通信层                             │ │
│ │    ┌─────────────┐  ┌─────────────┐  ┌─────────────┐   │ │
│ │    │任务接收     │  │结果上报     │  │状态同步     │   │ │
│ │    └─────────────┘  └─────────────┘  └─────────────┘   │ │
│ └─────────────────────────────────────────────────────────┘ │
│                              │                             │
│ ┌─────────────────────────────┴─────────────────────────┐ │
│ │                    核心处理层                         │ │
│ │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐   │ │
│ │  │任务调度引擎 │  │连接池管理   │  │数据聚合     │   │ │
│ │  └─────────────┘  └─────────────┘  └─────────────┘   │ │
│ └─────────────────────────────────────────────────────┘ │
│                              │                             │
│ ┌─────────────────────────────┴─────────────────────────┐ │
│ │                  Agent通信层                          │ │
│ │    ┌─────────────┐  ┌─────────────┐  ┌─────────────┐   │ │
│ │    │连接管理     │  │任务分发     │  │数据收集     │   │ │
│ │    └─────────────┘  └─────────────┘  └─────────────┘   │ │
│ └─────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
                              │
          ┌───────────────────┼───────────────────┐
          │                   │                   │
    ┌─────▼─────┐      ┌─────▼─────┐      ┌─────▼─────┐
    │   Agent1  │      │   Agent2  │      │   ...     │
    │   (改造)  │      │   (改造)  │      │ Agent500  │
    └───────────┘      └───────────┘      └───────────┘
```

### 2.2 技术栈

- **框架**: go-zero微服务框架
- **数据库**: Redis 6.0 (缓存 + 任务队列)
- **消息队列**: NATS Streaming (与Center通信)
- **连接管理**: gorilla/websocket + HTTP/2
- **监控**: Prometheus + 自定义指标
- **日志**: 结构化日志 + 日志聚合

## 3. 核心功能模块

### 3.1 Agent连接池管理

#### 3.1.1 功能特性
- 高效的长连接管理
- 连接健康检查和自动重连
- 连接负载均衡
- 连接状态监控
- 优雅的连接关闭

#### 3.1.2 数据结构
```go
// 连接池管理器
type ConnectionPool struct {
    connections    sync.Map                    // agentID -> *AgentConnection
    loadBalancer   *LoadBalancer              // 负载均衡器
    healthChecker  *HealthChecker             // 健康检查器
    metrics        *ConnectionMetrics         // 连接指标
    config         *ConnectionPoolConfig      // 配置
    logger         *slog.Logger               // 日志器
}

// Agent连接
type AgentConnection struct {
    ID             string                     // AgentID
    UUID           string                     // Agent UUID
    WSConn         *websocket.Conn            // WebSocket连接
    HTTPClient     *http.Client               // HTTP客户端
    Status         ConnectionStatus           // 连接状态
    LastSeen       time.Time                  // 最后心跳时间
    MessageQueue   chan *Message              // 消息队列
    ResultQueue    chan *TaskResult           // 结果队列
    Context        context.Context            // 上下文
    Cancel         context.CancelFunc         // 取消函数
    Metadata       AgentMetadata              // Agent元数据
}

// 连接状态
type ConnectionStatus int

const (
    StatusConnecting ConnectionStatus = iota
    StatusConnected
    StatusDisconnected
    StatusError
)

// Agent元数据
type AgentMetadata struct {
    OS           string            `json:"os"`
    Arch         string            `json:"arch"`
    Version      string            `json:"version"`
    IP           string            `json:"ip"`
    Hostname     string            `json:"hostname"`
    Tags         map[string]string `json:"tags"`
    Capabilities []string          `json:"capabilities"`
}
```

#### 3.1.3 连接管理API
```go
// 连接池接口
type ConnectionPoolInterface interface {
    // 注册Agent连接
    RegisterAgent(agentID string, conn *AgentConnection) error
    
    // 移除Agent连接
    UnregisterAgent(agentID string) error
    
    // 获取Agent连接
    GetConnection(agentID string) (*AgentConnection, error)
    
    // 获取所有连接
    GetAllConnections() map[string]*AgentConnection
    
    // 检查连接状态
    IsConnected(agentID string) bool
    
    // 获取连接统计
    GetStats() *ConnectionStats
}

// 连接统计
type ConnectionStats struct {
    Total        int               `json:"total"`
    Connected    int               `json:"connected"`
    Disconnected int               `json:"disconnected"`
    ByStatus     map[string]int    `json:"by_status"`
    ByOS         map[string]int    `json:"by_os"`
}
```

### 3.2 任务分发引擎

#### 3.2.1 功能特性
- 高效的任务分发算法
- 多种分发策略(轮询/加权/一致性哈希)
- 任务优先级管理
- 并发控制和限流
- 失败重试机制

#### 3.2.2 数据结构
```go
// 任务分发器
type TaskDistributor struct {
    connectionPool  *ConnectionPool           // 连接池
    scheduler       *TaskScheduler            // 任务调度器
    strategyManager *StrategyManager          // 策略管理器
    resultCollector *ResultCollector          // 结果收集器
    metrics         *DistributorMetrics       // 分发指标
}

// 子任务(从Center接收的任务分片)
type SubTask struct {
    ID           string                     `json:"id"`
    ParentID     string                     `json:"parent_id"`     // 父任务ID
    Name         string                     `json:"name"`
    Command      TaskCommand                `json:"command"`
    Targets      []string                   `json:"targets"`       // 目标AgentID列表
    Strategy     ExecutionStrategy          `json:"strategy"`
    Priority     TaskPriority               `json:"priority"`
    Timeout      time.Duration              `json:"timeout"`
    RetryCount   int                        `json:"retry_count"`
    CreatedAt    time.Time                  `json:"created_at"`
    StartedAt    *time.Time                 `json:"started_at"`
}

// 任务命令
type TaskCommand struct {
    Type        CommandType                `json:"type"`         // 命令类型
    Action      string                     `json:"action"`       // 具体动作
    Parameters  map[string]interface{}     `json:"parameters"`   // 参数
    Script      string                     `json:"script"`       // 脚本内容
    Timeout     int                        `json:"timeout"`      // 超时时间(秒)
    WorkDir     string                     `json:"work_dir"`     // 工作目录
    Environment map[string]string          `json:"environment"`  // 环境变量
}

// 命令类型枚举
type CommandType string

const (
    CommandTypeShell      CommandType = "shell"        // Shell脚本
    CommandTypeUser       CommandType = "user"         // 用户管理
    CommandTypePackage    CommandType = "package"      // 软件包管理
    CommandTypeSystem     CommandType = "system"       // 系统配置
    CommandTypeFile       CommandType = "file"         // 文件操作
    CommandTypeService    CommandType = "service"      // 服务管理
    CommandTypeNetwork    CommandType = "network"      // 网络配置
    CommandTypeCustom     CommandType = "custom"       // 自定义
)

// 执行策略
type ExecutionStrategy struct {
    Mode         ExecutionMode              `json:"mode"`          // 执行模式
    Concurrency  int                        `json:"concurrency"`   // 并发数
    BatchSize    int                        `json:"batch_size"`    // 批次大小
    Interval     time.Duration              `json:"interval"`      // 批次间间隔
    FailureMode  FailureMode                `json:"failure_mode"`  // 失败处理模式
}

// 执行模式
type ExecutionMode string

const (
    ModeParallel ExecutionMode = "parallel"  // 并行执行
    ModeSerial   ExecutionMode = "serial"    // 串行执行
    ModeBatch    ExecutionMode = "batch"     // 批次执行
)
```

#### 3.2.3 分发算法
```go
// 任务分发接口
type TaskDistributorInterface interface {
    // 执行子任务
    ExecuteSubTask(ctx context.Context, subTask *SubTask) error
    
    // 停止任务
    StopTask(taskID string) error
    
    // 获取任务状态
    GetTaskStatus(taskID string) (*TaskStatus, error)
    
    // 获取分发统计
    GetStats() *DistributorStats
}

// 智能分发算法
func (td *TaskDistributor) ExecuteSubTask(ctx context.Context, subTask *SubTask) error {
    // 1. 验证目标Agent连接状态
    validTargets := td.validateTargets(subTask.Targets)
    if len(validTargets) == 0 {
        return errors.New("no valid targets available")
    }
    
    // 2. 根据执行策略选择分发模式
    switch subTask.Strategy.Mode {
    case ModeParallel:
        return td.executeParallel(ctx, subTask, validTargets)
    case ModeSerial:
        return td.executeSerial(ctx, subTask, validTargets)
    case ModeBatch:
        return td.executeBatch(ctx, subTask, validTargets)
    default:
        return td.executeParallel(ctx, subTask, validTargets)
    }
}

// 并行执行
func (td *TaskDistributor) executeParallel(ctx context.Context, subTask *SubTask, targets []string) error {
    semaphore := make(chan struct{}, subTask.Strategy.Concurrency)
    var wg sync.WaitGroup
    
    for _, agentID := range targets {
        wg.Add(1)
        semaphore <- struct{}{}
        
        go func(agentID string) {
            defer wg.Done()
            defer func() { <-semaphore }()
            
            td.sendTaskToAgent(ctx, subTask, agentID)
        }(agentID)
    }
    
    wg.Wait()
    return nil
}
```

### 3.3 结果收集和上报

#### 3.3.1 功能特性
- 实时结果收集
- 结果聚合和统计
- 批量上报优化
- 结果缓存管理
- 异常处理和容错

#### 3.3.2 数据结构
```go
// 结果收集器
type ResultCollector struct {
    resultBuffer   *ResultBuffer             // 结果缓冲区
    aggregator     *ResultAggregator         // 结果聚合器
    uploader       *ResultUploader           // 结果上报器
    cache          *ResultCache              // 结果缓存
    config         *CollectorConfig          // 配置
}

// 任务结果
type TaskResult struct {
    TaskID     string                       `json:"task_id"`
    AgentID    string                       `json:"agent_id"`
    Status     TaskStatus                   `json:"status"`
    StartTime  time.Time                    `json:"start_time"`
    EndTime    time.Time                    `json:"end_time"`
    Duration   time.Duration                `json:"duration"`
    Output     string                       `json:"output"`
    Error      string                       `json:"error"`
    ExitCode   int                          `json:"exit_code"`
    Metadata   map[string]interface{}       `json:"metadata"`
}

// 任务状态
type TaskStatus string

const (
    StatusPending   TaskStatus = "pending"    // 等待执行
    StatusRunning   TaskStatus = "running"    // 执行中
    StatusSucceed   TaskStatus = "succeed"    // 执行成功
    StatusFailed    TaskStatus = "failed"     // 执行失败
    StatusTimeout   TaskStatus = "timeout"    // 执行超时
    StatusCancelled TaskStatus = "cancelled"  // 已取消
)

// 聚合结果
type AggregatedResult struct {
    TaskID      string                       `json:"task_id"`
    Total       int                          `json:"total"`
    Succeed     int                          `json:"succeed"`
    Failed      int                          `json:"failed"`
    Running     int                          `json:"running"`
    Pending     int                          `json:"pending"`
    Progress    float64                      `json:"progress"`
    Results     []*TaskResult                `json:"results"`
    Summary     *ResultSummary               `json:"summary"`
    UpdatedAt   time.Time                    `json:"updated_at"`
}
```

### 3.4 负载均衡和容错

#### 3.4.1 功能特性
- 多种负载均衡算法
- 健康检查和故障检测
- 自动故障转移
- 熔断和限流保护
- 性能监控和预警

#### 3.4.2 负载均衡策略
```go
// 负载均衡器
type LoadBalancer struct {
    strategy    BalanceStrategy              // 均衡策略
    weights     map[string]int               // 权重配置
    healthCheck *HealthChecker               // 健康检查
    metrics     *LoadBalancerMetrics         // 负载指标
}

// 负载均衡策略
type BalanceStrategy interface {
    // 选择Agent
    SelectAgent(agents []string, context *SelectContext) (string, error)
    
    // 更新权重
    UpdateWeight(agentID string, weight int)
}

// 轮询策略
type RoundRobinStrategy struct {
    counter uint64
}

func (rr *RoundRobinStrategy) SelectAgent(agents []string, context *SelectContext) (string, error) {
    if len(agents) == 0 {
        return "", errors.New("no agents available")
    }
    
    index := atomic.AddUint64(&rr.counter, 1) % uint64(len(agents))
    return agents[index], nil
}

// 加权轮询策略
type WeightedRoundRobinStrategy struct {
    weights map[string]int
    current map[string]int
    mutex   sync.RWMutex
}

// 一致性哈希策略
type ConsistentHashStrategy struct {
    hashRing *HashRing
}
```

### 3.5 与Center通信协议

#### 3.5.1 通信架构
- 基于NATS Streaming的异步消息通信
- 支持任务分发、结果上报、状态同步
- 消息去重和幂等性保证
- 断线重连和消息补偿

#### 3.5.2 消息格式
```go
// 消息类型
type MessageType string

const (
    MsgTypeTaskDispatch    MessageType = "task.dispatch"     // 任务分发
    MsgTypeTaskResult      MessageType = "task.result"       // 任务结果
    MsgTypeTaskStatus      MessageType = "task.status"       // 任务状态
    MsgTypeAgentStatus     MessageType = "agent.status"      // Agent状态
    MsgTypeHeartbeat       MessageType = "heartbeat"         // 心跳
    MsgTypeConfig          MessageType = "config"            // 配置推送
)

// 消息结构
type Message struct {
    ID          string                     `json:"id"`
    Type        MessageType                `json:"type"`
    ProxyID     string                     `json:"proxy_id"`
    Timestamp   time.Time                  `json:"timestamp"`
    Payload     interface{}                `json:"payload"`
    Headers     map[string]string          `json:"headers"`
    ReplyTo     string                     `json:"reply_to,omitempty"`
}

// 任务分发消息
type TaskDispatchMessage struct {
    SubTask     *SubTask                   `json:"sub_task"`
    Priority    int                        `json:"priority"`
    Deadline    time.Time                  `json:"deadline"`
}

// 结果上报消息
type TaskResultMessage struct {
    Results     []*TaskResult              `json:"results"`
    Summary     *ResultSummary             `json:"summary"`
    IsComplete  bool                       `json:"is_complete"`
}
```

## 4. 核心算法设计

### 4.1 连接管理算法

```go
// 连接健康检查
func (hc *HealthChecker) CheckHealth(conn *AgentConnection) error {
    // 1. 检查连接状态
    if conn.WSConn == nil {
        return errors.New("websocket connection is nil")
    }
    
    // 2. 发送ping消息
    pingMsg := &Message{
        Type: "ping",
        Timestamp: time.Now(),
    }
    
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    // 3. 等待pong响应
    if err := conn.SendMessage(ctx, pingMsg); err != nil {
        return fmt.Errorf("failed to send ping: %v", err)
    }
    
    return nil
}

// 自动重连机制
func (cp *ConnectionPool) reconnectAgent(agentID string) {
    backoff := time.Second
    maxBackoff := 5 * time.Minute
    
    for {
        select {
        case <-cp.ctx.Done():
            return
        case <-time.After(backoff):
            if err := cp.attemptReconnect(agentID); err == nil {
                cp.logger.Info("agent reconnected", "agent_id", agentID)
                return
            }
            
            // 指数退避
            backoff *= 2
            if backoff > maxBackoff {
                backoff = maxBackoff
            }
        }
    }
}
```

### 4.2 任务调度算法

```go
// 任务优先级调度
type PriorityScheduler struct {
    queues    map[TaskPriority]*TaskQueue  // 优先级队列
    executor  *TaskExecutor                // 任务执行器
    semaphore chan struct{}                // 并发控制
}

func (ps *PriorityScheduler) Schedule() {
    for {
        select {
        case <-ps.ctx.Done():
            return
        default:
            // 按优先级顺序处理任务
            for priority := PriorityHigh; priority >= PriorityLow; priority-- {
                if task := ps.queues[priority].Pop(); task != nil {
                    ps.semaphore <- struct{}{}
                    go func(task *SubTask) {
                        defer func() { <-ps.semaphore }()
                        ps.executor.Execute(task)
                    }(task)
                    break
                }
            }
            time.Sleep(100 * time.Millisecond)
        }
    }
}
```

## 5. 性能优化

### 5.1 连接池优化
- 连接复用和预热
- 连接数动态调整
- 内存池复用
- 零拷贝优化

### 5.2 任务分发优化
- 任务批量处理
- 管道并行处理
- 内存队列缓冲
- 压缩传输

### 5.3 结果收集优化
- 批量上报机制
- 结果压缩存储
- 异步处理管道
- 数据去重优化

## 6. 监控指标

### 6.1 连接指标
```go
type ConnectionMetrics struct {
    TotalConnections     int64   `json:"total_connections"`
    ActiveConnections    int64   `json:"active_connections"`
    FailedConnections    int64   `json:"failed_connections"`
    AvgConnectionTime    float64 `json:"avg_connection_time"`
    ConnectionFailureRate float64 `json:"connection_failure_rate"`
}
```

### 6.2 任务指标
```go
type TaskMetrics struct {
    TasksDispatched      int64   `json:"tasks_dispatched"`
    TasksCompleted       int64   `json:"tasks_completed"`
    TasksFailed          int64   `json:"tasks_failed"`
    AvgTaskDuration      float64 `json:"avg_task_duration"`
    TaskThroughput       float64 `json:"task_throughput"`
    TaskSuccessRate      float64 `json:"task_success_rate"`
}
```

## 7. 容错设计

### 7.1 故障类型
- 网络连接中断
- Agent节点故障
- 任务执行超时
- 消息丢失
- 系统资源不足

### 7.2 容错策略
- 自动重试机制
- 故障转移
- 熔断降级
- 资源隔离
- 优雅降级

## 8. 部署运维

### 8.1 部署架构
- Docker容器化部署
- 高可用集群部署
- 负载均衡配置
- 监控告警配置

### 8.2 运维管理
- 滚动更新
- 配置热更新
- 日志收集
- 性能调优

## 9. API接口

### 9.1 内部API (与Center通信)
```
POST   /internal/tasks                 # 接收任务分发
PUT    /internal/tasks/{id}/result     # 上报任务结果
GET    /internal/agents/status         # Agent状态查询
POST   /internal/config                # 配置更新
```

### 9.2 管理API (运维管理)
```
GET    /api/v1/stats                   # 统计信息
GET    /api/v1/agents                  # Agent列表
GET    /api/v1/tasks                   # 任务列表
GET    /api/v1/health                  # 健康检查
```

## 10. 配置管理

### 10.1 配置结构
```yaml
proxy:
  id: "proxy-001"
  region: "us-west-1"
  max_agents: 500
  
connection:
  max_connections: 1000
  read_timeout: 30s
  write_timeout: 30s
  ping_interval: 30s
  
task:
  max_concurrent_tasks: 100
  default_timeout: 300s
  retry_count: 3
  batch_size: 50
  
center:
  endpoints:
    - "nats://center1:4222"
    - "nats://center2:4222"
  subject_prefix: "ultrathink"
  
redis:
  address: "redis:6379"
  db: 0
  max_retries: 3
```

## 11. 开发计划

### 11.1 开发阶段
1. **基础框架** (1周): 项目结构、依赖管理
2. **连接管理** (1.5周): 连接池、健康检查
3. **任务分发** (1.5周): 调度器、分发算法
4. **结果收集** (1周): 聚合器、上报机制
5. **Center通信** (1周): 消息队列、协议实现
6. **监控运维** (0.5周): 指标收集、健康检查
7. **测试优化** (1周): 压力测试、性能优化

### 11.2 关键技术挑战
- 大规模连接管理的性能优化
- 任务分发的公平性和效率
- 网络异常的容错处理
- 内存和CPU资源的合理分配

### 11.3 成功标准
- 支持500台Agent稳定连接
- 任务分发延迟 < 100ms
- 连接成功率 > 99.5%
- 内存使用 < 2GB
- CPU使用率 < 70%