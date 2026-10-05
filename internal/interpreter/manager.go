package interpreter

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// interpreterManager 解释器管理器实现
type interpreterManager struct {
	mu                  sync.RWMutex
	detector            SystemDetector
	executor            CommandExecutor
	validator           InterpreterValidator
	config              *InterpreterConfig
	userPreferences     map[string]*InterpreterPreference
	availableInterpreters []*InterpreterInfo
	statistics          *InterpreterStatistics
	preferencesFile     string
	lastRefresh         time.Time
}

// NewInterpreterManager 创建解释器管理器
func NewInterpreterManager(config *InterpreterConfig) InterpreterManager {
	if config == nil {
		config = DefaultInterpreterConfig()
	}

	return &interpreterManager{
		detector:        NewSystemDetector(),
		executor:        NewCommandExecutor(),
		validator:       NewInterpreterValidator(),
		config:          config,
		userPreferences: make(map[string]*InterpreterPreference),
		statistics:      &InterpreterStatistics{
			UsageByInterpreter:  make(map[InterpreterType]int64),
			UsageByUser:         make(map[string]int64),
			ErrorsByInterpreter: make(map[InterpreterType]int64),
			LastUpdate:          time.Now(),
		},
		preferencesFile: "interpreter_preferences.json",
	}
}

// Initialize 初始化管理器
func (im *interpreterManager) Initialize() error {
	im.mu.Lock()
	defer im.mu.Unlock()

	// 检测可用解释器
	interpreters, err := im.detector.DetectAvailableInterpreters()
	if err != nil {
		return fmt.Errorf("failed to detect interpreters: %w", err)
	}
	im.availableInterpreters = interpreters
	im.lastRefresh = time.Now()

	// 加载用户首选项
	if err := im.loadUserPreferences(); err != nil {
		// 如果加载失败，继续执行但记录错误
		fmt.Printf("Warning: failed to load user preferences: %v\n", err)
	}

	// 初始化统计信息
	im.initializeStatistics()

	return nil
}

// SetUserPreference 设置用户首选解释器
func (im *interpreterManager) SetUserPreference(userID string, interpreterType InterpreterType) error {
	im.mu.Lock()
	defer im.mu.Unlock()

	// 验证解释器是否可用
	available, err := im.detector.IsInterpreterAvailable(interpreterType)
	if err != nil {
		return fmt.Errorf("failed to check interpreter availability: %w", err)
	}
	if !available {
		return fmt.Errorf("interpreter %s is not available on this system", interpreterType)
	}

	// 创建或更新用户首选项
	preference := &InterpreterPreference{
		UserID:        userID,
		PreferredType: interpreterType,
		FallbackTypes: im.generateFallbackOrder(interpreterType),
		AutoDetect:    true,
		LastUpdated:   time.Now(),
	}

	im.userPreferences[userID] = preference

	// 保存到文件
	if err := im.saveUserPreferences(); err != nil {
		return fmt.Errorf("failed to save user preferences: %w", err)
	}

	return nil
}

// GetUserPreference 获取用户首选解释器
func (im *interpreterManager) GetUserPreference(userID string) (InterpreterType, error) {
	im.mu.RLock()
	defer im.mu.RUnlock()

	if preference, exists := im.userPreferences[userID]; exists {
		return preference.PreferredType, nil
	}

	// 如果用户没有设置首选项，返回默认解释器
	return im.config.DefaultInterpreter, nil
}

// SelectBestInterpreter 选择最佳解释器
func (im *interpreterManager) SelectBestInterpreter(userID string, preferredType InterpreterType, command string) (*InterpreterInfo, error) {
	im.mu.RLock()
	defer im.mu.RUnlock()

	// 检查是否需要刷新解释器信息
	if time.Since(im.lastRefresh) > im.config.RefreshInterval {
		im.mu.RUnlock()
		if err := im.RefreshInterpreters(); err != nil {
			return nil, fmt.Errorf("failed to refresh interpreters: %w", err)
		}
		im.mu.RLock()
	}

	// 构建候选解释器列表
	candidates := im.buildCandidateList(userID, preferredType, command)

	// 选择最佳候选
	for _, interpreter := range candidates {
		if interpreter.Available {
			// 验证解释器与命令的兼容性
			if im.validator != nil {
				if validation, err := im.validator.ValidateCommand(command, interpreter); err == nil && validation.Valid {
					return interpreter, nil
				}
			} else {
				// 如果没有验证器，直接返回第一个可用的
				return interpreter, nil
			}
		}
	}

	return nil, fmt.Errorf("no suitable interpreter found for command")
}

// GetAvailableInterpreters 获取可用解释器列表
func (im *interpreterManager) GetAvailableInterpreters() ([]*InterpreterInfo, error) {
	im.mu.RLock()
	defer im.mu.RUnlock()

	if im.availableInterpreters == nil {
		im.mu.RUnlock()
		return im.RefreshInterpreters()
	}

	// 创建副本以避免并发修改
	interpreters := make([]*InterpreterInfo, len(im.availableInterpreters))
	copy(interpreters, im.availableInterpreters)

	return interpreters, nil
}

