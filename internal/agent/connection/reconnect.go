package connection

import (
	"context"
	"log/slog"
	"math"
	"time"
)

// NewReconnectManager 创建重连管理器
func NewReconnectManager(conn Connection, config *ConnectionConfig) *ReconnectManager {
	return &ReconnectManager{
		conn:          conn,
		config:        config,
		maxRetries:    config.RetryCount,
		retryInterval: config.RetryInterval,
		maxInterval:   config.MaxRetryInterval,
		reconnecting:  false,
	}
}

// Start 启动重连管理器
func (rm *ReconnectManager) Start() error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rm.reconnecting {
		return nil
	}

	rm.ctx, rm.cancel = context.WithCancel(context.Background())
	rm.retryCount = 0
	rm.reconnecting = true

	go rm.reconnectLoop()
	return nil
}

// Stop 停止重连管理器
func (rm *ReconnectManager) Stop() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if !rm.reconnecting {
		return
	}

	rm.reconnecting = false
	
	if rm.cancel != nil {
		rm.cancel()
	}
}

// TriggerReconnect 触发重连
func (rm *ReconnectManager) TriggerReconnect() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if !rm.reconnecting {
		return
	}

	// 重置重试计数
	rm.retryCount = 0
	
	select {
	case rm.reconnectChan <- struct{}{}:
	default:
		// 通道已满，忽略
	}
}

// reconnectLoop 重连循环
func (rm *ReconnectManager) reconnectLoop() {
	rm.reconnectChan = make(chan struct{}, 1)
	
	// 定期检查连接状态
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-rm.reconnectChan:
			rm.doReconnect()
			
		case <-ticker.C:
			// 定期检查连接状态
			if !rm.conn.IsConnected() && rm.retryCount < rm.maxRetries {
				rm.doReconnect()
			}
			
		case <-rm.ctx.Done():
			return
		}
	}
}

// doReconnect 执行重连
func (rm *ReconnectManager) doReconnect() {
	if rm.retryCount >= rm.maxRetries {
		slog.Error("max reconnect attempts reached", 
			"retry_count", rm.retryCount, 
			"max_retries", rm.maxRetries)
		return
	}

	// 计算退避延迟
	delay := rm.calculateBackoffDelay()
	slog.Info("attempting to reconnect", 
		"retry_count", rm.retryCount + 1, 
		"delay", delay)

	// 等待退避延迟
	select {
	case <-time.After(delay):
	case <-rm.ctx.Done():
		return
	}

	// 尝试重连
	if err := rm.conn.Connect(rm.ctx); err != nil {
		rm.retryCount++
		slog.Warn("reconnect failed", 
			"retry_count", rm.retryCount, 
			"error", err)
		
		// 如果未达到最大重试次数，继续重连
		if rm.retryCount < rm.maxRetries {
			go func() {
				select {
				case rm.reconnectChan <- struct{}{}:
				case <-rm.ctx.Done():
				}
			}()
		}
		return
	}

	// 重连成功，重置计数
	rm.retryCount = 0
	slog.Info("reconnected successfully")
}

// calculateBackoffDelay 计算指数退避延迟
func (rm *ReconnectManager) calculateBackoffDelay() time.Duration {
	// 指数退避: interval * 2^retryCount
	delay := time.Duration(float64(rm.retryInterval) * math.Pow(2, float64(rm.retryCount)))
	
	// 限制最大延迟
	if delay > rm.maxInterval {
		delay = rm.maxInterval
	}
	
	return delay
}