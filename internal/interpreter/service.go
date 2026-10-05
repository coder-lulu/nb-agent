package interpreter

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Service 解释器服务
type Service struct {
	mu         sync.RWMutex
	manager    InterpreterManager
	executor   CommandExecutor
	validator  InterpreterValidator
	detector   SystemDetector
	config     *InterpreterConfig
	initialized bool
}

// NewService 创建解释器服务
func NewService(config *InterpreterConfig) *Service {
	if config == nil {
		config = DefaultInterpreterConfig()
	}

	return &Service{
		config: config,
	}
}

// Initialize 初始化服务
func (s *Service) Initialize() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.initialized {
		return nil
	}

	// 初始化各个组件
	s.detector = NewSystemDetector()
	s.executor = NewCommandExecutor()
	s.validator = NewInterpreterValidator()
	s.manager = NewInterpreterManager(s.config)

	// 初始化管理器
	if err := s.manager.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize interpreter manager: %w", err)
	}

	s.initialized = true
	return nil
}

// ExecuteCommand 执行命令的主要入口点
func (s *Service) ExecuteCommand(ctx context.Context, request *ExecutionRequest) (*ExecutionResult, error) {
	if !s.initialized {
		if err := s.Initialize(); err != nil {
			return nil, fmt.Errorf("failed to initialize service: %w", err)
		}
	}

	// 选择最佳解释器
	interpreter, err := s.manager.SelectBestInterpreter(
		request.UserID,
		request.PreferredInterpreter,
		request.Command,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to select interpreter: %w", err)
	}

	// 验证命令与解释器兼容性
	if s.validator != nil {
		validation, err := s.validator.ValidateCommand(request.Command, interpreter)
		if err != nil {
			return nil, fmt.Errorf("failed to validate command: %w", err)
		}
		if !validation.Valid {
			return &ExecutionResult{
				Success: false,
				Error:   fmt.Sprintf("command validation failed: %v", validation.Errors),
			}, nil
		}
	}

	// 准备执行选项
	options := &ExecutionOptions{
		WorkingDir:      request.WorkingDir,
		Environment:     request.Environment,
		Timeout:         request.Timeout,
		InteractiveMode: request.InteractiveMode,
		CaptureOutput:   request.CaptureOutput,
		StdinInput:      request.StdinInput,
		UserID:          request.UserID,
		SessionID:       request.SessionID,
	}

	// 验证执行环境
	if s.validator != nil {
		envValidation, err := s.validator.ValidateEnvironment(interpreter, options)
		if err != nil {
			return nil, fmt.Errorf("failed to validate environment: %w", err)
		}
		if !envValidation.Valid {
			return &ExecutionResult{
				Success: false,
				Error:   fmt.Sprintf("environment validation failed: %v", envValidation.Errors),
			}, nil
		}
	}

	// 执行命令
	result, err := s.executor.ExecuteCommand(ctx, request.Command, interpreter, options)
	if err != nil {
		return nil, fmt.Errorf("failed to execute command: %w", err)
	}

	// 更新统计信息
	s.manager.UpdateStatistics(request.UserID, interpreter, result.Success, result.Duration)

	return result, nil
}

// GetAvailableInterpreters 获取可用解释器列表
func (s *Service) GetAvailableInterpreters() ([]*InterpreterInfo, error) {
	if !s.initialized {
		if err := s.Initialize(); err != nil {
			return nil, fmt.Errorf("failed to initialize service: %w", err)
		}
	}

	return s.manager.GetAvailableInterpreters()
}

// SetUserPreference 设置用户首选解释器
func (s *Service) SetUserPreference(userID string, interpreterType InterpreterType) error {
	if !s.initialized {
		if err := s.Initialize(); err != nil {
			return fmt.Errorf("failed to initialize service: %w", err)
		}
	}

	return s.manager.SetUserPreference(userID, interpreterType)
}

// GetUserPreference 获取用户首选解释器
func (s *Service) GetUserPreference(userID string) (InterpreterType, error) {
	if !s.initialized {
		if err := s.Initialize(); err != nil {
			return "", fmt.Errorf("failed to initialize service: %w", err)
		}
	}

	return s.manager.GetUserPreference(userID)
}

// GetUsageStatistics 获取使用统计
func (s *Service) GetUsageStatistics() (*InterpreterStatistics, error) {
	if !s.initialized {
		if err := s.Initialize(); err != nil {
			return nil, fmt.Errorf("failed to initialize service: %w", err)
		}
	}

	return s.manager.GetUsageStatistics()
}

