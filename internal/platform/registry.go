package platform

import (
	"fmt"
	"log/slog"
	"runtime"
	"sync"
)

// platformRegistry 平台注册表实现
type platformRegistry struct {
	providers map[Platform]PlatformProvider
	mu        sync.RWMutex
	logger    *slog.Logger
	
	// 当前平台信息
	currentPlatform     Platform
	currentArchitecture Architecture
}

// NewPlatformRegistry 创建平台注册表
func NewPlatformRegistry(logger *slog.Logger) PlatformRegistry {
	if logger == nil {
		logger = slog.Default()
	}

	registry := &platformRegistry{
		providers: make(map[Platform]PlatformProvider),
		logger:    logger.With("component", "platform_registry"),
	}

	// 自动检测当前平台
	registry.currentPlatform = registry.DetectPlatform()
	registry.currentArchitecture = registry.DetectArchitecture()

	registry.logger.Info("platform registry initialized",
		"platform", registry.currentPlatform,
		"architecture", registry.currentArchitecture)

	return registry
}

// RegisterProvider 注册平台提供者
func (pr *platformRegistry) RegisterProvider(platform Platform, provider PlatformProvider) error {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	if provider == nil {
		return fmt.Errorf("provider cannot be nil")
	}

	// 检查提供者是否支持该平台
	if !provider.IsSupported() {
		return fmt.Errorf("provider does not support platform %s", platform)
	}

	// 初始化提供者
	if err := provider.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize provider for platform %s: %w", platform, err)
	}

	pr.providers[platform] = provider
	pr.logger.Info("platform provider registered", "platform", platform)

	return nil
}

// UnregisterProvider 注销平台提供者
func (pr *platformRegistry) UnregisterProvider(platform Platform) error {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	provider, exists := pr.providers[platform]
	if !exists {
		return fmt.Errorf("provider for platform %s not found", platform)
	}

	// 清理提供者
	if err := provider.Cleanup(); err != nil {
		pr.logger.Warn("failed to cleanup provider", "platform", platform, "error", err)
	}

	delete(pr.providers, platform)
	pr.logger.Info("platform provider unregistered", "platform", platform)

	return nil
}

// GetProvider 获取指定平台的提供者
func (pr *platformRegistry) GetProvider(platform Platform) (PlatformProvider, error) {
	pr.mu.RLock()
	defer pr.mu.RUnlock()

	provider, exists := pr.providers[platform]
	if !exists {
		return nil, fmt.Errorf("provider for platform %s not found", platform)
	}

	return provider, nil
}

// GetCurrentProvider 获取当前平台的提供者
func (pr *platformRegistry) GetCurrentProvider() (PlatformProvider, error) {
	return pr.GetProvider(pr.currentPlatform)
}

// GetSupportedPlatforms 获取支持的平台列表
func (pr *platformRegistry) GetSupportedPlatforms() []Platform {
	pr.mu.RLock()
	defer pr.mu.RUnlock()

	platforms := make([]Platform, 0, len(pr.providers))
	for platform := range pr.providers {
		platforms = append(platforms, platform)
	}

	return platforms
}

// DetectPlatform 检测当前平台
func (pr *platformRegistry) DetectPlatform() Platform {
	switch runtime.GOOS {
	case "linux":
		return PlatformLinux
	case "windows":
		return PlatformWindows
	case "darwin":
		return PlatformMacOS
	case "freebsd":
		return PlatformFreeBSD
	default:
		pr.logger.Warn("unknown platform detected", "goos", runtime.GOOS)
		return PlatformUnknown
	}
}

// DetectArchitecture 检测当前架构
func (pr *platformRegistry) DetectArchitecture() Architecture {
	switch runtime.GOARCH {
	case "amd64":
		return ArchAMD64
	case "arm64":
		return ArchARM64
	case "386":
		return Arch386
	case "arm":
		return ArchARM
	default:
		pr.logger.Warn("unknown architecture detected", "goarch", runtime.GOARCH)
		return ArchAMD64 // 默认返回amd64
	}
}

// globalRegistry 全局平台注册表实例
var (
	globalRegistry PlatformRegistry
	registryOnce   sync.Once
)

// GetGlobalRegistry 获取全局平台注册表
func GetGlobalRegistry() PlatformRegistry {
	registryOnce.Do(func() {
		globalRegistry = NewPlatformRegistry(slog.Default())
	})
	return globalRegistry
}

// SetGlobalRegistry 设置全局平台注册表
func SetGlobalRegistry(registry PlatformRegistry) {
	globalRegistry = registry
}

// 便利函数，直接使用全局注册表

// GetCurrentPlatform 获取当前平台
func GetCurrentPlatform() Platform {
	return GetGlobalRegistry().DetectPlatform()
}

// GetCurrentArchitecture 获取当前架构
func GetCurrentArchitecture() Architecture {
	return GetGlobalRegistry().DetectArchitecture()
}

// GetSystemManager 获取系统管理器
func GetSystemManager() (SystemManager, error) {
	provider, err := GetGlobalRegistry().GetCurrentProvider()
	if err != nil {
		return nil, err
	}
	return provider.GetSystemManager(), nil
}

// GetProcessManager 获取进程管理器
func GetProcessManager() (ProcessManager, error) {
	provider, err := GetGlobalRegistry().GetCurrentProvider()
	if err != nil {
		return nil, err
	}
	return provider.GetProcessManager(), nil
}

// GetServiceManager 获取服务管理器
func GetServiceManager() (ServiceManager, error) {
	provider, err := GetGlobalRegistry().GetCurrentProvider()
	if err != nil {
		return nil, err
	}
	return provider.GetServiceManager(), nil
}

// GetNetworkManager 获取网络管理器
func GetNetworkManager() (NetworkManager, error) {
	provider, err := GetGlobalRegistry().GetCurrentProvider()
	if err != nil {
		return nil, err
	}
	return provider.GetNetworkManager(), nil
}

// GetUserManager 获取用户管理器
func GetUserManager() (UserManager, error) {
	provider, err := GetGlobalRegistry().GetCurrentProvider()
	if err != nil {
		return nil, err
	}
	return provider.GetUserManager(), nil
}

// GetPackageManager 获取包管理器
func GetPackageManager() (PackageManager, error) {
	provider, err := GetGlobalRegistry().GetCurrentProvider()
	if err != nil {
		return nil, err
	}
	return provider.GetPackageManager(), nil
}

// GetFileSystemManager 获取文件系统管理器
func GetFileSystemManager() (FileSystemManager, error) {
	provider, err := GetGlobalRegistry().GetCurrentProvider()
	if err != nil {
		return nil, err
	}
	return provider.GetFileSystemManager(), nil
}

// RegisterCurrentPlatform 注册当前平台的提供者
func RegisterCurrentPlatform(provider PlatformProvider) error {
	platform := GetCurrentPlatform()
	return GetGlobalRegistry().RegisterProvider(platform, provider)
}

// IsCurrentPlatformSupported 检查当前平台是否被支持
func IsCurrentPlatformSupported() bool {
	platform := GetCurrentPlatform()
	_, err := GetGlobalRegistry().GetProvider(platform)
	return err == nil
}

// InitializePlatform 初始化平台支持
func InitializePlatform() error {
	if IsCurrentPlatformSupported() {
		return nil
	}

	platform := GetCurrentPlatform()
	return fmt.Errorf("platform %s is not supported, please register a provider", platform)
}