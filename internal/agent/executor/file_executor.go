package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FileOperation 文件操作类型
type FileOperation string

const (
	FileOpCopy       FileOperation = "copy"
	FileOpMove       FileOperation = "move"
	FileOpDelete     FileOperation = "delete"
	FileOpCreate     FileOperation = "create"
	FileOpRead       FileOperation = "read"
	FileOpWrite      FileOperation = "write"
	FileOpChmod      FileOperation = "chmod"
	FileOpChown      FileOperation = "chown"
	FileOpList       FileOperation = "list"
	FileOpStat       FileOperation = "stat"
	FileOpMkdir      FileOperation = "mkdir"
	FileOpCompress   FileOperation = "compress"
	FileOpDecompress FileOperation = "decompress"
)

// fileExecutor 文件操作执行器
type fileExecutor struct {
	config    *ExecutorConfig
	status    ExecutorStatus
	logger    *slog.Logger
	baseDir   string
	allowedDirs []string
}

// FileOperationParams 文件操作参数
type FileOperationParams struct {
	Operation   FileOperation `json:"operation"`
	Source      string        `json:"source"`
	Destination string        `json:"destination"`
	Content     string        `json:"content"`
	Mode        string        `json:"mode"`
	Owner       string        `json:"owner"`
	Recursive   bool          `json:"recursive"`
	Force       bool          `json:"force"`
	CreateDirs  bool          `json:"create_dirs"`
}

// FileInfo 文件信息
type FileInfo struct {
	Name    string      `json:"name"`
	Path    string      `json:"path"`
	Size    int64       `json:"size"`
	Mode    os.FileMode `json:"mode"`
	ModTime time.Time   `json:"mod_time"`
	IsDir   bool        `json:"is_dir"`
}

// NewFileExecutor 创建文件执行器
func NewFileExecutor(config *ExecutorConfig, logger *slog.Logger) TaskExecutor {
	if logger == nil {
		logger = slog.Default()
	}

	if config == nil {
		config = &ExecutorConfig{
			Type:           ExecutorTypeFile,
			MaxConcurrency: 5,
			DefaultTimeout: 5 * time.Minute,
			WorkingDir:     "",
			Environment:    make(map[string]string),
			Settings:       make(map[string]interface{}),
		}
	}

	executor := &fileExecutor{
		config:      config,
		status:      ExecutorStatusStopped,
		logger:      logger.With("component", "file_executor"),
		allowedDirs: []string{"/tmp", "/var/tmp"},
	}

	// 从配置中获取基础目录和允许的目录
	if baseDir, ok := config.Settings["base_dir"].(string); ok {
		executor.baseDir = baseDir
	}
	
	if allowedDirs, ok := config.Settings["allowed_dirs"].([]string); ok {
		executor.allowedDirs = allowedDirs
	}

	return executor
}

// Initialize 初始化执行器
func (fe *fileExecutor) Initialize() error {
	fe.logger.Info("initializing file executor")
	
	// 验证基础目录
	if fe.baseDir != "" {
		if _, err := os.Stat(fe.baseDir); os.IsNotExist(err) {
			return fmt.Errorf("base directory does not exist: %s", fe.baseDir)
		}
	}
	
	return nil
}

// Start 启动执行器
func (fe *fileExecutor) Start(ctx context.Context) error {
	fe.logger.Info("starting file executor")
	fe.status = ExecutorStatusRunning
	return nil
}

// Stop 停止执行器
func (fe *fileExecutor) Stop() error {
	fe.logger.Info("stopping file executor")
	fe.status = ExecutorStatusStopped
	return nil
}

// Execute 执行文件操作
func (fe *fileExecutor) Execute(ctx context.Context, execution *TaskExecution) (*ExecutionResult, error) {
	fe.logger.Info("executing file operation", 
		"execution_id", execution.ID)

	startTime := time.Now()

	// 解析操作参数
	params, err := fe.parseParameters(execution.Parameters)
	if err != nil {
		return nil, fmt.Errorf("failed to parse parameters: %w", err)
	}

	// 验证路径安全性
	if err := fe.validatePaths(params); err != nil {
		return nil, fmt.Errorf("path validation failed: %w", err)
	}

	// 执行文件操作
	result, err := fe.executeOperation(ctx, params)
	
	duration := time.Since(startTime)
	
	if result == nil {
		result = &ExecutionResult{
			Data: make(map[string]interface{}),
		}
	}
	
	result.Duration = duration
	result.Success = err == nil
	
	if err != nil {
		result.Error = err.Error()
		fe.logger.Error("file operation failed",
			"execution_id", execution.ID,
			"operation", params.Operation,
			"error", err)
	} else {
		fe.logger.Info("file operation completed",
			"execution_id", execution.ID,
			"operation", params.Operation,
			"duration", duration)
	}

	return result, err
}

// GetStatus 获取执行器状态
func (fe *fileExecutor) GetStatus() ExecutorStatus {
	return fe.status
}

