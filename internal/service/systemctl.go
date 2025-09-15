package service

import (
	"net/http"

	"github.com/leonelquinteros/gotext"

	"github.com/coder-lulu/nb-agent/internal/http/request"
	"github.com/coder-lulu/nb-agent/pkg/systemctl"
)

type SystemctlService struct {
	t *gotext.Locale
}

func NewSystemctlService(t *gotext.Locale) *SystemctlService {
	return &SystemctlService{
		t: t,
	}
}

func (s *SystemctlService) Status(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.SystemctlService](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	status, err := systemctl.Status(req.Service)
	if err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to get %s service running status: %v", req.Service, err))
		return
	}

	Success(w, status)
}

func (s *SystemctlService) IsEnabled(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.SystemctlService](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	enabled, err := systemctl.IsEnabled(req.Service)
	if err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to get %s service enable status: %v", req.Service, err))
		return
	}

	Success(w, enabled)
}

func (s *SystemctlService) Enable(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.SystemctlService](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = systemctl.Enable(req.Service); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to enable %s service: %v", req.Service, err))
		return
	}

	Success(w, nil)
}

func (s *SystemctlService) Disable(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.SystemctlService](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = systemctl.Disable(req.Service); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to disable %s service: %v", req.Service, err))
		return
	}

	Success(w, nil)
}

func (s *SystemctlService) Restart(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.SystemctlService](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = systemctl.Restart(req.Service); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to restart %s service: %v", req.Service, err))
		return
	}

	Success(w, nil)
}

func (s *SystemctlService) Reload(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.SystemctlService](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = systemctl.Reload(req.Service); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to reload %s service: %v", req.Service, err))
		return
	}

	Success(w, nil)
}

func (s *SystemctlService) Start(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.SystemctlService](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = systemctl.Start(req.Service); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to start %s service: %v", req.Service, err))
		return
	}

	Success(w, nil)
}

func (s *SystemctlService) Stop(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.SystemctlService](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = systemctl.Stop(req.Service); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to stop %s service: %v", req.Service, err))
		return
	}

	Success(w, nil)
}
