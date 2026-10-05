package executor

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ScriptType 脚本类型
type ScriptType string

const (
	ScriptTypeBash       ScriptType = "bash"
	ScriptTypePython     ScriptType = "python"
	ScriptTypePython3    ScriptType = "python3"
	ScriptTypePowerShell ScriptType = "powershell"
	ScriptTypeNode       ScriptType = "node"
	ScriptTypePerl       ScriptType = "perl"
	ScriptTypeRuby       ScriptType = "ruby"
)

// scriptExecutor 脚本执行器
type scriptExecutor struct {
	config         *ExecutorConfig
	status         ExecutorStatus
	logger         *slog.Logger
	tempDir        string
	interpreters   map[ScriptType]string
}

// NewScriptExecutor 创建脚本执行器
func NewScriptExecutor(config *ExecutorConfig, logger *slog.Logger) TaskExecutor {
	if logger == nil {
		logger = slog.Default()
	}

	if config == nil {
		config = &ExecutorConfig{
			Type:           ExecutorTypeScript,
			MaxConcurrency: 3,
			DefaultTimeout: 10 * time.Minute,
			WorkingDir:     "",
			Environment:    make(map[string]string),
			Settings:       make(map[string]interface{}),
		}
	}

	executor := &scriptExecutor{
		config:       config,
		status:       ExecutorStatusStopped,
		logger:       logger.With("component", "script_executor"),
		interpreters: make(map[ScriptType]string),
	}

	// 初始化解释器映射
	executor.initializeInterpreters()

	return executor
}

// Initialize 初始化执行器
func (se *scriptExecutor) Initialize() error {
	se.logger.Info("initializing script executor")
	
	// 创建临时目录
	tempDir, err := ioutil.TempDir("", "nb-agent-scripts-")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	se.tempDir = tempDir
	
	// 检查可用的解释器
	se.checkAvailableInterpreters()
	
	return nil
}

// Start 启动执行器
func (se *scriptExecutor) Start(ctx context.Context) error {
	se.logger.Info("starting script executor")
	se.status = ExecutorStatusRunning
	return nil
}

// Stop 停止执行器
func (se *scriptExecutor) Stop() error {
	se.logger.Info("stopping script executor")
	
	// 清理临时目录
	if se.tempDir != "" {
		os.RemoveAll(se.tempDir)
	}
	
	se.status = ExecutorStatusStopped
	return nil
}

// Execute 执行脚本
func (se *scriptExecutor) Execute(ctx context.Context, execution *TaskExecution) (*ExecutionResult, error) {
	se.logger.Info("executing script", 
		"execution_id", execution.ID,
		"script_type", execution.Parameters["script_type"])

	startTime := time.Now()

	// 获取脚本类型
	scriptTypeStr, ok := execution.Parameters["script_type"].(string)
	if !ok {
		return nil, fmt.Errorf("script_type parameter is required")
	}
	scriptType := ScriptType(scriptTypeStr)

	// 获取脚本内容
	scriptContent, ok := execution.Parameters["script_content"].(string)
	if !ok && execution.Command == "" {
		return nil, fmt.Errorf("script_content parameter or command is required")
	}
	
	if scriptContent == "" {
		scriptContent = execution.Command
	}

	// 创建临时脚本文件
	scriptFile, err := se.createTempScript(scriptType, scriptContent)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp script: %w", err)
	}
	defer os.Remove(scriptFile)

	// 准备命令
	cmd, err := se.prepareScriptCommand(scriptType, scriptFile, execution)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare script command: %w", err)
	}

	// 设置输出缓冲区
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// 执行脚本
	if err := cmd.Start(); err != nil {
		return &ExecutionResult{
			Success:  false,
			ExitCode: -1,
			Error:    err.Error(),
			Duration: time.Since(startTime),
		}, err
	}

	// 等待脚本完成或上下文取消
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	var waitErr error
	select {
	case waitErr = <-done:
		// 脚本正常完成
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
			exitCode = exitError.ExitCode()
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

	// 添加脚本元数据
	result.Data["script_type"] = scriptType
	result.Data["script_file"] = scriptFile
	result.Data["exit_code"] = exitCode

	if waitErr != nil && waitErr != context.Canceled && waitErr != context.DeadlineExceeded {
		result.Error += "\nExecution error: " + waitErr.Error()
	}

	se.logger.Info("script execution completed",
		"execution_id", execution.ID,
		"script_type", scriptType,
		"success", result.Success,
		"duration", duration)

	return result, nil
}

// GetStatus 获取执行器状态
func (se *scriptExecutor) GetStatus() ExecutorStatus {
	return se.status
}

// GetSupportedTypes 获取支持的类型
func (se *scriptExecutor) GetSupportedTypes() []ExecutorType {
	return []ExecutorType{ExecutorTypeScript}
}

// UpdateConfig 更新配置
func (se *scriptExecutor) UpdateConfig(config map[string]interface{}) error {
	se.logger.Info("updating script executor config")
	
	// 更新解释器路径
	if interpreters, ok := config["interpreters"].(map[string]string); ok {
		for scriptType, path := range interpreters {
			se.interpreters[ScriptType(scriptType)] = path
		}
	}
	
	return nil
}

