package platform

import (
	"context"
	"io"
	"time"
)

// Platform 平台类型
type Platform string

const (
	PlatformLinux   Platform = "linux"
	PlatformWindows Platform = "windows"
	PlatformMacOS   Platform = "darwin"
	PlatformFreeBSD Platform = "freebsd"
	PlatformUnknown Platform = "unknown"
)

// Architecture 架构类型
type Architecture string

const (
	ArchAMD64 Architecture = "amd64"
	ArchARM64 Architecture = "arm64"
	Arch386   Architecture = "386"
	ArchARM   Architecture = "arm"
)

// SystemInfo 系统信息
type SystemInfo struct {
	Platform         Platform     `json:"platform"`
	Architecture     Architecture `json:"architecture"`
	Hostname         string       `json:"hostname"`
	KernelVersion    string       `json:"kernel_version"`
	OSVersion        string       `json:"os_version"`
	CPUCores         int          `json:"cpu_cores"`
	TotalMemory      int64        `json:"total_memory"`
	AvailableMemory  int64        `json:"available_memory"`
	TotalDisk        int64        `json:"total_disk"`
	AvailableDisk    int64        `json:"available_disk"`
	Uptime           time.Duration `json:"uptime"`
	LoadAverage      []float64    `json:"load_average"`
}

// ProcessInfo 进程信息
type ProcessInfo struct {
	PID         int       `json:"pid"`
	PPID        int       `json:"ppid"`
	Name        string    `json:"name"`
	Cmdline     string    `json:"cmdline"`
	Status      string    `json:"status"`
	CPUPercent  float64   `json:"cpu_percent"`
	MemoryUsage int64     `json:"memory_usage"`
	StartTime   time.Time `json:"start_time"`
	Username    string    `json:"username"`
}

// ServiceInfo 服务信息
type ServiceInfo struct {
	Name        string            `json:"name"`
	Status      ServiceStatus     `json:"status"`
	PID         int               `json:"pid"`
	Description string            `json:"description"`
	Properties  map[string]string `json:"properties"`
}

// ServiceStatus 服务状态
type ServiceStatus string

const (
	ServiceStatusRunning  ServiceStatus = "running"
	ServiceStatusStopped  ServiceStatus = "stopped"
	ServiceStatusFailed   ServiceStatus = "failed"
	ServiceStatusDisabled ServiceStatus = "disabled"
	ServiceStatusUnknown  ServiceStatus = "unknown"
)

// NetworkInfo 网络信息
type NetworkInfo struct {
	Interface   string   `json:"interface"`
	IPAddresses []string `json:"ip_addresses"`
	MACAddress  string   `json:"mac_address"`
	Status      string   `json:"status"`
	MTU         int      `json:"mtu"`
	BytesRecv   int64    `json:"bytes_recv"`
	BytesSent   int64    `json:"bytes_sent"`
	PacketsRecv int64    `json:"packets_recv"`
	PacketsSent int64    `json:"packets_sent"`
}

// UserInfo 用户信息
type UserInfo struct {
	Username string   `json:"username"`
	UID      int      `json:"uid"`
	GID      int      `json:"gid"`
	HomeDir  string   `json:"home_dir"`
	Shell    string   `json:"shell"`
	Groups   []string `json:"groups"`
}

// PackageInfo 软件包信息
type PackageInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Architecture string `json:"architecture"`
	Size        int64  `json:"size"`
	Installed   bool   `json:"installed"`
}

// CommandResult 命令执行结果
type CommandResult struct {
	ExitCode int           `json:"exit_code"`
	Stdout   string        `json:"stdout"`
	Stderr   string        `json:"stderr"`
	Duration time.Duration `json:"duration"`
}

// CommandOptions 命令执行选项
type CommandOptions struct {
	WorkingDir  string            `json:"working_dir"`
	Environment map[string]string `json:"environment"`
	Timeout     time.Duration     `json:"timeout"`
	User        string            `json:"user"`
	Group       string            `json:"group"`
}

// SystemManager 系统管理器接口
type SystemManager interface {
	// 系统信息
	GetSystemInfo() (*SystemInfo, error)
	GetHostname() (string, error)
	SetHostname(hostname string) error
	GetUptime() (time.Duration, error)
	Reboot() error
	Shutdown() error
	
	// 命令执行
	ExecuteCommand(ctx context.Context, command string, options *CommandOptions) (*CommandResult, error)
	ExecuteCommandStream(ctx context.Context, command string, options *CommandOptions, stdout, stderr io.Writer) error
}

// ProcessManager 进程管理器接口
type ProcessManager interface {
	// 进程查询
	ListProcesses() ([]*ProcessInfo, error)
	GetProcess(pid int) (*ProcessInfo, error)
	GetProcessesByName(name string) ([]*ProcessInfo, error)
	
	// 进程控制
	KillProcess(pid int) error
	KillProcessByName(name string) error
	StartProcess(command string, args []string, options *CommandOptions) (int, error)
}

