package biz

import (
	"time"

	"github.com/coder-lulu/nb-agent/internal/http/request"
	"github.com/coder-lulu/nb-agent/pkg/types"
)

type Monitor struct {
	ID        uint              `gorm:"primaryKey" json:"id"`
	Info      types.CurrentInfo `gorm:"not null;default:'{}';serializer:json" json:"info"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

type MonitorRepo interface {
	GetSetting() (*request.MonitorSetting, error)
	UpdateSetting(setting *request.MonitorSetting) error
	Clear() error
	List(start, end time.Time) ([]*Monitor, error)
}