// GetSupportedTypes 获取支持的类型
func (fe *fileExecutor) GetSupportedTypes() []ExecutorType {
	return []ExecutorType{ExecutorTypeFile}
}

// UpdateConfig 更新配置
func (fe *fileExecutor) UpdateConfig(config map[string]interface{}) error {
	fe.logger.Info("updating file executor config")
	
	if baseDir, ok := config["base_dir"].(string); ok {
		fe.baseDir = baseDir
	}
	
	if allowedDirs, ok := config["allowed_dirs"].([]string); ok {
		fe.allowedDirs = allowedDirs
	}
	
	return nil
}

// parseParameters 解析参数
func (fe *fileExecutor) parseParameters(params map[string]interface{}) (*FileOperationParams, error) {
	// 将参数转换为JSON后再解析
	data, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal parameters: %w", err)
	}
	
	var opParams FileOperationParams
	if err := json.Unmarshal(data, &opParams); err != nil {
		return nil, fmt.Errorf("failed to unmarshal parameters: %w", err)
	}
	
	return &opParams, nil
}

// validatePaths 验证路径安全性
func (fe *fileExecutor) validatePaths(params *FileOperationParams) error {
	// 检查源路径
	if params.Source != "" {
		if err := fe.validatePath(params.Source); err != nil {
			return fmt.Errorf("invalid source path: %w", err)
		}
	}
	
	// 检查目标路径
	if params.Destination != "" {
		if err := fe.validatePath(params.Destination); err != nil {
			return fmt.Errorf("invalid destination path: %w", err)
		}
	}
	
	return nil
}

// validatePath 验证单个路径
func (fe *fileExecutor) validatePath(path string) error {
	// 清理路径
	cleanPath := filepath.Clean(path)
	
	// 检查是否包含危险字符
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("path traversal not allowed: %s", path)
	}
	
	// 如果设置了基础目录，检查路径是否在基础目录内
	if fe.baseDir != "" {
		if !strings.HasPrefix(cleanPath, fe.baseDir) {
			return fmt.Errorf("path outside base directory: %s", path)
		}
	}
	
	// 检查是否在允许的目录列表中
	if len(fe.allowedDirs) > 0 {
		allowed := false
		for _, allowedDir := range fe.allowedDirs {
			if strings.HasPrefix(cleanPath, allowedDir) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("path not in allowed directories: %s", path)
		}
	}
	
	return nil
}

// executeOperation 执行文件操作
func (fe *fileExecutor) executeOperation(ctx context.Context, params *FileOperationParams) (*ExecutionResult, error) {
	switch params.Operation {
	case FileOpCopy:
		return fe.copyFile(params)
	case FileOpMove:
		return fe.moveFile(params)
	case FileOpDelete:
		return fe.deleteFile(params)
	case FileOpCreate:
		return fe.createFile(params)
	case FileOpRead:
		return fe.readFile(params)
	case FileOpWrite:
		return fe.writeFile(params)
	case FileOpList:
		return fe.listFiles(params)
	case FileOpStat:
		return fe.statFile(params)
	case FileOpMkdir:
		return fe.createDir(params)
	default:
		return nil, fmt.Errorf("unsupported operation: %s", params.Operation)
	}
}

// copyFile 复制文件
func (fe *fileExecutor) copyFile(params *FileOperationParams) (*ExecutionResult, error) {
	if params.Source == "" || params.Destination == "" {
		return nil, fmt.Errorf("source and destination are required for copy operation")
	}

	// 检查源文件是否存在
	sourceInfo, err := os.Stat(params.Source)
	if err != nil {
		return nil, fmt.Errorf("source file error: %w", err)
	}

	// 创建目标目录（如果需要）
	if params.CreateDirs {
		destDir := filepath.Dir(params.Destination)
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create destination directory: %w", err)
		}
	}

	// 复制文件
	if sourceInfo.IsDir() {
		if params.Recursive {
			err = fe.copyDir(params.Source, params.Destination)
		} else {
			return nil, fmt.Errorf("source is a directory, use recursive flag")
		}
	} else {
		err = fe.copyFileContent(params.Source, params.Destination)
	}

	if err != nil {
		return nil, err
	}

	return &ExecutionResult{
		Success: true,
		Data: map[string]interface{}{
			"operation": "copy",
			"source":    params.Source,
			"destination": params.Destination,
		},
	}, nil
}

// moveFile 移动文件
func (fe *fileExecutor) moveFile(params *FileOperationParams) (*ExecutionResult, error) {
	if params.Source == "" || params.Destination == "" {
		return nil, fmt.Errorf("source and destination are required for move operation")
	}

	// 创建目标目录（如果需要）
	if params.CreateDirs {
		destDir := filepath.Dir(params.Destination)
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create destination directory: %w", err)
		}
	}

	if err := os.Rename(params.Source, params.Destination); err != nil {
		return nil, fmt.Errorf("failed to move file: %w", err)
	}

	return &ExecutionResult{
		Success: true,
		Data: map[string]interface{}{
			"operation": "move",
			"source":    params.Source,
			"destination": params.Destination,
		},
	}, nil
}

