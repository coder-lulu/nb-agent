package interpreter

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// interpreterValidator 解释器验证器实现
type interpreterValidator struct {
	compatibilityRules map[InterpreterType][]string
	dangerousPatterns  []string
}

// NewInterpreterValidator 创建解释器验证器
func NewInterpreterValidator() InterpreterValidator {
	return &interpreterValidator{
		compatibilityRules: initCompatibilityRules(),
		dangerousPatterns:  initDangerousPatterns(),
	}
}

// ValidateCommand 验证命令与解释器兼容性
func (iv *interpreterValidator) ValidateCommand(command string, interpreter *InterpreterInfo) (*ValidationResult, error) {
	result := &ValidationResult{
		Valid:       true,
		Interpreter: interpreter,
		Errors:      []string{},
		Warnings:    []string{},
		Suggestions: []string{},
	}

	if interpreter == nil {
		result.Valid = false
		result.Errors = append(result.Errors, "interpreter cannot be nil")
		return result, nil
	}

	// 检查基本语法兼容性
	if err := iv.checkSyntaxCompatibility(command, interpreter, result); err != nil {
		return nil, err
	}

	// 检查功能特性兼容性
	if err := iv.checkFeatureCompatibility(command, interpreter, result); err != nil {
		return nil, err
	}

	// 检查危险模式
	if err := iv.checkDangerousPatterns(command, result); err != nil {
		return nil, err
	}

	// 检查特定解释器的已知问题
	if err := iv.checkKnownIssues(command, interpreter, result); err != nil {
		return nil, err
	}

	return result, nil
}

// CheckInterpreterHealth 检查解释器健康状态
func (iv *interpreterValidator) CheckInterpreterHealth(interpreter *InterpreterInfo) (*ValidationResult, error) {
	result := &ValidationResult{
		Valid:       true,
		Interpreter: interpreter,
		Errors:      []string{},
		Warnings:    []string{},
		Suggestions: []string{},
	}

	if interpreter == nil {
		result.Valid = false
		result.Errors = append(result.Errors, "interpreter cannot be nil")
		return result, nil
	}

	// 检查解释器是否可执行
	if err := iv.checkExecutability(interpreter, result); err != nil {
		return nil, err
	}

	// 检查基本功能
	if err := iv.checkBasicFunctionality(interpreter, result); err != nil {
		return nil, err
	}

	// 检查版本兼容性
	if err := iv.checkVersionCompatibility(interpreter, result); err != nil {
		return nil, err
	}

	return result, nil
}

// ValidateEnvironment 验证执行环境
func (iv *interpreterValidator) ValidateEnvironment(interpreter *InterpreterInfo, options *ExecutionOptions) (*ValidationResult, error) {
	result := &ValidationResult{
		Valid:       true,
		Interpreter: interpreter,
		Errors:      []string{},
		Warnings:    []string{},
		Suggestions: []string{},
	}

	if interpreter == nil {
		result.Valid = false
		result.Errors = append(result.Errors, "interpreter cannot be nil")
		return result, nil
	}

	if options == nil {
		result.Valid = false
		result.Errors = append(result.Errors, "execution options cannot be nil")
		return result, nil
	}

	// 检查工作目录
	if err := iv.checkWorkingDirectory(options, result); err != nil {
		return nil, err
	}

	// 检查环境变量
	if err := iv.checkEnvironmentVariables(options, result); err != nil {
		return nil, err
	}

	// 检查超时设置
	if err := iv.checkTimeoutSettings(options, result); err != nil {
		return nil, err
	}

	return result, nil
}

