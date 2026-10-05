package security

import (
	"context"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ThreatDetector 威胁检测器接口
type ThreatDetector interface {
	// 检测威胁
	DetectThreats(ctx context.Context, cmdCtx *CommandContext) ([]ThreatInfo, error)
	
	// 添加威胁特征
	AddThreatSignature(signature *ThreatSignature) error
	
	// 更新威胁情报
	UpdateThreatIntelligence(intelligence *ThreatIntelligence) error
	
	// 获取威胁统计
	GetThreatStatistics() (*ThreatStatistics, error)
}

// ThreatSignature 威胁特征
type ThreatSignature struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	ThreatType  ThreatType  `json:"threat_type"`
	Severity    CommandRisk `json:"severity"`
	Pattern     string      `json:"pattern"`
	Description string      `json:"description"`
	MITRE_ID    string      `json:"mitre_id"` // MITRE ATT&CK框架ID
	Indicators  []string    `json:"indicators"`
	Regex       *regexp.Regexp `json:"-"`
}

// ThreatIntelligence 威胁情报
type ThreatIntelligence struct {
	IOCs           []IOC           `json:"iocs"`           // Indicators of Compromise
	KnownMalware   []MalwareInfo   `json:"known_malware"`
	AttackPatterns []AttackPattern `json:"attack_patterns"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// IOC Indicator of Compromise
type IOC struct {
	Type        IOCType `json:"type"`
	Value       string  `json:"value"`
	Description string  `json:"description"`
	Severity    CommandRisk `json:"severity"`
	Source      string  `json:"source"`
	FirstSeen   time.Time `json:"first_seen"`
	LastSeen    time.Time `json:"last_seen"`
}

// IOCType IOC类型
type IOCType string

const (
	IOCTypeIP       IOCType = "ip"
	IOCTypeDomain   IOCType = "domain"
	IOCTypeURL      IOCType = "url"
	IOCTypeHash     IOCType = "hash"
	IOCTypeCommand  IOCType = "command"
	IOCTypeFilename IOCType = "filename"
)

// MalwareInfo 恶意软件信息
type MalwareInfo struct {
	Name        string      `json:"name"`
	Family      string      `json:"family"`
	Hashes      []string    `json:"hashes"`
	Commands    []string    `json:"commands"`
	Behaviors   []string    `json:"behaviors"`
	Severity    CommandRisk `json:"severity"`
	Description string      `json:"description"`
}

// AttackPattern 攻击模式
type AttackPattern struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Technique   string      `json:"technique"`
	Tactic      string      `json:"tactic"`
	Commands    []string    `json:"commands"`
	Indicators  []string    `json:"indicators"`
	Severity    CommandRisk `json:"severity"`
	Description string      `json:"description"`
}

// ThreatStatistics 威胁统计
type ThreatStatistics struct {
	TotalDetections     int64                  `json:"total_detections"`
	DetectionsByType    map[ThreatType]int64   `json:"detections_by_type"`
	DetectionsBySeverity map[CommandRisk]int64 `json:"detections_by_severity"`
	LastDetection       *time.Time             `json:"last_detection"`
	TopThreats          []ThreatSummary        `json:"top_threats"`
}

// ThreatSummary 威胁摘要
type ThreatSummary struct {
	ThreatType  ThreatType `json:"threat_type"`
	Count       int64      `json:"count"`
	LastSeen    time.Time  `json:"last_seen"`
	Severity    CommandRisk `json:"severity"`
}

// threatDetector 威胁检测器实现
type threatDetector struct {
	policy         *SecurityPolicy
	logger         *slog.Logger
	
	// 威胁特征库
	signatures     map[string]*ThreatSignature
	intelligence   *ThreatIntelligence
	
	// 检测规则
	malwarePatterns   []*regexp.Regexp
	exploitPatterns   []*regexp.Regexp
	persistencePatterns []*regexp.Regexp
	
	// 统计信息
	statistics     *ThreatStatistics
	mu             sync.RWMutex
	
	// 行为分析
	commandHistory map[string][]CommandHistoryEntry
	historyMu      sync.RWMutex
}

// CommandHistoryEntry 命令历史条目
type CommandHistoryEntry struct {
	Command   string    `json:"command"`
	Timestamp time.Time `json:"timestamp"`
	RiskLevel CommandRisk `json:"risk_level"`
}

// NewThreatDetector 创建威胁检测器
func NewThreatDetector(policy *SecurityPolicy, logger *slog.Logger) ThreatDetector {
	if logger == nil {
		logger = slog.Default()
	}

	td := &threatDetector{
		policy:          policy,
		logger:          logger.With("component", "threat_detector"),
		signatures:      make(map[string]*ThreatSignature),
		intelligence:    &ThreatIntelligence{},
		statistics:      &ThreatStatistics{
			DetectionsByType:     make(map[ThreatType]int64),
			DetectionsBySeverity: make(map[CommandRisk]int64),
		},
		commandHistory:  make(map[string][]CommandHistoryEntry),
	}

	// 初始化默认威胁特征
	td.initializeDefaultSignatures()
	
	// 编译检测模式
	td.compileDetectionPatterns()

	td.logger.Info("threat detector initialized")
	return td
}

// DetectThreats 检测威胁
func (td *threatDetector) DetectThreats(ctx context.Context, cmdCtx *CommandContext) ([]ThreatInfo, error) {
	td.mu.Lock()
	defer td.mu.Unlock()

	td.logger.Debug("detecting threats", "command", cmdCtx.Command, "user", cmdCtx.User)

	var threats []ThreatInfo

	// 记录命令历史
	td.recordCommandHistory(cmdCtx)

	// 检测恶意软件
	if malwareThreats := td.detectMalware(cmdCtx); len(malwareThreats) > 0 {
		threats = append(threats, malwareThreats...)
	}

	// 检测代码注入
	if injectionThreats := td.detectCodeInjection(cmdCtx); len(injectionThreats) > 0 {
		threats = append(threats, injectionThreats...)
	}

	// 检测权限提升
	if privescThreats := td.detectPrivilegeEscalation(cmdCtx); len(privescThreats) > 0 {
		threats = append(threats, privescThreats...)
	}

	// 检测数据泄露
	if exfilThreats := td.detectDataExfiltration(cmdCtx); len(exfilThreats) > 0 {
		threats = append(threats, exfilThreats...)
	}

	// 检测系统破坏
	if damageThreats := td.detectSystemDamage(cmdCtx); len(damageThreats) > 0 {
		threats = append(threats, damageThreats...)
	}

	// 检测持久化
	if persistenceThreats := td.detectPersistence(cmdCtx); len(persistenceThreats) > 0 {
		threats = append(threats, persistenceThreats...)
	}

	// 检测侦察活动
	if reconThreats := td.detectReconnaissance(cmdCtx); len(reconThreats) > 0 {
		threats = append(threats, reconThreats...)
	}

	// 行为分析
	if behaviorThreats := td.analyzeBehaviorPatterns(cmdCtx); len(behaviorThreats) > 0 {
		threats = append(threats, behaviorThreats...)
	}

	// 更新统计信息
	td.updateStatistics(threats)

	if len(threats) > 0 {
		td.logger.Warn("threats detected",
			"command", cmdCtx.Command,
			"threat_count", len(threats),
			"user", cmdCtx.User)
	}

	return threats, nil
}

// AddThreatSignature 添加威胁特征
func (td *threatDetector) AddThreatSignature(signature *ThreatSignature) error {
	td.mu.Lock()
	defer td.mu.Unlock()

	// 编译正则表达式
	if signature.Pattern != "" {
		regex, err := regexp.Compile(signature.Pattern)
		if err != nil {
			return err
		}
		signature.Regex = regex
	}

	td.signatures[signature.ID] = signature
	td.logger.Info("threat signature added", "id", signature.ID, "name", signature.Name)
	
	return nil
}

// UpdateThreatIntelligence 更新威胁情报
func (td *threatDetector) UpdateThreatIntelligence(intelligence *ThreatIntelligence) error {
	td.mu.Lock()
	defer td.mu.Unlock()

	td.intelligence = intelligence
	td.intelligence.UpdatedAt = time.Now()
	
	td.logger.Info("threat intelligence updated",
		"iocs", len(intelligence.IOCs),
		"malware", len(intelligence.KnownMalware),
		"patterns", len(intelligence.AttackPatterns))
	
	return nil
}

// GetThreatStatistics 获取威胁统计
func (td *threatDetector) GetThreatStatistics() (*ThreatStatistics, error) {
	td.mu.RLock()
	defer td.mu.RUnlock()

	// 返回统计信息副本
	statsCopy := *td.statistics
	statsCopy.DetectionsByType = make(map[ThreatType]int64)
	statsCopy.DetectionsBySeverity = make(map[CommandRisk]int64)
	
	for k, v := range td.statistics.DetectionsByType {
		statsCopy.DetectionsByType[k] = v
	}
	for k, v := range td.statistics.DetectionsBySeverity {
		statsCopy.DetectionsBySeverity[k] = v
	}

	return &statsCopy, nil
}

// initializeDefaultSignatures 初始化默认威胁特征
func (td *threatDetector) initializeDefaultSignatures() {
	defaultSignatures := []*ThreatSignature{
		{
			ID:          "malware_nc_reverse_shell",
			Name:        "Netcat Reverse Shell",
			ThreatType:  ThreatTypeMalware,
			Severity:    RiskCritical,
			Pattern:     `nc\s+.*-e\s+(\/bin\/sh|\/bin\/bash|cmd\.exe)`,
			Description: "Netcat reverse shell execution detected",
			MITRE_ID:    "T1059",
			Indicators:  []string{"nc", "-e", "/bin/sh", "/bin/bash"},
		},
		{
			ID:          "privesc_sudo_su",
			Name:        "Privilege Escalation via sudo/su",
			ThreatType:  ThreatTypePrivEscalation,
			Severity:    RiskHigh,
			Pattern:     `sudo\s+su\s*-|su\s+-.*root`,
			Description: "Privilege escalation attempt detected",
			MITRE_ID:    "T1548",
			Indicators:  []string{"sudo su", "su - root"},
		},
		{
			ID:          "exfil_data_curl",
			Name:        "Data Exfiltration via curl",
			ThreatType:  ThreatTypeDataExfiltration,
			Severity:    RiskHigh,
			Pattern:     `curl\s+.*-X\s+POST.*--data`,
			Description: "Potential data exfiltration via curl POST request",
			MITRE_ID:    "T1041",
			Indicators:  []string{"curl", "POST", "--data"},
		},
		{
			ID:          "system_damage_rm_rf",
			Name:        "System Destruction",
			ThreatType:  ThreatTypeSystemDamage,
			Severity:    RiskCritical,
			Pattern:     `rm\s+-rf\s+\/`,
			Description: "Attempt to delete system files detected",
			MITRE_ID:    "T1485",
			Indicators:  []string{"rm -rf /"},
		},
		{
			ID:          "persistence_crontab",
			Name:        "Persistence via Crontab",
			ThreatType:  ThreatTypeMalware,
			Severity:    RiskHigh,
			Pattern:     `crontab\s+-e|echo.*>.*\/var\/spool\/cron`,
			Description: "Persistence mechanism via crontab detected",
			MITRE_ID:    "T1053",
			Indicators:  []string{"crontab", "/var/spool/cron"},
		},
	}

	for _, signature := range defaultSignatures {
		td.AddThreatSignature(signature)
	}
}

// compileDetectionPatterns 编译检测模式
func (td *threatDetector) compileDetectionPatterns() {
	// 恶意软件模式
	malwarePatterns := []string{
		`wget.*\|\s*sh`,
		`curl.*\|\s*bash`,
		`python.*-c.*exec`,
		`perl.*-e.*exec`,
		`nc\s+.*-l.*-p`,
		`/dev/tcp/`,
		`base64\s+-d.*\|\s*sh`,
	}

	// 漏洞利用模式
	exploitPatterns := []string{
		`\$\(.*\)`,
		`\`.*\``,
		`\|\s*sh`,
		`\|\s*bash`,
		`&&\s*(rm|dd|mkfs)`,
		`;\s*(rm|dd|mkfs)`,
	}

	// 持久化模式
	persistencePatterns := []string{
		`echo.*>>.*\.bashrc`,
		`echo.*>>.*\.profile`,
		`echo.*>>.*\.bash_profile`,
		`crontab\s+-`,
		`systemctl.*enable`,
		`chkconfig.*on`,
	}

	td.malwarePatterns = td.compilePatterns(malwarePatterns)
	td.exploitPatterns = td.compilePatterns(exploitPatterns)
	td.persistencePatterns = td.compilePatterns(persistencePatterns)
}

