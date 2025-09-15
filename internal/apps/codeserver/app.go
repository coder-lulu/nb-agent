package codeserver

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/coder-lulu/nb-agent/internal/service"
	"github.com/coder-lulu/nb-agent/pkg/io"
	"github.com/coder-lulu/nb-agent/pkg/systemctl"
)

type App struct{}

func NewApp() *App {
	return &App{}
}

func (s *App) Route(r chi.Router) {
	r.Get("/config", s.GetConfig)
	r.Post("/config", s.UpdateConfig)
}

func (s *App) GetConfig(w http.ResponseWriter, r *http.Request) {
	config, _ := io.Read("/root/.config/code-server/config.yaml")
	service.Success(w, config)
}

func (s *App) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[UpdateConfig](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = io.Write("/root/.config/code-server/config.yaml", req.Config, 0600); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.Restart("code-server"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
}
