package service

import (
	"net/http"

	"github.com/libtnb/chix"

	"github.com/coder-lulu/nb-agent/internal/biz"
	"github.com/coder-lulu/nb-agent/internal/http/request"
)

type ContainerComposeService struct {
	containerComposeRepo biz.ContainerComposeRepo
}

func NewContainerComposeService(containerCompose biz.ContainerComposeRepo) *ContainerComposeService {
	return &ContainerComposeService{
		containerComposeRepo: containerCompose,
	}
}

func (s *ContainerComposeService) List(w http.ResponseWriter, r *http.Request) {
	composes, err := s.containerComposeRepo.List()
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	paged, total := Paginate(r, composes)

	Success(w, chix.M{
		"total": total,
		"items": paged,
	})
}

func (s *ContainerComposeService) Get(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ContainerComposeGet](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	compose, envs, err := s.containerComposeRepo.Get(req.Name)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, chix.M{
		"compose": compose,
		"envs":    envs,
	})
}

func (s *ContainerComposeService) Create(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ContainerComposeCreate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.containerComposeRepo.Create(req.Name, req.Compose, req.Envs); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *ContainerComposeService) Update(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ContainerComposeUpdate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.containerComposeRepo.Update(req.Name, req.Compose, req.Envs); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *ContainerComposeService) Up(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ContainerComposeUp](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.containerComposeRepo.Up(req.Name, req.Force); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *ContainerComposeService) Down(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ContainerComposeDown](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.containerComposeRepo.Down(req.Name); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *ContainerComposeService) Remove(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ContainerComposeRemove](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.containerComposeRepo.Remove(req.Name); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}