// RefreshInterpreters 刷新解释器信息
func (s *Service) RefreshInterpreters() error {
	if !s.initialized {
		if err := s.Initialize(); err != nil {
			return fmt.Errorf("failed to initialize service: %w", err)
		}
	}

	_, err := s.manager.RefreshInterpreters()
	return err
}

// ValidateInterpreter 验证解释器健康状态
func (s *Service) ValidateInterpreter(interpreterType InterpreterType) (*ValidationResult, error) {
	if !s.initialized {
		if err := s.Initialize(); err != nil {
			return nil, fmt.Errorf("failed to initialize service: %w", err)
		}
	}

	interpreters, err := s.manager.GetAvailableInterpreters()
	if err != nil {
		return nil, fmt.Errorf("failed to get available interpreters: %w", err)
	}

	for _, interpreter := range interpreters {
		if interpreter.Type == interpreterType {
			return s.validator.CheckInterpreterHealth(interpreter)
		}
	}

	return &ValidationResult{
		Valid:  false,
		Errors: []string{fmt.Sprintf("interpreter %s not found", interpreterType)},
	}, nil
}

// GetInterpreterInfo 获取指定解释器信息
func (s *Service) GetInterpreterInfo(interpreterType InterpreterType) (*InterpreterInfo, error) {
	interpreters, err := s.GetAvailableInterpreters()
	if err != nil {
		return nil, err
	}

	for _, interpreter := range interpreters {
		if interpreter.Type == interpreterType {
			return interpreter, nil
		}
	}

	return nil, fmt.Errorf("interpreter %s not found", interpreterType)
}

// DetectBestInterpreter 为特定命令检测最佳解释器
func (s *Service) DetectBestInterpreter(command string, userID string) (*InterpreterInfo, error) {
	if !s.initialized {
		if err := s.Initialize(); err != nil {
			return nil, fmt.Errorf("failed to initialize service: %w", err)
		}
	}

	return s.manager.SelectBestInterpreter(userID, "", command)
}

// GetSystemInfo 获取系统解释器信息
func (s *Service) GetSystemInfo() (*SystemInterpreterInfo, error) {
	if !s.initialized {
		if err := s.Initialize(); err != nil {
			return nil, fmt.Errorf("failed to initialize service: %w", err)
		}
	}

	interpreters, err := s.detector.DetectAvailableInterpreters()
	if err != nil {
		return nil, err
	}

	defaultInterpreter, err := s.detector.GetDefaultInterpreter()
	if err != nil {
		return nil, err
	}

	return &SystemInterpreterInfo{
		AvailableInterpreters: interpreters,
		DefaultInterpreter:    defaultInterpreter,
		TotalCount:           len(interpreters),
		SystemType:           "Linux",
	}, nil
}

// Shutdown 关闭服务
func (s *Service) Shutdown() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.initialized {
		return nil
	}

	// 这里可以添加清理逻辑
	// 例如保存状态、关闭连接等

	s.initialized = false
	return nil
}

// ExecutionRequest 执行请求
type ExecutionRequest struct {
	Command              string            `json:"command"`
	UserID               string            `json:"user_id"`
	SessionID            string            `json:"session_id"`
	PreferredInterpreter InterpreterType   `json:"preferred_interpreter,omitempty"`
	WorkingDir           string            `json:"working_dir,omitempty"`
	Environment          map[string]string `json:"environment,omitempty"`
	Timeout              time.Duration     `json:"timeout,omitempty"`
	InteractiveMode      bool              `json:"interactive_mode,omitempty"`
	CaptureOutput        bool              `json:"capture_output"`
	StdinInput           string            `json:"stdin_input,omitempty"`
}

// SystemInterpreterInfo 系统解释器信息
type SystemInterpreterInfo struct {
	AvailableInterpreters []*InterpreterInfo `json:"available_interpreters"`
	DefaultInterpreter    *InterpreterInfo   `json:"default_interpreter"`
	TotalCount           int                 `json:"total_count"`
	SystemType           string              `json:"system_type"`
}

// InterpreterServiceConfig 解释器服务配置
type InterpreterServiceConfig struct {
	*InterpreterConfig
	EnableValidation bool `yaml:"enable_validation"`
	EnableStatistics bool `yaml:"enable_statistics"`
	EnableCaching    bool `yaml:"enable_caching"`
}