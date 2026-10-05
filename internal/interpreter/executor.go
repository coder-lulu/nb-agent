package interpreter

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// commandExecutor 命令执行器实现
type commandExecutor struct {
	mu               sync.RWMutex
	activeExecutions map[string]*exec.Cmd
	maxConcurrent    int
	currentCount     int
}

// NewCommandExecutor 创建命令执行器
func NewCommandExecutor() CommandExecutor {
	return &commandExecutor{
		activeExecutions: make(map[string]*exec.Cmd),
		maxConcurrent:    10,
	}
}

// ExecuteCommand 执行命令
func (ce *commandExecutor) ExecuteCommand(ctx context.Context, command string, interpreter *InterpreterInfo, options *ExecutionOptions) (*ExecutionResult, error) {
	if interpreter == nil {
		return nil, fmt.Errorf("interpreter cannot be nil")
	}

	if options == nil {
		options = &ExecutionOptions{}
	}

	// 检查并发执行限制
	if err := ce.checkConcurrencyLimit(); err != nil {
		return nil, err
	}

	// 开始执行
	startTime := time.Now()
	
	// 准备执行环境
	if err := ce.PrepareEnvironment(interpreter, options); err != nil {
		return nil, fmt.Errorf("failed to prepare environment: %w", err)
	}

	// 构建命令
	cmd, err := ce.buildCommand(ctx, command, interpreter, options)
	if err != nil {
		return nil, fmt.Errorf("failed to build command: %w", err)
	}

	// 注册活动执行
	executionID := fmt.Sprintf("%s-%d", options.SessionID, time.Now().UnixNano())
	ce.registerExecution(executionID, cmd)
	defer ce.unregisterExecution(executionID)

	// 执行命令
	result, err := ce.executeCommand(ctx, cmd, options)
	if err != nil {
		return nil, err
	}

	// 设置执行时间和使用的解释器
	result.Duration = time.Since(startTime)
	result.InterpreterUsed = interpreter

	// 清理环境
	if cleanupErr := ce.CleanupEnvironment(interpreter); cleanupErr != nil {
		// 记录清理错误但不影响主要结果
		fmt.Printf("Warning: failed to cleanup environment: %v\n", cleanupErr)
	}

	return result, nil
}

// PrepareEnvironment 准备执行环境
func (ce *commandExecutor) PrepareEnvironment(interpreter *InterpreterInfo, options *ExecutionOptions) error {
	// 验证解释器路径
	if interpreter.Path == "" {
		return fmt.Errorf("interpreter path is empty")
	}

	if _, err := os.Stat(interpreter.Path); err != nil {
		return fmt.Errorf("interpreter not found at path %s: %w", interpreter.Path, err)
	}

	// 验证工作目录
	if options.WorkingDir != "" {
		if _, err := os.Stat(options.WorkingDir); err != nil {
			return fmt.Errorf("working directory not found: %w", err)
		}
	}

	return nil
}

// CleanupEnvironment 清理执行环境
func (ce *commandExecutor) CleanupEnvironment(interpreter *InterpreterInfo) error {
	// 目前没有特殊的清理需求
	// 未来可以在这里添加临时文件清理、环境重置等逻辑
	return nil
}

// buildCommand 构建执行命令
func (ce *commandExecutor) buildCommand(ctx context.Context, command string, interpreter *InterpreterInfo, options *ExecutionOptions) (*exec.Cmd, error) {
	var cmd *exec.Cmd

	// 处理超时
	if options.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, options.Timeout)
		defer cancel()
	}

	// 根据解释器类型构建命令
	switch interpreter.Type {
	case InterpreterBash, InterpreterZsh, InterpreterSh, InterpreterDash, InterpreterAsh, InterpreterKsh:
		cmd = ce.buildPOSIXCommand(ctx, command, interpreter, options)
	case InterpreterFish:
		cmd = ce.buildFishCommand(ctx, command, interpreter, options)
	case InterpreterCsh, InterpreterTcsh:
		cmd = ce.buildCshCommand(ctx, command, interpreter, options)
	default:
		// 默认使用POSIX方式
		cmd = ce.buildPOSIXCommand(ctx, command, interpreter, options)
	}

	// 设置工作目录
	if options.WorkingDir != "" {
		cmd.Dir = options.WorkingDir
	}

	// 设置环境变量
	if options.Environment != nil {
		env := os.Environ()
		for key, value := range options.Environment {
			env = append(env, fmt.Sprintf("%s=%s", key, value))
		}
		cmd.Env = env
	}

	return cmd, nil
}

// buildPOSIXCommand 构建POSIX兼容的命令
func (ce *commandExecutor) buildPOSIXCommand(ctx context.Context, command string, interpreter *InterpreterInfo, options *ExecutionOptions) *exec.Cmd {
	// 检查是否是脚本文件还是命令字符串
	if ce.isScriptFile(command) {
		return exec.CommandContext(ctx, interpreter.Path, command)
	}

	// 对于命令字符串，使用-c参数
	args := []string{"-c", command}
	
	// 某些解释器支持额外的选项
	switch interpreter.Type {
	case InterpreterBash:
		// bash支持更多选项
		if options.InteractiveMode {
			args = []string{"-i", "-c", command}
		}
	case InterpreterZsh:
		// zsh特定选项
		if options.InteractiveMode {
			args = []string{"-i", "-c", command}
		}
	}

	return exec.CommandContext(ctx, interpreter.Path, args...)
}

