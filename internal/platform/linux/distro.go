package linux

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// DistroInfo Linux发行版信息
type DistroInfo struct {
	ID             string            `json:"id"`              // 发行版ID (ubuntu, centos, debian等)
	Name           string            `json:"name"`            // 发行版名称
	Version        string            `json:"version"`         // 版本号
	VersionID      string            `json:"version_id"`      // 版本ID
	VersionCodename string           `json:"version_codename"` // 版本代号
	Family         DistroFamily      `json:"family"`          // 发行版家族
	PackageManager PackageManager    `json:"package_manager"` // 包管理器
	InitSystem     InitSystem        `json:"init_system"`     // 初始化系统
	Architecture   string            `json:"architecture"`    // 系统架构
	Kernel         string            `json:"kernel"`          // 内核版本
	Capabilities   []string          `json:"capabilities"`    // 系统能力
	Metadata       map[string]string `json:"metadata"`        // 额外元数据
}

// DistroFamily 发行版家族
type DistroFamily string

const (
	FamilyDebian    DistroFamily = "debian"    // Debian系列
	FamilyRedHat    DistroFamily = "redhat"    // Red Hat系列
	FamilyArch      DistroFamily = "arch"      // Arch系列
	FamilyAlpine    DistroFamily = "alpine"    // Alpine系列
	FamilySUSE      DistroFamily = "suse"      // SUSE系列
	FamilyGentoo    DistroFamily = "gentoo"    // Gentoo系列
	FamilyUnknown   DistroFamily = "unknown"   // 未知系列
)

// PackageManager 包管理器类型
type PackageManager string

const (
	PackageManagerAPT    PackageManager = "apt"     // APT (Debian/Ubuntu)
	PackageManagerYUM    PackageManager = "yum"     // YUM (CentOS/RHEL老版本)
	PackageManagerDNF    PackageManager = "dnf"     // DNF (Fedora/CentOS/RHEL新版本)
	PackageManagerPacman PackageManager = "pacman"  // Pacman (Arch Linux)
	PackageManagerAPK    PackageManager = "apk"     // APK (Alpine Linux)
	PackageManagerZypper PackageManager = "zypper"  // Zypper (openSUSE)
	PackageManagerPortage PackageManager = "portage" // Portage (Gentoo)
	PackageManagerSnap   PackageManager = "snap"    // Snap (Universal)
	PackageManagerFlatpak PackageManager = "flatpak" // Flatpak (Universal)
	PackageManagerUnknown PackageManager = "unknown" // 未知包管理器
)

// InitSystem 初始化系统类型
type InitSystem string

const (
	InitSystemSystemd InitSystem = "systemd" // systemd
	InitSystemUpstart InitSystem = "upstart" // upstart
	InitSystemSysV    InitSystem = "sysv"    // System V
	InitSystemOpenRC  InitSystem = "openrc"  // OpenRC
	InitSystemRunit   InitSystem = "runit"   // runit
	InitSystemUnknown InitSystem = "unknown" // 未知
)

// DistroDetector Linux发行版检测器
type DistroDetector struct {
	osReleaseFiles []string
	fallbackFiles  []string
}

// NewDistroDetector 创建发行版检测器
func NewDistroDetector() *DistroDetector {
	return &DistroDetector{
		osReleaseFiles: []string{
			"/etc/os-release",
			"/usr/lib/os-release",
		},
		fallbackFiles: []string{
			"/etc/lsb-release",
			"/etc/debian_version",
			"/etc/redhat-release",
			"/etc/centos-release",
			"/etc/fedora-release",
			"/etc/alpine-release",
			"/etc/arch-release",
			"/etc/gentoo-release",
			"/etc/SuSE-release",
		},
	}
}