// initializeInterpreters 初始化解释器映射
func (se *scriptExecutor) initializeInterpreters() {
	if runtime.GOOS == "windows" {
		se.interpreters[ScriptTypeBash] = "bash"
		se.interpreters[ScriptTypePython] = "python"
		se.interpreters[ScriptTypePython3] = "python"
		se.interpreters[ScriptTypePowerShell] = "powershell"
		se.interpreters[ScriptTypeNode] = "node"
		se.interpreters[ScriptTypePerl] = "perl"
		se.interpreters[ScriptTypeRuby] = "ruby"
	} else {
		se.interpreters[ScriptTypeBash] = "/bin/bash"
		se.interpreters[ScriptTypePython] = "/usr/bin/python"
		se.interpreters[ScriptTypePython3] = "/usr/bin/python3"
		se.interpreters[ScriptTypeNode] = "/usr/bin/node"
		se.interpreters[ScriptTypePerl] = "/usr/bin/perl"
		se.interpreters[ScriptTypeRuby] = "/usr/bin/ruby"
	}
}

// checkAvailableInterpreters 检查可用的解释器
func (se *scriptExecutor) checkAvailableInterpreters() {
	available := make([]string, 0)
	
	for scriptType, interpreterPath := range se.interpreters {
		if _, err := exec.LookPath(interpreterPath); err == nil {
			available = append(available, string(scriptType))
		} else {
			se.logger.Warn("interpreter not found", "type", scriptType, "path", interpreterPath)
		}
	}
	
	se.logger.Info("available script interpreters", "interpreters", available)
}

// createTempScript 创建临时脚本文件
func (se *scriptExecutor) createTempScript(scriptType ScriptType, content string) (string, error) {
	// 确定文件扩展名
	var ext string
	switch scriptType {
	case ScriptTypeBash:
		ext = ".sh"
	case ScriptTypePython, ScriptTypePython3:
		ext = ".py"
	case ScriptTypePowerShell:
		ext = ".ps1"
	case ScriptTypeNode:
		ext = ".js"
	case ScriptTypePerl:
		ext = ".pl"
	case ScriptTypeRuby:
		ext = ".rb"
	default:
		ext = ".txt"
	}

	// 创建临时文件
	tempFile, err := ioutil.TempFile(se.tempDir, fmt.Sprintf("script-*%s", ext))
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	
	// 写入脚本内容
	if _, err := tempFile.WriteString(content); err != nil {
		tempFile.Close()
		os.Remove(tempFile.Name())
		return "", fmt.Errorf("failed to write script content: %w", err)
	}
	
	tempFile.Close()
	
	// 设置执行权限（Unix系统）
	if runtime.GOOS != "windows" {
		if err := os.Chmod(tempFile.Name(), 0755); err != nil {
			os.Remove(tempFile.Name())
			return "", fmt.Errorf("failed to set script permissions: %w", err)
		}
	}
	
	return tempFile.Name(), nil
}

// prepareScriptCommand 准备脚本命令
func (se *scriptExecutor) prepareScriptCommand(scriptType ScriptType, scriptFile string, execution *TaskExecution) (*exec.Cmd, error) {
	interpreterPath, exists := se.interpreters[scriptType]
	if !exists {
		return nil, fmt.Errorf("unsupported script type: %s", scriptType)
	}

	// 检查解释器是否可用
	if _, err := exec.LookPath(interpreterPath); err != nil {
		return nil, fmt.Errorf("interpreter not found: %s", interpreterPath)
	}

	var cmd *exec.Cmd

	// 构建命令
	switch scriptType {
	case ScriptTypePowerShell:
		cmd = exec.Command(interpreterPath, "-ExecutionPolicy", "Bypass", "-File", scriptFile)
	default:
		cmd = exec.Command(interpreterPath, scriptFile)
	}

	// 设置工作目录
	workingDir := execution.WorkingDir
	if workingDir == "" {
		workingDir = se.config.WorkingDir
	}
	if workingDir == "" {
		workingDir = filepath.Dir(scriptFile)
	}
	cmd.Dir = workingDir

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
func (se *scriptExecutor) killProcess(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}

	se.logger.Warn("killing script process due to cancellation", "pid", cmd.Process.Pid)
	
	// 直接杀死进程
	cmd.Process.Kill()
}

// validateScript 验证脚本
func (se *scriptExecutor) validateScript(scriptType ScriptType, content string) error {
	if content == "" {
		return fmt.Errorf("script content cannot be empty")
	}

	// 检查脚本长度
	if len(content) > 1000000 { // 1MB限制
		return fmt.Errorf("script too large (max 1MB)")
	}

	// 基本的安全检查
	dangerousPatterns := []string{
		"rm -rf /",
		"mkfs",
		"dd if=",
		"shutdown",
		"reboot",
	}

	contentLower := strings.ToLower(content)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(contentLower, pattern) {
			return fmt.Errorf("dangerous pattern detected: %s", pattern)
		}
	}

	return nil
}