package security

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// AuditLogger 审计日志记录器接口
type AuditLogger interface {
	// 记录审计事件
	LogEvent(event *AuditEvent) error
	
	// 查询审计事件
	GetEvents(filter *AuditFilter) ([]*AuditEvent, error)
	
	// 清理过期事件
	CleanupExpiredEvents() error
	
	// 导出审计日志
	ExportEvents(filter *AuditFilter, format string) ([]byte, error)
	
	// 获取统计信息
	GetStatistics() (*AuditStatistics, error)
}

// AuditStatistics 审计统计信息
type AuditStatistics struct {
	TotalEvents        int64                     `json:"total_events"`
	EventsByType       map[AuditEventType]int64  `json:"events_by_type"`
	EventsByUser       map[string]int64          `json:"events_by_user"`
	EventsByLevel      map[SecurityLevel]int64   `json:"events_by_level"`
	EventsByHour       map[int]int64             `json:"events_by_hour"`
	LastEvent          *time.Time                `json:"last_event"`
	OldestEvent        *time.Time                `json:"oldest_event"`
	StorageSize        int64                     `json:"storage_size_bytes"`
}

// auditLogger 审计日志记录器实现
type auditLogger struct {
	policy      *SecurityPolicy
	logger      *slog.Logger
	
	// 存储配置
	logDir      string
	maxFileSize int64
	
	// 运行时状态
	currentFile *os.File
	fileSize    int64
	mu          sync.RWMutex
	
	// 内存缓存（可选）
	enableCache bool
	eventCache  []*AuditEvent
	cacheSize   int
	cacheMu     sync.RWMutex
}

// NewAuditLogger 创建审计日志记录器
func NewAuditLogger(policy *SecurityPolicy, logger *slog.Logger) AuditLogger {
	if logger == nil {
		logger = slog.Default()
	}

	al := &auditLogger{
		policy:      policy,
		logger:      logger.With("component", "audit_logger"),
		logDir:      "/var/log/nb-agent/audit",
		maxFileSize: 100 * 1024 * 1024, // 100MB
		enableCache: true,
		cacheSize:   1000,
		eventCache:  make([]*AuditEvent, 0),
	}

	// 确保日志目录存在
	if err := al.ensureLogDirectory(); err != nil {
		al.logger.Error("failed to create audit log directory", "error", err)
	}

	// 打开当前日志文件
	if err := al.openCurrentLogFile(); err != nil {
		al.logger.Error("failed to open audit log file", "error", err)
	}

	al.logger.Info("audit logger initialized", "log_dir", al.logDir)
	return al
}

// LogEvent 记录审计事件
func (al *auditLogger) LogEvent(event *AuditEvent) error {
	al.mu.Lock()
	defer al.mu.Unlock()

	// 设置事件ID和时间戳
	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// 序列化事件
	eventData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal audit event: %w", err)
	}

	// 写入日志文件
	if err := al.writeToFile(eventData); err != nil {
		return fmt.Errorf("failed to write audit event to file: %w", err)
	}

	// 添加到内存缓存
	if al.enableCache {
		al.addToCache(event)
	}

	al.logger.Debug("audit event logged",
		"event_id", event.ID,
		"event_type", event.EventType,
		"user", event.User)

	return nil
}

// GetEvents 查询审计事件
func (al *auditLogger) GetEvents(filter *AuditFilter) ([]*AuditEvent, error) {
	al.mu.RLock()
	defer al.mu.RUnlock()

	// 首先检查内存缓存
	if al.enableCache && al.isFilterCacheCompatible(filter) {
		return al.getEventsFromCache(filter), nil
	}

	// 从文件系统读取
	return al.getEventsFromFiles(filter)
}

// CleanupExpiredEvents 清理过期事件
func (al *auditLogger) CleanupExpiredEvents() error {
	al.mu.Lock()
	defer al.mu.Unlock()

	if al.policy.LogRetentionDays <= 0 {
		return nil // 不清理
	}

	cutoffTime := time.Now().AddDate(0, 0, -al.policy.LogRetentionDays)
	
	// 获取所有日志文件
	files, err := al.getLogFiles()
	if err != nil {
		return fmt.Errorf("failed to get log files: %w", err)
	}

	deletedCount := 0
	for _, file := range files {
		// 检查文件修改时间
		info, err := os.Stat(file)
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoffTime) {
			if err := os.Remove(file); err != nil {
				al.logger.Warn("failed to remove expired audit log file",
					"file", file, "error", err)
			} else {
				deletedCount++
				al.logger.Info("removed expired audit log file", "file", file)
			}
		}
	}

	// 清理内存缓存中的过期事件
	if al.enableCache {
		al.cleanupCacheExpiredEvents(cutoffTime)
	}

	al.logger.Info("audit log cleanup completed", "deleted_files", deletedCount)
	return nil
}

