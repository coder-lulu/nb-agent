# UltraThink 中心层(Center) 功能设计文档

## 1. 概述

中心层(Center)是UltraThink分布式运维平台的核心控制组件，负责统一管理、任务调度、资源配置和数据聚合。基于go-zero微服务架构实现，为用户提供Web控制台和API接口。

## 2. 系统架构

### 2.1 微服务架构图

```
┌─────────────────────────────────────────────────────────────┐
│                 UltraThink Center                           │
│ ┌─────────────────────────────────────────────────────────┐ │
│ │                API Gateway                              │ │
│ └─────────────────────────────────────────────────────────┘ │
│                              │                             │
│     ┌─────────────────┬──────┴──────┬─────────────────┐    │
│     │                 │             │                 │    │
│ ┌───▼───┐ ┌──────▼──────┐ ┌─────▼─────┐ ┌──────▼──────┐  │
│ │用户管理│ │任务调度引擎 │ │资源管理   │ │监控告警     │  │
│ │服务   │ │服务        │ │服务       │ │服务         │  │
│ └───────┘ └─────────────┘ └───────────┘ └─────────────┘  │
│                              │                             │
│ ┌─────────────────────────────┴─────────────────────────┐ │
│ │              配置中心 & 服务发现                      │ │
│ └─────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 技术栈

- **框架**: go-zero微服务框架
- **数据库**: MySQL 8.0 (主存储) + Redis 6.0 (缓存)
- **消息队列**: NATS Streaming (任务分发) + Redis Stream (实时消息)
- **前端**: Vue 3 + TypeScript + Naive UI
- **监控**: Prometheus + Grafana
- **日志**: ELK Stack (Elasticsearch + Logstash + Kibana)

## 3. 核心功能模块

### 3.1 用户管理和权限系统

#### 3.1.1 功能特性
- 多租户支持
- 基于RBAC的权限控制
- SSO单点登录集成
- 操作审计日志

#### 3.1.2 数据模型
```go
// 用户模型
type User struct {
    ID          int64     `json:"id"`
    Username    string    `json:"username"`
    Email       string    `json:"email"`
    DisplayName string    `json:"display_name"`
    TenantID    int64     `json:"tenant_id"`
    Status      int       `json:"status"`
    Roles       []Role    `json:"roles"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}

// 角色模型
type Role struct {
    ID          int64        `json:"id"`
    Name        string       `json:"name"`
    Description string       `json:"description"`
    Permissions []Permission `json:"permissions"`
    TenantID    int64        `json:"tenant_id"`
}

// 权限模型
type Permission struct {
    ID       int64  `json:"id"`
    Resource string `json:"resource"` // agent, task, monitor等
    Action   string `json:"action"`   // create, read, update, delete
    Scope    string `json:"scope"`    // tenant, group, self
}
```

#### 3.1.3 API设计
```
POST   /api/v1/users                   # 创建用户
GET    /api/v1/users                   # 用户列表
GET    /api/v1/users/{id}              # 用户详情
PUT    /api/v1/users/{id}              # 更新用户
DELETE /api/v1/users/{id}              # 删除用户
POST   /api/v1/users/{id}/roles        # 分配角色
GET    /api/v1/roles                   # 角色列表
POST   /api/v1/roles                   # 创建角色
PUT    /api/v1/roles/{id}              # 更新角色
```

### 3.2 批量任务调度引擎

#### 3.2.1 功能特性
- 大规模批量任务执行(支持1000+台主机)
- 多种执行策略(并行/串行/批次)
- 任务依赖管理
- 实时进度追踪
- 失败重试和容错
- 任务模板管理

#### 3.2.2 数据模型
```go
// 批量任务模型
type BatchTask struct {
    ID            int64                `json:"id"`
    Name          string               `json:"name"`
    Description   string               `json:"description"`
    Type          TaskType             `json:"type"`
    Command       TaskCommand          `json:"command"`
    Targets       []AgentTarget        `json:"targets"`
    Strategy      ExecutionStrategy    `json:"strategy"`
    Status        TaskStatus           `json:"status"`
    Progress      TaskProgress         `json:"progress"`
    Results       []TaskResult         `json:"results"`
    CreatedBy     int64                `json:"created_by"`
    TenantID      int64                `json:"tenant_id"`
    CreatedAt     time.Time            `json:"created_at"`
    StartedAt     *time.Time           `json:"started_at"`
    CompletedAt   *time.Time           `json:"completed_at"`
}

// 任务命令
type TaskCommand struct {
    Type        CommandType            `json:"type"`
    Action      string                `json:"action"`
    Parameters  map[string]interface{} `json:"parameters"`
    Script      string                `json:"script"`
    Timeout     int                   `json:"timeout"`
}

// 执行策略
type ExecutionStrategy struct {
    Mode         ExecutionMode `json:"mode"`          // parallel, serial, batch
    Concurrency  int          `json:"concurrency"`   // 并发数
    BatchSize    int          `json:"batch_size"`    // 批次大小
    Timeout      int          `json:"timeout"`       // 总超时时间
    RetryCount   int          `json:"retry_count"`   // 重试次数
    FailureMode  FailureMode  `json:"failure_mode"`  // continue, stop
}

// 任务进度
type TaskProgress struct {
    Total      int                 `json:"total"`
    Running    int                 `json:"running"`
    Succeeded  int                 `json:"succeeded"`
    Failed     int                 `json:"failed"`
    Percentage float64             `json:"percentage"`
    Details    map[string]AgentStatus `json:"details"`
}
```

#### 3.2.3 API设计
```
POST   /api/v1/tasks                   # 创建批量任务
GET    /api/v1/tasks                   # 任务列表
GET    /api/v1/tasks/{id}              # 任务详情
PUT    /api/v1/tasks/{id}/start        # 启动任务
PUT    /api/v1/tasks/{id}/stop         # 停止任务
GET    /api/v1/tasks/{id}/progress     # 任务进度
GET    /api/v1/tasks/{id}/results      # 任务结果
GET    /api/v1/tasks/{id}/logs         # 任务日志
```

#### 3.2.4 任务调度算法
```go
// 任务分发策略
type TaskDistributor struct {
    proxyManager   *ProxyManager
    loadBalancer   *LoadBalancer
    failureHandler *FailureHandler
}

// 智能分发算法
func (td *TaskDistributor) DistributeTask(task *BatchTask) error {
    // 1. 按地理位置/网络拓扑分组Agent
    proxyGroups := td.groupAgentsByProxy(task.Targets)
    
    // 2. 根据Proxy负载情况调整分发策略
    for proxyID, agents := range proxyGroups {
        proxyLoad := td.loadBalancer.GetProxyLoad(proxyID)
        subTask := td.createSubTask(task, agents, proxyLoad)
        
        // 3. 异步分发到Proxy
        go td.sendToProxy(proxyID, subTask)
    }
    
    return nil
}
```

### 3.3 资源管理服务

#### 3.3.1 功能特性
- Agent资源清单管理
- 主机分组和标签管理
- 配置模板管理
- 软件库和脚本库管理
- 资源统计和报表

#### 3.3.2 数据模型
```go
// Agent资源
type Agent struct {
    ID           int64            `json:"id"`
    UUID         string           `json:"uuid"`
    Name         string           `json:"name"`
    IP           string           `json:"ip"`
    OS           string           `json:"os"`
    Arch         string           `json:"arch"`
    Version      string           `json:"version"`
    ProxyID      int64            `json:"proxy_id"`
    GroupIDs     []int64          `json:"group_ids"`
    Tags         map[string]string `json:"tags"`
    Status       AgentStatus      `json:"status"`
    LastSeen     time.Time        `json:"last_seen"`
    Metadata     AgentMetadata    `json:"metadata"`
    TenantID     int64            `json:"tenant_id"`
}

// 主机分组
type AgentGroup struct {
    ID          int64             `json:"id"`
    Name        string            `json:"name"`
    Description string            `json:"description"`
    Tags        map[string]string `json:"tags"`
    AgentCount  int               `json:"agent_count"`
    TenantID    int64             `json:"tenant_id"`
}

// 配置模板
type ConfigTemplate struct {
    ID          int64                  `json:"id"`
    Name        string                 `json:"name"`
    Type        TemplateType           `json:"type"`
    Content     map[string]interface{} `json:"content"`
    Variables   []TemplateVariable     `json:"variables"`
    TenantID    int64                  `json:"tenant_id"`
}
```

### 3.4 监控告警服务

#### 3.4.1 功能特性
- 实时监控数据聚合
- 多维度告警规则
- 告警通知(邮件/短信/钉钉/企微)
- 监控大屏展示
- 性能报表生成

#### 3.4.2 数据模型
```go
// 监控指标
type MetricData struct {
    AgentID    int64                  `json:"agent_id"`
    Timestamp  time.Time              `json:"timestamp"`
    Metrics    map[string]interface{} `json:"metrics"`
    TenantID   int64                  `json:"tenant_id"`
}

// 告警规则
type AlertRule struct {
    ID         int64           `json:"id"`
    Name       string          `json:"name"`
    MetricName string          `json:"metric_name"`
    Condition  AlertCondition  `json:"condition"`
    Threshold  float64         `json:"threshold"`
    Duration   int             `json:"duration"`
    Severity   AlertSeverity   `json:"severity"`
    Enabled    bool            `json:"enabled"`
    TenantID   int64           `json:"tenant_id"`
}

// 告警事件
type AlertEvent struct {
    ID          int64         `json:"id"`
    RuleID      int64         `json:"rule_id"`
    AgentID     int64         `json:"agent_id"`
    Status      AlertStatus   `json:"status"`
    Message     string        `json:"message"`
    Value       float64       `json:"value"`
    StartsAt    time.Time     `json:"starts_at"`
    EndsAt      *time.Time    `json:"ends_at"`
    TenantID    int64         `json:"tenant_id"`
}
```

### 3.5 配置中心

#### 3.5.1 功能特性
- 全局配置管理
- 环境配置隔离
- 配置版本控制
- 配置推送和回滚
- 配置审计

#### 3.5.2 数据模型
```go
// 配置项
type Config struct {
    ID          int64     `json:"id"`
    Key         string    `json:"key"`
    Value       string    `json:"value"`
    Type        string    `json:"type"`
    Environment string    `json:"environment"`
    Description string    `json:"description"`
    Version     int       `json:"version"`
    TenantID    int64     `json:"tenant_id"`
    UpdatedBy   int64     `json:"updated_by"`
    UpdatedAt   time.Time `json:"updated_at"`
}

// 配置推送记录
type ConfigDeployment struct {
    ID          int64              `json:"id"`
    ConfigID    int64              `json:"config_id"`
    Version     int                `json:"version"`
    Targets     []int64            `json:"targets"`
    Status      DeploymentStatus   `json:"status"`
    DeployedBy  int64              `json:"deployed_by"`
    DeployedAt  time.Time          `json:"deployed_at"`
}
```

## 4. 数据库设计

### 4.1 核心表结构

```sql
-- 用户表
CREATE TABLE users (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    username VARCHAR(64) UNIQUE NOT NULL,
    email VARCHAR(128) UNIQUE,
    password_hash VARCHAR(128),
    display_name VARCHAR(128),
    tenant_id BIGINT NOT NULL,
    status TINYINT DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_tenant_id (tenant_id),
    INDEX idx_username (username)
);

-- 批量任务表
CREATE TABLE batch_tasks (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    name VARCHAR(128) NOT NULL,
    type VARCHAR(32) NOT NULL,
    command JSON NOT NULL,
    targets JSON NOT NULL,
    strategy JSON NOT NULL,
    status VARCHAR(32) DEFAULT 'pending',
    progress JSON,
    created_by BIGINT NOT NULL,
    tenant_id BIGINT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    started_at TIMESTAMP NULL,
    completed_at TIMESTAMP NULL,
    INDEX idx_tenant_status (tenant_id, status),
    INDEX idx_created_by (created_by)
);

-- Agent资源表
CREATE TABLE agents (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    uuid VARCHAR(64) UNIQUE NOT NULL,
    name VARCHAR(128) NOT NULL,
    ip VARCHAR(45) NOT NULL,
    os VARCHAR(32),
    arch VARCHAR(32),
    version VARCHAR(32),
    proxy_id BIGINT,
    status VARCHAR(32) DEFAULT 'offline',
    last_seen TIMESTAMP,
    metadata JSON,
    tenant_id BIGINT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_tenant_id (tenant_id),
    INDEX idx_proxy_id (proxy_id),
    INDEX idx_status (status)
);
```

## 5. API 接口规范

### 5.1 认证授权
- 使用JWT Token进行API认证
- Token有效期为24小时，支持自动刷新
- 所有API请求需要在Header中携带Authorization字段

### 5.2 响应格式
```json
{
    "code": 200,
    "message": "success",
    "data": {},
    "timestamp": 1643723400
}
```

### 5.3 错误码定义
```
200 - 成功
400 - 请求参数错误
401 - 未授权
403 - 权限不足
404 - 资源不存在
500 - 服务器内部错误
```

## 6. 部署架构

### 6.1 服务部署
- 使用Docker容器化部署
- Kubernetes集群管理
- 支持水平扩展
- 服务网格(Istio)治理

### 6.2 高可用设计
- 数据库主从复制
- Redis集群模式
- 多活数据中心
- 自动故障转移

### 6.3 性能指标
- 支持10000+并发用户
- 批量任务响应时间 < 3秒
- API响应时间 < 500ms
- 99.9%服务可用性

## 7. 安全设计

### 7.1 数据安全
- 数据库连接加密
- 敏感数据加密存储
- 定期数据备份
- 数据访问审计

### 7.2 网络安全
- HTTPS强制加密
- API限流防护
- 防SQL注入
- 跨域访问控制

### 7.3 权限安全
- 最小权限原则
- 操作审计日志
- 会话超时控制
- 多因素认证

## 8. 监控运维

### 8.1 服务监控
- 健康检查接口
- 关键指标监控
- 告警规则配置
- 日志集中管理

### 8.2 性能监控
- 响应时间监控
- 吞吐量统计
- 资源使用监控
- 慢查询分析

## 9. 开发计划

### 9.1 开发阶段
1. **基础框架搭建** (1周)
2. **用户权限系统** (1.5周)
3. **任务调度引擎** (2周)
4. **资源管理服务** (1.5周)
5. **监控告警服务** (1周)
6. **前端界面开发** (2周)
7. **集成测试** (1周)

### 9.2 技术风险
- 大规模并发任务调度的性能优化
- 分布式事务一致性保证
- 跨地域网络延迟问题
- 数据库性能瓶颈

### 9.3 解决方案
- 使用消息队列解耦任务分发
- 采用最终一致性设计
- 就近部署Proxy减少延迟
- 读写分离和分库分表