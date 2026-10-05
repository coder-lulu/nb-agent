package security

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// securityManager 安全管理器实现
type securityManager struct {
	policy          *SecurityPolicy
	commandFilter   CommandSecurityFilter
	auditLogger     AuditLogger
	threatDetector  ThreatDetector
	sandboxManager  SandboxManager
	alertManager    AlertManager
	metrics         *SecurityMetrics
	
	mu              sync.RWMutex
	logger          *slog.Logger
	
	// 运行时状态
	activeSandboxes map[string]SandboxEnvironment
	activeCommands  map[string]*CommandContext
	userSessions    map[string]*UserSession
}

// UserSession 用户会话
type UserSession struct {
	UserID          string    `json:"user_id"`
	SessionID       string    `json:"session_id"`
	StartTime       time.Time `json:"start_time"`
	LastActivity    time.Time `json:"last_activity"`
	CommandCount    int       `json:"command_count"`
	FailedAttempts  int       `json:"failed_attempts"`
	RiskScore       float64   `json:"risk_score"`
	Authenticated   bool      `json:"authenticated"`
}

// NewSecurityManager 创建安全管理器
func NewSecurityManager(policy *SecurityPolicy, logger *slog.Logger) SecurityManager {
	if logger == nil {
		logger = slog.Default()
	}
	
	if policy == nil {
		policy = DefaultSecurityPolicy()
	}

	sm := &securityManager{
		policy:          policy,
		metrics:         &SecurityMetrics{},
		activeSandboxes: make(map[string]SandboxEnvironment),
		activeCommands:  make(map[string]*CommandContext),
		userSessions:    make(map[string]*UserSession),
		logger:          logger.With("component", "security_manager"),
	}

	// 初始化子组件
	sm.commandFilter = NewCommandSecurityFilter(policy, logger)
	sm.auditLogger = NewAuditLogger(policy, logger)
	sm.threatDetector = NewThreatDetector(policy, logger)
	sm.sandboxManager = NewSandboxManager(policy, logger)
	sm.alertManager = NewAlertManager(policy, logger)

	sm.logger.Info("security manager initialized")
	return sm
}

// LoadPolicy 加载安全策略
func (sm *securityManager) LoadPolicy(policy *SecurityPolicy) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if policy == nil {
		return fmt.Errorf("policy cannot be nil")
	}

	sm.policy = policy
	
	// 更新子组件策略
	if err := sm.updateComponentPolicies(); err != nil {
		return fmt.Errorf("failed to update component policies: %w", err)
	}

	sm.logger.Info("security policy loaded successfully")
	return nil
}

// GetPolicy 获取当前安全策略
func (sm *securityManager) GetPolicy() *SecurityPolicy {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	// 返回策略副本
	policyCopy := *sm.policy
	return &policyCopy
}

// UpdatePolicy 更新安全策略
func (sm *securityManager) UpdatePolicy(policy *SecurityPolicy) error {
	return sm.LoadPolicy(policy)
}

// CheckCommand 检查命令安全性
func (sm *securityManager) CheckCommand(ctx context.Context, cmdCtx *CommandContext) (*SecurityCheck, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.logger.Info("checking command security",
		"command", cmdCtx.Command,
		"user", cmdCtx.User,
		"source", cmdCtx.Source)

	// 记录命令执行请求
	sm.activeCommands[cmdCtx.RequestID] = cmdCtx

	// 更新用户会话
	sm.updateUserSession(cmdCtx)

	// 基础安全检查
	check, err := sm.performBasicSecurityCheck(ctx, cmdCtx)
	if err != nil {
		return nil, fmt.Errorf("basic security check failed: %w", err)
	}

	// 如果基础检查未通过，直接返回
	if !check.Allowed {
		sm.logSecurityViolation(cmdCtx, check.Reason)
		return check, nil
	}

	// 高级威胁检测
	threats, err := sm.threatDetector.DetectThreats(ctx, cmdCtx)
	if err != nil {
		sm.logger.Warn("threat detection failed", "error", err)
	} else if len(threats) > 0 {
		check.Allowed = false
		check.RiskLevel = RiskCritical
		check.Reason = "Threat detected"
		check.Warnings = sm.threatsToWarnings(threats)
		sm.logThreatDetection(cmdCtx, threats)
		return check, nil
	}

	// 更新安全指标
	sm.updateSecurityMetrics(cmdCtx, check)

	// 记录审计事件
	sm.logCommandCheck(cmdCtx, check)

	sm.logger.Info("command security check completed",
		"allowed", check.Allowed,
		"risk_level", check.RiskLevel,
		"require_confirm", check.RequireConfirm)

	return check, nil
}

