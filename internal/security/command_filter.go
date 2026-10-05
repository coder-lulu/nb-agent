package security

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
)

// commandSecurityFilter 命令安全过滤器实现
type commandSecurityFilter struct {
	policy         *SecurityPolicy
	dangerousPatterns []*regexp.Regexp
	commandParsers    map[string]CommandParser
	mu               sync.RWMutex
	logger           *slog.Logger
}

// CommandParser 命令解析器接口
type CommandParser interface {
	Parse(command string) (*ParsedCommand, error)
	GetCommandType() CommandType
	RequiredPermissions() []string
}

// NewCommandSecurityFilter 创建命令安全过滤器
func NewCommandSecurityFilter(policy *SecurityPolicy, logger *slog.Logger) CommandSecurityFilter {
	if logger == nil {
		logger = slog.Default()
	}

	filter := &commandSecurityFilter{
		policy:         policy,
		commandParsers: make(map[string]CommandParser),
		logger:         logger.With("component", "command_filter"),
	}

	// 编译危险模式正则表达式
	filter.compileDangerousPatterns()
	
	// 注册默认命令解析器
	filter.registerDefaultParsers()

	return filter
}

// AnalyzeCommand 分析命令
func (csf *commandSecurityFilter) AnalyzeCommand(command string) (*CommandAnalysis, error) {
	csf.mu.RLock()
	defer csf.mu.RUnlock()

	csf.logger.Debug("analyzing command", "command", command)

	// 解析命令
	parsedCmd, err := csf.parseCommand(command)
	if err != nil {
		return nil, fmt.Errorf("failed to parse command: %w", err)
	}

	// 确定命令类型
	cmdType := csf.determineCommandType(parsedCmd)

	// 分析权限需求
	requiredPerms := csf.analyzePermissionRequirements(parsedCmd)

	// 分析访问路径
	accessedPaths := csf.analyzeAccessedPaths(parsedCmd)

	// 检查网络访问
	networkAccess := csf.checkNetworkAccess(parsedCmd)

	// 检查系统修改
	systemModify := csf.checkSystemModification(parsedCmd)

	// 检查权限提升
	privEscalation := csf.checkPrivilegeEscalation(parsedCmd)

	analysis := &CommandAnalysis{
		ParsedCommand:  parsedCmd,
		CommandType:    cmdType,
		RequiredPerms:  requiredPerms,
		AccessedPaths:  accessedPaths,
		NetworkAccess:  networkAccess,
		SystemModify:   systemModify,
		PrivEscalation: privEscalation,
	}

	csf.logger.Debug("command analysis completed",
		"command_type", cmdType,
		"network_access", networkAccess,
		"system_modify", systemModify,
		"priv_escalation", privEscalation)

	return analysis, nil
}

// DetectDangerousPatterns 检测危险模式
func (csf *commandSecurityFilter) DetectDangerousPatterns(command string) ([]DangerousPattern, error) {
	csf.mu.RLock()
	defer csf.mu.RUnlock()

	var patterns []DangerousPattern

	// 检查预定义的危险模式
	for i, regex := range csf.dangerousPatterns {
		if regex.MatchString(command) {
			pattern := csf.getDangerousPatternInfo(i, regex.String())
			patterns = append(patterns, pattern)
		}
	}

	// 检查特定的危险命令
	patterns = append(patterns, csf.checkSpecificDangerousCommands(command)...)

	// 检查命令注入
	patterns = append(patterns, csf.checkCommandInjection(command)...)

	csf.logger.Debug("dangerous pattern detection completed",
		"patterns_found", len(patterns))

	return patterns, nil
}

// AssessRisk 评估命令风险
func (csf *commandSecurityFilter) AssessRisk(ctx context.Context, cmdCtx *CommandContext) (CommandRisk, error) {
	csf.mu.RLock()
	defer csf.mu.RUnlock()

	// 分析命令
	analysis, err := csf.AnalyzeCommand(cmdCtx.Command)
	if err != nil {
		return RiskHigh, fmt.Errorf("failed to analyze command: %w", err)
	}

	// 检测危险模式
	dangerousPatterns, err := csf.DetectDangerousPatterns(cmdCtx.Command)
	if err != nil {
		return RiskHigh, fmt.Errorf("failed to detect dangerous patterns: %w", err)
	}

	// 计算基础风险分数
	riskScore := csf.calculateBaseRiskScore(analysis)

	// 基于危险模式调整风险分数
	riskScore += csf.calculatePatternRiskScore(dangerousPatterns)

	// 基于用户上下文调整风险分数
	riskScore += csf.calculateContextRiskScore(cmdCtx)

	// 基于环境因素调整风险分数
	riskScore += csf.calculateEnvironmentRiskScore(cmdCtx)

	// 转换为风险级别
	risk := csf.scoreToRiskLevel(riskScore)

	csf.logger.Info("risk assessment completed",
		"command", cmdCtx.Command,
		"risk_score", riskScore,
		"risk_level", risk)

	return risk, nil
}

