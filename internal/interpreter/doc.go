/*
Package interpreter provides an intelligent shell interpreter management system
for the NewBee Agent. This package addresses the need for flexible command execution
across different Linux environments with varying shell interpreters.

# Core Features

1. **Intelligent Interpreter Detection**: Automatically detects available shell
   interpreters on the system including bash, zsh, fish, sh, dash, ash, csh, tcsh, and ksh.

2. **User Preference Management**: Allows users to specify their preferred interpreters
   with intelligent fallback to compatible alternatives when the preferred interpreter
   is not available.

3. **Command Compatibility Analysis**: Validates commands against interpreter capabilities
   and provides warnings for potential compatibility issues.

4. **Security Integration**: Works with the security framework to ensure safe command
   execution with proper validation and threat detection.

5. **Performance Monitoring**: Tracks interpreter usage statistics and execution
   performance for optimization.

# Architecture

The package is organized into several key components:

	┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
	│    Service      │───▶│    Manager       │───▶│    Detector     │
	│   (Main API)    │    │  (Preferences)   │    │   (Discovery)   │
	└─────────────────┘    └──────────────────┘    └─────────────────┘
	         │                       │                        │
	         ▼                       ▼                        ▼
	┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
	│    Executor     │    │    Validator     │    │  Statistics     │
	│  (Command Run)  │    │ (Compatibility)  │    │   (Metrics)     │
	└─────────────────┘    └──────────────────┘    └─────────────────┘

# Quick Start

	// Initialize the interpreter service
	config := interpreter.DefaultInterpreterConfig()
	service := interpreter.NewService(config)
	
	if err := service.Initialize(); err != nil {
		log.Fatal(err)
	}

	// Set user preference
	service.SetUserPreference("admin", interpreter.InterpreterBash)

	// Execute a command
	request := &interpreter.ExecutionRequest{
		Command:       "echo 'Hello World'",
		UserID:        "admin",
		SessionID:     "session-123",
		CaptureOutput: true,
	}

	result, err := service.ExecuteCommand(context.Background(), request)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Output: %s\n", result.Stdout)

# Supported Interpreters

The system supports the following shell interpreters with intelligent fallback:

- **bash**: Bourne Again Shell (default, highest priority)
- **zsh**: Z Shell (advanced features, good bash compatibility)
- **fish**: Friendly Interactive Shell (different syntax)
- **sh**: POSIX Shell (basic, highest compatibility)
- **dash**: Debian Almquist Shell (lightweight, fast)
- **ash**: Almquist Shell (embedded systems)
- **csh**: C Shell (C-like syntax)
- **tcsh**: Enhanced C Shell
- **ksh**: Korn Shell (POSIX with extensions)

# Interpreter Selection Logic

The system uses the following priority order for interpreter selection:

1. **User-specified interpreter** (if provided in request)
2. **User preference** (set via SetUserPreference)
3. **Command analysis** (automatic detection based on command syntax)
4. **System fallback order** (configured fallback sequence)
5. **Available interpreters** (by priority: bash > zsh > sh > dash > others)

# Command Validation

The validator component performs several checks:

- **Syntax compatibility**: Detects interpreter-specific syntax that may not work
- **Feature requirements**: Ensures the interpreter supports required features
- **Security patterns**: Identifies potentially dangerous command patterns
- **Environment validation**: Checks execution environment safety

# Configuration

The system can be configured via YAML:

	default_interpreter: bash
	enable_auto_detection: true
	refresh_interval: 5m
	fallback_order:
	  - bash
	  - sh
	  - dash
	  - zsh

# Integration with Security Framework

This package integrates seamlessly with the security framework:

	// The service automatically integrates with security checks
	result, err := service.ExecuteCommand(ctx, request)
	// Security validation, threat detection, and audit logging
	// are handled transparently

# Performance Considerations

- Interpreter detection is cached and refreshed periodically
- User preferences are persisted to disk
- Execution statistics are tracked for performance optimization
- Concurrent execution limits prevent resource exhaustion

# Error Handling

The package provides comprehensive error handling:

- Interpreter not found: Falls back to alternatives
- Command validation failures: Returns detailed error messages
- Execution timeouts: Gracefully handled with cleanup
- Resource limits: Prevents system overload

# Thread Safety

All components are designed to be thread-safe and can handle concurrent requests
safely with proper synchronization.

# Extensibility

The package is designed for extensibility:

- New interpreters can be added by extending the InterpreterType enum
- Custom validators can be plugged in
- Additional execution engines can be implemented
- Statistics collectors can be customized

# Examples

See integration.go for comprehensive examples including:

- Basic command execution
- User preference management
- Performance benchmarking
- Error handling scenarios

# Dependencies

This package integrates with:

- internal/security: For command validation and threat detection
- internal/platform: For system detection and OS-specific behavior
- Standard library: os, exec, context, sync for core functionality

For more detailed examples and usage patterns, see the integration.go file
and the test files in this package.
*/
package interpreter