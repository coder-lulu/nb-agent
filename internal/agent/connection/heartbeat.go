package connection

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// NewHeartbeatManager 创建心跳管理器
func NewHeartbeatManager(conn Connection, interval, timeout time.Duration) *HeartbeatManager {
	return &HeartbeatManager{
		conn:     conn,
		interval: interval,
		timeout:  timeout,
		running:  false,
	}
}

// Start 启动心跳
func (hm *HeartbeatManager) Start() error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	if hm.running {
		return nil
	}

	hm.ctx, hm.cancel = context.WithCancel(context.Background())
	hm.ticker = time.NewTicker(hm.interval)
	hm.running = true

	go hm.heartbeatLoop()
	return nil
}

// Stop 停止心跳
func (hm *HeartbeatManager) Stop() {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	if !hm.running {
		return
	}

	hm.running = false
	
	if hm.cancel != nil {
		hm.cancel()
	}
	
	if hm.ticker != nil {
		hm.ticker.Stop()
	}
}

// heartbeatLoop 心跳循环
func (hm *HeartbeatManager) heartbeatLoop() {
	for {
		select {
		case <-hm.ticker.C:
			if err := hm.sendHeartbeat(); err != nil {
				slog.Error("failed to send heartbeat", "error", err)
			}
		case <-hm.ctx.Done():
			return
		}
	}
}

// sendHeartbeat 发送心跳消息
func (hm *HeartbeatManager) sendHeartbeat() error {
	if !hm.conn.IsConnected() {
		return nil
	}

	heartbeatMsg := &Message{
		ID:        uuid.New().String(),
		Type:      MsgTypeHeartbeat,
		Timestamp: time.Now(),
		Payload:   map[string]interface{}{"ping": time.Now().Unix()},
	}

	return hm.conn.SendMessage(heartbeatMsg)
}