// compilePatterns 编译正则表达式模式
func (td *threatDetector) compilePatterns(patterns []string) []*regexp.Regexp {
	var compiled []*regexp.Regexp
	for _, pattern := range patterns {
		if regex, err := regexp.Compile(pattern); err == nil {
			compiled = append(compiled, regex)
		} else {
			td.logger.Warn("failed to compile threat pattern", "pattern", pattern, "error", err)
		}
	}
	return compiled
}

// detectMalware 检测恶意软件
func (td *threatDetector) detectMalware(cmdCtx *CommandContext) []ThreatInfo {
	var threats []ThreatInfo

	command := cmdCtx.Command

	// 检查已知恶意软件签名
	for _, signature := range td.signatures {
		if signature.ThreatType == ThreatTypeMalware && signature.Regex != nil {
			if signature.Regex.MatchString(command) {
				threats = append(threats, ThreatInfo{
					ThreatType:  signature.ThreatType,
					Severity:    signature.Severity,
					Description: signature.Description,
					Indicators:  signature.Indicators,
					Mitigation:  "Block command execution and investigate",
				})
			}
		}
	}

	// 检查恶意软件模式
	for _, pattern := range td.malwarePatterns {
		if pattern.MatchString(command) {
			threats = append(threats, ThreatInfo{
				ThreatType:  ThreatTypeMalware,
				Severity:    RiskHigh,
				Description: "Malicious command pattern detected",
				Indicators:  []string{command},
				Mitigation:  "Review command and block if malicious",
			})
		}
	}

	// 检查已知IOC
	for _, ioc := range td.intelligence.IOCs {
		if ioc.Type == IOCTypeCommand && strings.Contains(command, ioc.Value) {
			threats = append(threats, ThreatInfo{
				ThreatType:  ThreatTypeMalware,
				Severity:    ioc.Severity,
				Description: fmt.Sprintf("Known malicious IOC detected: %s", ioc.Description),
				Indicators:  []string{ioc.Value},
				Mitigation:  "Block immediately - known threat",
			})
		}
	}

	return threats
}