// ServiceManager 服务管理器接口
type ServiceManager interface {
	// 服务查询
	ListServices() ([]*ServiceInfo, error)
	GetService(name string) (*ServiceInfo, error)
	
	// 服务控制
	StartService(name string) error
	StopService(name string) error
	RestartService(name string) error
	EnableService(name string) error
	DisableService(name string) error
	
	// 服务状态
	IsServiceRunning(name string) (bool, error)
	GetServiceStatus(name string) (ServiceStatus, error)
}

// NetworkManager 网络管理器接口
type NetworkManager interface {
	// 网络信息
	GetNetworkInterfaces() ([]*NetworkInfo, error)
	GetNetworkInterface(name string) (*NetworkInfo, error)
	
	// 网络配置
	SetIPAddress(interface_, ip, netmask string) error
	SetDNSServers(servers []string) error
	
	// 网络连通性
	Ping(host string, count int) (*CommandResult, error)
	TestPort(host string, port int) (bool, error)
}

// UserManager 用户管理器接口
type UserManager interface {
	// 用户查询
	ListUsers() ([]*UserInfo, error)
	GetUser(username string) (*UserInfo, error)
	GetCurrentUser() (*UserInfo, error)
	
	// 用户管理
	CreateUser(username, password string, options map[string]string) error
	DeleteUser(username string) error
	ChangePassword(username, newPassword string) error
	
	// 组管理
	ListGroups() ([]string, error)
	AddUserToGroup(username, group string) error
	RemoveUserFromGroup(username, group string) error
}

// PackageManager 包管理器接口
type PackageManager interface {
	// 包查询
	ListInstalledPackages() ([]*PackageInfo, error)
	SearchPackages(query string) ([]*PackageInfo, error)
	GetPackageInfo(name string) (*PackageInfo, error)
	
	// 包管理
	InstallPackage(name string) error
	UninstallPackage(name string) error
	UpdatePackage(name string) error
	UpdateAllPackages() error
	
	// 包缓存
	UpdatePackageCache() error
	CleanPackageCache() error
}

// FileSystemManager 文件系统管理器接口
type FileSystemManager interface {
	// 磁盘信息
	GetDiskUsage(path string) (total, used, available int64, err error)
	GetMountPoints() ([]MountPoint, error)
	
	// 权限管理
	ChangeOwner(path, owner, group string) error
	ChangePermissions(path string, mode uint32) error
	
	// 文件操作
	CreateSymlink(target, linkpath string) error
	ReadSymlink(linkpath string) (string, error)
	FindFiles(pattern string, dirs []string) ([]string, error)
}

// MountPoint 挂载点信息
type MountPoint struct {
	Device     string `json:"device"`
	MountPoint string `json:"mount_point"`
	FileSystem string `json:"file_system"`
	Options    string `json:"options"`
	Total      int64  `json:"total"`
	Used       int64  `json:"used"`
	Available  int64  `json:"available"`
}

// PlatformProvider 平台提供者接口
type PlatformProvider interface {
	// 平台信息
	GetPlatform() Platform
	GetArchitecture() Architecture
	IsSupported() bool
	
	// 管理器获取
	GetSystemManager() SystemManager
	GetProcessManager() ProcessManager
	GetServiceManager() ServiceManager
	GetNetworkManager() NetworkManager
	GetUserManager() UserManager
	GetPackageManager() PackageManager
	GetFileSystemManager() FileSystemManager
	
	// 初始化和清理
	Initialize() error
	Cleanup() error
}

// PlatformRegistry 平台注册表接口
type PlatformRegistry interface {
	// 平台注册
	RegisterProvider(platform Platform, provider PlatformProvider) error
	UnregisterProvider(platform Platform) error
	
	// 平台获取
	GetProvider(platform Platform) (PlatformProvider, error)
	GetCurrentProvider() (PlatformProvider, error)
	GetSupportedPlatforms() []Platform
	
	// 自动检测
	DetectPlatform() Platform
	DetectArchitecture() Architecture
}

// SecurityManager 安全管理器接口
type SecurityManager interface {
	// 权限检查
	HasPermission(operation string) (bool, error)
	RequireRoot() error
	GetCurrentUserPrivileges() ([]string, error)
	
	// 安全策略
	IsCommandAllowed(command string) (bool, error)
	IsPathAccessible(path string) (bool, error)
	
	// 审计日志
	LogSecurityEvent(event string, details map[string]interface{}) error
}

// MonitoringData 监控数据
type MonitoringData struct {
	Timestamp   time.Time              `json:"timestamp"`
	SystemInfo  *SystemInfo            `json:"system_info"`
	Processes   []*ProcessInfo         `json:"processes"`
	Services    []*ServiceInfo         `json:"services"`
	NetworkInfo []*NetworkInfo         `json:"network_info"`
	CustomData  map[string]interface{} `json:"custom_data"`
}

// PlatformMonitor 平台监控器接口
type PlatformMonitor interface {
	// 监控控制
	Start(ctx context.Context) error
	Stop() error
	
	// 数据收集
	CollectSystemData() (*MonitoringData, error)
	GetMetrics() (map[string]interface{}, error)
	
	// 事件通知
	SetEventCallback(callback func(event string, data interface{}))
	
	// 监控配置
	SetMonitoringInterval(interval time.Duration)
	EnableMetric(metric string) error
	DisableMetric(metric string) error
}