// ValidateCommand 验证命令格式和安全性
func (sm *securityManager) ValidateCommand(command string) (*SecurityCheck, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// 创建临时命令上下文
	cmdCtx := &CommandContext{
		Command:   command,
		User:      "system",
		Source:    "validation",
		Timestamp: time.Now(),
		RequestID: uuid.New().String(),
	}

	// 使用命令过滤器进行分析
	analysis, err := sm.commandFilter.AnalyzeCommand(command)
	if err != nil {
		return &SecurityCheck{
			Allowed:   false,
			RiskLevel: RiskHigh,
			Reason:    fmt.Sprintf("Command analysis failed: %v", err),
		}, nil
	}

	// 检测危险模式
	patterns, err := sm.commandFilter.DetectDangerousPatterns(command)
	if err != nil {
		return &SecurityCheck{
			Allowed:   false,
			RiskLevel: RiskHigh,
			Reason:    fmt.Sprintf("Pattern detection failed: %v", err),
		}, nil
	}

	// 评估风险
	risk, err := sm.commandFilter.AssessRisk(context.Background(), cmdCtx)
	if err != nil {
		return &SecurityCheck{
			Allowed:   false,
			RiskLevel: RiskHigh,
			Reason:    fmt.Sprintf("Risk assessment failed: %v", err),
		}, nil
	}

	// 构建检查结果
	check := &SecurityCheck{
		Allowed:        len(patterns) == 0 || risk < RiskCritical,
		RiskLevel:      risk,
		RequireConfirm: risk >= RiskHigh || sm.RequireConfirmation(command),
		Warnings:       sm.patternsToWarnings(patterns),
	}

	if !check.Allowed {
		check.Reason = "Command contains dangerous patterns or high risk"
		check.Suggestions = sm.generateSecuritySuggestions(analysis, patterns)
	}

	return check, nil
}

// CheckPermission 检查用户权限
func (sm *securityManager) CheckPermission(user, operation string) (bool, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// 检查用户是否在黑名单中
	for _, blockedUser := range sm.policy.BlockedUsers {
		if user == blockedUser {
			return false, fmt.Errorf("user %s is blocked", user)
		}
	}

	// 检查用户是否在白名单中
	if len(sm.policy.AllowedUsers) > 0 {
		allowed := false
		for _, allowedUser := range sm.policy.AllowedUsers {
			if user == allowedUser {
				allowed = true
				break
			}
		}
		if !allowed {
			return false, fmt.Errorf("user %s is not in allowed users list", user)
		}
	}

	// 检查特定操作权限
	switch operation {
	case "root_access":
		return user == "root", nil
	case "system_modify":
		return sm.hasSystemModifyPermission(user), nil
	case "network_access":
		return sm.hasNetworkPermission(user), nil
	default:
		return true, nil
	}
}

// RequireConfirmation 检查命令是否需要确认
func (sm *securityManager) RequireConfirmation(command string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	for _, confirmCmd := range sm.policy.RequireConfirmation {
		if command == confirmCmd {
			return true
		}
	}

	// 检查高风险命令模式
	highRiskPatterns := []string{
		"rm -rf", "dd if=", "mkfs", "fdisk", "sudo", "su -",
	}

	for _, pattern := range highRiskPatterns {
		if command == pattern {
			return true
		}
	}

	return false
}

// LogAuditEvent 记录审计事件
func (sm *securityManager) LogAuditEvent(event *AuditEvent) error {
	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	return sm.auditLogger.LogEvent(event)
}

// GetAuditEvents 获取审计事件
func (sm *securityManager) GetAuditEvents(filter *AuditFilter) ([]*AuditEvent, error) {
	return sm.auditLogger.GetEvents(filter)
}

// DetectThreats 检测威胁
func (sm *securityManager) DetectThreats(ctx context.Context, cmdCtx *CommandContext) ([]ThreatInfo, error) {
	return sm.threatDetector.DetectThreats(ctx, cmdCtx)
}

// CreateSandbox 创建沙箱环境
func (sm *securityManager) CreateSandbox(ctx context.Context) (SandboxEnvironment, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if !sm.policy.EnableSandbox {
		return nil, fmt.Errorf("sandbox is disabled in security policy")
	}

	sandbox, err := sm.sandboxManager.CreateSandbox(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create sandbox: %w", err)
	}

	// 记录活跃沙箱
	sandboxID := uuid.New().String()
	sm.activeSandboxes[sandboxID] = sandbox

	sm.logger.Info("sandbox created", "sandbox_id", sandboxID)
	return sandbox, nil
}