// detectCodeInjection 检测代码注入
func (td *threatDetector) detectCodeInjection(cmdCtx *CommandContext) []ThreatInfo {
	var threats []ThreatInfo

	command := cmdCtx.Command

	// 检查命令注入模式
	injectionPatterns := []string{
		`;`, `&&`, `||`, `|`, `$(`, "`", `${}`,
	}

	for _, pattern := range injectionPatterns {
		if strings.Contains(command, pattern) {
			threats = append(threats, ThreatInfo{
				ThreatType:  ThreatTypeCodeInjection,
				Severity:    RiskMedium,
				Description: fmt.Sprintf("Command injection pattern detected: %s", pattern),
				Indicators:  []string{pattern},
				Mitigation:  "Sanitize command input and validate",
			})
		}
	}

	// 检查脚本注入
	scriptPatterns := []regexp.Regexp{
		*regexp.MustCompile(`<script.*>`),
		*regexp.MustCompile(`javascript:`),
		*regexp.MustCompile(`eval\s*\(`),
		*regexp.MustCompile(`exec\s*\(`),
	}

	for _, pattern := range scriptPatterns {
		if pattern.MatchString(command) {
			threats = append(threats, ThreatInfo{
				ThreatType:  ThreatTypeCodeInjection,
				Severity:    RiskHigh,
				Description: "Script injection detected",
				Indicators:  []string{pattern.String()},
				Mitigation:  "Block execution and sanitize input",
			})
		}
	}

	return threats
}