// SanitizeCommand 清理命令
func (csf *commandSecurityFilter) SanitizeCommand(command string) (string, error) {
	csf.mu.RLock()
	defer csf.mu.RUnlock()

	sanitized := command

	// 移除潜在的命令注入
	sanitized = csf.removeCommandInjection(sanitized)

	// 转义特殊字符
	sanitized = csf.escapeSpecialCharacters(sanitized)

	// 移除多余的空白字符
	sanitized = strings.TrimSpace(sanitized)
	sanitized = regexp.MustCompile(`\s+`).ReplaceAllString(sanitized, " ")

	csf.logger.Debug("command sanitized",
		"original", command,
		"sanitized", sanitized)

	return sanitized, nil
}

// compileDangerousPatterns 编译危险模式正则表达式
func (csf *commandSecurityFilter) compileDangerousPatterns() {
	patterns := []string{
		// 文件系统破坏
		`rm\s+(-rf?|--recursive|--force)?\s*\/`,
		`rm\s+.*\*`,
		`dd\s+if=.*of=\/dev\/`,
		`mkfs\.`,
		`fdisk.*\/dev\/`,
		
		// 网络攻击
		`nc\s+-.*l.*-p\s+\d+`,
		`nmap\s+.*-.*[sS][sS]`,
		`wget.*\|\s*sh`,
		`curl.*\|\s*bash`,
		
		// 权限提升
		`sudo\s+su\s*-`,
		`su\s+-.*root`,
		`chmod\s+[4-7][0-7][0-7]`,
		`chown\s+root`,
		
		// 进程操作
		`kill\s+-9.*1$`,
		`killall\s+-9`,
		`pkill\s+-f`,
		
		// 系统修改
		`echo.*>.*\/etc\/`,
		`cat.*>.*\/etc\/`,
		`:\(\)\{\s*:\|\:&\s*\};:`,  // fork bomb
		
		// 数据泄露
		`cat\s+\/etc\/passwd`,
		`cat\s+\/etc\/shadow`,
		`grep\s+.*\/etc\/`,
		
		// 后门和持久化
		`crontab\s+-.*`,
		`echo.*>>.*bashrc`,
		`echo.*>>.*profile`,
		
		// 恶意下载
		`wget.*http.*\|\s*sh`,
		`curl.*ftp.*\|\s*bash`,
		`python.*-c.*urllib`,
	}

	csf.dangerousPatterns = make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		if regex, err := regexp.Compile(pattern); err == nil {
			csf.dangerousPatterns = append(csf.dangerousPatterns, regex)
		} else {
			csf.logger.Warn("failed to compile dangerous pattern", "pattern", pattern, "error", err)
		}
	}

	csf.logger.Info("compiled dangerous patterns", "count", len(csf.dangerousPatterns))
}

