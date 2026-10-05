package service

import (
	"encoding/json"
	"net/http"

	"github.com/libtnb/chix"

	"github.com/coder-lulu/nb-agent/internal/biz"
	"github.com/coder-lulu/nb-agent/internal/http/request"
)

type RemoteTaskService struct {
	remoteTaskRepo biz.RemoteTaskRepo
	dispatcher     biz.RemoteTaskDispatcher
}

// CreateRemoteTaskRequest 创建远程任务请求
type CreateRemoteTaskRequest struct {
	TaskID     string                 `json:"task_id" validate:"required"`
	Name       string                 `json:"name" validate:"required"`
	Type       biz.TaskType          `json:"type" validate:"required"`
	Source     biz.RemoteTaskSource  `json:"source" validate:"required"`
	Command    string                 `json:"command" validate:"required"`
	Parameters map[string]interface{} `json:"parameters"`
	WorkingDir string                 `json:"working_dir"`
	Timeout    int                    `json:"timeout"`
}

// UpdateTaskStatusRequest 更新任务状态请求
type UpdateTaskStatusRequest struct {
	TaskID string          `json:"task_id" validate:"required"`
	Status biz.TaskStatus `json:"status" validate:"required"`
	Result string          `json:"result"`
	Error  string          `json:"error"`
}

func NewRemoteTaskService(remoteTaskRepo biz.RemoteTaskRepo, dispatcher biz.RemoteTaskDispatcher) *RemoteTaskService {
	return &RemoteTaskService{
		remoteTaskRepo: remoteTaskRepo,
		dispatcher:     dispatcher,
	}
}

// Create 创建远程任务
func (s *RemoteTaskService) Create(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[CreateRemoteTaskRequest](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	// 创建远程任务实例
	remoteTask := &biz.RemoteTask{
		TaskID:     req.TaskID,
		Name:       req.Name,
		Type:       req.Type,
		Status:     biz.TaskStatusWaiting,
		Source:     req.Source,
		Command:    req.Command,
		WorkingDir: req.WorkingDir,
		Timeout:    req.Timeout,
	}

	// 设置参数
	if req.Parameters != nil {
		if err := remoteTask.SetParameters(req.Parameters); err != nil {
			Error(w, http.StatusBadRequest, "invalid parameters: %v", err)
			return
		}
	}

	// 设置默认超时时间
	if remoteTask.Timeout <= 0 {
		remoteTask.Timeout = 300 // 默认5分钟
	}

	// 保存任务
	if err := s.remoteTaskRepo.Create(remoteTask); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	// 分发任务
	if err := s.dispatcher.DispatchTask(remoteTask); err != nil {
		// 分发失败，更新任务状态
		s.remoteTaskRepo.UpdateStatus(remoteTask.TaskID, biz.TaskStatusFailed)
		s.remoteTaskRepo.UpdateProgress(remoteTask.TaskID, "", err.Error())
		Error(w, http.StatusInternalServerError, "failed to dispatch task: %v", err)
		return
	}

	Success(w, remoteTask)
}

// List 列出远程任务
func (s *RemoteTaskService) List(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.Paginate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	tasks, total, err := s.remoteTaskRepo.List(req.Page, req.Limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, chix.M{
		"total": total,
		"items": tasks,
	})
}

// Get 获取远程任务
func (s *RemoteTaskService) Get(w http.ResponseWriter, r *http.Request) {
	taskID := chix.URLParam(r, "task_id")
	if taskID == "" {
		Error(w, http.StatusBadRequest, "task_id is required")
		return
	}

	task, err := s.remoteTaskRepo.Get(taskID)
	if err != nil {
		Error(w, http.StatusNotFound, "%v", err)
		return
	}

	Success(w, task)
}

// UpdateStatus 更新任务状态
func (s *RemoteTaskService) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[UpdateTaskStatusRequest](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	// 更新任务状态
	if err := s.remoteTaskRepo.UpdateStatus(req.TaskID, req.Status); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	// 更新任务进度
	if req.Result != "" || req.Error != "" {
		if err := s.remoteTaskRepo.UpdateProgress(req.TaskID, req.Result, req.Error); err != nil {
			Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
	}

	Success(w, chix.M{"message": "status updated successfully"})
}

// Delete 删除远程任务
func (s *RemoteTaskService) Delete(w http.ResponseWriter, r *http.Request) {
	taskID := chix.URLParam(r, "task_id")
	if taskID == "" {
		Error(w, http.StatusBadRequest, "task_id is required")
		return
	}

	err := s.remoteTaskRepo.Delete(taskID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

// ListByStatus 根据状态列出任务
func (s *RemoteTaskService) ListByStatus(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.Paginate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	statusStr := chix.URLParam(r, "status")
	if statusStr == "" {
		Error(w, http.StatusBadRequest, "status is required")
		return
	}

	tasks, total, err := s.remoteTaskRepo.ListByStatus(biz.TaskStatus(statusStr), req.Page, req.Limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, chix.M{
		"total": total,
		"items": tasks,
	})
}

// Stats 获取任务统计
func (s *RemoteTaskService) Stats(w http.ResponseWriter, r *http.Request) {
	// 统计各状态的任务数量
	waitingCount, _ := s.remoteTaskRepo.CountByStatus(biz.TaskStatusWaiting)
	runningCount, _ := s.remoteTaskRepo.CountByStatus(biz.TaskStatusRunning)
	successCount, _ := s.remoteTaskRepo.CountByStatus(biz.TaskStatusSuccess)
	failedCount, _ := s.remoteTaskRepo.CountByStatus(biz.TaskStatusFailed)

	// 获取分发器统计
	dispatchStats := s.dispatcher.GetDispatchStats()

	stats := chix.M{
		"task_counts": chix.M{
			"waiting": waitingCount,
			"running": runningCount,
			"success": successCount,
			"failed":  failedCount,
			"total":   waitingCount + runningCount + successCount + failedCount,
		},
		"dispatch_stats": dispatchStats,
		"has_running":    s.remoteTaskRepo.HasRunningTask(),
	}

	Success(w, stats)
}

// CompleteTask 任务完成回调（供内部调用）
func (s *RemoteTaskService) CompleteTask(taskID string, result interface{}, err error) error {
	var status biz.TaskStatus
	var resultStr, errorStr string

	if err != nil {
		status = biz.TaskStatusFailed
		errorStr = err.Error()
	} else {
		status = biz.TaskStatusSuccess
		if result != nil {
			resultData, _ := json.Marshal(result)
			resultStr = string(resultData)
		}
	}

	// 更新任务状态
	if updateErr := s.remoteTaskRepo.UpdateStatus(taskID, status); updateErr != nil {
		return updateErr
	}

	// 更新任务结果
	return s.remoteTaskRepo.UpdateProgress(taskID, resultStr, errorStr)
}