// checkSyntaxCompatibility 检查语法兼容性
func (iv *interpreterValidator) checkSyntaxCompatibility(command string, interpreter *InterpreterInfo, result *ValidationResult) error {
	command = strings.TrimSpace(command)

	// 检查bash特定语法
	if interpreter.Type != InterpreterBash && interpreter.Type != InterpreterZsh {
		bashSpecificPatterns := []string{
			`\[\[.*\]\]`,           // [[ ]] 测试语法
			`\$\{.*\#.*\}`,         // 参数扩展
			`\$\{.*\%.*\}`,         // 参数扩展
			`\$\(\(.*\)\)`,         // 算术扩展
			`\{.*\.\.[0-9]+.*\}`,   // 大括号扩展
		}

		for _, pattern := range bashSpecificPatterns {
			if matched, _ := regexp.MatchString(pattern, command); matched {
				result.Warnings = append(result.Warnings, 
					fmt.Sprintf("Command contains bash-specific syntax that may not work with %s", interpreter.Type))
				result.Suggestions = append(result.Suggestions, 
					"Consider using bash or zsh for better compatibility")
			}
		}
	}

	// 检查fish特定语法
	if interpreter.Type == InterpreterFish {
		posixPatterns := []string{
			`\$\{.*\}`,     // POSIX变量扩展
			`export\s+\w+=`, // export语法
			`source\s+`,    // source命令
		}

		for _, pattern := range posixPatterns {
			if matched, _ := regexp.MatchString(pattern, command); matched {
				result.Warnings = append(result.Warnings, 
					"Command contains POSIX syntax that may not work with fish shell")
				result.Suggestions = append(result.Suggestions, 
					"Fish uses different syntax for variables and sourcing")
			}
		}
	}

	// 检查C shell特定问题
	if interpreter.Type == InterpreterCsh || interpreter.Type == InterpreterTcsh {
		problematicPatterns := []string{
			`\$\(.*\)`,     // 命令替换
			`.*\|.*\|.*`,   // 多重管道
		}

		for _, pattern := range problematicPatterns {
			if matched, _ := regexp.MatchString(pattern, command); matched {
				result.Warnings = append(result.Warnings, 
					"Command contains syntax that may not work well with C shell variants")
			}
		}
	}

	return nil
}

// checkFeatureCompatibility 检查功能特性兼容性
func (iv *interpreterValidator) checkFeatureCompatibility(command string, interpreter *InterpreterInfo, result *ValidationResult) error {
	// 检查是否需要数组支持
	if needsArraySupport(command) {
		if !hasFeature(interpreter, string(FeatureArraySupport)) {
			result.Warnings = append(result.Warnings, 
				fmt.Sprintf("%s may not support arrays properly", interpreter.Type))
		}
	}

	// 检查是否需要高级globbing
	if needsAdvancedGlobbing(command) {
		if !hasFeature(interpreter, string(FeatureAdvancedGlobbing)) {
			result.Warnings = append(result.Warnings, 
				fmt.Sprintf("%s may not support advanced globbing patterns", interpreter.Type))
		}
	}

	// 检查是否需要进程替换
	if needsProcessSubstitution(command) {
		if !hasFeature(interpreter, string(FeatureProcessSubstitution)) {
			result.Warnings = append(result.Warnings, 
				fmt.Sprintf("%s does not support process substitution", interpreter.Type))
			result.Suggestions = append(result.Suggestions, 
				"Use temporary files or pipes instead of process substitution")
		}
	}

	return nil
}

// checkDangerousPatterns 检查危险模式
func (iv *interpreterValidator) checkDangerousPatterns(command string, result *ValidationResult) error {
	for _, pattern := range iv.dangerousPatterns {
		if matched, _ := regexp.MatchString(pattern, command); matched {
			result.Warnings = append(result.Warnings, 
				fmt.Sprintf("Command contains potentially dangerous pattern: %s", pattern))
		}
	}

	return nil
}

// checkKnownIssues 检查特定解释器的已知问题
func (iv *interpreterValidator) checkKnownIssues(command string, interpreter *InterpreterInfo, result *ValidationResult) error {
	switch interpreter.Type {
	case InterpreterDash:
		// dash不支持某些bash扩展
		if strings.Contains(command, "echo -e") {
			result.Warnings = append(result.Warnings, 
				"dash may not support 'echo -e', use printf instead")
			result.Suggestions = append(result.Suggestions, 
				"Replace 'echo -e' with 'printf' for better compatibility")
		}

	case InterpreterAsh:
		// ash的限制
		if strings.Contains(command, "function ") {
			result.Warnings = append(result.Warnings, 
				"ash may not support the 'function' keyword")
			result.Suggestions = append(result.Suggestions, 
				"Use 'funcname() { }' syntax instead")
		}

	case InterpreterFish:
		// fish的特殊性
		if strings.Contains(command, "&&") || strings.Contains(command, "||") {
			result.Warnings = append(result.Warnings, 
				"fish uses 'and'/'or' instead of '&&'/'||'")
			result.Suggestions = append(result.Suggestions, 
				"Replace '&&' with 'and' and '||' with 'or' for fish compatibility")
		}
	}

	return nil
}

// checkExecutability 检查解释器可执行性
func (iv *interpreterValidator) checkExecutability(interpreter *InterpreterInfo, result *ValidationResult) error {
	if interpreter.Path == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "interpreter path is empty")
		return nil
	}

	// 尝试执行基本命令检查
	cmd := exec.Command(interpreter.Path, "--help")
	if err := cmd.Run(); err != nil {
		result.Warnings = append(result.Warnings, 
			fmt.Sprintf("interpreter may not be functioning properly: %v", err))
	}

	return nil
}