// detectPrivilegeEscalation 检测权限提升
func (td *threatDetector) detectPrivilegeEscalation(cmdCtx *CommandContext) []ThreatInfo {
	var threats []ThreatInfo

	command := cmdCtx.Command

	// 检查权限提升命令
	privescCommands := []string{
		"sudo", "su", "pkexec", "runuser",
	}

	for _, cmd := range privescCommands {
		if strings.HasPrefix(command, cmd+" ") {
			severity := RiskMedium
			if cmd == "sudo" && strings.Contains(command, "su -") {
				severity = RiskHigh
			}

			threats = append(threats, ThreatInfo{
				ThreatType:  ThreatTypePrivEscalation,
				Severity:    severity,
				Description: fmt.Sprintf("Privilege escalation attempt via %s", cmd),
				Indicators:  []string{cmd},
				Mitigation:  "Verify user authorization for privilege escalation",
			})
		}
	}

	// 检查SUID/SGID操作
	if strings.Contains(command, "chmod") && (strings.Contains(command, "4") || strings.Contains(command, "2")) {
		threats = append(threats, ThreatInfo{
			ThreatType:  ThreatTypePrivEscalation,
			Severity:    RiskHigh,
			Description: "SUID/SGID bit manipulation detected",
			Indicators:  []string{"chmod", "suid", "sgid"},
			Mitigation:  "Review file permissions change carefully",
		})
	}

	return threats
}

