package service

import (
	"net/http"

	"github.com/coder-lulu/nb-agent/internal/biz"
	"github.com/coder-lulu/nb-agent/internal/http/request"
	"github.com/coder-lulu/nb-agent/pkg/tools"
)

type SettingService struct {
	settingRepo biz.SettingRepo
}

func NewSettingService(setting biz.SettingRepo) *SettingService {
	return &SettingService{
		settingRepo: setting,
	}
}

func (s *SettingService) Get(w http.ResponseWriter, r *http.Request) {
	setting, err := s.settingRepo.GetPanel()
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, setting)
}

func (s *SettingService) Update(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.SettingPanel](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	restart := false
	if restart, err = s.settingRepo.UpdatePanel(req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if restart {
		tools.RestartPanel()
	}

	Success(w, nil)
}

// UpdateCert 用于自动化工具更新证书
func (s *SettingService) UpdateCert(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.SettingCert](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.settingRepo.UpdateCert(req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	tools.RestartPanel()

	Success(w, nil)
}
