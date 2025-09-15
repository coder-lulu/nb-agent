package biz

import (
	"time"

	"github.com/coder-lulu/nb-agent/internal/http/request"
	"github.com/coder-lulu/nb-agent/pkg/acme"
)

type CertDNS struct {
	ID        uint          `gorm:"primaryKey" json:"id"`
	Name      string        `gorm:"not null;default:''" json:"name"`       // 备注名称
	Type      acme.DnsType  `gorm:"not null;default:'aliyun'" json:"type"` // DNS 提供商
	Data      acme.DNSParam `gorm:"not null;serializer:json" json:"dns_param"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`

	Certs []*Cert `gorm:"foreignKey:DNSID" json:"-"`
}

type CertDNSRepo interface {
	List(page, limit uint) ([]*CertDNS, int64, error)
	Get(id uint) (*CertDNS, error)
	Create(req *request.CertDNSCreate) (*CertDNS, error)
	Update(req *request.CertDNSUpdate) error
	Delete(id uint) error
}