// RefreshInterpreters 刷新解释器信息
func (im *interpreterManager) RefreshInterpreters() ([]*InterpreterInfo, error) {
	interpreters, err := im.detector.DetectAvailableInterpreters()
	if err != nil {
		return nil, fmt.Errorf("failed to detect interpreters: %w", err)
	}

	im.mu.Lock()
	im.availableInterpreters = interpreters
	im.lastRefresh = time.Now()
	im.mu.Unlock()

	return interpreters, nil
}

// GetUsageStatistics 获取解释器统计信息
func (im *interpreterManager) GetUsageStatistics() (*InterpreterStatistics, error) {
	im.mu.RLock()
	defer im.mu.RUnlock()

	// 创建统计信息副本
	stats := &InterpreterStatistics{
		TotalExecutions:      im.statistics.TotalExecutions,
		UsageByInterpreter:   make(map[InterpreterType]int64),
		UsageByUser:          make(map[string]int64),
		SuccessRate:          im.statistics.SuccessRate,
		AverageExecutionTime: im.statistics.AverageExecutionTime,
		LastUpdate:           im.statistics.LastUpdate,
		ErrorsByInterpreter:  make(map[InterpreterType]int64),
	}

	for k, v := range im.statistics.UsageByInterpreter {
		stats.UsageByInterpreter[k] = v
	}
	for k, v := range im.statistics.UsageByUser {
		stats.UsageByUser[k] = v
	}
	for k, v := range im.statistics.ErrorsByInterpreter {
		stats.ErrorsByInterpreter[k] = v
	}

	return stats, nil
}

// UpdateStatistics 更新统计信息
func (im *interpreterManager) UpdateStatistics(userID string, interpreter *InterpreterInfo, success bool, duration time.Duration) {
	im.mu.Lock()
	defer im.mu.Unlock()

	im.statistics.TotalExecutions++
	im.statistics.UsageByInterpreter[interpreter.Type]++
	im.statistics.UsageByUser[userID]++

	if !success {
		im.statistics.ErrorsByInterpreter[interpreter.Type]++
	}

	// 更新平均执行时间
	if im.statistics.TotalExecutions == 1 {
		im.statistics.AverageExecutionTime = duration
	} else {
		// 使用移动平均
		im.statistics.AverageExecutionTime = time.Duration(
			(int64(im.statistics.AverageExecutionTime)*9 + int64(duration)) / 10,
		)
	}

	// 计算成功率
	totalErrors := int64(0)
	for _, errors := range im.statistics.ErrorsByInterpreter {
		totalErrors += errors
	}
	im.statistics.SuccessRate = float64(im.statistics.TotalExecutions-totalErrors) / float64(im.statistics.TotalExecutions) * 100

	im.statistics.LastUpdate = time.Now()
}

// buildCandidateList 构建候选解释器列表
func (im *interpreterManager) buildCandidateList(userID string, preferredType InterpreterType, command string) []*InterpreterInfo {
	var candidates []*InterpreterInfo

	// 1. 如果指定了首选类型，优先考虑
	if preferredType != "" {
		if interpreter := im.findInterpreterByType(preferredType); interpreter != nil {
			candidates = append(candidates, interpreter)
		}
	}

	// 2. 检查用户首选项
	if preference, exists := im.userPreferences[userID]; exists {
		if interpreter := im.findInterpreterByType(preference.PreferredType); interpreter != nil {
			// 避免重复添加
			if preferredType != preference.PreferredType {
				candidates = append(candidates, interpreter)
			}
		}

		// 添加用户的回退选项
		for _, fallbackType := range preference.FallbackTypes {
			if interpreter := im.findInterpreterByType(fallbackType); interpreter != nil {
				candidates = append(candidates, interpreter)
			}
		}
	}

	// 3. 基于命令类型选择合适的解释器
	if commandInterpreter := im.selectInterpreterForCommand(command); commandInterpreter != nil {
		candidates = append(candidates, commandInterpreter)
	}

	// 4. 添加系统回退顺序
	for _, fallbackType := range im.config.FallbackOrder {
		if interpreter := im.findInterpreterByType(fallbackType); interpreter != nil {
			candidates = append(candidates, interpreter)
		}
	}

	// 5. 最后添加所有可用解释器（按优先级排序）
	candidates = append(candidates, im.availableInterpreters...)

	// 去重
	return im.deduplicateCandidates(candidates)
}

// findInterpreterByType 根据类型查找解释器
func (im *interpreterManager) findInterpreterByType(interpreterType InterpreterType) *InterpreterInfo {
	for _, interpreter := range im.availableInterpreters {
		if interpreter.Type == interpreterType {
			return interpreter
		}
	}
	return nil
}

