package platform

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// CommandExecutor 通用命令执行器
type CommandExecutor struct {
	logger Logger
}

// Logger 简化的日志接口
type Logger interface {
	Info(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
}

// NewCommandExecutor 创建命令执行器
func NewCommandExecutor(logger Logger) *CommandExecutor {
	return &CommandExecutor{logger: logger}
}

// Execute 执行命令
func (ce *CommandExecutor) Execute(ctx context.Context, command string, options *CommandOptions) (*CommandResult, error) {
	startTime := time.Now()

	// 解析命令
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(ctx, "/bin/bash", "-c", command)
	}

	// 设置工作目录
	if options != nil && options.WorkingDir != "" {
		cmd.Dir = options.WorkingDir
	}

	// 设置环境变量
	if options != nil && len(options.Environment) > 0 {
		env := os.Environ()
		for k, v := range options.Environment {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		cmd.Env = env
	}

	// 设置输出缓冲区
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// 执行命令
	err := cmd.Run()
	duration := time.Since(startTime)

	// 获取退出码
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
				exitCode = status.ExitStatus()
			}
		}
	}

	result := &CommandResult{
		ExitCode: exitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration,
	}

	if ce.logger != nil {
		ce.logger.Info("command executed",
			"command", command,
			"exit_code", exitCode,
			"duration", duration)
	}

	return result, nil
}

// ExecuteStream 流式执行命令
func (ce *CommandExecutor) ExecuteStream(ctx context.Context, command string, options *CommandOptions, stdout, stderr io.Writer) error {
	// 解析命令
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(ctx, "/bin/bash", "-c", command)
	}

	// 设置工作目录
	if options != nil && options.WorkingDir != "" {
		cmd.Dir = options.WorkingDir
	}

	// 设置环境变量
	if options != nil && len(options.Environment) > 0 {
		env := os.Environ()
		for k, v := range options.Environment {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		cmd.Env = env
	}

	// 设置输出流
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	// 执行命令
	return cmd.Run()
}

// FileUtils 文件工具
type FileUtils struct{}

// NewFileUtils 创建文件工具
func NewFileUtils() *FileUtils {
	return &FileUtils{}
}

// Exists 检查文件或目录是否存在
func (fu *FileUtils) Exists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// IsDirectory 检查是否为目录
func (fu *FileUtils) IsDirectory(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// IsFile 检查是否为文件
func (fu *FileUtils) IsFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// GetFileSize 获取文件大小
func (fu *FileUtils) GetFileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// EnsureDirectory 确保目录存在
func (fu *FileUtils) EnsureDirectory(path string) error {
	return os.MkdirAll(path, 0755)
}

// CopyFile 复制文件
func (fu *FileUtils) CopyFile(src, dst string) error {
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

// FindExecutable 查找可执行文件
func (fu *FileUtils) FindExecutable(name string) (string, error) {
	return exec.LookPath(name)
}

// NetworkUtils 网络工具
type NetworkUtils struct{}

// NewNetworkUtils 创建网络工具
func NewNetworkUtils() *NetworkUtils {
	return &NetworkUtils{}
}

// IsPortOpen 检查端口是否开放
func (nu *NetworkUtils) IsPortOpen(host string, port int) bool {
	address := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()
	return true
}

// GetLocalIPs 获取本地IP地址
func (nu *NetworkUtils) GetLocalIPs() ([]string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}

	var ips []string
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ips = append(ips, ipnet.IP.String())
			}
		}
	}

	return ips, nil
}

// ProcessUtils 进程工具
type ProcessUtils struct{}

// NewProcessUtils 创建进程工具
func NewProcessUtils() *ProcessUtils {
	return &ProcessUtils{}
}

// IsProcessRunning 检查进程是否运行
func (pu *ProcessUtils) IsProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// 在Unix系统上，发送信号0来检查进程是否存在
	if runtime.GOOS != "windows" {
		return process.Signal(syscall.Signal(0)) == nil
	}

	// Windows上的实现可能需要更复杂的逻辑
	return true
}

// GetCurrentPID 获取当前进程PID
func (pu *ProcessUtils) GetCurrentPID() int {
	return os.Getpid()
}

// GetParentPID 获取父进程PID
func (pu *ProcessUtils) GetParentPID() int {
	return os.Getppid()
}

// SystemUtils 系统工具
type SystemUtils struct{}

// NewSystemUtils 创建系统工具
func NewSystemUtils() *SystemUtils {
	return &SystemUtils{}
}

// GetEnvironmentVariable 获取环境变量
func (su *SystemUtils) GetEnvironmentVariable(name string) string {
	return os.Getenv(name)
}

// SetEnvironmentVariable 设置环境变量
func (su *SystemUtils) SetEnvironmentVariable(name, value string) error {
	return os.Setenv(name, value)
}