// deleteFile 删除文件
func (fe *fileExecutor) deleteFile(params *FileOperationParams) (*ExecutionResult, error) {
	if params.Source == "" {
		return nil, fmt.Errorf("source is required for delete operation")
	}

	var err error
	if params.Recursive {
		err = os.RemoveAll(params.Source)
	} else {
		err = os.Remove(params.Source)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to delete file: %w", err)
	}

	return &ExecutionResult{
		Success: true,
		Data: map[string]interface{}{
			"operation": "delete",
			"path":      params.Source,
		},
	}, nil
}

// createFile 创建文件
func (fe *fileExecutor) createFile(params *FileOperationParams) (*ExecutionResult, error) {
	if params.Destination == "" {
		return nil, fmt.Errorf("destination is required for create operation")
	}

	// 创建目录（如果需要）
	if params.CreateDirs {
		destDir := filepath.Dir(params.Destination)
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory: %w", err)
		}
	}

	file, err := os.Create(params.Destination)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// 写入内容（如果提供）
	if params.Content != "" {
		if _, err := file.WriteString(params.Content); err != nil {
			return nil, fmt.Errorf("failed to write content: %w", err)
		}
	}

	return &ExecutionResult{
		Success: true,
		Data: map[string]interface{}{
			"operation": "create",
			"path":      params.Destination,
			"size":      len(params.Content),
		},
	}, nil
}

// readFile 读取文件
func (fe *fileExecutor) readFile(params *FileOperationParams) (*ExecutionResult, error) {
	if params.Source == "" {
		return nil, fmt.Errorf("source is required for read operation")
	}

	content, err := os.ReadFile(params.Source)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return &ExecutionResult{
		Success: true,
		Output:  string(content),
		Data: map[string]interface{}{
			"operation": "read",
			"path":      params.Source,
			"size":      len(content),
		},
	}, nil
}

// writeFile 写入文件
func (fe *fileExecutor) writeFile(params *FileOperationParams) (*ExecutionResult, error) {
	if params.Destination == "" {
		return nil, fmt.Errorf("destination is required for write operation")
	}

	if err := os.WriteFile(params.Destination, []byte(params.Content), 0644); err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	return &ExecutionResult{
		Success: true,
		Data: map[string]interface{}{
			"operation": "write",
			"path":      params.Destination,
			"size":      len(params.Content),
		},
	}, nil
}

// listFiles 列出文件
func (fe *fileExecutor) listFiles(params *FileOperationParams) (*ExecutionResult, error) {
	path := params.Source
	if path == "" {
		path = "."
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("failed to list directory: %w", err)
	}

	files := make([]FileInfo, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		files = append(files, FileInfo{
			Name:    info.Name(),
			Path:    filepath.Join(path, info.Name()),
			Size:    info.Size(),
			Mode:    info.Mode(),
			ModTime: info.ModTime(),
			IsDir:   info.IsDir(),
		})
	}

	return &ExecutionResult{
		Success: true,
		Data: map[string]interface{}{
			"operation": "list",
			"path":      path,
			"files":     files,
			"count":     len(files),
		},
	}, nil
}

// statFile 获取文件状态
func (fe *fileExecutor) statFile(params *FileOperationParams) (*ExecutionResult, error) {
	if params.Source == "" {
		return nil, fmt.Errorf("source is required for stat operation")
	}

	info, err := os.Stat(params.Source)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	fileInfo := FileInfo{
		Name:    info.Name(),
		Path:    params.Source,
		Size:    info.Size(),
		Mode:    info.Mode(),
		ModTime: info.ModTime(),
		IsDir:   info.IsDir(),
	}

	return &ExecutionResult{
		Success: true,
		Data: map[string]interface{}{
			"operation": "stat",
			"file":      fileInfo,
		},
	}, nil
}

// createDir 创建目录
func (fe *fileExecutor) createDir(params *FileOperationParams) (*ExecutionResult, error) {
	if params.Destination == "" {
		return nil, fmt.Errorf("destination is required for mkdir operation")
	}

	var err error
	if params.Recursive {
		err = os.MkdirAll(params.Destination, 0755)
	} else {
		err = os.Mkdir(params.Destination, 0755)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	return &ExecutionResult{
		Success: true,
		Data: map[string]interface{}{
			"operation": "mkdir",
			"path":      params.Destination,
		},
	}, nil
}

// copyFileContent 复制文件内容
func (fe *fileExecutor) copyFileContent(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// copyDir 复制目录
func (fe *fileExecutor) copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 计算相对路径
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}

		return fe.copyFileContent(path, destPath)
	})
}