// DetectDistribution 检测Linux发行版
func (dd *DistroDetector) DetectDistribution() (*DistroInfo, error) {
	// 优先尝试从os-release文件读取信息
	if info := dd.detectFromOSRelease(); info != nil {
		dd.enrichDistroInfo(info)
		return info, nil
	}

	// 回退到其他检测方法
	if info := dd.detectFromFallbackFiles(); info != nil {
		dd.enrichDistroInfo(info)
		return info, nil
	}

	// 如果都失败，尝试从uname等命令获取基本信息
	if info := dd.detectFromSystem(); info != nil {
		dd.enrichDistroInfo(info)
		return info, nil
	}

	return nil, fmt.Errorf("unable to detect Linux distribution")
}

// detectFromOSRelease 从os-release文件检测
func (dd *DistroDetector) detectFromOSRelease() *DistroInfo {
	for _, filePath := range dd.osReleaseFiles {
		if data, err := dd.parseKeyValueFile(filePath); err == nil {
			return dd.buildDistroInfoFromOSRelease(data)
		}
	}
	return nil
}

// detectFromFallbackFiles 从回退文件检测
func (dd *DistroDetector) detectFromFallbackFiles() *DistroInfo {
	// 检测特定发行版文件
	detectors := map[string]func() *DistroInfo{
		"/etc/debian_version":  dd.detectDebian,
		"/etc/redhat-release":  dd.detectRedHat,
		"/etc/centos-release":  dd.detectCentOS,
		"/etc/fedora-release":  dd.detectFedora,
		"/etc/alpine-release":  dd.detectAlpine,
		"/etc/arch-release":    dd.detectArch,
		"/etc/gentoo-release":  dd.detectGentoo,
		"/etc/SuSE-release":    dd.detectSUSE,
		"/etc/lsb-release":     dd.detectLSB,
	}

	for filePath, detector := range detectors {
		if _, err := os.Stat(filePath); err == nil {
			if info := detector(); info != nil {
				return info
			}
		}
	}

	return nil
}

// detectFromSystem 从系统命令检测
func (dd *DistroDetector) detectFromSystem() *DistroInfo {
	// 基本的系统信息检测
	info := &DistroInfo{
		ID:           "linux",
		Name:         "Generic Linux",
		Family:       FamilyUnknown,
		Metadata:     make(map[string]string),
	}

	// 尝试从uname获取架构信息
	if arch := dd.getArchitecture(); arch != "" {
		info.Architecture = arch
	}

	// 尝试从uname获取内核版本
	if kernel := dd.getKernelVersion(); kernel != "" {
		info.Kernel = kernel
	}

	return info
}

// buildDistroInfoFromOSRelease 从os-release数据构建发行版信息
func (dd *DistroDetector) buildDistroInfoFromOSRelease(data map[string]string) *DistroInfo {
	info := &DistroInfo{
		Metadata: make(map[string]string),
	}

	// 基本信息
	info.ID = strings.ToLower(data["ID"])
	info.Name = dd.unquote(data["NAME"])
	info.Version = dd.unquote(data["VERSION"])
	info.VersionID = dd.unquote(data["VERSION_ID"])
	info.VersionCodename = dd.unquote(data["VERSION_CODENAME"])

	// 如果没有VERSION_CODENAME，尝试从其他字段获取
	if info.VersionCodename == "" {
		info.VersionCodename = dd.unquote(data["UBUNTU_CODENAME"])
	}

	// 确定发行版家族
	info.Family = dd.determineFamily(info.ID, data)

	// 存储额外的元数据
	for key, value := range data {
		if !dd.isStandardField(key) {
			info.Metadata[key] = dd.unquote(value)
		}
	}

	return info
}

