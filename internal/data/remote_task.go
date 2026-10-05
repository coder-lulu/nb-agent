package data

import (
	"errors"
	"log/slog"
	"time"

	"github.com/leonelquinteros/gotext"
	"gorm.io/gorm"

	"github.com/coder-lulu/nb-agent/internal/biz"
)

type remoteTaskRepo struct {
	t   *gotext.Locale
	db  *gorm.DB
	log *slog.Logger
}

// NewRemoteTaskRepo 创建远程任务仓库
func NewRemoteTaskRepo(t *gotext.Locale, db *gorm.DB, log *slog.Logger) biz.RemoteTaskRepo {
	return &remoteTaskRepo{
		t:   t,
		db:  db,
		log: log,
	}
}

// Create 创建远程任务
func (r *remoteTaskRepo) Create(task *biz.RemoteTask) error {
	// 检查任务ID是否已存在
	var count int64
	if err := r.db.Model(&biz.RemoteTask{}).Where("task_id = ?", task.TaskID).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New(r.t.Get("task already exists"))
	}

	return r.db.Create(task).Error
}

// Get 根据任务ID获取任务
func (r *remoteTaskRepo) Get(taskID string) (*biz.RemoteTask, error) {
	task := new(biz.RemoteTask)
	err := r.db.Model(&biz.RemoteTask{}).Where("task_id = ?", taskID).First(task).Error
	return task, err
}

// GetByID 根据自增ID获取任务
func (r *remoteTaskRepo) GetByID(id uint) (*biz.RemoteTask, error) {
	task := new(biz.RemoteTask)
	err := r.db.Model(&biz.RemoteTask{}).Where("id = ?", id).First(task).Error
	return task, err
}

// Update 更新任务
func (r *remoteTaskRepo) Update(task *biz.RemoteTask) error {
	return r.db.Save(task).Error
}

// Delete 删除任务
func (r *remoteTaskRepo) Delete(taskID string) error {
	return r.db.Model(&biz.RemoteTask{}).Where("task_id = ?", taskID).Delete(&biz.RemoteTask{}).Error
}

// UpdateStatus 更新任务状态
func (r *remoteTaskRepo) UpdateStatus(taskID string, status biz.TaskStatus) error {
	updates := map[string]interface{}{
		"status": status,
	}

	// 根据状态设置相应的时间戳
	now := time.Now()
	switch status {
	case biz.TaskStatusRunning:
		updates["started_at"] = &now
	case biz.TaskStatusSuccess, biz.TaskStatusFailed:
		updates["completed_at"] = &now
	}

	return r.db.Model(&biz.RemoteTask{}).Where("task_id = ?", taskID).Updates(updates).Error
}

// UpdateProgress 更新任务进度
func (r *remoteTaskRepo) UpdateProgress(taskID string, result string, errMsg string) error {
	updates := map[string]interface{}{}
	
	if result != "" {
		updates["result"] = result
	}
	if errMsg != "" {
		updates["error"] = errMsg
	}

	if len(updates) == 0 {
		return nil
	}

	return r.db.Model(&biz.RemoteTask{}).Where("task_id = ?", taskID).Updates(updates).Error
}

// List 分页列出任务
func (r *remoteTaskRepo) List(page, limit uint) ([]*biz.RemoteTask, int64, error) {
	tasks := make([]*biz.RemoteTask, 0)
	var total int64

	err := r.db.Model(&biz.RemoteTask{}).
		Order("id desc").
		Count(&total).
		Offset(int((page - 1) * limit)).
		Limit(int(limit)).
		Find(&tasks).Error

	return tasks, total, err
}

// ListByStatus 根据状态分页列出任务
func (r *remoteTaskRepo) ListByStatus(status biz.TaskStatus, page, limit uint) ([]*biz.RemoteTask, int64, error) {
	tasks := make([]*biz.RemoteTask, 0)
	var total int64

	err := r.db.Model(&biz.RemoteTask{}).
		Where("status = ?", status).
		Order("id desc").
		Count(&total).
		Offset(int((page - 1) * limit)).
		Limit(int(limit)).
		Find(&tasks).Error

	return tasks, total, err
}

// ListBySource 根据来源分页列出任务
func (r *remoteTaskRepo) ListBySource(source biz.RemoteTaskSource, page, limit uint) ([]*biz.RemoteTask, int64, error) {
	tasks := make([]*biz.RemoteTask, 0)
	var total int64

	err := r.db.Model(&biz.RemoteTask{}).
		Where("source = ?", source).
		Order("id desc").
		Count(&total).
		Offset(int((page - 1) * limit)).
		Limit(int(limit)).
		Find(&tasks).Error

	return tasks, total, err
}

// GetPendingTasks 获取待处理任务
func (r *remoteTaskRepo) GetPendingTasks(limit int) ([]*biz.RemoteTask, error) {
	tasks := make([]*biz.RemoteTask, 0)

	err := r.db.Model(&biz.RemoteTask{}).
		Where("status = ?", biz.TaskStatusWaiting).
		Order("created_at asc").
		Limit(limit).
		Find(&tasks).Error

	return tasks, err
}

// HasRunningTask 检查是否有正在运行的任务
func (r *remoteTaskRepo) HasRunningTask() bool {
	var count int64
	r.db.Model(&biz.RemoteTask{}).
		Where("status = ? OR status = ?", biz.TaskStatusRunning, biz.TaskStatusWaiting).
		Count(&count)
	return count > 0
}

// CountByStatus 根据状态统计任务数量
func (r *remoteTaskRepo) CountByStatus(status biz.TaskStatus) (int64, error) {
	var count int64
	err := r.db.Model(&biz.RemoteTask{}).Where("status = ?", status).Count(&count).Error
	return count, err
}

// ClearZombieTasks 清理僵尸任务
func (r *remoteTaskRepo) ClearZombieTasks() error {
	// 将长时间运行的任务标记为失败
	timeout := time.Now().Add(-30 * time.Minute) // 30分钟超时

	return r.db.Model(&biz.RemoteTask{}).
		Where("status = ? AND started_at < ?", biz.TaskStatusRunning, timeout).
		Updates(map[string]interface{}{
			"status":       biz.TaskStatusFailed,
			"error":        "Task timeout - marked as zombie task",
			"completed_at": time.Now(),
		}).Error
}

// CleanupOldTasks 清理旧任务
func (r *remoteTaskRepo) CleanupOldTasks(days int) error {
	cutoffTime := time.Now().AddDate(0, 0, -days)

	return r.db.Model(&biz.RemoteTask{}).
		Where("completed_at < ? AND (status = ? OR status = ?)", 
			cutoffTime, biz.TaskStatusSuccess, biz.TaskStatusFailed).
		Delete(&biz.RemoteTask{}).Error
}