// GetAllEnvironmentVariables 获取所有环境变量
func (su *SystemUtils) GetAllEnvironmentVariables() []string {
	return os.Environ()
}

// GetTempDirectory 获取临时目录
func (su *SystemUtils) GetTempDirectory() string {
	return os.TempDir()
}

// GetHomeDirectory 获取用户主目录
func (su *SystemUtils) GetHomeDirectory() (string, error) {
	return os.UserHomeDir()
}

// PathUtils 路径工具
type PathUtils struct{}

// NewPathUtils 创建路径工具
func NewPathUtils() *PathUtils {
	return &PathUtils{}
}

// Join 连接路径
func (pu *PathUtils) Join(paths ...string) string {
	return filepath.Join(paths...)
}

// Absolute 获取绝对路径
func (pu *PathUtils) Absolute(path string) (string, error) {
	return filepath.Abs(path)
}

// Base 获取路径的最后一个元素
func (pu *PathUtils) Base(path string) string {
	return filepath.Base(path)
}

// Directory 获取路径的目录部分
func (pu *PathUtils) Directory(path string) string {
	return filepath.Dir(path)
}

// Extension 获取文件扩展名
func (pu *PathUtils) Extension(path string) string {
	return filepath.Ext(path)
}

// Clean 清理路径
func (pu *PathUtils) Clean(path string) string {
	return filepath.Clean(path)
}

// StringUtils 字符串工具
type StringUtils struct{}

// NewStringUtils 创建字符串工具
func NewStringUtils() *StringUtils {
	return &StringUtils{}
}

// IsEmpty 检查字符串是否为空
func (su *StringUtils) IsEmpty(s string) bool {
	return strings.TrimSpace(s) == ""
}

// Split 分割字符串
func (su *StringUtils) Split(s, sep string) []string {
	return strings.Split(s, sep)
}

// Join 连接字符串
func (su *StringUtils) Join(parts []string, sep string) string {
	return strings.Join(parts, sep)
}

// Contains 检查字符串是否包含子串
func (su *StringUtils) Contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// ToInt 字符串转整数
func (su *StringUtils) ToInt(s string) (int, error) {
	return strconv.Atoi(s)
}

// ToInt64 字符串转64位整数
func (su *StringUtils) ToInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

// ToFloat64 字符串转64位浮点数
func (su *StringUtils) ToFloat64(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

// PlatformUtils 平台特定工具集合
type PlatformUtils struct {
	Command *CommandExecutor
	File    *FileUtils
	Network *NetworkUtils
	Process *ProcessUtils
	System  *SystemUtils
	Path    *PathUtils
	String  *StringUtils
}

// NewPlatformUtils 创建平台工具集合
func NewPlatformUtils(logger Logger) *PlatformUtils {
	return &PlatformUtils{
		Command: NewCommandExecutor(logger),
		File:    NewFileUtils(),
		Network: NewNetworkUtils(),
		Process: NewProcessUtils(),
		System:  NewSystemUtils(),
		Path:    NewPathUtils(),
		String:  NewStringUtils(),
	}
}

// GetPlatformSpecificPath 获取平台特定路径
func GetPlatformSpecificPath(basePath string) string {
	switch runtime.GOOS {
	case "windows":
		return strings.ReplaceAll(basePath, "/", "\\")
	default:
		return basePath
	}
}

// GetPlatformSpecificExecutable 获取平台特定可执行文件名
func GetPlatformSpecificExecutable(name string) string {
	if runtime.GOOS == "windows" && !strings.HasSuffix(name, ".exe") {
		return name + ".exe"
	}
	return name
}

// IsSupportedPlatform 检查是否为支持的平台
func IsSupportedPlatform(platform Platform) bool {
	supportedPlatforms := []Platform{
		PlatformLinux,
		PlatformWindows,
		PlatformMacOS,
		PlatformFreeBSD,
	}

	for _, supported := range supportedPlatforms {
		if platform == supported {
			return true
		}
	}

	return false
}

// GetSystemLanguage 获取系统语言
func GetSystemLanguage() string {
	// 尝试从环境变量获取语言设置
	if lang := os.Getenv("LANG"); lang != "" {
		return strings.Split(lang, ".")[0]
	}
	if lang := os.Getenv("LANGUAGE"); lang != "" {
		return strings.Split(lang, ":")[0]
	}
	
	// 默认返回英语
	return "en_US"
}

// GetSystemTimezone 获取系统时区
func GetSystemTimezone() string {
	// 尝试从环境变量获取时区
	if tz := os.Getenv("TZ"); tz != "" {
		return tz
	}
	
	// 获取系统时区
	now := time.Now()
	zone, _ := now.Zone()
	return zone
}