// registerDefaultParsers 注册默认命令解析器
func (csf *commandSecurityFilter) registerDefaultParsers() {
	// 系统命令解析器
	csf.commandParsers["ls"] = &SystemCommandParser{commandType: CommandTypeFile}
	csf.commandParsers["cat"] = &SystemCommandParser{commandType: CommandTypeFile}
	csf.commandParsers["rm"] = &SystemCommandParser{commandType: CommandTypeFile}
	csf.commandParsers["cp"] = &SystemCommandParser{commandType: CommandTypeFile}
	csf.commandParsers["mv"] = &SystemCommandParser{commandType: CommandTypeFile}
	csf.commandParsers["chmod"] = &SystemCommandParser{commandType: CommandTypeFile}
	csf.commandParsers["chown"] = &SystemCommandParser{commandType: CommandTypeFile}
	
	// 进程命令解析器
	csf.commandParsers["ps"] = &SystemCommandParser{commandType: CommandTypeProcess}
	csf.commandParsers["kill"] = &SystemCommandParser{commandType: CommandTypeProcess}
	csf.commandParsers["killall"] = &SystemCommandParser{commandType: CommandTypeProcess}
	csf.commandParsers["pkill"] = &SystemCommandParser{commandType: CommandTypeProcess}
	
	// 网络命令解析器
	csf.commandParsers["wget"] = &SystemCommandParser{commandType: CommandTypeNetwork}
	csf.commandParsers["curl"] = &SystemCommandParser{commandType: CommandTypeNetwork}
	csf.commandParsers["nc"] = &SystemCommandParser{commandType: CommandTypeNetwork}
	csf.commandParsers["nmap"] = &SystemCommandParser{commandType: CommandTypeNetwork}
	
	// 用户管理命令解析器
	csf.commandParsers["useradd"] = &SystemCommandParser{commandType: CommandTypeUser}
	csf.commandParsers["usermod"] = &SystemCommandParser{commandType: CommandTypeUser}
	csf.commandParsers["userdel"] = &SystemCommandParser{commandType: CommandTypeUser}
	csf.commandParsers["passwd"] = &SystemCommandParser{commandType: CommandTypeUser}
	
	// 包管理命令解析器
	csf.commandParsers["apt"] = &SystemCommandParser{commandType: CommandTypePackage}
	csf.commandParsers["yum"] = &SystemCommandParser{commandType: CommandTypePackage}
	csf.commandParsers["dnf"] = &SystemCommandParser{commandType: CommandTypePackage}
	csf.commandParsers["pip"] = &SystemCommandParser{commandType: CommandTypePackage}
	
	// 服务管理命令解析器
	csf.commandParsers["systemctl"] = &SystemCommandParser{commandType: CommandTypeService}
	csf.commandParsers["service"] = &SystemCommandParser{commandType: CommandTypeService}
}

// parseCommand 解析命令
func (csf *commandSecurityFilter) parseCommand(command string) (*ParsedCommand, error) {
	// 简单的命令解析实现
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	binary := parts[0]
	args := parts[1:]

	// 解析管道
	pipes := csf.parsePipes(command)

	// 解析重定向
	redirects := csf.parseRedirects(command)

	// 解析环境变量
	env := csf.parseEnvironment(command)

	// 解析标志
	flags := csf.parseFlags(args)

	return &ParsedCommand{
		Binary:      binary,
		Arguments:   args,
		Flags:       flags,
		Pipes:       pipes,
		Redirects:   redirects,
		Environment: env,
	}, nil
}

// determineCommandType 确定命令类型
func (csf *commandSecurityFilter) determineCommandType(cmd *ParsedCommand) CommandType {
	if parser, exists := csf.commandParsers[cmd.Binary]; exists {
		return parser.GetCommandType()
	}
	
	// 基于二进制文件路径推断
	binary := strings.ToLower(cmd.Binary)
	
	if strings.Contains(binary, "apt") || strings.Contains(binary, "yum") || 
	   strings.Contains(binary, "pip") || strings.Contains(binary, "npm") {
		return CommandTypePackage
	}
	
	if strings.Contains(binary, "systemctl") || strings.Contains(binary, "service") {
		return CommandTypeService
	}
	
	if strings.Contains(binary, "wget") || strings.Contains(binary, "curl") || 
	   strings.Contains(binary, "nc") || strings.Contains(binary, "ssh") {
		return CommandTypeNetwork
	}
	
	return CommandTypeCustom
}

// calculateBaseRiskScore 计算基础风险分数
func (csf *commandSecurityFilter) calculateBaseRiskScore(analysis *CommandAnalysis) float64 {
	score := 0.0

	// 基于命令类型的基础分数
	switch analysis.CommandType {
	case CommandTypeSystem:
		score += 3.0
	case CommandTypeNetwork:
		score += 4.0
	case CommandTypeUser:
		score += 5.0
	case CommandTypePackage:
		score += 3.0
	case CommandTypeService:
		score += 4.0
	case CommandTypeFile:
		score += 2.0
	default:
		score += 1.0
	}

	// 基于特殊行为的额外分数
	if analysis.NetworkAccess {
		score += 2.0
	}
	if analysis.SystemModify {
		score += 3.0
	}
	if analysis.PrivEscalation {
		score += 5.0
	}

	// 基于访问路径的分数
	for _, path := range analysis.AccessedPaths {
		if strings.HasPrefix(path, "/etc/") {
			score += 2.0
		} else if strings.HasPrefix(path, "/sys/") {
			score += 1.5
		} else if strings.HasPrefix(path, "/proc/") {
			score += 1.0
		}
	}

	return score
}