// determineFamily 确定发行版家族
func (dd *DistroDetector) determineFamily(id string, data map[string]string) DistroFamily {
	// 检查ID_LIKE字段
	if idLike, exists := data["ID_LIKE"]; exists {
		idLikeList := strings.Fields(dd.unquote(idLike))
		for _, like := range idLikeList {
			switch like {
			case "debian":
				return FamilyDebian
			case "rhel", "fedora":
				return FamilyRedHat
			case "arch":
				return FamilyArch
			case "alpine":
				return FamilyAlpine
			case "suse", "opensuse":
				return FamilySUSE
			}
		}
	}

	// 根据ID直接判断
	switch id {
	case "ubuntu", "debian", "linuxmint", "elementary", "kali", "raspbian":
		return FamilyDebian
	case "rhel", "centos", "fedora", "rocky", "almalinux", "oracle":
		return FamilyRedHat
	case "arch", "manjaro", "antergos", "endeavouros":
		return FamilyArch
	case "alpine":
		return FamilyAlpine
	case "opensuse", "opensuse-leap", "opensuse-tumbleweed", "sles":
		return FamilySUSE
	case "gentoo":
		return FamilyGentoo
	default:
		return FamilyUnknown
	}
}

// 各种特定发行版的检测方法
func (dd *DistroDetector) detectDebian() *DistroInfo {
	content, err := dd.readFileContent("/etc/debian_version")
	if err != nil {
		return nil
	}

	version := strings.TrimSpace(content)
	return &DistroInfo{
		ID:      "debian",
		Name:    "Debian GNU/Linux",
		Version: version,
		Family:  FamilyDebian,
		Metadata: map[string]string{
			"detected_from": "/etc/debian_version",
		},
	}
}

func (dd *DistroDetector) detectRedHat() *DistroInfo {
	content, err := dd.readFileContent("/etc/redhat-release")
	if err != nil {
		return nil
	}

	// 解析Red Hat release信息
	line := strings.TrimSpace(content)
	if strings.Contains(strings.ToLower(line), "centos") {
		return dd.parseRedHatRelease(line, "centos", "CentOS")
	} else if strings.Contains(strings.ToLower(line), "red hat") {
		return dd.parseRedHatRelease(line, "rhel", "Red Hat Enterprise Linux")
	}

	return dd.parseRedHatRelease(line, "redhat", "Red Hat Linux")
}

func (dd *DistroDetector) detectCentOS() *DistroInfo {
	content, err := dd.readFileContent("/etc/centos-release")
	if err != nil {
		return nil
	}

	line := strings.TrimSpace(content)
	return dd.parseRedHatRelease(line, "centos", "CentOS")
}

func (dd *DistroDetector) detectFedora() *DistroInfo {
	content, err := dd.readFileContent("/etc/fedora-release")
	if err != nil {
		return nil
	}

	line := strings.TrimSpace(content)
	return dd.parseRedHatRelease(line, "fedora", "Fedora")
}

func (dd *DistroDetector) detectAlpine() *DistroInfo {
	content, err := dd.readFileContent("/etc/alpine-release")
	if err != nil {
		return nil
	}

	version := strings.TrimSpace(content)
	return &DistroInfo{
		ID:      "alpine",
		Name:    "Alpine Linux",
		Version: version,
		Family:  FamilyAlpine,
		Metadata: map[string]string{
			"detected_from": "/etc/alpine-release",
		},
	}
}

func (dd *DistroDetector) detectArch() *DistroInfo {
	return &DistroInfo{
		ID:     "arch",
		Name:   "Arch Linux",
		Family: FamilyArch,
		Metadata: map[string]string{
			"detected_from": "/etc/arch-release",
		},
	}
}

func (dd *DistroDetector) detectGentoo() *DistroInfo {
	content, err := dd.readFileContent("/etc/gentoo-release")
	if err != nil {
		return nil
	}

	line := strings.TrimSpace(content)
	version := dd.extractVersionFromLine(line)

	return &DistroInfo{
		ID:      "gentoo",
		Name:    "Gentoo Linux",
		Version: version,
		Family:  FamilyGentoo,
		Metadata: map[string]string{
			"detected_from": "/etc/gentoo-release",
		},
	}
}

func (dd *DistroDetector) detectSUSE() *DistroInfo {
	content, err := dd.readFileContent("/etc/SuSE-release")
	if err != nil {
		return nil
	}

	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return nil
	}

	name := strings.TrimSpace(lines[0])
	version := ""
	if len(lines) > 1 {
		version = dd.extractVersionFromLine(lines[1])
	}

	return &DistroInfo{
		ID:      "opensuse",
		Name:    name,
		Version: version,
		Family:  FamilySUSE,
		Metadata: map[string]string{
			"detected_from": "/etc/SuSE-release",
		},
	}
}

