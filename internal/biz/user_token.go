package biz

import (
	"net/http"
	"time"

	"github.com/libtnb/utils/crypt"
	"gorm.io/gorm"

	"github.com/coder-lulu/nb-agent/internal/app"
)

type UserToken struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index" json:"user_id"`
	Token     string    `gorm:"not null;default:'';unique" json:"-"`
	IPs       []string  `gorm:"not null;default:'[]';serializer:json" json:"ips"`
	ExpiredAt time.Time `json:"expired_at"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (r *UserToken) BeforeSave(tx *gorm.DB) error {
	crypter, err := crypt.NewXChacha20Poly1305([]byte(app.Key))
	if err != nil {
		return err
	}

	r.Token, err = crypter.Encrypt([]byte(r.Token))
	if err != nil {
		return err
	}

	return nil
}

func (r *UserToken) AfterFind(tx *gorm.DB) error {
	crypter, err := crypt.NewXChacha20Poly1305([]byte(app.Key))
	if err != nil {
		return err
	}

	token, err := crypter.Decrypt(r.Token)
	if err == nil {
		r.Token = string(token)
	}

	return nil
}

type UserTokenRepo interface {
	List(userID, page, limit uint) ([]*UserToken, int64, error)
	Create(userID uint, ips []string, expired time.Time) (*UserToken, error)
	Get(id uint) (*UserToken, error)
	Delete(id uint) error
	Update(id uint, ips []string, expired time.Time) (*UserToken, error)
	ValidateReq(req *http.Request) (uint, error)
}