// checkBasicFunctionality 检查基本功能
func (iv *interpreterValidator) checkBasicFunctionality(interpreter *InterpreterInfo, result *ValidationResult) error {
	// 测试基本echo命令
	cmd := exec.Command(interpreter.Path, "-c", "echo test")
	output, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(output)) != "test" {
		result.Warnings = append(result.Warnings, 
			"interpreter basic functionality test failed")
	}

	return nil
}

// checkVersionCompatibility 检查版本兼容性
func (iv *interpreterValidator) checkVersionCompatibility(interpreter *InterpreterInfo, result *ValidationResult) error {
	// 这里可以添加特定版本的兼容性检查
	// 目前只是一个占位符实现
	if interpreter.Version == "unknown" {
		result.Warnings = append(result.Warnings, 
			"interpreter version is unknown, compatibility cannot be verified")
	}

	return nil
}

// checkWorkingDirectory 检查工作目录
func (iv *interpreterValidator) checkWorkingDirectory(options *ExecutionOptions, result *ValidationResult) error {
	if options.WorkingDir != "" {
		// 这里可以添加工作目录的检查逻辑
		// 例如检查目录是否存在、是否有权限等
	}
	return nil
}

// checkEnvironmentVariables 检查环境变量
func (iv *interpreterValidator) checkEnvironmentVariables(options *ExecutionOptions, result *ValidationResult) error {
	if options.Environment != nil {
		// 检查可能的危险环境变量
		dangerousVars := []string{"LD_PRELOAD", "PATH"}
		for _, dangerousVar := range dangerousVars {
			if _, exists := options.Environment[dangerousVar]; exists {
				result.Warnings = append(result.Warnings, 
					fmt.Sprintf("potentially dangerous environment variable: %s", dangerousVar))
			}
		}
	}
	return nil
}

// checkTimeoutSettings 检查超时设置
func (iv *interpreterValidator) checkTimeoutSettings(options *ExecutionOptions, result *ValidationResult) error {
	if options.Timeout > 0 {
		// 检查超时设置是否合理
		if options.Timeout.Seconds() > 3600 { // 1小时
			result.Warnings = append(result.Warnings, 
				"execution timeout is very long (>1 hour)")
		}
	}
	return nil
}

// 辅助函数
func needsArraySupport(command string) bool {
	arrayPatterns := []string{
		`\w+\[.*\]`,     // 数组访问
		`declare -a`,    // 数组声明
		`\w+\+=\(`,      // 数组追加
	}

	for _, pattern := range arrayPatterns {
		if matched, _ := regexp.MatchString(pattern, command); matched {
			return true
		}
	}
	return false
}

func needsAdvancedGlobbing(command string) bool {
	globPatterns := []string{
		`\*\*`,          // 递归globbing
		`\{.*,.*\}`,     // 大括号扩展
		`\?\(.*\)`,      // 扩展globbing
	}

	for _, pattern := range globPatterns {
		if matched, _ := regexp.MatchString(pattern, command); matched {
			return true
		}
	}
	return false
}

func needsProcessSubstitution(command string) bool {
	return strings.Contains(command, "<(") || strings.Contains(command, ">(")
}

func hasFeature(interpreter *InterpreterInfo, feature string) bool {
	for _, f := range interpreter.Features {
		if f == feature {
			return true
		}
	}
	return false
}

// initCompatibilityRules 初始化兼容性规则
func initCompatibilityRules() map[InterpreterType][]string {
	return map[InterpreterType][]string{
		InterpreterBash: {
			"arrays", "arithmetic", "extended_test", "process_substitution",
		},
		InterpreterZsh: {
			"arrays", "arithmetic", "extended_test", "process_substitution",
		},
		InterpreterFish: {
			"arrays", "different_syntax",
		},
		InterpreterSh: {
			"posix_only",
		},
		InterpreterDash: {
			"posix_only", "minimal_features",
		},
	}
}

// initDangerousPatterns 初始化危险模式
func initDangerousPatterns() []string {
	return []string{
		`rm\s+-rf\s+/`,          // 危险的删除命令
		`dd\s+.*of=/dev/`,       // 危险的dd命令
		`mkfs\s+`,               // 格式化命令
		`fdisk\s+`,              // 磁盘分区命令
		`wget.*\|\s*sh`,         // 从网络下载并执行
		`curl.*\|\s*sh`,         // 从网络下载并执行
	}
}