func (dd *DistroDetector) detectLSB() *DistroInfo {
	data, err := dd.parseKeyValueFile("/etc/lsb-release")
	if err != nil {
		return nil
	}

	id := strings.ToLower(data["DISTRIB_ID"])
	name := data["DISTRIB_DESCRIPTION"]
	version := data["DISTRIB_RELEASE"]
	codename := data["DISTRIB_CODENAME"]

	info := &DistroInfo{
		ID:              id,
		Name:            dd.unquote(name),
		Version:         version,
		VersionCodename: codename,
		Metadata: map[string]string{
			"detected_from": "/etc/lsb-release",
		},
	}

	info.Family = dd.determineFamily(id, data)
	return info
}

// enrichDistroInfo 丰富发行版信息
func (dd *DistroDetector) enrichDistroInfo(info *DistroInfo) {
	// 检测包管理器
	info.PackageManager = dd.detectPackageManager(info.Family, info.ID)

	// 检测初始化系统
	info.InitSystem = dd.detectInitSystem()

	// 获取架构信息
	if info.Architecture == "" {
		info.Architecture = dd.getArchitecture()
	}

	// 获取内核版本
	if info.Kernel == "" {
		info.Kernel = dd.getKernelVersion()
	}

	// 检测系统能力
	info.Capabilities = dd.detectCapabilities(info)
}

// detectPackageManager 检测包管理器
func (dd *DistroDetector) detectPackageManager(family DistroFamily, id string) PackageManager {
	// 首先检查包管理器命令是否存在
	packageManagers := []struct {
		cmd     string
		manager PackageManager
	}{
		{"apt", PackageManagerAPT},
		{"dnf", PackageManagerDNF},
		{"yum", PackageManagerYUM},
		{"pacman", PackageManagerPacman},
		{"apk", PackageManagerAPK},
		{"zypper", PackageManagerZypper},
		{"emerge", PackageManagerPortage},
	}

	for _, pm := range packageManagers {
		if dd.commandExists(pm.cmd) {
			return pm.manager
		}
	}

	// 根据发行版家族推断
	switch family {
	case FamilyDebian:
		return PackageManagerAPT
	case FamilyRedHat:
		// 根据ID进一步判断
		if id == "fedora" || dd.commandExists("dnf") {
			return PackageManagerDNF
		}
		return PackageManagerYUM
	case FamilyArch:
		return PackageManagerPacman
	case FamilyAlpine:
		return PackageManagerAPK
	case FamilySUSE:
		return PackageManagerZypper
	case FamilyGentoo:
		return PackageManagerPortage
	default:
		return PackageManagerUnknown
	}
}

// detectInitSystem 检测初始化系统
func (dd *DistroDetector) detectInitSystem() InitSystem {
	// 检查各种初始化系统的标志
	if dd.pathExists("/run/systemd/system") || dd.commandExists("systemctl") {
		return InitSystemSystemd
	}

	if dd.pathExists("/etc/init") && dd.commandExists("initctl") {
		return InitSystemUpstart
	}

	if dd.pathExists("/etc/rc.d") || dd.pathExists("/etc/init.d") {
		return InitSystemSysV
	}

	if dd.commandExists("rc-update") {
		return InitSystemOpenRC
	}

	if dd.commandExists("sv") {
		return InitSystemRunit
	}

	return InitSystemUnknown
}

