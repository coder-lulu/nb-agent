package executor

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"
)

// shellExecutor Shell命令执行器
type shellExecutor struct {
	config *ExecutorConfig
	status ExecutorStatus
	logger *slog.Logger
}

// NewShellExecutor 创建Shell执行器
func NewShellExecutor(config *ExecutorConfig, logger *slog.Logger) TaskExecutor {
	if logger == nil {
		logger = slog.Default()
	}

	if config == nil {
		config = &ExecutorConfig{
			Type:           ExecutorTypeShell,
			MaxConcurrency: 5,
			DefaultTimeout: 5 * time.Minute,
			WorkingDir:     "",
			Environment:    make(map[string]string),
			Settings:       make(map[string]interface{}),
		}
	}

	return &shellExecutor{
		config: config,
		status: ExecutorStatusStopped,
		logger: logger.With("component", "shell_executor"),
	}
}

// Initialize 初始化执行器
func (se *shellExecutor) Initialize() error {
	se.logger.Info("initializing shell executor")
	// Shell执行器通常不需要特殊初始化
	return nil
}

// Start 启动执行器
func (se *shellExecutor) Start(ctx context.Context) error {
	se.logger.Info("starting shell executor")
	se.status = ExecutorStatusRunning
	return nil
}

// Stop 停止执行器
func (se *shellExecutor) Stop() error {
	se.logger.Info("stopping shell executor")
	se.status = ExecutorStatusStopped
	return nil
}

// Execute 执行Shell命令
func (se *shellExecutor) Execute(ctx context.Context, execution *TaskExecution) (*ExecutionResult, error) {
	se.logger.Info("executing shell command", 
		"execution_id", execution.ID,
		"command", execution.Command,
		"working_dir", execution.WorkingDir)

	startTime := time.Now()

	// 准备命令
	cmd, err := se.prepareCommand(execution)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare command: %w", err)
	}

	// 设置输出缓冲区
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// 启动命令
	if err := cmd.Start(); err != nil {
		return &ExecutionResult{
			Success:  false,
			ExitCode: -1,
			Error:    err.Error(),
			Duration: time.Since(startTime),
		}, err
	}

	// 等待命令完成或上下文取消
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	var waitErr error
	select {
	case waitErr = <-done:
		// 命令正常完成
	case <-ctx.Done():
		// 上下文取消，需要终止进程
		se.killProcess(cmd)
		<-done // 等待进程清理完成
		waitErr = ctx.Err()
	}

	duration := time.Since(startTime)
	
	// 获取退出码
	exitCode := 0
	if waitErr != nil {
		if exitError, ok := waitErr.(*exec.ExitError); ok {
			if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
				exitCode = status.ExitStatus()
			}
		}
	}

	// 构建执行结果
	result := &ExecutionResult{
		Success:  waitErr == nil && exitCode == 0,
		ExitCode: exitCode,
		Output:   stdout.String(),
		Error:    stderr.String(),
		Duration: duration,
		Data:     make(map[string]interface{}),
	}

	// 添加执行元数据
	result.Data["command"] = execution.Command
	result.Data["working_dir"] = execution.WorkingDir
	result.Data["exit_code"] = exitCode

	if waitErr != nil && waitErr != context.Canceled && waitErr != context.DeadlineExceeded {
		result.Error += "\nExecution error: " + waitErr.Error()
	}

	se.logger.Info("shell command execution completed",
		"execution_id", execution.ID,
		"success", result.Success,
		"exit_code", exitCode,
		"duration", duration)

	return result, nil
}

// GetStatus 获取执行器状态
func (se *shellExecutor) GetStatus() ExecutorStatus {
	return se.status
}

// GetSupportedTypes 获取支持的类型
func (se *shellExecutor) GetSupportedTypes() []ExecutorType {
	return []ExecutorType{ExecutorTypeShell}
}

// UpdateConfig 更新配置
func (se *shellExecutor) UpdateConfig(config map[string]interface{}) error {
	se.logger.Info("updating shell executor config")
	
	// 更新工作目录
	if workingDir, ok := config["working_dir"].(string); ok {
		se.config.WorkingDir = workingDir
	}
	
	// 更新环境变量
	if env, ok := config["environment"].(map[string]string); ok {
		se.config.Environment = env
	}
	
	// 更新超时时间
	if timeout, ok := config["default_timeout"].(time.Duration); ok {
		se.config.DefaultTimeout = timeout
	}

	return nil
}

// prepareCommand 准备命令
func (se *shellExecutor) prepareCommand(execution *TaskExecution) (*exec.Cmd, error) {
	var cmd *exec.Cmd

	// 根据操作系统选择Shell
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/C", execution.Command)
	default:
		// Unix-like系统 (Linux, macOS, etc.)
		cmd = exec.Command("/bin/bash", "-c", execution.Command)
	}

	// 设置工作目录
	workingDir := execution.WorkingDir
	if workingDir == "" {
		workingDir = se.config.WorkingDir
	}
	if workingDir != "" {
		cmd.Dir = workingDir
	}

	// 设置环境变量
	env := os.Environ()
	
	// 添加配置中的环境变量
	for k, v := range se.config.Environment {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	
	// 添加执行特定的环境变量
	for k, v := range execution.Environment {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	
	cmd.Env = env

	return cmd, nil
}

// killProcess 终止进程
func (se *shellExecutor) killProcess(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}

	se.logger.Warn("killing process due to cancellation", "pid", cmd.Process.Pid)

	// 尝试优雅终止
	if runtime.GOOS == "windows" {
		cmd.Process.Kill()
	} else {
		cmd.Process.Signal(syscall.SIGTERM)
		
		// 给进程一些时间优雅退出
		time.Sleep(1 * time.Second)
		
		// 如果还没退出，强制杀死
		cmd.Process.Kill()
	}
}

// parseShellCommand 解析Shell命令
func (se *shellExecutor) parseShellCommand(command string) (string, []string) {
	// 简单的命令解析，支持基本的参数分割
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "", nil
	}
	
	return parts[0], parts[1:]
}

// validateCommand 验证命令
func (se *shellExecutor) validateCommand(execution *TaskExecution) error {
	if execution.Command == "" {
		return fmt.Errorf("command cannot be empty")
	}

	// 检查命令长度
	if len(execution.Command) > 10000 {
		return fmt.Errorf("command too long (max 10000 characters)")
	}

	// 可以添加更多安全检查，比如禁止某些危险命令
	dangerousCommands := []string{"rm -rf /", "mkfs", "dd if=", ":(){ :|:& };:"}
	commandLower := strings.ToLower(execution.Command)
	
	for _, dangerous := range dangerousCommands {
		if strings.Contains(commandLower, dangerous) {
			return fmt.Errorf("dangerous command detected: %s", dangerous)
		}
	}

	return nil
}