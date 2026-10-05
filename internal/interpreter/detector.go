package interpreter

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// systemDetector 系统解释器检测器
type systemDetector struct {
	mu                sync.RWMutex
	cachedInterpreters []*InterpreterInfo
	searchPaths       []string
}

// NewSystemDetector 创建系统检测器
func NewSystemDetector() SystemDetector {
	return &systemDetector{
		searchPaths: []string{
			"/bin",
			"/usr/bin",
			"/usr/local/bin",
			"/opt/bin",
		},
	}
}

// DetectAvailableInterpreters 检测可用解释器
func (sd *systemDetector) DetectAvailableInterpreters() ([]*InterpreterInfo, error) {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	var interpreters []*InterpreterInfo
	
	// 检测所有支持的解释器类型
	interpreterTypes := []InterpreterType{
		InterpreterBash,
		InterpreterZsh,
		InterpreterFish,
		InterpreterSh,
		InterpreterDash,
		InterpreterAsh,
		InterpreterCsh,
		InterpreterTcsh,
		InterpreterKsh,
	}

	for _, interpreterType := range interpreterTypes {
		if info := sd.detectInterpreter(interpreterType); info != nil {
			interpreters = append(interpreters, info)
		}
	}

	// 按优先级排序
	sort.Slice(interpreters, func(i, j int) bool {
		return interpreters[i].Priority > interpreters[j].Priority
	})

	sd.cachedInterpreters = interpreters
	return interpreters, nil
}

// GetDefaultInterpreter 获取默认解释器
func (sd *systemDetector) GetDefaultInterpreter() (*InterpreterInfo, error) {
	interpreters, err := sd.GetCachedInterpreters()
	if err != nil {
		return nil, err
	}

	if len(interpreters) == 0 {
		return nil, fmt.Errorf("no interpreters available")
	}

	// 优先返回bash，如果不可用则返回优先级最高的
	for _, interpreter := range interpreters {
		if interpreter.Type == InterpreterBash && interpreter.Available {
			return interpreter, nil
		}
	}

	// 返回第一个可用的（优先级最高的）
	for _, interpreter := range interpreters {
		if interpreter.Available {
			return interpreter, nil
		}
	}

	return nil, fmt.Errorf("no available interpreters found")
}

// IsInterpreterAvailable 检查解释器是否可用
func (sd *systemDetector) IsInterpreterAvailable(interpreterType InterpreterType) (bool, error) {
	path, err := sd.GetInterpreterPath(interpreterType)
	if err != nil {
		return false, err
	}

	if path == "" {
		return false, nil
	}

	// 检查文件是否存在且可执行
	if stat, err := os.Stat(path); err != nil {
		return false, nil
	} else if stat.Mode()&0111 == 0 {
		return false, nil
	}

	return true, nil
}

// GetInterpreterPath 获取解释器路径
func (sd *systemDetector) GetInterpreterPath(interpreterType InterpreterType) (string, error) {
	interpreterName := string(interpreterType)
	
	// 首先检查PATH环境变量
	if path, err := exec.LookPath(interpreterName); err == nil {
		return path, nil
	}

	// 在常见路径中搜索
	for _, searchPath := range sd.searchPaths {
		fullPath := filepath.Join(searchPath, interpreterName)
		if stat, err := os.Stat(fullPath); err == nil && stat.Mode()&0111 != 0 {
			return fullPath, nil
		}
	}

	return "", fmt.Errorf("interpreter %s not found", interpreterType)
}

// GetInterpreterVersion 获取解释器版本
func (sd *systemDetector) GetInterpreterVersion(interpreterType InterpreterType) (string, error) {
	path, err := sd.GetInterpreterPath(interpreterType)
	if err != nil {
		return "", err
	}

	// 根据不同解释器类型使用不同的版本检查命令
	var cmd *exec.Cmd
	switch interpreterType {
	case InterpreterBash:
		cmd = exec.Command(path, "--version")
	case InterpreterZsh:
		cmd = exec.Command(path, "--version")
	case InterpreterFish:
		cmd = exec.Command(path, "--version")
	case InterpreterSh, InterpreterDash, InterpreterAsh:
		// 这些解释器通常没有--version选项，尝试其他方法
		cmd = exec.Command(path, "-c", "echo $0")
	case InterpreterCsh, InterpreterTcsh:
		cmd = exec.Command(path, "--version")
	case InterpreterKsh:
		cmd = exec.Command(path, "--version")
	default:
		cmd = exec.Command(path, "--version")
	}

	output, err := cmd.Output()
	if err != nil {
		// 如果版本检查失败，返回基本信息
		return "unknown", nil
	}

	version := sd.parseVersion(string(output), interpreterType)
	return version, nil
}

// GetCachedInterpreters 获取缓存的解释器信息
func (sd *systemDetector) GetCachedInterpreters() ([]*InterpreterInfo, error) {
	sd.mu.RLock()
	defer sd.mu.RUnlock()

	if sd.cachedInterpreters == nil {
		sd.mu.RUnlock()
		return sd.DetectAvailableInterpreters()
	}

	return sd.cachedInterpreters, nil
}

// detectInterpreter 检测单个解释器
func (sd *systemDetector) detectInterpreter(interpreterType InterpreterType) *InterpreterInfo {
	path, err := sd.GetInterpreterPath(interpreterType)
	if err != nil {
		return &InterpreterInfo{
			Type:        interpreterType,
			Path:        "",
			Available:   false,
			Priority:    sd.getInterpreterPriority(interpreterType),
			Description: sd.getInterpreterDescription(interpreterType),
		}
	}

	version, _ := sd.GetInterpreterVersion(interpreterType)
	features := sd.getInterpreterFeatures(interpreterType)

	return &InterpreterInfo{
		Type:        interpreterType,
		Path:        path,
		Version:     version,
		Available:   true,
		Features:    features,
		Priority:    sd.getInterpreterPriority(interpreterType),
		Description: sd.getInterpreterDescription(interpreterType),
	}
}

