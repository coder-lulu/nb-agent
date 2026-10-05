package interpreter

import (
	"context"
	"fmt"
	"time"
)

// IntegrateWithAgent 演示如何在现有agent中集成智能解释器系统
func IntegrateWithAgent() error {
	// 1. 创建解释器服务
	config := DefaultInterpreterConfig()
	service := NewService(config)

	// 2. 初始化服务
	if err := service.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize interpreter service: %w", err)
	}

	// 3. 获取系统解释器信息
	systemInfo, err := service.GetSystemInfo()
	if err != nil {
		return fmt.Errorf("failed to get system info: %w", err)
	}

	fmt.Printf("Found %d interpreters on the system:\n", systemInfo.TotalCount)
	for _, interpreter := range systemInfo.AvailableInterpreters {
		fmt.Printf("- %s: %s (version: %s)\n", 
			interpreter.Type, interpreter.Path, interpreter.Version)
	}

	// 4. 设置用户首选项示例
	userID := "admin"
	if err := service.SetUserPreference(userID, InterpreterBash); err != nil {
		return fmt.Errorf("failed to set user preference: %w", err)
	}

	// 5. 执行命令示例
	examples := []struct {
		name     string
		command  string
		preferred InterpreterType
	}{
		{
			name:     "Basic echo command",
			command:  "echo 'Hello, World!'",
			preferred: "",
		},
		{
			name:     "Bash-specific command",
			command:  "if [[ $USER == 'root' ]]; then echo 'Running as root'; fi",
			preferred: InterpreterBash,
		},
		{
			name:     "POSIX compatible command",
			command:  "ls -la | grep '^d'",
			preferred: InterpreterSh,
		},
		{
			name:     "Script with shebang",
			command:  "#!/bin/bash\necho \"Script execution\"",
			preferred: "",
		},
	}

	for _, example := range examples {
		fmt.Printf("\n--- %s ---\n", example.name)
		
		request := &ExecutionRequest{
			Command:              example.command,
			UserID:               userID,
			SessionID:           fmt.Sprintf("session-%d", time.Now().Unix()),
			PreferredInterpreter: example.preferred,
			CaptureOutput:       true,
			Timeout:             10 * time.Second,
		}

		result, err := service.ExecuteCommand(context.Background(), request)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		fmt.Printf("Interpreter used: %s\n", result.InterpreterUsed.Type)
		fmt.Printf("Exit code: %d\n", result.ExitCode)
		fmt.Printf("Success: %t\n", result.Success)
		if result.Stdout != "" {
			fmt.Printf("Output: %s\n", result.Stdout)
		}
		if result.Stderr != "" {
			fmt.Printf("Error output: %s\n", result.Stderr)
		}
		fmt.Printf("Duration: %v\n", result.Duration)
	}

	// 6. 获取使用统计
	stats, err := service.GetUsageStatistics()
	if err != nil {
		return fmt.Errorf("failed to get usage statistics: %w", err)
	}

	fmt.Printf("\n--- Usage Statistics ---\n")
	fmt.Printf("Total executions: %d\n", stats.TotalExecutions)
	fmt.Printf("Success rate: %.2f%%\n", stats.SuccessRate)
	fmt.Printf("Average execution time: %v\n", stats.AverageExecutionTime)
	
	fmt.Printf("Usage by interpreter:\n")
	for interpreterType, count := range stats.UsageByInterpreter {
		if count > 0 {
			fmt.Printf("  %s: %d executions\n", interpreterType, count)
		}
	}

	return nil
}