// DestroySandbox 销毁沙箱环境
func (sm *securityManager) DestroySandbox(sandbox SandboxEnvironment) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// 从活跃沙箱中移除
	for id, sb := range sm.activeSandboxes {
		if sb == sandbox {
			delete(sm.activeSandboxes, id)
			break
		}
	}

	return sm.sandboxManager.DestroySandbox(sandbox)
}

// performBasicSecurityCheck 执行基础安全检查
func (sm *securityManager) performBasicSecurityCheck(ctx context.Context, cmdCtx *CommandContext) (*SecurityCheck, error) {
	// 检查命令是否为空
	if cmdCtx.Command == "" {
		return &SecurityCheck{
			Allowed:   false,
			RiskLevel: RiskLow,
			Reason:    "Empty command",
		}, nil
	}

	// 检查命令是否在黑名单中
	for _, blockedCmd := range sm.policy.BlockedCommands {
		if cmdCtx.Command == blockedCmd {
			return &SecurityCheck{
				Allowed:   false,
				RiskLevel: RiskCritical,
				Reason:    "Command is in blocked list",
			}, nil
		}
	}

	// 检查用户权限
	allowed, err := sm.CheckPermission(cmdCtx.User, "command_execution")
	if err != nil {
		return &SecurityCheck{
			Allowed:   false,
			RiskLevel: RiskHigh,
			Reason:    fmt.Sprintf("Permission check failed: %v", err),
		}, nil
	}
	if !allowed {
		return &SecurityCheck{
			Allowed:   false,
			RiskLevel: RiskHigh,
			Reason:    "User does not have permission to execute commands",
		}, nil
	}

	// 使用命令过滤器评估风险
	risk, err := sm.commandFilter.AssessRisk(ctx, cmdCtx)
	if err != nil {
		return &SecurityCheck{
			Allowed:   false,
			RiskLevel: RiskHigh,
			Reason:    fmt.Sprintf("Risk assessment failed: %v", err),
		}, nil
	}

	// 检查是否需要确认
	requireConfirm := sm.RequireConfirmation(cmdCtx.Command) || risk >= RiskHigh

	return &SecurityCheck{
		Allowed:        risk < RiskCritical,
		RiskLevel:      risk,
		RequireConfirm: requireConfirm,
		Reason:         "Command passed basic security checks",
	}, nil
}

// updateUserSession 更新用户会话
func (sm *securityManager) updateUserSession(cmdCtx *CommandContext) {
	session, exists := sm.userSessions[cmdCtx.User]
	if !exists {
		session = &UserSession{
			UserID:        cmdCtx.User,
			SessionID:     cmdCtx.SessionID,
			StartTime:     time.Now(),
			Authenticated: true,
		}
		sm.userSessions[cmdCtx.User] = session
	}

	session.LastActivity = time.Now()
	session.CommandCount++
}

// updateSecurityMetrics 更新安全指标
func (sm *securityManager) updateSecurityMetrics(cmdCtx *CommandContext, check *SecurityCheck) {
	sm.metrics.TotalCommands++

	if !check.Allowed {
		sm.metrics.BlockedCommands++
	}

	if check.RiskLevel >= RiskHigh {
		sm.metrics.HighRiskCommands++
	}

	// 更新用户活动统计
	if sm.metrics.UserActivity == nil {
		sm.metrics.UserActivity = make(map[string]int64)
	}
	sm.metrics.UserActivity[cmdCtx.User]++

	// 计算平均风险级别
	totalRisk := sm.metrics.AverageRiskLevel * float64(sm.metrics.TotalCommands-1)
	totalRisk += float64(check.RiskLevel)
	sm.metrics.AverageRiskLevel = totalRisk / float64(sm.metrics.TotalCommands)
}

// logSecurityViolation 记录安全违规
func (sm *securityManager) logSecurityViolation(cmdCtx *CommandContext, reason string) {
	event := &AuditEvent{
		EventType:     EventTypeSecurityViolation,
		SecurityLevel: SecurityLevelHigh,
		User:          cmdCtx.User,
		Command:       cmdCtx.Command,
		Metadata: map[string]interface{}{
			"violation_reason": reason,
			"source":          cmdCtx.Source,
		},
	}

	sm.LogAuditEvent(event)
	sm.metrics.SecurityViolations++
}

