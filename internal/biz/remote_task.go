package biz

import (
	"encoding/json"
	"time"
)

// TaskType 任务类型
type TaskType string

const (
	TaskTypeLocal  TaskType = "local"  // 本地任务
	TaskTypeRemote TaskType = "remote" // 远程任务
)

// RemoteTaskSource 远程任务来源
type RemoteTaskSource string

const (
	RemoteTaskSourceProxy  RemoteTaskSource = "proxy"  // 来自Proxy
	RemoteTaskSourceCenter RemoteTaskSource = "center" // 来自Center
)

// RemoteTask 远程任务扩展
type RemoteTask struct {
	ID          uint                   `gorm:"primaryKey" json:"id"`
	TaskID      string                 `gorm:"not null;uniqueIndex" json:"task_id"`
	Name        string                 `gorm:"not null;default:''" json:"name"`
	Type        TaskType               `gorm:"not null;default:'local'" json:"type"`
	Status      TaskStatus             `gorm:"not null;default:'waiting'" json:"status"`
	Source      RemoteTaskSource       `gorm:"not null;default:'proxy'" json:"source"`
	Command     string                 `gorm:"not null;default:''" json:"command"`
	Parameters  string                 `gorm:"type:text" json:"-"` // JSON存储参数
	WorkingDir  string                 `gorm:"not null;default:''" json:"working_dir"`
	Timeout     int                    `gorm:"not null;default:300" json:"timeout"` // 超时时间(秒)
	Result      string                 `gorm:"type:text" json:"result"`
	Error       string                 `gorm:"type:text" json:"error"`
	Log         string                 `gorm:"type:text" json:"log"`
	StartedAt   *time.Time             `json:"started_at"`
	CompletedAt *time.Time             `json:"completed_at"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// GetParameters 获取任务参数
func (rt *RemoteTask) GetParameters() (map[string]interface{}, error) {
	if rt.Parameters == "" {
		return make(map[string]interface{}), nil
	}
	
	var params map[string]interface{}
	err := json.Unmarshal([]byte(rt.Parameters), &params)
	return params, err
}

// SetParameters 设置任务参数
func (rt *RemoteTask) SetParameters(params map[string]interface{}) error {
	data, err := json.Marshal(params)
	if err != nil {
		return err
	}
	rt.Parameters = string(data)
	return nil
}

// RemoteTaskRepo 远程任务仓库接口
type RemoteTaskRepo interface {
	// 基础CRUD操作
	Create(task *RemoteTask) error
	Get(taskID string) (*RemoteTask, error)
	GetByID(id uint) (*RemoteTask, error)
	Update(task *RemoteTask) error
	Delete(taskID string) error
	
	// 状态管理
	UpdateStatus(taskID string, status TaskStatus) error
	UpdateProgress(taskID string, result string, err string) error
	
	// 查询操作
	List(page, limit uint) ([]*RemoteTask, int64, error)
	ListByStatus(status TaskStatus, page, limit uint) ([]*RemoteTask, int64, error)
	ListBySource(source RemoteTaskSource, page, limit uint) ([]*RemoteTask, int64, error)
	
	// 任务队列操作
	GetPendingTasks(limit int) ([]*RemoteTask, error)
	HasRunningTask() bool
	CountByStatus(status TaskStatus) (int64, error)
	
	// 清理操作
	ClearZombieTasks() error
	CleanupOldTasks(days int) error
}

// RemoteTaskExecutor 远程任务执行器接口
type RemoteTaskExecutor interface {
	// 任务执行
	Execute(task *RemoteTask) error
	
	// 状态管理
	Start() error
	Stop() error
	IsRunning() bool
	
	// 配置管理
	SetConcurrency(count int)
	GetStats() *TaskExecutorStats
}

// TaskExecutorStats 任务执行器统计
type TaskExecutorStats struct {
	RunningTasks    int64     `json:"running_tasks"`
	PendingTasks    int64     `json:"pending_tasks"`
	CompletedTasks  int64     `json:"completed_tasks"`
	FailedTasks     int64     `json:"failed_tasks"`
	TotalTasks      int64     `json:"total_tasks"`
	LastExecutedAt  *time.Time `json:"last_executed_at"`
	Uptime          time.Duration `json:"uptime"`
}

// RemoteTaskDispatcher 远程任务分发器接口
type RemoteTaskDispatcher interface {
	// 任务分发
	DispatchTask(task *RemoteTask) error
	
	// 结果回调
	OnTaskCompleted(taskID string, result interface{}, err error) error
	
	// 连接管理
	SetConnectionManager(connMgr interface{}) // 设置连接管理器
	
	// 统计信息
	GetDispatchStats() *TaskDispatchStats
}

// TaskDispatchStats 任务分发统计
type TaskDispatchStats struct {
	DispatchedTasks int64 `json:"dispatched_tasks"`
	CompletedTasks  int64 `json:"completed_tasks"`
	FailedTasks     int64 `json:"failed_tasks"`
	ActiveTasks     int64 `json:"active_tasks"`
}