// ExportEvents 导出审计事件
func (al *auditLogger) ExportEvents(filter *AuditFilter, format string) ([]byte, error) {
	events, err := al.GetEvents(filter)
	if err != nil {
		return nil, fmt.Errorf("failed to get events: %w", err)
	}

	switch format {
	case "json":
		return json.MarshalIndent(events, "", "  ")
	case "csv":
		return al.eventsToCSV(events), nil
	default:
		return nil, fmt.Errorf("unsupported export format: %s", format)
	}
}

// GetStatistics 获取统计信息
func (al *auditLogger) GetStatistics() (*AuditStatistics, error) {
	al.mu.RLock()
	defer al.mu.RUnlock()

	stats := &AuditStatistics{
		EventsByType:  make(map[AuditEventType]int64),
		EventsByUser:  make(map[string]int64),
		EventsByLevel: make(map[SecurityLevel]int64),
		EventsByHour:  make(map[int]int64),
	}

	// 从缓存计算统计信息（如果启用）
	if al.enableCache {
		al.calculateStatsFromCache(stats)
	} else {
		// 从文件计算统计信息
		if err := al.calculateStatsFromFiles(stats); err != nil {
			return nil, fmt.Errorf("failed to calculate statistics: %w", err)
		}
	}

	// 计算存储大小
	if size, err := al.calculateStorageSize(); err == nil {
		stats.StorageSize = size
	}

	return stats, nil
}

// ensureLogDirectory 确保日志目录存在
func (al *auditLogger) ensureLogDirectory() error {
	return os.MkdirAll(al.logDir, 0755)
}

// openCurrentLogFile 打开当前日志文件
func (al *auditLogger) openCurrentLogFile() error {
	filename := al.getCurrentLogFileName()
	
	// 检查文件是否存在以及大小
	if info, err := os.Stat(filename); err == nil {
		al.fileSize = info.Size()
		if al.fileSize >= al.maxFileSize {
			// 轮转日志文件
			if err := al.rotateLogFile(); err != nil {
				return fmt.Errorf("failed to rotate log file: %w", err)
			}
			filename = al.getCurrentLogFileName()
			al.fileSize = 0
		}
	} else {
		al.fileSize = 0
	}

	// 打开文件
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open audit log file %s: %w", filename, err)
	}

	// 关闭之前的文件
	if al.currentFile != nil {
		al.currentFile.Close()
	}

	al.currentFile = file
	return nil
}

// getCurrentLogFileName 获取当前日志文件名
func (al *auditLogger) getCurrentLogFileName() string {
	now := time.Now()
	filename := fmt.Sprintf("audit-%s.log", now.Format("2006-01-02"))
	return filepath.Join(al.logDir, filename)
}

// rotateLogFile 轮转日志文件
func (al *auditLogger) rotateLogFile() error {
	if al.currentFile != nil {
		al.currentFile.Close()
		al.currentFile = nil
	}

	// 重命名当前文件
	currentName := al.getCurrentLogFileName()
	rotatedName := fmt.Sprintf("%s.%d", currentName, time.Now().Unix())
	
	if err := os.Rename(currentName, rotatedName); err != nil {
		return fmt.Errorf("failed to rotate log file: %w", err)
	}

	al.logger.Info("audit log file rotated", "from", currentName, "to", rotatedName)
	return nil
}

// writeToFile 写入日志文件
func (al *auditLogger) writeToFile(data []byte) error {
	// 检查是否需要轮转文件
	if al.fileSize+int64(len(data)) >= al.maxFileSize {
		if err := al.rotateLogFile(); err != nil {
			return err
		}
		if err := al.openCurrentLogFile(); err != nil {
			return err
		}
	}

	// 确保文件已打开
	if al.currentFile == nil {
		if err := al.openCurrentLogFile(); err != nil {
			return err
		}
	}

	// 写入数据
	data = append(data, '\n')
	n, err := al.currentFile.Write(data)
	if err != nil {
		return err
	}

	al.fileSize += int64(n)
	
	// 强制刷新到磁盘
	return al.currentFile.Sync()
}