// logThreatDetection 记录威胁检测
func (sm *securityManager) logThreatDetection(cmdCtx *CommandContext, threats []ThreatInfo) {
	for _, threat := range threats {
		event := &AuditEvent{
			EventType:     EventTypeSecurityViolation,
			SecurityLevel: SecurityLevelCritical,
			User:          cmdCtx.User,
			Command:       cmdCtx.Command,
			Metadata: map[string]interface{}{
				"threat_type":    threat.ThreatType,
				"threat_severity": threat.Severity,
				"description":    threat.Description,
				"indicators":     threat.Indicators,
			},
		}

		sm.LogAuditEvent(event)

		// 更新威胁统计
		if sm.metrics.ThreatsByType == nil {
			sm.metrics.ThreatsByType = make(map[ThreatType]int64)
		}
		sm.metrics.ThreatsByType[threat.ThreatType]++
	}
}

// logCommandCheck 记录命令检查
func (sm *securityManager) logCommandCheck(cmdCtx *CommandContext, check *SecurityCheck) {
	if sm.policy.AuditLevel >= AuditLevelBasic {
		event := &AuditEvent{
			EventType:     EventTypeCommandExecution,
			SecurityLevel: SecurityLevelLow,
			User:          cmdCtx.User,
			Command:       cmdCtx.Command,
			RiskLevel:     check.RiskLevel,
			Metadata: map[string]interface{}{
				"allowed":        check.Allowed,
				"require_confirm": check.RequireConfirm,
				"source":         cmdCtx.Source,
			},
		}

		sm.LogAuditEvent(event)
	}
}

// 辅助方法
func (sm *securityManager) threatsToWarnings(threats []ThreatInfo) []string {
	warnings := make([]string, len(threats))
	for i, threat := range threats {
		warnings[i] = fmt.Sprintf("%s: %s", threat.ThreatType, threat.Description)
	}
	return warnings
}

func (sm *securityManager) patternsToWarnings(patterns []DangerousPattern) []string {
	warnings := make([]string, len(patterns))
	for i, pattern := range patterns {
		warnings[i] = pattern.Description
	}
	return warnings
}

func (sm *securityManager) generateSecuritySuggestions(analysis *CommandAnalysis, patterns []DangerousPattern) []string {
	var suggestions []string

	if analysis.PrivEscalation {
		suggestions = append(suggestions, "Avoid privilege escalation if not necessary")
	}

	if analysis.NetworkAccess {
		suggestions = append(suggestions, "Network access detected - ensure trusted sources only")
	}

	if analysis.SystemModify {
		suggestions = append(suggestions, "System modification detected - backup important data first")
	}

	for _, pattern := range patterns {
		if pattern.Suggestion != "" {
			suggestions = append(suggestions, pattern.Suggestion)
		}
	}

	return suggestions
}

func (sm *securityManager) hasSystemModifyPermission(user string) bool {
	// 简化实现：只有root用户可以修改系统
	return user == "root"
}

func (sm *securityManager) hasNetworkPermission(user string) bool {
	// 简化实现：所有用户都可以访问网络
	return true
}

func (sm *securityManager) updateComponentPolicies() error {
	// 更新各个组件的策略配置
	// 这里可以添加具体的组件策略更新逻辑
	return nil
}

// DefaultSecurityPolicy 返回默认安全策略
func DefaultSecurityPolicy() *SecurityPolicy {
	return &SecurityPolicy{
		AllowedCommands:     []string{},
		BlockedCommands:     []string{"rm -rf /", "dd if=/dev/zero", "mkfs", ":(){ :|:& };:"},
		DangerousPatterns:   []string{},
		RequireConfirmation: []string{"rm -rf", "sudo", "su -", "passwd", "userdel"},
		MaxConcurrentTasks:  10,
		MaxExecutionTime:    300 * time.Second,
		MaxOutputSize:       10 * 1024 * 1024, // 10MB
		MaxMemoryUsage:      1024,              // 1GB
		MaxCPUPercent:       80.0,
		RequireRootConfirm:  true,
		AllowedUsers:        []string{},
		BlockedUsers:        []string{},
		AllowedProxies:      []string{},
		TLSRequired:         false,
		CertificateVerify:   true,
		AuditLevel:          AuditLevelBasic,
		LogRetentionDays:    30,
		AlertThreshold:      5,
		EnableSandbox:       false,
		SandboxPath:         "/tmp/nb-agent-sandbox",
		RestrictNetworkAccess: false,
	}
}