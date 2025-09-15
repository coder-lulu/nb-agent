package minio

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
	r.Get("/env", s.GetEnv)
	r.Post("/env", s.UpdateEnv)
}

func (s *App) GetEnv(w http.ResponseWriter, r *http.Request) {
	env, _ := io.Read("/etc/default/minio")
	service.Success(w, env)
}

func (s *App) UpdateEnv(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[UpdateEnv](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = io.Write("/etc/default/minio", req.Env, 0600); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.Restart("minio"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
}