// calculatePatternRiskScore 基于危险模式计算风险分数
func (csf *commandSecurityFilter) calculatePatternRiskScore(patterns []DangerousPattern) float64 {
	score := 0.0
	
	for _, pattern := range patterns {
		switch pattern.Risk {
		case RiskLow:
			score += 1.0
		case RiskMedium:
			score += 2.0
		case RiskHigh:
			score += 4.0
		case RiskCritical:
			score += 8.0
		}
	}
	
	return score
}

// calculateContextRiskScore 基于上下文计算风险分数
func (csf *commandSecurityFilter) calculateContextRiskScore(cmdCtx *CommandContext) float64 {
	score := 0.0

	// 基于用户的风险调整
	if cmdCtx.User == "root" {
		score += 2.0
	}

	// 基于来源的风险调整
	switch cmdCtx.Source {
	case "remote":
		score += 3.0
	case "web":
		score += 2.0
	}

	// 基于工作目录的风险调整
	if strings.HasPrefix(cmdCtx.WorkingDir, "/etc/") {
		score += 2.0
	} else if strings.HasPrefix(cmdCtx.WorkingDir, "/root/") {
		score += 1.5
	}

	return score
}

// calculateEnvironmentRiskScore 基于环境因素计算风险分数
func (csf *commandSecurityFilter) calculateEnvironmentRiskScore(cmdCtx *CommandContext) float64 {
	score := 0.0

	// 检查环境变量中的可疑内容
	for key, value := range cmdCtx.Environment {
		if strings.Contains(key, "PASS") || strings.Contains(key, "TOKEN") {
			score += 1.0
		}
		if strings.Contains(value, "http://") && !strings.Contains(value, "localhost") {
			score += 1.5
		}
	}

	return score
}

// scoreToRiskLevel 将分数转换为风险级别
func (csf *commandSecurityFilter) scoreToRiskLevel(score float64) CommandRisk {
	if score >= 10.0 {
		return RiskCritical
	} else if score >= 6.0 {
		return RiskHigh
	} else if score >= 3.0 {
		return RiskMedium
	}
	return RiskLow
}

// getDangerousPatternInfo 获取危险模式信息
func (csf *commandSecurityFilter) getDangerousPatternInfo(index int, pattern string) DangerousPattern {
	// 基于模式索引返回相应的危险模式信息
	descriptions := map[string]DangerousPattern{
		"rm.*-rf.*\\/": {
			Pattern:     pattern,
			Risk:        RiskCritical,
			Description: "Recursive force deletion of root directory",
			Suggestion:  "Use specific paths instead of root directory",
			Category:    "file_system_damage",
		},
		"dd.*if=.*of=\\/dev\\/": {
			Pattern:     pattern,
			Risk:        RiskCritical,
			Description: "Potential disk/device overwrite operation",
			Suggestion:  "Verify target device before using dd command",
			Category:    "system_damage",
		},
		// 添加更多模式描述...
	}

	for patternKey, info := range descriptions {
		if matched, _ := regexp.MatchString(patternKey, pattern); matched {
			return info
		}
	}

	// 默认危险模式信息
	return DangerousPattern{
		Pattern:     pattern,
		Risk:        RiskMedium,
		Description: "Potentially dangerous command pattern detected",
		Suggestion:  "Review command carefully before execution",
		Category:    "general",
	}
}

// SystemCommandParser 系统命令解析器
type SystemCommandParser struct {
	commandType CommandType
}

func (scp *SystemCommandParser) Parse(command string) (*ParsedCommand, error) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	return &ParsedCommand{
		Binary:    parts[0],
		Arguments: parts[1:],
		Flags:     make(map[string]string),
	}, nil
}

func (scp *SystemCommandParser) GetCommandType() CommandType {
	return scp.commandType
}

func (scp *SystemCommandParser) RequiredPermissions() []string {
	return []string{}
}

// 辅助解析方法的简单实现
func (csf *commandSecurityFilter) parsePipes(command string) []*ParsedCommand {
	// 简化实现：检测管道符号
	if strings.Contains(command, "|") {
		// 这里可以实现更复杂的管道解析逻辑
		return []*ParsedCommand{}
	}
	return nil
}

func (csf *commandSecurityFilter) parseRedirects(command string) []Redirect {
	var redirects []Redirect
	// 简化实现：检测重定向符号
	if strings.Contains(command, ">") {
		// 这里可以实现更复杂的重定向解析逻辑
	}
	return redirects
}

func (csf *commandSecurityFilter) parseEnvironment(command string) map[string]string {
	env := make(map[string]string)
	// 简化实现：解析环境变量设置
	if strings.Contains(command, "=") {
		// 这里可以实现环境变量解析逻辑
	}
	return env
}