// parseVersion 解析版本信息
func (sd *systemDetector) parseVersion(output string, interpreterType InterpreterType) string {
	output = strings.TrimSpace(output)
	lines := strings.Split(output, "\n")
	
	if len(lines) == 0 {
		return "unknown"
	}

	firstLine := lines[0]
	
	// 使用正则表达式提取版本号
	versionPatterns := []string{
		`version\s+([0-9]+\.[0-9]+\.[0-9]+[^\s]*)`,
		`([0-9]+\.[0-9]+\.[0-9]+[^\s]*)`,
		`([0-9]+\.[0-9]+)`,
	}

	for _, pattern := range versionPatterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(strings.ToLower(firstLine)); len(matches) > 1 {
			return matches[1]
		}
	}

	// 如果无法解析版本，返回第一行的清理版本
	if len(firstLine) > 50 {
		firstLine = firstLine[:50] + "..."
	}
	
	return strings.TrimSpace(firstLine)
}

// getInterpreterPriority 获取解释器优先级
func (sd *systemDetector) getInterpreterPriority(interpreterType InterpreterType) int {
	priorities := map[InterpreterType]int{
		InterpreterBash: 100,
		InterpreterZsh:  90,
		InterpreterSh:   80,
		InterpreterDash: 70,
		InterpreterKsh:  60,
		InterpreterFish: 50,
		InterpreterAsh:  40,
		InterpreterTcsh: 30,
		InterpreterCsh:  20,
	}

	if priority, exists := priorities[interpreterType]; exists {
		return priority
	}
	return 10
}

// getInterpreterDescription 获取解释器描述
func (sd *systemDetector) getInterpreterDescription(interpreterType InterpreterType) string {
	descriptions := map[InterpreterType]string{
		InterpreterBash: "Bourne Again Shell - Most widely used shell",
		InterpreterZsh:  "Z Shell - Extended Bourne shell with improvements",
		InterpreterFish: "Friendly Interactive Shell - User-friendly shell",
		InterpreterSh:   "POSIX Shell - Basic POSIX compliant shell",
		InterpreterDash: "Debian Almquist Shell - Lightweight POSIX shell",
		InterpreterAsh:  "Almquist Shell - Lightweight shell",
		InterpreterCsh:  "C Shell - Shell with C-like syntax",
		InterpreterTcsh: "Enhanced C Shell - Improved version of csh",
		InterpreterKsh:  "Korn Shell - POSIX shell with advanced features",
	}

	if desc, exists := descriptions[interpreterType]; exists {
		return desc
	}
	return "Unknown shell interpreter"
}

// getInterpreterFeatures 获取解释器特性
func (sd *systemDetector) getInterpreterFeatures(interpreterType InterpreterType) []string {
	featureMap := map[InterpreterType][]string{
		InterpreterBash: {
			string(FeaturePOSIXCompliant),
			string(FeatureJobControl),
			string(FeatureCommandHistory),
			string(FeatureTabCompletion),
			string(FeatureArraySupport),
			string(FeatureAdvancedGlobbing),
			string(FeatureBuiltinCommands),
			string(FeatureExtendedTest),
			string(FeatureArithmeticExpansion),
			string(FeatureProcessSubstitution),
		},
		InterpreterZsh: {
			string(FeaturePOSIXCompliant),
			string(FeatureJobControl),
			string(FeatureCommandHistory),
			string(FeatureTabCompletion),
			string(FeatureArraySupport),
			string(FeatureAdvancedGlobbing),
			string(FeatureBuiltinCommands),
			string(FeatureExtendedTest),
			string(FeatureArithmeticExpansion),
			string(FeatureProcessSubstitution),
		},
		InterpreterFish: {
			string(FeatureJobControl),
			string(FeatureCommandHistory),
			string(FeatureTabCompletion),
			string(FeatureArraySupport),
			string(FeatureBuiltinCommands),
		},
		InterpreterSh: {
			string(FeaturePOSIXCompliant),
			string(FeatureBasicGlobbing),
			string(FeatureBuiltinCommands),
		},
		InterpreterDash: {
			string(FeaturePOSIXCompliant),
			string(FeatureBuiltinCommands),
		},
		InterpreterAsh: {
			string(FeaturePOSIXCompliant),
			string(FeatureBuiltinCommands),
		},
		InterpreterKsh: {
			string(FeaturePOSIXCompliant),
			string(FeatureJobControl),
			string(FeatureCommandHistory),
			string(FeatureTabCompletion),
			string(FeatureArraySupport),
			string(FeatureAdvancedGlobbing),
			string(FeatureBuiltinCommands),
			string(FeatureExtendedTest),
			string(FeatureArithmeticExpansion),
		},
		InterpreterCsh: {
			string(FeatureJobControl),
			string(FeatureCommandHistory),
			string(FeatureArraySupport),
		},
		InterpreterTcsh: {
			string(FeatureJobControl),
			string(FeatureCommandHistory),
			string(FeatureTabCompletion),
			string(FeatureArraySupport),
		},
	}

	if features, exists := featureMap[interpreterType]; exists {
		return features
	}
	return []string{}
}

// 添加基本全局化特性常量
const FeatureBasicGlobbing InterpreterFeature = "basic_globbing"