// detectDataExfiltration 检测数据泄露
func (td *threatDetector) detectDataExfiltration(cmdCtx *CommandContext) []ThreatInfo {
	var threats []ThreatInfo

	command := cmdCtx.Command

	// 检查敏感文件访问
	sensitiveFiles := []string{
		"/etc/passwd", "/etc/shadow", "/etc/hosts", "/root/.ssh",
		"/home/*/.ssh", "*.key", "*.pem", "*.p12", "*.pfx",
	}

	for _, file := range sensitiveFiles {
		if strings.Contains(command, file) {
			threats = append(threats, ThreatInfo{
				ThreatType:  ThreatTypeDataExfiltration,
				Severity:    RiskHigh,
				Description: fmt.Sprintf("Access to sensitive file detected: %s", file),
				Indicators:  []string{file},
				Mitigation:  "Monitor file access and ensure authorized use",
			})
		}
	}

	// 检查网络传输
	networkCommands := []string{"curl", "wget", "scp", "rsync", "nc"}
	for _, netCmd := range networkCommands {
		if strings.Contains(command, netCmd) && 
		   (strings.Contains(command, "POST") || strings.Contains(command, "PUT")) {
			threats = append(threats, ThreatInfo{
				ThreatType:  ThreatTypeDataExfiltration,
				Severity:    RiskMedium,
				Description: "Potential data upload via network command",
				Indicators:  []string{netCmd, "upload"},
				Mitigation:  "Monitor network traffic and verify data transfer",
			})
		}
	}

	return threats
}

// detectSystemDamage 检测系统破坏
func (td *threatDetector) detectSystemDamage(cmdCtx *CommandContext) []ThreatInfo {
	var threats []ThreatInfo

	command := cmdCtx.Command

	// 检查破坏性命令
	destructivePatterns := []string{
		"rm -rf /", "dd if=/dev/zero", "mkfs", "fdisk", 
		":(){ :|:& };:", "fork()", "while true",
	}

	for _, pattern := range destructivePatterns {
		if strings.Contains(command, pattern) {
			threats = append(threats, ThreatInfo{
				ThreatType:  ThreatTypeSystemDamage,
				Severity:    RiskCritical,
				Description: fmt.Sprintf("Destructive operation detected: %s", pattern),
				Indicators:  []string{pattern},
				Mitigation:  "Block immediately - system destruction risk",
			})
		}
	}

	return threats
}

// detectPersistence 检测持久化
func (td *threatDetector) detectPersistence(cmdCtx *CommandContext) []ThreatInfo {
	var threats []ThreatInfo

	command := cmdCtx.Command

	// 检查持久化模式
	for _, pattern := range td.persistencePatterns {
		if pattern.MatchString(command) {
			threats = append(threats, ThreatInfo{
				ThreatType:  ThreatTypeMalware,
				Severity:    RiskMedium,
				Description: "Persistence mechanism detected",
				Indicators:  []string{pattern.String()},
				Mitigation:  "Review startup/configuration changes",
			})
		}
	}

	return threats
}