func (csf *commandSecurityFilter) parseFlags(args []string) map[string]string {
	flags := make(map[string]string)
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			flags[arg] = ""
		}
	}
	return flags
}

func (csf *commandSecurityFilter) analyzePermissionRequirements(cmd *ParsedCommand) []string {
	var perms []string
	
	// 基于命令分析所需权限
	binary := strings.ToLower(cmd.Binary)
	
	if binary == "sudo" || binary == "su" {
		perms = append(perms, "sudo")
	}
	
	if strings.Contains(binary, "mount") || strings.Contains(binary, "umount") {
		perms = append(perms, "mount")
	}
	
	return perms
}

func (csf *commandSecurityFilter) analyzeAccessedPaths(cmd *ParsedCommand) []string {
	var paths []string
	
	// 从参数中提取路径
	for _, arg := range cmd.Arguments {
		if strings.HasPrefix(arg, "/") || strings.HasPrefix(arg, "./") || strings.HasPrefix(arg, "../") {
			paths = append(paths, arg)
		}
	}
	
	return paths
}

func (csf *commandSecurityFilter) checkNetworkAccess(cmd *ParsedCommand) bool {
	networkCommands := []string{"wget", "curl", "nc", "nmap", "ssh", "scp", "rsync", "ping", "telnet", "ftp"}
	
	for _, netCmd := range networkCommands {
		if cmd.Binary == netCmd {
			return true
		}
	}
	
	return false
}

func (csf *commandSecurityFilter) checkSystemModification(cmd *ParsedCommand) bool {
	modifyCommands := []string{"rm", "rmdir", "mv", "chmod", "chown", "ln", "mkdir", "touch", "dd"}
	
	for _, modCmd := range modifyCommands {
		if cmd.Binary == modCmd {
			return true
		}
	}
	
	return false
}

func (csf *commandSecurityFilter) checkPrivilegeEscalation(cmd *ParsedCommand) bool {
	escalationCommands := []string{"sudo", "su", "pkexec"}
	
	for _, escCmd := range escalationCommands {
		if cmd.Binary == escCmd {
			return true
		}
	}
	
	// 检查是否试图修改关键系统文件
	for _, arg := range cmd.Arguments {
		if strings.Contains(arg, "/etc/passwd") || strings.Contains(arg, "/etc/shadow") {
			return true
		}
	}
	
	return false
}

func (csf *commandSecurityFilter) checkSpecificDangerousCommands(command string) []DangerousPattern {
	var patterns []DangerousPattern
	
	dangerousCommands := map[string]DangerousPattern{
		":(){ :|:& };:": {
			Pattern:     "fork_bomb",
			Risk:        RiskCritical,
			Description: "Fork bomb attack detected",
			Suggestion:  "This command will consume all system resources",
			Category:    "denial_of_service",
		},
		"rm -rf /": {
			Pattern:     "root_deletion",
			Risk:        RiskCritical,
			Description: "Attempt to delete entire root filesystem",
			Suggestion:  "This will destroy the entire system",
			Category:    "system_destruction",
		},
	}
	
	for cmdPattern, pattern := range dangerousCommands {
		if strings.Contains(command, cmdPattern) {
			patterns = append(patterns, pattern)
		}
	}
	
	return patterns
}

func (csf *commandSecurityFilter) checkCommandInjection(command string) []DangerousPattern {
	var patterns []DangerousPattern
	
	// 检查命令注入模式
	injectionPatterns := []string{
		";", "&&", "||", "|", "`", "$(", "${",
	}
	
	for _, pattern := range injectionPatterns {
		if strings.Contains(command, pattern) {
			patterns = append(patterns, DangerousPattern{
				Pattern:     pattern,
				Risk:        RiskHigh,
				Description: "Potential command injection detected",
				Suggestion:  "Avoid using shell metacharacters in commands",
				Category:    "command_injection",
			})
		}
	}
	
	return patterns
}

func (csf *commandSecurityFilter) removeCommandInjection(command string) string {
	// 移除潜在的命令注入字符
	dangerous := []string{";", "&&", "||", "`"}
	
	result := command
	for _, char := range dangerous {
		result = strings.ReplaceAll(result, char, "")
	}
	
	return result
}

func (csf *commandSecurityFilter) escapeSpecialCharacters(command string) string {
	// 转义特殊字符
	specialChars := map[string]string{
		"$": "\\$",
		"`": "\\`",
		"\"": "\\\"",
		"'": "\\'",
	}
	
	result := command
	for char, escaped := range specialChars {
		result = strings.ReplaceAll(result, char, escaped)
	}
	
	return result
}