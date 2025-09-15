package data

import (
	"errors"
	"fmt"

	"github.com/leonelquinteros/gotext"
	"gorm.io/gorm"

	"github.com/coder-lulu/nb-agent/internal/biz"
	"github.com/coder-lulu/nb-agent/internal/http/request"
	pkgssh "github.com/coder-lulu/nb-agent/pkg/ssh"
)

type sshRepo struct {
	t  *gotext.Locale
	db *gorm.DB
}

func NewSSHRepo(t *gotext.Locale, db *gorm.DB) biz.SSHRepo {
	return &sshRepo{
		t:  t,
		db: db,
	}
}

func (r *sshRepo) List(page, limit uint) ([]*biz.SSH, int64, error) {
	ssh := make([]*biz.SSH, 0)
	var total int64
	err := r.db.Model(&biz.SSH{}).Omit("Hosts").Order("id desc").Count(&total).Offset(int((page - 1) * limit)).Limit(int(limit)).Find(&ssh).Error
	return ssh, total, err
}

func (r *sshRepo) Get(id uint) (*biz.SSH, error) {
	ssh := new(biz.SSH)
	if err := r.db.Where("id = ?", id).First(ssh).Error; err != nil {
		return nil, err
	}

	return ssh, nil
}

func (r *sshRepo) Create(req *request.SSHCreate) error {
	conf := pkgssh.ClientConfig{
		AuthMethod: pkgssh.AuthMethod(req.AuthMethod),
		Host:       fmt.Sprintf("%s:%d", req.Host, req.Port),
		User:       req.User,
		Password:   req.Password,
		Key:        req.Key,
	}
	_, err := pkgssh.NewSSHClient(conf)
	if err != nil {
		return errors.New(r.t.Get("failed to check ssh connection: %v", err))
	}

	ssh := &biz.SSH{
		Name:   req.Name,
		Host:   req.Host,
		Port:   req.Port,
		Config: conf,
		Remark: req.Remark,
	}

	return r.db.Create(ssh).Error
}

func (r *sshRepo) Update(req *request.SSHUpdate) error {
	conf := pkgssh.ClientConfig{
		AuthMethod: pkgssh.AuthMethod(req.AuthMethod),
		Host:       fmt.Sprintf("%s:%d", req.Host, req.Port),
		User:       req.User,
		Password:   req.Password,
		Key:        req.Key,
	}
	_, err := pkgssh.NewSSHClient(conf)
	if err != nil {
		return errors.New(r.t.Get("failed to check ssh connection: %v", err))
	}

	ssh := &biz.SSH{
		ID:     req.ID,
		Name:   req.Name,
		Host:   req.Host,
		Port:   req.Port,
		Config: conf,
		Remark: req.Remark,
	}

	return r.db.Model(ssh).Where("id = ?", req.ID).Select("*").Updates(ssh).Error
}

func (r *sshRepo) Delete(id uint) error {
	return r.db.Delete(&biz.SSH{}, id).Error
}