// detectReconnaissance 检测侦察活动
func (td *threatDetector) detectReconnaissance(cmdCtx *CommandContext) []ThreatInfo {
	var threats []ThreatInfo

	command := cmdCtx.Command

	// 检查侦察命令
	reconCommands := []string{
		"nmap", "masscan", "zmap", "netstat", "ss", 
		"ps aux", "lsof", "netcat", "telnet", "dig", "nslookup",
	}

	for _, reconCmd := range reconCommands {
		if strings.Contains(command, reconCmd) {
			threats = append(threats, ThreatInfo{
				ThreatType:  ThreatTypeRecon,
				Severity:    RiskLow,
				Description: fmt.Sprintf("Reconnaissance activity detected: %s", reconCmd),
				Indicators:  []string{reconCmd},
				Mitigation:  "Monitor for suspicious scanning activity",
			})
		}
	}

	return threats
}

// analyzeBehaviorPatterns 分析行为模式
func (td *threatDetector) analyzeBehaviorPatterns(cmdCtx *CommandContext) []ThreatInfo {
	td.historyMu.RLock()
	defer td.historyMu.RUnlock()

	var threats []ThreatInfo

	history, exists := td.commandHistory[cmdCtx.User]
	if !exists || len(history) < 3 {
		return threats
	}

	// 检查快速连续的高风险命令
	recentCommands := 0
	highRiskCommands := 0
	now := time.Now()

	for _, entry := range history {
		if now.Sub(entry.Timestamp) < 5*time.Minute {
			recentCommands++
			if entry.RiskLevel >= RiskHigh {
				highRiskCommands++
			}
		}
	}

	if recentCommands > 10 {
		threats = append(threats, ThreatInfo{
			ThreatType:  ThreatTypeSuspiciousActivity,
			Severity:    RiskMedium,
			Description: "Unusually high command frequency detected",
			Indicators:  []string{"high_frequency"},
			Mitigation:  "Monitor user activity for automation or compromise",
		})
	}

	if highRiskCommands > 3 {
		threats = append(threats, ThreatInfo{
			ThreatType:  ThreatTypeSuspiciousActivity,
			Severity:    RiskHigh,
			Description: "Multiple high-risk commands in short timeframe",
			Indicators:  []string{"high_risk_sequence"},
			Mitigation:  "Investigate user session for potential compromise",
		})
	}

	return threats
}

// recordCommandHistory 记录命令历史
func (td *threatDetector) recordCommandHistory(cmdCtx *CommandContext) {
	td.historyMu.Lock()
	defer td.historyMu.Unlock()

	// 假设风险级别已经计算过
	riskLevel := RiskLow // 这里应该从之前的分析中获取

	entry := CommandHistoryEntry{
		Command:   cmdCtx.Command,
		Timestamp: time.Now(),
		RiskLevel: riskLevel,
	}

	history, exists := td.commandHistory[cmdCtx.User]
	if !exists {
		history = make([]CommandHistoryEntry, 0)
	}

	// 添加新条目
	history = append(history, entry)

	// 保持历史记录在合理大小内
	maxHistory := 100
	if len(history) > maxHistory {
		history = history[len(history)-maxHistory:]
	}

	td.commandHistory[cmdCtx.User] = history
}

// updateStatistics 更新统计信息
func (td *threatDetector) updateStatistics(threats []ThreatInfo) {
	if len(threats) == 0 {
		return
	}

	td.statistics.TotalDetections += int64(len(threats))
	now := time.Now()
	td.statistics.LastDetection = &now

	for _, threat := range threats {
		td.statistics.DetectionsByType[threat.ThreatType]++
		td.statistics.DetectionsBySeverity[threat.Severity]++
	}
}

// ThreatTypeSuspiciousActivity 可疑活动威胁类型
const ThreatTypeSuspiciousActivity ThreatType = "suspicious_activity"