// buildFishCommand 构建Fish shell命令
func (ce *commandExecutor) buildFishCommand(ctx context.Context, command string, interpreter *InterpreterInfo, options *ExecutionOptions) *exec.Cmd {
	if ce.isScriptFile(command) {
		return exec.CommandContext(ctx, interpreter.Path, command)
	}

	args := []string{"-c", command}
	if options.InteractiveMode {
		args = []string{"-i", "-c", command}
	}

	return exec.CommandContext(ctx, interpreter.Path, args...)
}

// buildCshCommand 构建C shell命令
func (ce *commandExecutor) buildCshCommand(ctx context.Context, command string, interpreter *InterpreterInfo, options *ExecutionOptions) *exec.Cmd {
	if ce.isScriptFile(command) {
		return exec.CommandContext(ctx, interpreter.Path, command)
	}

	// C shell使用不同的语法
	args := []string{"-c", command}
	return exec.CommandContext(ctx, interpreter.Path, args...)
}

// executeCommand 执行命令并收集结果
func (ce *commandExecutor) executeCommand(ctx context.Context, cmd *exec.Cmd, options *ExecutionOptions) (*ExecutionResult, error) {
	result := &ExecutionResult{
		Success: false,
	}

	// 设置标准输入
	if options.StdinInput != "" {
		cmd.Stdin = strings.NewReader(options.StdinInput)
	}

	// 执行命令并捕获输出
	var stdout, stderr strings.Builder
	if options.CaptureOutput {
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
	}

	// 执行命令
	startTime := time.Now()
	err := cmd.Run()
	result.Duration = time.Since(startTime)

	// 设置输出
	if options.CaptureOutput {
		result.Stdout = stdout.String()
		result.Stderr = stderr.String()
	}

	// 处理执行结果
	if err != nil {
		result.Error = err.Error()
		
		// 尝试获取退出码
		if exitError, ok := err.(*exec.ExitError); ok {
			if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
				result.ExitCode = status.ExitStatus()
			} else {
				result.ExitCode = 1
			}
		} else {
			result.ExitCode = 1
		}
	} else {
		result.Success = true
		result.ExitCode = 0
	}

	// 收集资源使用信息（如果可用）
	if cmd.ProcessState != nil {
		if usage := cmd.ProcessState.SysUsage(); usage != nil {
			if rusage, ok := usage.(*syscall.Rusage); ok {
				result.MemoryUsage = rusage.Maxrss * 1024 // 转换为字节
				// CPU使用率计算（简化版本）
				totalTime := float64(rusage.Utime.Sec + rusage.Stime.Sec)
				if result.Duration.Seconds() > 0 {
					result.CPUUsage = (totalTime / result.Duration.Seconds()) * 100
				}
			}
		}
	}

	return result, nil
}

// isScriptFile 检查是否是脚本文件
func (ce *commandExecutor) isScriptFile(command string) bool {
	command = strings.TrimSpace(command)
	
	// 检查是否是文件路径
	if strings.Contains(command, "/") || strings.Contains(command, "\\") {
		if _, err := os.Stat(command); err == nil {
			return true
		}
	}

	// 检查是否是相对路径的脚本文件
	if !strings.Contains(command, " ") && !strings.Contains(command, ";") && !strings.Contains(command, "|") {
		if _, err := os.Stat(command); err == nil {
			return true
		}
		
		// 检查当前目录
		if _, err := os.Stat(filepath.Join(".", command)); err == nil {
			return true
		}
	}

	return false
}

// checkConcurrencyLimit 检查并发执行限制
func (ce *commandExecutor) checkConcurrencyLimit() error {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	if ce.currentCount >= ce.maxConcurrent {
		return fmt.Errorf("maximum concurrent executions (%d) reached", ce.maxConcurrent)
	}

	ce.currentCount++
	return nil
}

// registerExecution 注册活动执行
func (ce *commandExecutor) registerExecution(executionID string, cmd *exec.Cmd) {
	ce.mu.Lock()
	defer ce.mu.Unlock()
	ce.activeExecutions[executionID] = cmd
}

// unregisterExecution 注销活动执行
func (ce *commandExecutor) unregisterExecution(executionID string) {
	ce.mu.Lock()
	defer ce.mu.Unlock()
	delete(ce.activeExecutions, executionID)
	ce.currentCount--
}

// GetActiveExecutions 获取活动执行列表
func (ce *commandExecutor) GetActiveExecutions() map[string]*exec.Cmd {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	result := make(map[string]*exec.Cmd)
	for k, v := range ce.activeExecutions {
		result[k] = v
	}
	return result
}

// KillExecution 终止指定的执行
func (ce *commandExecutor) KillExecution(executionID string) error {
	ce.mu.RLock()
	cmd, exists := ce.activeExecutions[executionID]
	ce.mu.RUnlock()

	if !exists {
		return fmt.Errorf("execution %s not found", executionID)
	}

	if cmd.Process != nil {
		return cmd.Process.Kill()
	}

	return fmt.Errorf("process not available for execution %s", executionID)
}

// KillAllExecutions 终止所有活动执行
func (ce *commandExecutor) KillAllExecutions() error {
	ce.mu.RLock()
	executions := make(map[string]*exec.Cmd)
	for k, v := range ce.activeExecutions {
		executions[k] = v
	}
	ce.mu.RUnlock()

	var lastError error
	for executionID := range executions {
		if err := ce.KillExecution(executionID); err != nil {
			lastError = err
		}
	}

	return lastError
}

// SetMaxConcurrentExecutions 设置最大并发执行数
func (ce *commandExecutor) SetMaxConcurrentExecutions(max int) {
	ce.mu.Lock()
	defer ce.mu.Unlock()
	ce.maxConcurrent = max
}