// ExampleCommandExecution 演示不同类型命令的执行
func ExampleCommandExecution(service *Service, userID string) error {
	ctx := context.Background()

	// 测试用例：不同复杂度的命令
	testCases := []struct {
		description string
		command     string
		expectError bool
	}{
		{
			description: "Simple echo command",
			command:     "echo 'Hello World'",
			expectError: false,
		},
		{
			description: "Command with pipes",
			command:     "ps aux | grep bash | head -5",
			expectError: false,
		},
		{
			description: "Bash array usage",
			command:     "arr=(a b c); echo ${arr[1]}",
			expectError: false,
		},
		{
			description: "File operations",
			command:     "ls -la /tmp | wc -l",
			expectError: false,
		},
		{
			description: "Conditional execution",
			command:     "if [ -d /tmp ]; then echo 'Directory exists'; fi",
			expectError: false,
		},
	}

	for i, testCase := range testCases {
		fmt.Printf("\n=== Test Case %d: %s ===\n", i+1, testCase.description)
		
		// 1. 首先检测最佳解释器
		bestInterpreter, err := service.DetectBestInterpreter(testCase.command, userID)
		if err != nil {
			fmt.Printf("Failed to detect best interpreter: %v\n", err)
			continue
		}
		fmt.Printf("Best interpreter detected: %s\n", bestInterpreter.Type)

		// 2. 验证命令
		validation, err := service.validator.ValidateCommand(testCase.command, bestInterpreter)
		if err != nil {
			fmt.Printf("Validation error: %v\n", err)
			continue
		}

		if !validation.Valid {
			fmt.Printf("Command validation failed: %v\n", validation.Errors)
			if !testCase.expectError {
				continue
			}
		}

		if len(validation.Warnings) > 0 {
			fmt.Printf("Warnings: %v\n", validation.Warnings)
		}

		if len(validation.Suggestions) > 0 {
			fmt.Printf("Suggestions: %v\n", validation.Suggestions)
		}

		// 3. 执行命令
		request := &ExecutionRequest{
			Command:       testCase.command,
			UserID:        userID,
			SessionID:     fmt.Sprintf("test-session-%d", i),
			CaptureOutput: true,
			Timeout:       5 * time.Second,
		}

		result, err := service.ExecuteCommand(ctx, request)
		if err != nil {
			fmt.Printf("Execution error: %v\n", err)
			if !testCase.expectError {
				continue
			}
		}

		fmt.Printf("Result: Success=%t, ExitCode=%d, Duration=%v\n", 
			result.Success, result.ExitCode, result.Duration)
		
		if result.Stdout != "" {
			fmt.Printf("Stdout: %s\n", result.Stdout)
		}
		
		if result.Stderr != "" {
			fmt.Printf("Stderr: %s\n", result.Stderr)
		}
	}

	return nil
}

// ExampleUserPreferenceManagement 演示用户首选项管理
func ExampleUserPreferenceManagement(service *Service) error {
	users := []struct {
		userID     string
		preference InterpreterType
	}{
		{"admin", InterpreterBash},
		{"developer", InterpreterZsh},
		{"operator", InterpreterSh},
	}

	fmt.Printf("\n=== User Preference Management ===\n")

	for _, user := range users {
		// 设置用户首选项
		if err := service.SetUserPreference(user.userID, user.preference); err != nil {
			fmt.Printf("Failed to set preference for %s: %v\n", user.userID, err)
			continue
		}

		// 获取用户首选项
		preference, err := service.GetUserPreference(user.userID)
		if err != nil {
			fmt.Printf("Failed to get preference for %s: %v\n", user.userID, err)
			continue
		}

		fmt.Printf("User %s prefers %s interpreter\n", user.userID, preference)

		// 测试首选项是否生效
		request := &ExecutionRequest{
			Command:       "echo 'Testing user preference'",
			UserID:        user.userID,
			SessionID:     fmt.Sprintf("pref-test-%s", user.userID),
			CaptureOutput: true,
		}

		result, err := service.ExecuteCommand(context.Background(), request)
		if err != nil {
			fmt.Printf("Failed to execute test command for %s: %v\n", user.userID, err)
			continue
		}

		fmt.Printf("User %s command executed with %s interpreter\n", 
			user.userID, result.InterpreterUsed.Type)
	}

	return nil
}

// BenchmarkInterpreterPerformance 性能基准测试
func BenchmarkInterpreterPerformance(service *Service, userID string) error {
	fmt.Printf("\n=== Interpreter Performance Benchmark ===\n")

	interpreters, err := service.GetAvailableInterpreters()
	if err != nil {
		return fmt.Errorf("failed to get available interpreters: %w", err)
	}

	testCommand := "echo 'performance test'"
	iterations := 10

	for _, interpreter := range interpreters {
		if !interpreter.Available {
			continue
		}

		fmt.Printf("\nTesting %s interpreter:\n", interpreter.Type)
		
		var totalDuration time.Duration
		successCount := 0

		for i := 0; i < iterations; i++ {
			request := &ExecutionRequest{
				Command:              testCommand,
				UserID:               userID,
				SessionID:           fmt.Sprintf("bench-%s-%d", interpreter.Type, i),
				PreferredInterpreter: interpreter.Type,
				CaptureOutput:       true,
				Timeout:             1 * time.Second,
			}

			result, err := service.ExecuteCommand(context.Background(), request)
			if err != nil {
				fmt.Printf("  Iteration %d failed: %v\n", i+1, err)
				continue
			}

			if result.Success {
				successCount++
				totalDuration += result.Duration
			}
		}

		if successCount > 0 {
			avgDuration := totalDuration / time.Duration(successCount)
			successRate := float64(successCount) / float64(iterations) * 100
			
			fmt.Printf("  Success rate: %.1f%% (%d/%d)\n", successRate, successCount, iterations)
			fmt.Printf("  Average execution time: %v\n", avgDuration)
		} else {
			fmt.Printf("  All executions failed\n")
		}
	}

	return nil
}