// detectCapabilities 检测系统能力
func (dd *DistroDetector) detectCapabilities(info *DistroInfo) []string {
	var capabilities []string

	// 基础能力
	capabilities = append(capabilities, "filesystem")

	// 根据包管理器添加能力
	switch info.PackageManager {
	case PackageManagerAPT:
		capabilities = append(capabilities, "apt", "dpkg")
	case PackageManagerYUM:
		capabilities = append(capabilities, "yum", "rpm")
	case PackageManagerDNF:
		capabilities = append(capabilities, "dnf", "rpm")
	case PackageManagerPacman:
		capabilities = append(capabilities, "pacman")
	case PackageManagerAPK:
		capabilities = append(capabilities, "apk")
	case PackageManagerZypper:
		capabilities = append(capabilities, "zypper", "rpm")
	case PackageManagerPortage:
		capabilities = append(capabilities, "portage")
	}

	// 根据初始化系统添加能力
	switch info.InitSystem {
	case InitSystemSystemd:
		capabilities = append(capabilities, "systemd", "service_management")
	case InitSystemUpstart:
		capabilities = append(capabilities, "upstart", "service_management")
	case InitSystemSysV:
		capabilities = append(capabilities, "sysv", "service_management")
	case InitSystemOpenRC:
		capabilities = append(capabilities, "openrc", "service_management")
	}

	// 检查容器环境
	if dd.isContainer() {
		capabilities = append(capabilities, "container")
	}

	// 检查虚拟化环境
	if virt := dd.detectVirtualization(); virt != "" {
		capabilities = append(capabilities, "virtualized", virt)
	}

	return capabilities
}

// 辅助方法
func (dd *DistroDetector) parseKeyValueFile(filePath string) (map[string]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data := make(map[string]string)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if idx := strings.Index(line, "="); idx != -1 {
			key := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+1:])
			data[key] = value
		}
	}

	return data, scanner.Err()
}

func (dd *DistroDetector) readFileContent(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (dd *DistroDetector) unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && ((s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'')) {
		return s[1 : len(s)-1]
	}
	return s
}

func (dd *DistroDetector) isStandardField(field string) bool {
	standardFields := []string{
		"ID", "NAME", "VERSION", "VERSION_ID", "VERSION_CODENAME",
		"PRETTY_NAME", "ANSI_COLOR", "CPE_NAME", "HOME_URL",
		"DOCUMENTATION_URL", "SUPPORT_URL", "BUG_REPORT_URL",
		"PRIVACY_POLICY_URL", "BUILD_ID", "VARIANT", "VARIANT_ID",
	}

	for _, sf := range standardFields {
		if field == sf {
			return true
		}
	}
	return false
}

func (dd *DistroDetector) parseRedHatRelease(line, id, name string) *DistroInfo {
	version := dd.extractVersionFromLine(line)
	return &DistroInfo{
		ID:      id,
		Name:    name,
		Version: version,
		Family:  FamilyRedHat,
		Metadata: map[string]string{
			"release_string": line,
		},
	}
}

func (dd *DistroDetector) extractVersionFromLine(line string) string {
	// 使用正则表达式提取版本号
	versionRegex := regexp.MustCompile(`(\d+(?:\.\d+)*(?:\.\d+)*)`)
	matches := versionRegex.FindStringSubmatch(line)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func (dd *DistroDetector) getArchitecture() string {
	// 这里可以调用uname -m获取架构
	// 暂时返回placeholder
	return "x86_64"
}

func (dd *DistroDetector) getKernelVersion() string {
	// 这里可以调用uname -r获取内核版本
	// 暂时返回placeholder
	return "unknown"
}

func (dd *DistroDetector) commandExists(cmd string) bool {
	// 检查命令是否存在
	_, err := os.Stat("/usr/bin/" + cmd)
	if err == nil {
		return true
	}
	_, err = os.Stat("/bin/" + cmd)
	return err == nil
}

func (dd *DistroDetector) pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (dd *DistroDetector) isContainer() bool {
	// 检查是否运行在容器中
	return dd.pathExists("/.dockerenv") || dd.pathExists("/run/secrets/kubernetes.io")
}

func (dd *DistroDetector) detectVirtualization() string {
	// 检查虚拟化环境
	if dd.pathExists("/proc/xen") {
		return "xen"
	}
	// 这里可以添加更多虚拟化检测逻辑
	return ""
}