// selectInterpreterForCommand 基于命令选择合适的解释器
func (im *interpreterManager) selectInterpreterForCommand(command string) *InterpreterInfo {
	command = strings.TrimSpace(strings.ToLower(command))

	// 如果命令以shebang开始，解析指定的解释器
	if strings.HasPrefix(command, "#!") {
		if interpreterType := im.parseShebang(command); interpreterType != "" {
			return im.findInterpreterByType(interpreterType)
		}
	}

	// 基于命令内容的启发式选择
	if strings.Contains(command, "[[") || strings.Contains(command, "bash") {
		return im.findInterpreterByType(InterpreterBash)
	}

	if strings.Contains(command, "fish") {
		return im.findInterpreterByType(InterpreterFish)
	}

	if strings.Contains(command, "zsh") {
		return im.findInterpreterByType(InterpreterZsh)
	}

	return nil
}

// parseShebang 解析shebang行
func (im *interpreterManager) parseShebang(command string) InterpreterType {
	lines := strings.Split(command, "\n")
	if len(lines) == 0 {
		return ""
	}

	shebang := strings.TrimSpace(lines[0])
	if !strings.HasPrefix(shebang, "#!") {
		return ""
	}

	shebang = strings.TrimPrefix(shebang, "#!")
	shebang = strings.TrimSpace(shebang)

	// 解析解释器路径
	parts := strings.Fields(shebang)
	if len(parts) == 0 {
		return ""
	}

	interpreterPath := parts[0]
	interpreterName := filepath.Base(interpreterPath)

	// 映射到解释器类型
	switch interpreterName {
	case "bash":
		return InterpreterBash
	case "zsh":
		return InterpreterZsh
	case "fish":
		return InterpreterFish
	case "sh":
		return InterpreterSh
	case "dash":
		return InterpreterDash
	case "ash":
		return InterpreterAsh
	case "csh":
		return InterpreterCsh
	case "tcsh":
		return InterpreterTcsh
	case "ksh":
		return InterpreterKsh
	default:
		return ""
	}
}

// deduplicateCandidates 去重候选解释器列表
func (im *interpreterManager) deduplicateCandidates(candidates []*InterpreterInfo) []*InterpreterInfo {
	seen := make(map[InterpreterType]bool)
	var result []*InterpreterInfo

	for _, candidate := range candidates {
		if !seen[candidate.Type] {
			seen[candidate.Type] = true
			result = append(result, candidate)
		}
	}

	return result
}

// generateFallbackOrder 生成回退顺序
func (im *interpreterManager) generateFallbackOrder(preferredType InterpreterType) []InterpreterType {
	fallback := []InterpreterType{}

	// 基于首选类型生成智能回退顺序
	switch preferredType {
	case InterpreterBash:
		fallback = []InterpreterType{InterpreterSh, InterpreterDash, InterpreterZsh}
	case InterpreterZsh:
		fallback = []InterpreterType{InterpreterBash, InterpreterSh}
	case InterpreterFish:
		fallback = []InterpreterType{InterpreterBash, InterpreterZsh, InterpreterSh}
	default:
		fallback = []InterpreterType{InterpreterBash, InterpreterSh, InterpreterDash}
	}

	return fallback
}

// loadUserPreferences 加载用户首选项
func (im *interpreterManager) loadUserPreferences() error {
	data, err := os.ReadFile(im.preferencesFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 文件不存在是正常的
		}
		return err
	}

	var preferences map[string]*InterpreterPreference
	if err := json.Unmarshal(data, &preferences); err != nil {
		return err
	}

	im.userPreferences = preferences
	return nil
}

// saveUserPreferences 保存用户首选项
func (im *interpreterManager) saveUserPreferences() error {
	data, err := json.MarshalIndent(im.userPreferences, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(im.preferencesFile, data, 0644)
}

// initializeStatistics 初始化统计信息
func (im *interpreterManager) initializeStatistics() {
	im.statistics.UsageByInterpreter = make(map[InterpreterType]int64)
	im.statistics.UsageByUser = make(map[string]int64)
	im.statistics.ErrorsByInterpreter = make(map[InterpreterType]int64)

	// 为所有可用解释器初始化计数器
	for _, interpreter := range im.availableInterpreters {
		im.statistics.UsageByInterpreter[interpreter.Type] = 0
		im.statistics.ErrorsByInterpreter[interpreter.Type] = 0
	}
}

// DefaultInterpreterConfig 默认解释器配置
func DefaultInterpreterConfig() *InterpreterConfig {
	return &InterpreterConfig{
		DefaultInterpreter:    InterpreterBash,
		EnableAutoDetection:   true,
		RefreshInterval:       5 * time.Minute,
		CacheExpiry:          30 * time.Minute,
		FallbackOrder: []InterpreterType{
			InterpreterBash,
			InterpreterSh,
			InterpreterDash,
			InterpreterZsh,
			InterpreterAsh,
		},
		InterpreterPaths:        make(map[InterpreterType]string),
		FeatureRequirements:     make(map[string][]InterpreterFeature),
		ExecutionTimeout:        30 * time.Second,
		MaxConcurrentExecutions: 10,
	}
}