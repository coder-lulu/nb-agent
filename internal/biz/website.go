package biz

import (
	"context"
	"time"

	"github.com/coder-lulu/nb-agent/internal/http/request"
	"github.com/coder-lulu/nb-agent/pkg/types"
)

type Website struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"not null;default:'';unique" json:"name"`
	Type      string    `gorm:"not null;default:'php'" json:"type"`
	Status    bool      `gorm:"not null;default:true" json:"status"`
	Path      string    `gorm:"not null;default:''" json:"path"`
	Https     bool      `gorm:"not null;default:false" json:"https"`
	Remark    string    `gorm:"not null;default:''" json:"remark"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	CertExpire string `gorm:"-:all" json:"cert_expire"` // 仅显示

	Cert *Cert `gorm:"foreignKey:WebsiteID" json:"cert"`
}

type WebsiteRepo interface {
	GetRewrites() (map[string]string, error)
	UpdateDefaultConfig(req *request.WebsiteDefaultConfig) error
	Count() (int64, error)
	Get(id uint) (*types.WebsiteSetting, error)
	GetByName(name string) (*types.WebsiteSetting, error)
	List(page, limit uint) ([]*Website, int64, error)
	Create(req *request.WebsiteCreate) (*Website, error)
	Update(req *request.WebsiteUpdate) error
	Delete(req *request.WebsiteDelete) error
	ClearLog(id uint) error
	UpdateRemark(id uint, remark string) error
	ResetConfig(id uint) error
	UpdateStatus(id uint, status bool) error
	UpdateCert(req *request.WebsiteUpdateCert) error
	ObtainCert(ctx context.Context, id uint) error
}