// addToCache 添加到内存缓存
func (al *auditLogger) addToCache(event *AuditEvent) {
	al.cacheMu.Lock()
	defer al.cacheMu.Unlock()

	// 添加事件到缓存
	al.eventCache = append(al.eventCache, event)

	// 检查缓存大小，移除旧事件
	if len(al.eventCache) > al.cacheSize {
		// 移除最旧的事件
		copy(al.eventCache, al.eventCache[1:])
		al.eventCache = al.eventCache[:al.cacheSize]
	}
}

// isFilterCacheCompatible 检查过滤器是否适合使用缓存
func (al *auditLogger) isFilterCacheCompatible(filter *AuditFilter) bool {
	if filter == nil {
		return true
	}

	// 如果时间范围太大，不使用缓存
	if filter.StartTime != nil && time.Since(*filter.StartTime) > 24*time.Hour {
		return false
	}

	// 如果请求的数量超过缓存大小，不使用缓存
	if filter.Limit > al.cacheSize {
		return false
	}

	return true
}

// getEventsFromCache 从缓存获取事件
func (al *auditLogger) getEventsFromCache(filter *AuditFilter) []*AuditEvent {
	al.cacheMu.RLock()
	defer al.cacheMu.RUnlock()

	var results []*AuditEvent

	for _, event := range al.eventCache {
		if al.matchesFilter(event, filter) {
			results = append(results, event)
		}
	}

	// 按时间排序（最新的在前）
	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp.After(results[j].Timestamp)
	})

	// 应用限制和偏移
	if filter != nil {
		if filter.Offset > 0 && filter.Offset < len(results) {
			results = results[filter.Offset:]
		}
		if filter.Limit > 0 && filter.Limit < len(results) {
			results = results[:filter.Limit]
		}
	}

	return results
}

// getEventsFromFiles 从文件获取事件
func (al *auditLogger) getEventsFromFiles(filter *AuditFilter) ([]*AuditEvent, error) {
	files, err := al.getLogFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to get log files: %w", err)
	}

	var allEvents []*AuditEvent

	for _, file := range files {
		events, err := al.readEventsFromFile(file)
		if err != nil {
			al.logger.Warn("failed to read events from file", "file", file, "error", err)
			continue
		}

		// 应用过滤器
		for _, event := range events {
			if al.matchesFilter(event, filter) {
				allEvents = append(allEvents, event)
			}
		}
	}

	// 按时间排序
	sort.Slice(allEvents, func(i, j int) bool {
		return allEvents[i].Timestamp.After(allEvents[j].Timestamp)
	})

	// 应用限制和偏移
	if filter != nil {
		if filter.Offset > 0 && filter.Offset < len(allEvents) {
			allEvents = allEvents[filter.Offset:]
		}
		if filter.Limit > 0 && filter.Limit < len(allEvents) {
			allEvents = allEvents[:filter.Limit]
		}
	}

	return allEvents, nil
}

// getLogFiles 获取所有日志文件
func (al *auditLogger) getLogFiles() ([]string, error) {
	files, err := filepath.Glob(filepath.Join(al.logDir, "audit-*.log*"))
	if err != nil {
		return nil, err
	}

	// 按修改时间排序（最新的在前）
	sort.Slice(files, func(i, j int) bool {
		info1, err1 := os.Stat(files[i])
		info2, err2 := os.Stat(files[j])
		if err1 != nil || err2 != nil {
			return false
		}
		return info1.ModTime().After(info2.ModTime())
	})

	return files, nil
}

// readEventsFromFile 从文件读取事件
func (al *auditLogger) readEventsFromFile(filename string) ([]*AuditEvent, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var events []*AuditEvent
	decoder := json.NewDecoder(file)

	for decoder.More() {
		var event AuditEvent
		if err := decoder.Decode(&event); err != nil {
			al.logger.Warn("failed to decode audit event", "file", filename, "error", err)
			continue
		}
		events = append(events, &event)
	}

	return events, nil
}

// matchesFilter 检查事件是否匹配过滤器
func (al *auditLogger) matchesFilter(event *AuditEvent, filter *AuditFilter) bool {
	if filter == nil {
		return true
	}

	// 时间范围过滤
	if filter.StartTime != nil && event.Timestamp.Before(*filter.StartTime) {
		return false
	}
	if filter.EndTime != nil && event.Timestamp.After(*filter.EndTime) {
		return false
	}

	// 事件类型过滤
	if len(filter.EventTypes) > 0 {
		found := false
		for _, eventType := range filter.EventTypes {
			if event.EventType == eventType {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 安全级别过滤
	if filter.SecurityLevel != nil && event.SecurityLevel != *filter.SecurityLevel {
		return false
	}

	// 用户过滤
	if filter.User != "" && event.User != filter.User {
		return false
	}

	// 命令过滤
	if filter.Command != "" && event.Command != filter.Command {
		return false
	}

	// 风险级别过滤
	if filter.RiskLevel != nil && event.RiskLevel != *filter.RiskLevel {
		return false
	}

	return true
}

// cleanupCacheExpiredEvents 清理缓存中的过期事件
func (al *auditLogger) cleanupCacheExpiredEvents(cutoffTime time.Time) {
	al.cacheMu.Lock()
	defer al.cacheMu.Unlock()

	var validEvents []*AuditEvent
	for _, event := range al.eventCache {
		if event.Timestamp.After(cutoffTime) {
			validEvents = append(validEvents, event)
		}
	}

	al.eventCache = validEvents
}

// eventsToCSV 将事件转换为CSV格式
func (al *auditLogger) eventsToCSV(events []*AuditEvent) []byte {
	var result []byte
	
	// CSV头部
	header := "ID,Timestamp,EventType,SecurityLevel,User,Command,RiskLevel,Duration\n"
	result = append(result, []byte(header)...)

	// 事件数据
	for _, event := range events {
		line := fmt.Sprintf("%s,%s,%s,%d,%s,%s,%d,%d\n",
			event.ID,
			event.Timestamp.Format(time.RFC3339),
			event.EventType,
			event.SecurityLevel,
			event.User,
			event.Command,
			event.RiskLevel,
			event.Duration,
		)
		result = append(result, []byte(line)...)
	}

	return result
}

// calculateStatsFromCache 从缓存计算统计信息
func (al *auditLogger) calculateStatsFromCache(stats *AuditStatistics) {
	al.cacheMu.RLock()
	defer al.cacheMu.RUnlock()

	stats.TotalEvents = int64(len(al.eventCache))

	for _, event := range al.eventCache {
		// 按类型统计
		stats.EventsByType[event.EventType]++
		
		// 按用户统计
		stats.EventsByUser[event.User]++
		
		// 按安全级别统计
		stats.EventsByLevel[event.SecurityLevel]++
		
		// 按小时统计
		hour := event.Timestamp.Hour()
		stats.EventsByHour[hour]++
		
		// 更新最新和最旧事件时间
		if stats.LastEvent == nil || event.Timestamp.After(*stats.LastEvent) {
			stats.LastEvent = &event.Timestamp
		}
		if stats.OldestEvent == nil || event.Timestamp.Before(*stats.OldestEvent) {
			stats.OldestEvent = &event.Timestamp
		}
	}
}

// calculateStatsFromFiles 从文件计算统计信息
func (al *auditLogger) calculateStatsFromFiles(stats *AuditStatistics) error {
	files, err := al.getLogFiles()
	if err != nil {
		return err
	}

	for _, file := range files {
		events, err := al.readEventsFromFile(file)
		if err != nil {
			continue
		}

		stats.TotalEvents += int64(len(events))

		for _, event := range events {
			// 统计逻辑与缓存相同
			stats.EventsByType[event.EventType]++
			stats.EventsByUser[event.User]++
			stats.EventsByLevel[event.SecurityLevel]++
			
			hour := event.Timestamp.Hour()
			stats.EventsByHour[hour]++
			
			if stats.LastEvent == nil || event.Timestamp.After(*stats.LastEvent) {
				stats.LastEvent = &event.Timestamp
			}
			if stats.OldestEvent == nil || event.Timestamp.Before(*stats.OldestEvent) {
				stats.OldestEvent = &event.Timestamp
			}
		}
	}

	return nil
}

// calculateStorageSize 计算存储大小
func (al *auditLogger) calculateStorageSize() (int64, error) {
	files, err := al.getLogFiles()
	if err != nil {
		return 0, err
	}

	var totalSize int64
	for _, file := range files {
		if info, err := os.Stat(file); err == nil {
			totalSize += info.Size()
		}
	}

	return totalSize, nil
}